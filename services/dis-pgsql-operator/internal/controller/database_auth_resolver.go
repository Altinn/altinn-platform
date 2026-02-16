package controller

import (
	"context"
	"fmt"
	"strings"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
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

		var appIdentity identityv1alpha1.ApplicationIdentity
		if err := r.Get(ctx, types.NamespacedName{Name: refName, Namespace: db.Namespace}, &appIdentity); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("ApplicationIdentity not found yet", "role", role, "name", refName)
				return resolvedIdentity{}, true, nil
			}
			return resolvedIdentity{}, false, fmt.Errorf("get ApplicationIdentity %s/%s: %w", db.Namespace, refName, err)
		}

		ready, readyFound := applicationIdentityReady(&appIdentity)
		if readyFound && !ready {
			logger.Info("ApplicationIdentity not ready yet", "role", role, "name", refName)
			return resolvedIdentity{}, true, nil
		}

		var managedIdentityName string
		if appIdentity.Status.ManagedIdentityName != nil {
			managedIdentityName = strings.TrimSpace(*appIdentity.Status.ManagedIdentityName)
		}
		var principalID string
		if appIdentity.Status.PrincipalID != nil {
			principalID = strings.TrimSpace(*appIdentity.Status.PrincipalID)
		}
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

func applicationIdentityReady(identity *identityv1alpha1.ApplicationIdentity) (bool, bool) {
	for _, cond := range identity.Status.Conditions {
		if cond.Type != string(identityv1alpha1.ConditionReady) {
			continue
		}
		return cond.Status == metav1.ConditionTrue, true
	}
	return false, false
}
