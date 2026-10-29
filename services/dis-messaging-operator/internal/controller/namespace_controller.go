package controller

import (
	"context"
	"errors"
	"maps"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/messaging"
)

// ReasonNotManaged is the reason of the Warning event for a dis-messaging
// object that the operator did not create.
const ReasonNotManaged = "NotManaged"

var errNotManaged = errors.New("object exists without the managed-by label")

// NamespaceReconciler reconciles the objects that the operator keeps once
// per product namespace: the ResourceQuota that limits the Queues, and the
// ConfigMap with the connection details of every Queue. The ConfigMap is
// built from all the Queues of the namespace, so the namespace is the
// reconcile key. The Queues with the same queue names in other namespaces
// decide which of them get a key.
type NamespaceReconciler struct {
	client.Client
	// APIReader reads from the API server, not from the cache.
	APIReader client.Reader
	Config    config.OperatorConfig
	Recorder  events.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=configmaps;resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile keeps the quota in every product namespace, so the API server
// limits the Queues from the first one. The ConfigMap exists while the
// namespace has a Queue, with a key for each Queue that has a valid name, is
// not being deleted and holds its queue name. A key does not wait for
// QueueReady: apps know the key when they write the Queue, and a short Azure
// failure must not remove a key that a restarting app needs. A namespace
// that is no longer a product namespace loses both objects.
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var namespace corev1.Namespace
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, &namespace); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// The deletion of the namespace deletes both objects.
	if !namespace.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	quota := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{
		Name: messaging.ResourceQuotaName, Namespace: namespace.Name,
	}}
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: messaging.ConnectionConfigMapName, Namespace: namespace.Name,
	}}
	if _, err := messaging.Prefix(namespace.Name, namespace.Annotations); err != nil {
		return ctrl.Result{}, errors.Join(r.deleteManaged(ctx, quota), r.deleteManaged(ctx, configMap))
	}

	wantQuota, err := messaging.BuildResourceQuota(namespace.Name, r.Config.MaxQueuesPerNamespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.upsert(ctx, &namespace, quota, wantQuota.Labels, func() {
		quota.Spec.Hard = wantQuota.Spec.Hard
	}); err != nil {
		return ctrl.Result{}, err
	}

	var queues messagingv1alpha1.QueueList
	if err := r.List(ctx, &queues, client.InNamespace(namespace.Name)); err != nil {
		return ctrl.Result{}, err
	}
	if len(queues.Items) == 0 {
		return ctrl.Result{}, r.deleteManaged(ctx, configMap)
	}
	var sameName []messagingv1alpha1.Queue
	for i := range queues.Items {
		named, err := queuesNamed(ctx, r.Client, queues.Items[i].Spec.Name)
		if err != nil {
			return ctrl.Result{}, err
		}
		sameName = append(sameName, named...)
	}
	wantConfigMap, err := messaging.BuildConnectionConfigMap(namespace.Name, r.Config.Endpoint,
		messaging.ConnectedQueues(namespace.Name, namespace.Annotations, queues.Items, sameName))
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, r.upsert(ctx, &namespace, configMap, wantConfigMap.Labels, func() {
		configMap.Data = wantConfigMap.Data
		configMap.BinaryData = nil
	})
}

// upsert creates or updates obj, adds labels to its labels and lets mutate
// set its content. An object of the same name without the managed-by label
// belongs to someone else: upsert leaves it as it is and emits a Warning
// event on it, in its namespace, where its owner can see it. Only the owner
// can fix that, so upsert returns no error and the reconcile does not retry.
// The cache holds only managed objects, so a foreign object makes Get report
// NotFound and Create report AlreadyExists. The cache can also lag behind a
// Create of the last reconcile, so on AlreadyExists the object is read from
// the API server before it counts as foreign.
func (r *NamespaceReconciler) upsert(
	ctx context.Context,
	namespace *corev1.Namespace,
	obj client.Object,
	labels map[string]string,
	mutate func(),
) error {
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		if obj.GetResourceVersion() != "" && !managed(obj) {
			return errNotManaged
		}
		merged := obj.GetLabels()
		if merged == nil {
			merged = map[string]string{}
		}
		maps.Copy(merged, labels)
		obj.SetLabels(merged)
		mutate()
		return nil
	})
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errNotManaged):
	case apierrors.IsAlreadyExists(err):
		if readErr := r.APIReader.Get(ctx, client.ObjectKeyFromObject(obj), obj); readErr != nil {
			return readErr
		}
		if managed(obj) {
			// The operator's own object: the retry sees it in the cache.
			return err
		}
	default:
		return err
	}

	gvk, err := r.GroupVersionKindFor(obj)
	if err != nil {
		return err
	}
	r.Recorder.Eventf(obj, namespace, corev1.EventTypeWarning, ReasonNotManaged, "Reconcile",
		"%s %s exists without the label %s=%s, so the operator does not change it",
		gvk.Kind, obj.GetName(), messaging.ManagedByLabel, messaging.ManagedByValue)
	logf.FromContext(ctx).Info("skipped an object without the managed-by label",
		"kind", gvk.Kind, "name", obj.GetName(), "namespace", namespace.Name)

	return nil
}

// deleteManaged deletes obj when it exists and has the managed-by label.
func (r *NamespaceReconciler) deleteManaged(ctx context.Context, obj client.Object) error {
	if err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !managed(obj) {
		return nil
	}

	return client.IgnoreNotFound(r.Delete(ctx, obj))
}

func managed(obj client.Object) bool {
	return obj.GetLabels()[messaging.ManagedByLabel] == messaging.ManagedByValue
}

// SetupWithManager sets up the controller with the Manager. A namespace is
// reconciled when it is created, when its prefix annotation changes, when a
// Queue in it or a Queue with the same queue name as one of its Queues is
// created, changed or deleted, and when its managed ConfigMap or quota
// changes. At start-up every namespace comes as a create event, so a new
// quota limit or endpoint reaches every namespace. IndexQueues must run
// first.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	toNamespace := handler.EnqueueRequestsFromMapFunc(namespaceOf)
	managedOnly := builder.WithPredicates(predicate.NewPredicateFuncs(managed))

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}, builder.WithPredicates(namespaceChanged())).
		Watches(&messagingv1alpha1.Queue{}, handler.EnqueueRequestsFromMapFunc(r.queueNamespaces),
			builder.WithPredicates(queueChanged())).
		Watches(&corev1.ConfigMap{}, toNamespace, managedOnly).
		Watches(&corev1.ResourceQuota{}, toNamespace, managedOnly).
		Named("namespace").
		Complete(r)
}

// namespaceOf maps an object to its namespace.
func namespaceOf(_ context.Context, obj client.Object) []ctrl.Request {
	return []ctrl.Request{{NamespacedName: client.ObjectKey{Name: obj.GetNamespace()}}}
}

// queueNamespaces maps a Queue to its namespace and to the namespaces of the
// Queues with the same queue name. The key of a name moves to another
// namespace when the Queue that holds the name goes.
func (r *NamespaceReconciler) queueNamespaces(ctx context.Context, obj client.Object) []ctrl.Request {
	requests := namespaceOf(ctx, obj)
	queue, ok := obj.(*messagingv1alpha1.Queue)
	if !ok {
		return requests
	}
	sameName, err := queuesNamed(ctx, r.Client, queue.Spec.Name)
	if err != nil {
		logf.FromContext(ctx).Error(err, "cannot list the Queues with the same queue name",
			"queue", client.ObjectKeyFromObject(obj))
		return requests
	}

	for i := range sameName {
		requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKey{Name: sameName[i].Namespace}})
	}
	return requests
}

// namespaceChanged lets a new namespace through, and an update that sets,
// changes or removes the prefix annotation. The deletion of a namespace
// deletes its objects.
func namespaceChanged() predicate.Funcs {
	return predicate.Funcs{
		DeleteFunc: func(event.DeleteEvent) bool { return false },
		UpdateFunc: prefixAnnotationChanged().UpdateFunc,
	}
}

// queueChanged lets a Queue update through when its spec changed or its
// deletion started. A deletion does not change the generation of a Queue
// with a finalizer, and a status write changes nothing in the ConfigMap.
func queueChanged() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() ||
				e.ObjectOld.GetDeletionTimestamp().IsZero() != e.ObjectNew.GetDeletionTimestamp().IsZero()
		},
	}
}
