package controller

import (
	"context"
	"maps"
	"time"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	redisv1alpha1 "github.com/Altinn/altinn-platform/services/dis-redis-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-redis-operator/internal/config"
	redispkg "github.com/Altinn/altinn-platform/services/dis-redis-operator/internal/redis"
	cachev1 "github.com/Azure/azure-service-operator/v2/api/cache/v1api20250401"
	pev1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	networkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20240601"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	identityRequeueDelay     = 5 * time.Second
	provisioningRequeueDelay = 30 * time.Second
)

// RedisReconciler reconciles a Redis object.
type RedisReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config config.OperatorConfig
}

// +kubebuilder:rbac:groups=redis.dis.altinn.cloud,resources=redises,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redis.dis.altinn.cloud,resources=redises/status,verbs=get;update;patch

// ASO: Cache (Redis Enterprise)
// +kubebuilder:rbac:groups=cache.azure.com,resources=redisenterprises,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.azure.com,resources=redisenterprises/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.azure.com,resources=redisenterprisesdatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.azure.com,resources=redisenterprisesdatabases/status,verbs=get;update;patch

// ASO: Network
// +kubebuilder:rbac:groups=network.azure.com,resources=privateendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.azure.com,resources=privateendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszones,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszones/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszonesvirtualnetworklinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.azure.com,resources=privatednszonesvirtualnetworklinks/status,verbs=get;update;patch

// ApplicationIdentity
// +kubebuilder:rbac:groups=application.dis.altinn.cloud,resources=applicationidentities,verbs=get;list;watch

// Core
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch

// Reconcile drives a Redis CR towards the desired Azure state.
func (r *RedisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("redis", req.NamespacedName)

	var redisObj redisv1alpha1.Redis
	if err := r.Get(ctx, req.NamespacedName, &redisObj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !redisObj.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	azureName := redispkg.DeterministicAzureRedisName(redisObj.Namespace, redisObj.Name, r.Config.Environment)

	identity, identityPending, err := redispkg.ResolveOwnerIdentity(ctx, r.Client, &redisObj)
	if err != nil {
		return ctrl.Result{}, err
	}

	var (
		clusterReady  redispkg.ASOReadyCondition
		databaseReady redispkg.ASOReadyCondition
		peReady       redispkg.ASOReadyCondition
		dnsReady      redispkg.ASOReadyCondition
		cluster       *cachev1.RedisEnterprise
		database      *cachev1.RedisEnterpriseDatabase
	)

	if !identityPending {
		desiredCluster, err := redispkg.BuildASORedisEnterprise(&redisObj, r.Config, azureName)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := r.upsertCluster(ctx, &redisObj, desiredCluster); err != nil {
			return ctrl.Result{}, err
		}

		desiredDB, err := redispkg.BuildASODatabase(&redisObj, desiredCluster.Name)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := r.upsertDatabase(ctx, &redisObj, desiredDB); err != nil {
			return ctrl.Result{}, err
		}

		desiredPE, err := redispkg.BuildPrivateEndpoint(&redisObj, r.Config, desiredCluster.Name)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := r.upsertPrivateEndpoint(ctx, &redisObj, desiredPE); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.ensureSharedPrivateDNS(ctx, &redisObj); err != nil {
			return ctrl.Result{}, err
		}
	}

	cluster, clusterReady, err = r.getCurrentCluster(ctx, &redisObj)
	if err != nil {
		return ctrl.Result{}, err
	}
	database, databaseReady, err = r.getCurrentDatabase(ctx, &redisObj)
	if err != nil {
		return ctrl.Result{}, err
	}
	_, peReady, err = r.getCurrentPrivateEndpoint(ctx, &redisObj)
	if err != nil {
		return ctrl.Result{}, err
	}
	dnsReady, err = r.getSharedDNSReady(ctx, &redisObj)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updateStatus(
		ctx,
		&redisObj,
		azureName,
		identity,
		identityPending,
		cluster,
		clusterReady,
		database,
		databaseReady,
		peReady,
		dnsReady,
	); err != nil {
		return ctrl.Result{}, err
	}

	if identityPending {
		return ctrl.Result{RequeueAfter: identityRequeueDelay}, nil
	}

	logger.Info("reconciled Redis dependencies", "azureName", azureName, "principalId", identity.PrincipalID)

	if !clusterReady.Found || clusterReady.Status != metav1.ConditionTrue ||
		!databaseReady.Found || databaseReady.Status != metav1.ConditionTrue {
		return ctrl.Result{RequeueAfter: provisioningRequeueDelay}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager wires the reconciler with the controller manager.
func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1alpha1.Redis{}).
		Owns(&cachev1.RedisEnterprise{}).
		Owns(&cachev1.RedisEnterpriseDatabase{}).
		Owns(&pev1.PrivateEndpoint{}).
		Watches(&identityv1alpha1.ApplicationIdentity{}, handler.EnqueueRequestsFromMapFunc(r.mapApplicationIdentityToRedises)).
		Watches(&corev1.ServiceAccount{}, handler.EnqueueRequestsFromMapFunc(r.mapServiceAccountToRedises)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

func (r *RedisReconciler) upsertCluster(ctx context.Context, owner *redisv1alpha1.Redis, desired *cachev1.RedisEnterprise) error {
	current := &cachev1.RedisEnterprise{}
	current.SetName(desired.GetName())
	current.SetNamespace(desired.GetNamespace())

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, current, func() error {
		current.Labels = mergeStringMaps(current.Labels, desired.Labels)
		current.Spec = desired.Spec
		return ctrl.SetControllerReference(owner, current, r.Scheme)
	})
	return err
}

func (r *RedisReconciler) upsertDatabase(ctx context.Context, owner *redisv1alpha1.Redis, desired *cachev1.RedisEnterpriseDatabase) error {
	current := &cachev1.RedisEnterpriseDatabase{}
	current.SetName(desired.GetName())
	current.SetNamespace(desired.GetNamespace())

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, current, func() error {
		current.Labels = mergeStringMaps(current.Labels, desired.Labels)
		current.Spec = desired.Spec
		return ctrl.SetControllerReference(owner, current, r.Scheme)
	})
	return err
}

func (r *RedisReconciler) upsertPrivateEndpoint(ctx context.Context, owner *redisv1alpha1.Redis, desired *pev1.PrivateEndpoint) error {
	current := &pev1.PrivateEndpoint{}
	current.SetName(desired.GetName())
	current.SetNamespace(desired.GetNamespace())

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, current, func() error {
		current.Labels = mergeStringMaps(current.Labels, desired.Labels)
		current.Spec = desired.Spec
		return ctrl.SetControllerReference(owner, current, r.Scheme)
	})
	return err
}

func (r *RedisReconciler) getCurrentCluster(ctx context.Context, redisObj *redisv1alpha1.Redis) (*cachev1.RedisEnterprise, redispkg.ASOReadyCondition, error) {
	current := &cachev1.RedisEnterprise{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      redispkg.ClusterKubernetesName(redisObj.Name),
		Namespace: redisObj.Namespace,
	}, current); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, redispkg.ASOReadyCondition{}, nil
		}
		return nil, redispkg.ASOReadyCondition{}, err
	}
	return current, redispkg.FromASOConditions(current.Status.Conditions), nil
}

func (r *RedisReconciler) getCurrentDatabase(ctx context.Context, redisObj *redisv1alpha1.Redis) (*cachev1.RedisEnterpriseDatabase, redispkg.ASOReadyCondition, error) {
	current := &cachev1.RedisEnterpriseDatabase{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      redispkg.DatabaseKubernetesName(redisObj.Name),
		Namespace: redisObj.Namespace,
	}, current); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, redispkg.ASOReadyCondition{}, nil
		}
		return nil, redispkg.ASOReadyCondition{}, err
	}
	return current, redispkg.FromASOConditions(current.Status.Conditions), nil
}

func (r *RedisReconciler) getCurrentPrivateEndpoint(ctx context.Context, redisObj *redisv1alpha1.Redis) (*pev1.PrivateEndpoint, redispkg.ASOReadyCondition, error) {
	current := &pev1.PrivateEndpoint{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      redispkg.PrivateEndpointKubernetesName(redisObj.Name),
		Namespace: redisObj.Namespace,
	}, current); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, redispkg.ASOReadyCondition{}, nil
		}
		return nil, redispkg.ASOReadyCondition{}, err
	}
	return current, redispkg.FromASOConditions(current.Status.Conditions), nil
}

func (r *RedisReconciler) getSharedDNSReady(ctx context.Context, redisObj *redisv1alpha1.Redis) (redispkg.ASOReadyCondition, error) {
	zone := &networkv1.PrivateDnsZone{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      redispkg.RedisPrivateLinkZoneName,
		Namespace: redisObj.Namespace,
	}, zone); err != nil {
		if apierrors.IsNotFound(err) {
			return redispkg.ASOReadyCondition{}, nil
		}
		return redispkg.ASOReadyCondition{}, err
	}
	return redispkg.FromASOConditions(zone.Status.Conditions), nil
}

func setStatusCondition(redisObj *redisv1alpha1.Redis, condition metav1.Condition) bool {
	return apimeta.SetStatusCondition(&redisObj.Status.Conditions, condition)
}

func mergeStringMaps(existing, desired map[string]string) map[string]string {
	if len(existing) == 0 && len(desired) == 0 {
		return nil
	}
	merged := make(map[string]string, len(existing)+len(desired))
	maps.Copy(merged, existing)
	maps.Copy(merged, desired)
	return merged
}
