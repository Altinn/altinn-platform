package controller

import (
	"context"
	"fmt"
	"maps"
	"strings"

	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *DatabaseReconciler) ensureUserProvisionJobForTarget(
	ctx context.Context,
	logger logr.Logger,
	spec userProvisionJobSpec,
) error {
	return ensureUserProvisionJobForReconciler(ctx, logger, r, spec)
}

type userProvisionJobSpec struct {
	Owner client.Object

	JobName string
	Labels  map[string]string

	ServiceAccountName string
	AdminIdentityName  string

	ServerName   string
	DatabaseHost string
	DatabaseName string
	SchemaName   string

	AccessPrincipals []dbUtil.AccessPrincipal

	RevokePublicConnect bool
	SearchPathScope     string
}

type userProvisionJobReconciler interface {
	List(context.Context, client.ObjectList, ...client.ListOption) error
	Delete(context.Context, client.Object, ...client.DeleteOption) error
	Create(context.Context, client.Object, ...client.CreateOption) error

	userProvisionJobScheme() *runtime.Scheme
	userProvisionJobImage() string
	userProvisionJobUseAzFakes() bool
}

func (r *DatabaseReconciler) userProvisionJobScheme() *runtime.Scheme {
	return r.Scheme
}

func (r *DatabaseReconciler) userProvisionJobImage() string {
	return r.Config.UserProvisionImage
}

func (r *DatabaseReconciler) userProvisionJobUseAzFakes() bool {
	return r.Config.UseAzFakes
}

func ensureUserProvisionJobForReconciler(
	ctx context.Context,
	logger logr.Logger,
	r userProvisionJobReconciler,
	spec userProvisionJobSpec,
) error {
	ns := spec.Owner.GetNamespace()
	jobName := spec.JobName
	useAzFakes := r.userProvisionJobUseAzFakes()

	if err := validateUserProvisionJobSpec(spec, useAzFakes); err != nil {
		return err
	}

	ttlSeconds := int32(300)
	// Run pod at a time
	parallelism := int32(1)
	completions := int32(1)
	image := strings.TrimSpace(r.userProvisionJobImage())
	if image == "" {
		return fmt.Errorf("user provision image is not configured")
	}
	labels := userProvisionJobLabels(spec.Labels)

	var jobs batchv1.JobList
	if err := r.List(ctx, &jobs, client.InNamespace(ns), client.MatchingLabels(labels)); err != nil {
		return fmt.Errorf("list user provisioning jobs for %s/%s: %w", ns, spec.Owner.GetName(), err)
	}

	hasCurrent := false
	deletedCurrent := false
	for i := range jobs.Items {
		job := jobs.Items[i]
		if job.Name == jobName {
			if jobConditionTrue(&job, batchv1.JobFailed) {
				policy := metav1.DeletePropagationBackground
				if err := r.Delete(ctx, &job, &client.DeleteOptions{
					PropagationPolicy: &policy,
				}); err != nil && !apierrors.IsNotFound(err) {
					return fmt.Errorf("delete failed user provisioning Job %s/%s: %w", job.Namespace, job.Name, err)
				}
				logger.Info("deleting failed user provisioning Job for database access",
					"jobName", job.Name,
					"namespace", job.Namespace,
				)
				deletedCurrent = true
				continue
			}
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

	if hasCurrent || deletedCurrent {
		return nil
	}

	job := buildUserProvisionJob(ns, jobName, image, labels, spec, parallelism, completions, ttlSeconds)

	// If we're using AzFakes, we need to disable AAD authentication in the provisioner
	// since we're running on Kind
	if useAzFakes {
		job.Spec.Template.Spec.Containers[0].Env = append(
			job.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  dbUtil.DisableAADEnv,
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

	if err := controllerutil.SetControllerReference(spec.Owner, job, r.userProvisionJobScheme()); err != nil {
		return fmt.Errorf("set controller reference on user provisioning Job: %w", err)
	}

	logger.Info("creating user provisioning Job for database access",
		"jobName", jobName,
		"namespace", ns,
		"serviceAccount", spec.ServiceAccountName,
		"accessPrincipalCount", len(spec.AccessPrincipals),
	)

	if err := r.Create(ctx, job); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create user provisioning Job %s/%s: %w", ns, jobName, err)
	}

	return nil
}

func validateUserProvisionJobSpec(spec userProvisionJobSpec, useAzFakes bool) error {
	if spec.ServiceAccountName == "" {
		return fmt.Errorf("serviceAccountName must be set for user provisioning")
	}
	if spec.AdminIdentityName == "" && !useAzFakes {
		return fmt.Errorf("admin identity name must be set for user provisioning")
	}
	if spec.ServerName == "" {
		return fmt.Errorf("server name must be set for user provisioning")
	}
	if spec.SchemaName == "" {
		return fmt.Errorf("schema name must be set for user provisioning")
	}
	if len(spec.AccessPrincipals) == 0 {
		return fmt.Errorf("at least one access principal must be set for user provisioning")
	}
	for i, principal := range spec.AccessPrincipals {
		if principal.Name == "" {
			return fmt.Errorf("access principal %d name must be set for user provisioning", i)
		}
		if principal.PrincipalID == "" && !useAzFakes {
			return fmt.Errorf("access principal %d principal ID must be set for user provisioning", i)
		}
		switch principal.PrincipalType {
		case dbUtil.PrincipalTypeService, dbUtil.PrincipalTypeGroup:
		default:
			return fmt.Errorf("access principal %d principal type must be service or group", i)
		}
		switch principal.Role {
		case dbUtil.AccessRoleReader, dbUtil.AccessRoleWriter, dbUtil.AccessRoleOwner:
		default:
			return fmt.Errorf("access principal %d role must be Reader, Writer, or Owner", i)
		}
	}
	if _, err := dbUtil.MarshalAccessPrincipals(spec.AccessPrincipals); err != nil {
		return err
	}
	return nil
}

func userProvisionJobLabels(specLabels map[string]string) map[string]string {
	labels := maps.Clone(specLabels)
	if labels == nil {
		labels = map[string]string{}
	}
	labels[userProvisionLabelKey] = labelValueTrue
	return labels
}

func buildUserProvisionJob(
	namespace,
	jobName,
	image string,
	labels map[string]string,
	spec userProvisionJobSpec,
	parallelism,
	completions,
	ttlSeconds int32,
) *batchv1.Job {
	podLabels := map[string]string{
		"azure.workload.identity/use": labelValueTrue,
	}
	maps.Copy(podLabels, labels)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
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
					ServiceAccountName: spec.ServiceAccountName,
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  "provision-user",
							Image: image,
							Args:  []string{"--provision-user"},
							Env:   userProvisionJobEnv(spec),
						},
					},
				},
			},
		},
	}
}

func userProvisionJobEnv(spec userProvisionJobSpec) []corev1.EnvVar {
	accessPrincipals, err := dbUtil.MarshalAccessPrincipals(spec.AccessPrincipals)
	if err != nil {
		// Unreachable: validateUserProvisionJobSpec already runs MarshalAccessPrincipals
		// and rejects specs that fail to marshal before we reach this point.
		panic(fmt.Sprintf("userProvisionJobEnv: MarshalAccessPrincipals failed after validation: %v", err))
	}

	env := []corev1.EnvVar{
		{Name: dbUtil.AdminAppIdentityEnv, Value: spec.AdminIdentityName},
		{Name: dbUtil.DatabaseServerNameEnv, Value: spec.ServerName},
		{Name: dbUtil.DBSchemaEnv, Value: spec.SchemaName},
		{Name: dbUtil.AccessPrincipalsEnv, Value: accessPrincipals},
	}
	if spec.DatabaseHost != "" {
		env = append(env, corev1.EnvVar{Name: dbUtil.DBHostEnv, Value: spec.DatabaseHost})
	}
	if spec.DatabaseName != "" {
		env = append(env, corev1.EnvVar{Name: dbUtil.DBNameEnv, Value: spec.DatabaseName})
	}
	if spec.RevokePublicConnect {
		env = append(env, corev1.EnvVar{Name: dbUtil.RevokePublicConnectEnv, Value: "1"})
	}
	if spec.SearchPathScope != "" {
		env = append(env, corev1.EnvVar{Name: dbUtil.DBSearchPathScopeEnv, Value: spec.SearchPathScope})
	}
	return env
}

func jobConditionTrue(job *batchv1.Job, conditionType batchv1.JobConditionType) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == conditionType && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
