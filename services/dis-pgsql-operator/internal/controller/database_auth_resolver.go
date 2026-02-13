package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
)

const (
	applicationIdentityGroup   = "application.dis.altinn.cloud"
	applicationIdentityVersion = "v1alpha1"
	applicationIdentityKind    = "ApplicationIdentity"
)

type resolvedIdentity struct {
	Name        string
	PrincipalID string
}

type resolvedAdminIdentity struct {
	resolvedIdentity
	ServiceAccountName string
}

type resolvedDatabaseAuth struct {
	Admin resolvedAdminIdentity
	User  resolvedIdentity
}

func (r *DatabaseReconciler) resolveAdminIdentity(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
) (resolvedAdminIdentity, bool, error) {
	identity, requeue, err := r.resolveIdentitySource(ctx, logger, db, "admin", db.Spec.Auth.Admin.Identity)
	if err != nil || requeue {
		return resolvedAdminIdentity{}, requeue, err
	}

	serviceAccountName := strings.TrimSpace(db.Spec.Auth.Admin.ServiceAccountName)
	if serviceAccountName == "" && db.Spec.Auth.Admin.Identity.IdentityRef != nil {
		serviceAccountName = db.Spec.Auth.Admin.Identity.IdentityRef.Name
	}
	if serviceAccountName == "" {
		return resolvedAdminIdentity{}, false, fmt.Errorf("spec.auth.admin.serviceAccountName must be set when identityRef is not provided")
	}

	return resolvedAdminIdentity{
		resolvedIdentity:   identity,
		ServiceAccountName: serviceAccountName,
	}, false, nil
}

func (r *DatabaseReconciler) resolveUserIdentity(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
) (resolvedIdentity, bool, error) {
	return r.resolveIdentitySource(ctx, logger, db, "user", db.Spec.Auth.User.Identity)
}

func (r *DatabaseReconciler) resolveIdentitySource(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.Database,
	role string,
	source storagev1alpha1.IdentitySource,
) (resolvedIdentity, bool, error) {
	if source.IdentityRef != nil {
		if strings.TrimSpace(source.Name) != "" || strings.TrimSpace(source.PrincipalId) != "" {
			return resolvedIdentity{}, false, fmt.Errorf("spec.auth.%s.identity cannot set both identityRef and name/principalId", role)
		}

		refName := strings.TrimSpace(source.IdentityRef.Name)
		if refName == "" {
			return resolvedIdentity{}, false, fmt.Errorf("spec.auth.%s.identity.identityRef.name must be set", role)
		}

		appIdentity := &unstructured.Unstructured{}
		appIdentity.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   applicationIdentityGroup,
			Version: applicationIdentityVersion,
			Kind:    applicationIdentityKind,
		})

		if err := r.Get(ctx, types.NamespacedName{Name: refName, Namespace: db.Namespace}, appIdentity); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("ApplicationIdentity not found yet", "role", role, "name", refName)
				return resolvedIdentity{}, true, nil
			}
			return resolvedIdentity{}, false, fmt.Errorf("get ApplicationIdentity %s/%s: %w", db.Namespace, refName, err)
		}

		ready, readyFound, err := applicationIdentityReady(appIdentity)
		if err != nil {
			return resolvedIdentity{}, false, err
		}
		if readyFound && !ready {
			logger.Info("ApplicationIdentity not ready yet", "role", role, "name", refName)
			return resolvedIdentity{}, true, nil
		}

		managedIdentityName, _, err := unstructured.NestedString(appIdentity.Object, "status", "managedIdentityName")
		if err != nil {
			return resolvedIdentity{}, false, fmt.Errorf("read ApplicationIdentity status.managedIdentityName: %w", err)
		}
		principalID, _, err := unstructured.NestedString(appIdentity.Object, "status", "principalId")
		if err != nil {
			return resolvedIdentity{}, false, fmt.Errorf("read ApplicationIdentity status.principalId: %w", err)
		}

		managedIdentityName = strings.TrimSpace(managedIdentityName)
		principalID = strings.TrimSpace(principalID)
		if managedIdentityName == "" || principalID == "" {
			logger.Info("ApplicationIdentity status not populated yet", "role", role, "name", refName)
			return resolvedIdentity{}, true, nil
		}

		return resolvedIdentity{
			Name:        managedIdentityName,
			PrincipalID: principalID,
		}, false, nil
	}

	name := strings.TrimSpace(source.Name)
	principalID := strings.TrimSpace(source.PrincipalId)
	if name == "" || principalID == "" {
		return resolvedIdentity{}, false, fmt.Errorf("spec.auth.%s.identity must set both name and principalId when identityRef is not provided", role)
	}

	return resolvedIdentity{
		Name:        name,
		PrincipalID: principalID,
	}, false, nil
}

func applicationIdentityReady(obj *unstructured.Unstructured) (bool, bool, error) {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil {
		return false, false, fmt.Errorf("read ApplicationIdentity status.conditions: %w", err)
	}
	if !found {
		return false, false, nil
	}
	for _, cond := range conditions {
		condMap, ok := cond.(map[string]any)
		if !ok {
			continue
		}
		condType, _, _ := unstructured.NestedString(condMap, "type")
		if condType != "Ready" {
			continue
		}
		status, _, _ := unstructured.NestedString(condMap, "status")
		return status == "True", true, nil
	}
	return false, false, nil
}
