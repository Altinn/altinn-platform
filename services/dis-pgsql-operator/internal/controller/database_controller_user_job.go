package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"strings"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
)

func userProvisionJobName(dbName string, auth resolvedDatabaseAuth) string {
	hash := userProvisionSpecHash(dbName, auth)
	base := fmt.Sprintf("%s-user-provision", dbName)
	maxBaseLen := max(63-1-len(hash), 1)
	if len(base) > maxBaseLen {
		base = strings.TrimRight(base[:maxBaseLen], "-")
		if base == "" {
			base = "db"
		}
	}
	return fmt.Sprintf("%s-%s", base, hash)
}

func userProvisionSpecHash(dbName string, auth resolvedDatabaseAuth) string {
	payload := fmt.Sprintf("adminSA=%s;admin=%s;user=%s;userPID=%s;db=%s",
		auth.Admin.ServiceAccountName,
		auth.Admin.Name,
		auth.User.Name,
		auth.User.PrincipalID,
		dbName,
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])[:8]
}

func (r *DatabaseReconciler) ensureUserProvisionJob(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
	auth resolvedDatabaseAuth,
) error {
	ns := db.Namespace
	jobName := userProvisionJobName(db.Name, auth)

	if auth.Admin.ServiceAccountName == "" {
		return fmt.Errorf("admin serviceAccountName must be set for user provisioning")
	}
	if auth.User.Name == "" {
		return fmt.Errorf("user identity name must be set for user provisioning")
	}
	if auth.User.PrincipalID == "" {
		return fmt.Errorf("user principal ID must be set for user provisioning")
	}

	ttlSeconds := int32(300)
	// Run pod at a time
	parallelism := int32(1)
	completions := int32(1)
	image := strings.TrimSpace(r.Config.UserProvisionImage)
	if image == "" {
		return fmt.Errorf("user provision image is not configured")
	}
	labels := map[string]string{
		"dis.altinn.cloud/database-name":  db.Name,
		"dis.altinn.cloud/user-provision": "true",
	}

	var jobs batchv1.JobList
	if err := r.List(ctx, &jobs, client.InNamespace(ns), client.MatchingLabels(labels)); err != nil {
		return fmt.Errorf("list user provisioning jobs for %s/%s: %w", ns, db.Name, err)
	}

	hasCurrent := false
	for i := range jobs.Items {
		job := jobs.Items[i]
		if job.Name == jobName {
			hasCurrent = true
			continue
		}
		policy := metav1.DeletePropagationBackground
		if err := r.Delete(ctx, &job, &client.DeleteOptions{
			PropagationPolicy: &policy,
		}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete outdated user provisioning Job %s/%s: %w", job.Namespace, job.Name, err)
		}
	}

	if hasCurrent {
		return nil
	}

	podLabels := map[string]string{
		"azure.workload.identity/use": "true",
	}
	maps.Copy(podLabels, labels)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Parallelism:             &parallelism,
			Completions:             &completions,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: auth.Admin.ServiceAccountName,
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  "provision-user",
							Image: image,
							Args:  []string{"--provision-user"},
							Env: []corev1.EnvVar{
								{
									Name:  "DISPG_USER_APP_IDENTITY",
									Value: auth.User.Name,
								},
								{
									Name:  "DISPG_USER_APP_PRINCIPAL_ID",
									Value: auth.User.PrincipalID,
								},
								{
									Name:  "DISPG_ADMIN_APP_IDENTITY",
									Value: auth.Admin.Name,
								},
								{
									Name:  "DISPG_DATABASE_NAME",
									Value: db.Name,
								},
								{
									Name:  "DISPG_DB_SCHEMA",
									Value: db.Name,
								},
							},
						},
					},
				},
			},
		},
	}

	// If we're using AzFakes, we need to disable AAD authentication in the provisioner
	// since we're running on Kind
	if r.Config.UseAzFakes {
		job.Spec.Template.Spec.Containers[0].Env = append(
			job.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  "DISPG_DISABLE_AAD",
				Value: "1",
			},
		)
		job.Spec.Template.Spec.InitContainers = append(
			job.Spec.Template.Spec.InitContainers,
			corev1.Container{
				Name:  "wait-for-postgres",
				Image: "postgres:16",
				Command: []string{
					"sh",
					"-c",
					"until pg_isready -h postgres.default.svc -p 5432; do sleep 2; done",
				},
			},
		)
	}

	if err := controllerutil.SetControllerReference(db, job, r.Scheme); err != nil {
		return fmt.Errorf("set controller reference on user provisioning Job: %w", err)
	}

	logger.Info("creating user provisioning Job for database",
		"jobName", jobName,
		"namespace", ns,
		"serviceAccount", auth.Admin.ServiceAccountName,
		"userIdentity", auth.User.Name,
	)

	if err := r.Create(ctx, job); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create user provisioning Job %s/%s: %w", ns, jobName, err)
	}

	return nil
}
