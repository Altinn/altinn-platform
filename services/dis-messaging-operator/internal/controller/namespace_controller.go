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

	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/messaging"
)

// ReasonNotManaged is the reason of the Warning event for a dis-messaging
// object that the operator did not create.
const ReasonNotManaged = "NotManaged"

var errNotManaged = errors.New("object exists without the managed-by label")

// NamespaceReconciler reconciles the objects that the operator keeps once
// per product namespace, such as the ResourceQuota that limits the Queues.
type NamespaceReconciler struct {
	client.Client
	Config   config.OperatorConfig
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile keeps the quota in every product namespace, so the API server
// limits the Queues from the first one. A namespace that is no longer a
// product namespace loses the quota.
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var namespace corev1.Namespace
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, &namespace); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// The deletion of the namespace deletes the quota.
	if !namespace.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	quota := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{
		Name: messaging.ResourceQuotaName, Namespace: namespace.Name,
	}}
	if _, err := messaging.Prefix(namespace.Name, namespace.Annotations); err != nil {
		return ctrl.Result{}, r.deleteManaged(ctx, quota)
	}

	want, err := messaging.BuildResourceQuota(namespace.Name, r.Config.MaxQueuesPerNamespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, r.upsert(ctx, &namespace, quota, want.Labels, func() {
		quota.Spec.Hard = want.Spec.Hard
	})
}

// upsert creates or updates obj, adds labels to its labels and lets mutate
// set its content. An object of the same name without the managed-by label
// belongs to someone else: upsert leaves it as it is and emits a Warning
// event on the namespace. Only its owner can fix that, so upsert returns no
// error and the reconcile does not retry. The cache holds only managed
// objects, so for a foreign object Get reports NotFound and Create reports
// AlreadyExists.
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
	if !errors.Is(err, errNotManaged) && !apierrors.IsAlreadyExists(err) {
		return err
	}

	gvk, err := r.GroupVersionKindFor(obj)
	if err != nil {
		return err
	}
	r.Recorder.Eventf(namespace, nil, corev1.EventTypeWarning, ReasonNotManaged, "Reconcile",
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
// reconciled when it is created, when its prefix annotation changes, and
// when its managed quota changes. At start-up every namespace comes as a
// create event, so a new quota limit reaches every namespace.
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}, builder.WithPredicates(namespaceChanged())).
		Watches(&corev1.ResourceQuota{}, handler.EnqueueRequestsFromMapFunc(namespaceOf),
			builder.WithPredicates(predicate.NewPredicateFuncs(managed))).
		Named("namespace").
		Complete(r)
}

// namespaceOf maps an object to its namespace.
func namespaceOf(_ context.Context, obj client.Object) []ctrl.Request {
	return []ctrl.Request{{NamespacedName: client.ObjectKey{Name: obj.GetNamespace()}}}
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
