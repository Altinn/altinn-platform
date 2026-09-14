package controller

import (
	"context"
	"crypto/rand"

	policyv1alpha1 "github.com/linkerd/linkerd2/controller/gen/apis/policy/v1alpha1"
	serverv1beta3 "github.com/linkerd/linkerd2/controller/gen/apis/server/v1beta3"
	valkeyv1alpha1 "github.com/valkey-io/valkey-operator/api/v1alpha1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
	cachepkg "github.com/Altinn/altinn-platform/services/dis-cache-operator/internal/cache"
)

// fieldOwner is the server-side apply field manager of this operator.
const fieldOwner = "dis-cache-operator"

// CacheReconciler reconciles a Cache object.
type CacheReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// Images are the container images set on every ValkeyCluster.
	Images cachepkg.Images
}

// The operator only creates Secrets. It never reads one back, so it has no
// get, list, or watch on Secrets, and the manager cache excludes them.
// The other owned objects are watched (list, watch) and written with
// server-side apply (create, patch). Owner references delete all of them.
// The finalizers permission is for clusters that run the
// OwnerReferencesPermissionEnforcement admission plugin: it checks that
// permission when an owner reference sets blockOwnerDeletion.
// +kubebuilder:rbac:groups="",resources=secrets,verbs=create
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups=policy.linkerd.io,resources=servers;meshtlsauthentications;authorizationpolicies,verbs=list;watch;create;patch
// +kubebuilder:rbac:groups=cache.dis.altinn.cloud,resources=caches,verbs=get;list;watch
// +kubebuilder:rbac:groups=cache.dis.altinn.cloud,resources=caches/status,verbs=patch
// +kubebuilder:rbac:groups=cache.dis.altinn.cloud,resources=caches/finalizers,verbs=update
// +kubebuilder:rbac:groups=valkey.io,resources=valkeyclusters,verbs=list;watch;create;patch

// Reconcile creates the objects for a Cache and mirrors the ValkeyCluster
// readiness into the Cache status. The Secret and the access policies come
// first, so the Valkey pods find the password and the network rules on their
// first start. An error in any step is recorded in the Ready condition.
func (r *CacheReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var cache cachev1alpha1.Cache
	if err := r.Get(ctx, req.NamespacedName, &cache); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// Owner references delete everything the operator created.
	if !cache.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	if err := r.ensureAuthSecret(ctx, &cache); err != nil {
		return ctrl.Result{}, r.failed(ctx, &cache, ReasonSecretCreateFailed, err)
	}
	if _, err := r.apply(ctx, &cache, cachepkg.BuildNetworkPolicy(&cache)); err != nil {
		return ctrl.Result{}, r.failed(ctx, &cache, ReasonApplyFailed, err)
	}
	if err := r.applyMeshPolicies(ctx, &cache); err != nil {
		return ctrl.Result{}, r.failed(ctx, &cache, ReasonApplyFailed, err)
	}

	desired := cachepkg.BuildValkeyCluster(&cache, r.Images)
	applied, err := r.apply(ctx, &cache, desired)
	if err != nil {
		return ctrl.Result{}, r.failed(ctx, &cache, ReasonApplyFailed, err)
	}
	// The apply response is the stored object, with defaults, generation, and
	// status. The informer cache can still hold the version before the apply.
	current := &valkeyv1alpha1.ValkeyCluster{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(applied.Object, current); err != nil {
		return ctrl.Result{}, r.failed(ctx, &cache, ReasonApplyFailed, err)
	}
	specMismatch := !usersMatch(desired.Spec.Users, current.Spec.Users)

	if err := r.writeStatus(ctx, &cache, readyCondition(cache.Generation, current, specMismatch)); err != nil {
		return ctrl.Result{}, err
	}
	logger.V(1).Info("reconciled", "valkeyCluster", desired.Name, "specMismatch", specMismatch)

	return ctrl.Result{}, nil
}

// ensureAuthSecret creates the credentials Secret with a new random password.
// When the Secret exists, the create fails with AlreadyExists and the stored
// password stays. The operator does not read the Secret back, so a new
// password is generated on every reconcile and dropped when it is not needed.
//
// A Secret with the same name that someone else created is kept as it is: it
// becomes the password source, it gets no owner reference, and the operator
// cannot check its content. A missing password key shows up as a failed
// ValkeyCluster.
func (r *CacheReconciler) ensureAuthSecret(ctx context.Context, owner *cachev1alpha1.Cache) error {
	secret := cachepkg.BuildAuthSecret(owner, rand.Text())
	if err := controllerutil.SetControllerReference(owner, secret, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	// This is the only signal that a password was generated; apps must read
	// the Secret again after it.
	logf.FromContext(ctx).Info("created the auth Secret", "secret", secret.Name)

	return nil
}

// applyMeshPolicies writes the linkerd objects that let clients reach the
// Valkey pods under the clusters' default inbound policy (deny). The
// authorizations go before the Servers, so a Server never exists without
// its authorization.
func (r *CacheReconciler) applyMeshPolicies(ctx context.Context, owner *cachev1alpha1.Cache) error {
	policies := cachepkg.BuildMeshPolicies(owner)
	for _, obj := range []client.Object{
		policies.Authentication,
		policies.ClientPolicy,
		policies.BusPolicy,
		policies.ClientServer,
		policies.BusServer,
	} {
		if _, err := r.apply(ctx, owner, obj); err != nil {
			return err
		}
	}

	return nil
}

// apply writes obj with server-side apply and the Cache as its controller,
// and returns the stored object from the API server response. The API server
// and the CRDs default many fields; a read-modify-write would fight those
// defaults on every reconcile, and a merge patch would not remove fields that
// another writer added.
func (r *CacheReconciler) apply(ctx context.Context, owner *cachev1alpha1.Cache, obj client.Object) (*unstructured.Unstructured, error) {
	gvk, err := apiutil.GVKForObject(obj, r.Scheme)
	if err != nil {
		return nil, err
	}
	obj.GetObjectKind().SetGroupVersionKind(gvk)
	if err := controllerutil.SetControllerReference(owner, obj, r.Scheme); err != nil {
		return nil, err
	}

	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	applied := &unstructured.Unstructured{Object: content}
	// An apply configuration must not carry a status or a creation timestamp.
	// The converter omits both today; this keeps it so when a type changes.
	unstructured.RemoveNestedField(applied.Object, "status")
	unstructured.RemoveNestedField(applied.Object, "metadata", "creationTimestamp")

	if err := r.Apply(ctx, client.ApplyConfigurationFromUnstructured(applied), client.FieldOwner(fieldOwner), client.ForceOwnership); err != nil {
		return nil, err
	}

	return applied, nil
}

// SetupWithManager sets up the controller with the Manager. A Cache is
// reconciled again when its spec changes, not when the operator writes its
// status. Updates of the owned objects only matter when their spec generation
// or their status changes; metadata-only writes (for example managed fields
// after an apply) must not trigger another reconcile. Secrets are not watched.
func (r *CacheReconciler) SetupWithManager(mgr ctrl.Manager) error {
	generationChanged := builder.WithPredicates(predicate.GenerationChangedPredicate{})

	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.Cache{}, generationChanged).
		Owns(&netv1.NetworkPolicy{}, generationChanged).
		Owns(&serverv1beta3.Server{}, generationChanged).
		Owns(&policyv1alpha1.MeshTLSAuthentication{}, generationChanged).
		Owns(&policyv1alpha1.AuthorizationPolicy{}, generationChanged).
		Owns(&valkeyv1alpha1.ValkeyCluster{}, builder.WithPredicates(valkeyClusterChanged())).
		Named("cache").
		Complete(r)
}

func valkeyClusterChanged() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCluster, ok := e.ObjectOld.(*valkeyv1alpha1.ValkeyCluster)
			if !ok {
				return true
			}
			newCluster, ok := e.ObjectNew.(*valkeyv1alpha1.ValkeyCluster)
			if !ok {
				return true
			}

			return oldCluster.Generation != newCluster.Generation ||
				!equality.Semantic.DeepEqual(oldCluster.Status, newCluster.Status)
		},
	}
}
