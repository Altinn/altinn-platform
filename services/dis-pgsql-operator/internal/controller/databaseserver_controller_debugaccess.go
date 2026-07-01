package controller

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	genruntime "github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	// debugAccessComponentLabelKey marks RoleAssignments created for debug access
	// so they can be listed and pruned independently of any other RoleAssignments
	// the operator may own.
	debugAccessComponentLabelKey   = "dis.altinn.cloud/component"
	debugAccessComponentLabelValue = "debug-access"

	// debugAccessReaderRole is the built-in Azure role granted to debug principals
	// for read-only portal/control-plane visibility of the Flexible Server.
	debugAccessReaderRole = "Reader"

	// roleAssignmentMaxNameLen bounds the generated Kubernetes object name for a
	// RoleAssignment (DNS-1123 subdomain).
	roleAssignmentMaxNameLen = 253
)

// resolvedDebugAccessPrincipal is a debug principal resolved to the values ASO
// needs to create the Azure Reader role assignment.
type resolvedDebugAccessPrincipal struct {
	PrincipalID   string
	PrincipalType authorizationv1.RoleAssignmentProperties_PrincipalType
}

// ensureDebugAccessRoleAssignments reconciles the Azure Reader role assignments
// that grant read-only debug access to this server's Flexible Server. Shared
// servers are a no-op. When debugAccess is unset or has no principals, any
// previously-created debug-access role assignments are pruned.
func (r *DatabaseServerReconciler) ensureDebugAccessRoleAssignments(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
) error {
	// Debug-access role assignments are scoped to a dedicated server's Flexible
	// Server. Shared servers must not create them.
	if databaseServerMode(db) == storagev1alpha1.DatabaseServerModeShared {
		return nil
	}

	desired := map[string]*authorizationv1.RoleAssignment{}
	if db.Spec.DebugAccess != nil {
		for _, principal := range db.Spec.DebugAccess.Principals {
			resolved, ok, err := r.resolveDebugAccessPrincipal(ctx, logger, db, principal)
			if err != nil {
				return err
			}
			if !ok {
				// Not-ready identityRef: skip this principal without failing the whole
				// reconcile; a later reconcile picks it up once the identity is ready.
				continue
			}

			roleAssignment, err := buildDebugAccessRoleAssignment(db, resolved.PrincipalID, resolved.PrincipalType)
			if err != nil {
				return err
			}
			desired[roleAssignment.Name] = roleAssignment
		}
	}

	for name := range desired {
		if err := r.upsertDebugAccessRoleAssignment(ctx, logger, db, desired[name]); err != nil {
			return err
		}
	}

	return r.pruneDebugAccessRoleAssignments(ctx, logger, db, desired)
}

// resolveDebugAccessPrincipal resolves one debug principal to its Entra object id
// and principal type. It returns ok=false (without error) when an identityRef is
// not yet ready, so the caller can skip it for now.
func (r *DatabaseServerReconciler) resolveDebugAccessPrincipal(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
	principal storagev1alpha1.DebugAccessPrincipalSpec,
) (resolvedDebugAccessPrincipal, bool, error) {
	switch {
	case principal.Group != nil:
		principalID := strings.TrimSpace(principal.Group.PrincipalId)
		if principalID == "" {
			return resolvedDebugAccessPrincipal{}, false, fmt.Errorf("debug access group principalId must not be empty")
		}
		return resolvedDebugAccessPrincipal{
			PrincipalID:   principalID,
			PrincipalType: authorizationv1.RoleAssignmentProperties_PrincipalType_Group,
		}, true, nil

	case principal.ServicePrincipal != nil:
		principalID := strings.TrimSpace(principal.ServicePrincipal.PrincipalId)
		if principalID == "" {
			return resolvedDebugAccessPrincipal{}, false, fmt.Errorf("debug access servicePrincipal principalId must not be empty")
		}
		return resolvedDebugAccessPrincipal{
			PrincipalID:   principalID,
			PrincipalType: authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal,
		}, true, nil

	case principal.IdentityRef != nil:
		refName := strings.TrimSpace(principal.IdentityRef.Name)
		if refName == "" {
			return resolvedDebugAccessPrincipal{}, false, fmt.Errorf("debug access identityRef.name must not be empty")
		}

		var appIdentity identityv1alpha1.ApplicationIdentity
		if err := r.Get(ctx, client.ObjectKey{Name: refName, Namespace: db.Namespace}, &appIdentity); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("debug access ApplicationIdentity not found yet; skipping principal", "name", refName)
				return resolvedDebugAccessPrincipal{}, false, nil
			}
			return resolvedDebugAccessPrincipal{}, false, fmt.Errorf("get ApplicationIdentity %s/%s: %w", db.Namespace, refName, err)
		}

		if ready, readyFound := applicationIdentityReady(&appIdentity); readyFound && !ready {
			logger.Info("debug access ApplicationIdentity not ready yet; skipping principal", "name", refName)
			return resolvedDebugAccessPrincipal{}, false, nil
		}

		var principalID string
		if appIdentity.Status.PrincipalID != nil {
			principalID = strings.TrimSpace(*appIdentity.Status.PrincipalID)
		}
		if principalID == "" {
			logger.Info("debug access ApplicationIdentity status not populated yet; skipping principal", "name", refName)
			return resolvedDebugAccessPrincipal{}, false, nil
		}

		return resolvedDebugAccessPrincipal{
			PrincipalID:   principalID,
			PrincipalType: authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal,
		}, true, nil

	default:
		return resolvedDebugAccessPrincipal{}, false, fmt.Errorf("debug access principal must set exactly one of identityRef, group, or servicePrincipal")
	}
}

// upsertDebugAccessRoleAssignment creates or updates the desired RoleAssignment,
// setting the DatabaseServer as the Kubernetes controller owner so it is garbage
// collected with the server.
func (r *DatabaseServerReconciler) upsertDebugAccessRoleAssignment(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
	desired *authorizationv1.RoleAssignment,
) error {
	current := &authorizationv1.RoleAssignment{}
	current.SetName(desired.GetName())
	current.SetNamespace(desired.GetNamespace())

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, current, func() error {
		current.Labels = mergeDebugAccessLabels(current.Labels, desired.Labels)
		current.Spec = desired.Spec
		return controllerutil.SetControllerReference(db, current, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile debug access RoleAssignment %s/%s: %w", desired.Namespace, desired.Name, err)
	}
	if op != controllerutil.OperationResultNone {
		logger.Info("reconciled debug access RoleAssignment",
			"name", desired.Name,
			"namespace", desired.Namespace,
			"operation", op,
		)
	}
	return nil
}

// pruneDebugAccessRoleAssignments deletes debug-access RoleAssignments owned by
// this DatabaseServer whose principal is no longer desired.
func (r *DatabaseServerReconciler) pruneDebugAccessRoleAssignments(
	ctx context.Context,
	logger logr.Logger,
	db *storagev1alpha1.DatabaseServer,
	desired map[string]*authorizationv1.RoleAssignment,
) error {
	var list authorizationv1.RoleAssignmentList
	if err := r.List(
		ctx,
		&list,
		client.InNamespace(db.Namespace),
		client.MatchingLabels{
			databaseServerNameLabelKey:   db.Name,
			debugAccessComponentLabelKey: debugAccessComponentLabelValue,
		},
	); err != nil {
		return fmt.Errorf("list debug access RoleAssignments: %w", err)
	}

	for i := range list.Items {
		item := list.Items[i]
		if !metav1.IsControlledBy(&item, db) {
			continue
		}
		if _, keep := desired[item.Name]; keep {
			continue
		}
		logger.Info("pruning debug access RoleAssignment", "name", item.Name, "namespace", item.Namespace)
		if err := client.IgnoreNotFound(r.Delete(ctx, &item)); err != nil {
			return fmt.Errorf("delete debug access RoleAssignment %s/%s: %w", item.Namespace, item.Name, err)
		}
	}

	return nil
}

// buildDebugAccessRoleAssignment builds the desired Azure Reader RoleAssignment
// for one debug principal, scoped to this server's Flexible Server. It mirrors
// dis-vault's role assignment builder: an ArbitraryOwnerReference pins the Azure
// scope to the Flexible Server, and the Azure name is a deterministic UUID so the
// assignment is idempotent across reconciles.
func buildDebugAccessRoleAssignment(
	db *storagev1alpha1.DatabaseServer,
	principalID string,
	principalType authorizationv1.RoleAssignmentProperties_PrincipalType,
) (*authorizationv1.RoleAssignment, error) {
	if db == nil {
		return nil, fmt.Errorf("databaseServer must not be nil")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return nil, fmt.Errorf("principalID must not be empty")
	}

	// The FlexibleServer Kubernetes object name is the DatabaseServer name; see
	// ensurePostgresServer.
	flexibleServerName := db.Name

	owner := genruntime.ArbitraryOwnerReference{
		Group: dbforpostgresqlv1.GroupVersion.Group,
		Kind:  "FlexibleServer",
		Name:  flexibleServerName,
	}

	pt := principalType

	return &authorizationv1.RoleAssignment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      debugAccessRoleAssignmentName(db.Name, principalID),
			Namespace: db.Namespace,
			Labels: map[string]string{
				databaseServerNameLabelKey:   db.Name,
				debugAccessComponentLabelKey: debugAccessComponentLabelValue,
			},
		},
		Spec: authorizationv1.RoleAssignment_Spec{
			AzureName:     debugAccessRoleAssignmentAzureName(db.Namespace, db.Name, principalID),
			Owner:         &owner,
			PrincipalId:   &principalID,
			PrincipalType: &pt,
			RoleDefinitionReference: &genruntime.WellKnownResourceReference{
				WellKnownName: debugAccessReaderRole,
			},
		},
	}, nil
}

// debugAccessRoleAssignmentName returns a deterministic, DNS-1123 Kubernetes
// object name that is unique per (server, principal) so multiple debug principals
// do not collide.
func debugAccessRoleAssignmentName(serverName, principalID string) string {
	suffix := fmt.Sprintf("-dbg-ra-%s", naming.StableSHA1Hex(principalID)[:10])
	base := naming.EnsureLowerAlphaPrefix(naming.SanitizeLowerHyphen(serverName), "db")
	return naming.WithRequiredSuffix(base, suffix, roleAssignmentMaxNameLen, "db")
}

// debugAccessRoleAssignmentAzureName returns the deterministic UUID Azure name
// for the Reader role assignment, seeded so it is stable across reconciles.
func debugAccessRoleAssignmentAzureName(namespace, serverName, principalID string) string {
	seed := strings.Join([]string{namespace, serverName, principalID, debugAccessReaderRole}, "/")
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(seed)).String()
}

func mergeDebugAccessLabels(existing, desired map[string]string) map[string]string {
	if existing == nil {
		existing = map[string]string{}
	}
	maps.Copy(existing, desired)
	return existing
}
