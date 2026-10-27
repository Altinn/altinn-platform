/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-common/identityref"
	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	servicebusv1 "github.com/Azure/azure-service-operator/v2/api/servicebus/v1api20240101"
	"github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/messaging"
)

const (
	// fieldOwner is the server-side apply field manager of this operator.
	fieldOwner = "dis-messaging-operator"
	// identityRefIndex indexes a Queue by the namespace/name of each
	// ApplicationIdentity that it can grant a role.
	identityRefIndex = "spec.access.principals.identityRef"
	// queueNameIndex indexes a Queue by messaging.NameKey of spec.name.
	queueNameIndex = "spec.name"
	// identityRequeueDelay is the requeue of a Queue with an identity that is
	// not ready. The ApplicationIdentity watch normally comes first.
	identityRequeueDelay = time.Minute
)

// QueueReconciler reconciles a Queue object.
type QueueReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config config.OperatorConfig
}

// +kubebuilder:rbac:groups=messaging.dis.altinn.cloud,resources=queues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=messaging.dis.altinn.cloud,resources=queues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=messaging.dis.altinn.cloud,resources=queues/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=servicebus.azure.com,resources=namespacesqueues,verbs=get;list;watch;create;patch;delete
// +kubebuilder:rbac:groups=authorization.azure.com,resources=roleassignments,verbs=get;list;watch;create;patch;delete
// +kubebuilder:rbac:groups=application.dis.altinn.cloud,resources=applicationidentities,verbs=get;list;watch

// Reconcile checks the queue name against the prefix of the namespace and
// against the other Queues with the same name (messaging.NameHolder),
// applies the ASO NamespacesQueue and a RoleAssignment for each role of each
// principal, and mirrors their Ready conditions into the Queue status. There
// is no finalizer: the owner references delete the ASO resources, and the
// reconcile policy of the NamespacesQueue decides if Azure keeps the queue.
func (r *QueueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var queue messagingv1alpha1.Queue
	if err := r.Get(ctx, req.NamespacedName, &queue); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !queue.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	var namespace corev1.Namespace
	if err := r.Get(ctx, client.ObjectKey{Name: queue.Namespace}, &namespace); err != nil {
		return ctrl.Result{}, err
	}
	// Only a change of the Queue or of the namespace prefix fixes the name,
	// and both bring their own event, so an invalid name is not requeued.
	if err := messaging.ValidateName(namespace.Name, namespace.Annotations, queue.Spec.Name); err != nil {
		return ctrl.Result{}, r.writeStatus(ctx, &queue, noQueue, newCondition(messagingv1alpha1.ConditionQueueReady,
			queue.Generation, metav1.ConditionFalse, messaging.ReasonInvalidName, err.Error()),
			waitingForQueue(queue.Generation))
	}

	sameName, err := queuesNamed(ctx, r.Client, queue.Spec.Name)
	if err != nil {
		return ctrl.Result{}, err
	}
	if holder := messaging.NameHolder(&queue, sameName); holder != &queue {
		return ctrl.Result{}, r.yield(ctx, &queue, holder)
	}

	nsQueue, err := r.applyNamespacesQueue(ctx, &queue)
	if err != nil {
		return ctrl.Result{}, r.failed(ctx, &queue, nil,
			applyFailed(messagingv1alpha1.ConditionQueueReady, queue.Generation, err), waitingForQueue(queue.Generation), err)
	}
	fields, queueReady := azureQueue(queue.Spec.Name, nsQueue), queueReadyCondition(queue.Generation, nsQueue)

	accessReady, err := r.reconcileAccess(ctx, &queue, nsQueue)
	if err != nil {
		return ctrl.Result{}, r.failed(ctx, &queue, fields, queueReady,
			applyFailed(messagingv1alpha1.ConditionAccessReady, queue.Generation, err), err)
	}

	// The NamespacesQueue, RoleAssignment and ApplicationIdentity watches
	// bring the next reconcile when ASO or the identity operator reports.
	var result ctrl.Result
	if accessReady.Reason == identityref.ReasonIdentityNotReady {
		result.RequeueAfter = identityRequeueDelay
	}
	return result, r.writeStatus(ctx, &queue, fields, queueReady, accessReady)
}

// applyNamespacesQueue writes the NamespacesQueue and returns the stored
// object with its status. Server-side apply also removes the reconcile-policy
// annotation when the deletion policy changes from Retain to Delete, and it
// keeps the annotations that ASO writes.
func (r *QueueReconciler) applyNamespacesQueue(
	ctx context.Context,
	queue *messagingv1alpha1.Queue,
) (*servicebusv1.NamespacesQueue, error) {
	nsQueue, err := messaging.BuildNamespacesQueue(queue, r.Config.NamespaceResourceID)
	if err != nil {
		return nil, err
	}

	return nsQueue, r.apply(ctx, queue, nsQueue)
}

// yield reports that holder holds the queue name of queue, and removes what
// queue applied. Two Queues created in the same second can both apply
// before either one sees the other.
func (r *QueueReconciler) yield(ctx context.Context, queue, holder *messagingv1alpha1.Queue) error {
	if err := r.writeStatus(ctx, queue, noQueue, newCondition(messagingv1alpha1.ConditionQueueReady,
		queue.Generation, metav1.ConditionFalse, messaging.ReasonConflict,
		fmt.Sprintf("Queue %s/%s holds the queue name %q: the oldest Queue holds a name, and Azure ignores case in names",
			holder.Namespace, holder.Name, holder.Spec.Name)),
		waitingForQueue(queue.Generation)); err != nil {
		return err
	}
	if err := r.deleteStaleGrants(ctx, queue, nil); err != nil {
		return err
	}

	return r.release(ctx, queue)
}

// release deletes the NamespacesQueue of queue and keeps the Azure queue,
// which the holder of the name uses. ASO reads the reconcile policy when it
// deletes, so detach-on-delete is set before the delete.
func (r *QueueReconciler) release(ctx context.Context, queue *messagingv1alpha1.Queue) error {
	var nsQueue servicebusv1.NamespacesQueue
	key := client.ObjectKey{Namespace: queue.Namespace, Name: messaging.NamespacesQueueName(queue)}
	if err := r.Get(ctx, key, &nsQueue); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !metav1.IsControlledBy(&nsQueue, queue) || !nsQueue.DeletionTimestamp.IsZero() {
		return nil
	}

	patch := client.MergeFrom(nsQueue.DeepCopy())
	metav1.SetMetaDataAnnotation(&nsQueue.ObjectMeta, annotations.ReconcilePolicy,
		string(annotations.ReconcilePolicyDetachOnDelete))
	if err := r.Patch(ctx, &nsQueue, patch); err != nil {
		return err
	}

	return client.IgnoreNotFound(r.Delete(ctx, &nsQueue))
}

// apply writes obj with server-side apply and the Queue as its controller,
// and reads the stored object, status included, back into obj.
func (r *QueueReconciler) apply(ctx context.Context, queue *messagingv1alpha1.Queue, obj client.Object) error {
	gvk, err := apiutil.GVKForObject(obj, r.Scheme)
	if err != nil {
		return err
	}
	// The operator does not take over an ASO resource it does not own. With
	// ForceOwnership the apply would adopt an object that a team created by
	// hand, so a foreign object of the same name stops the apply.
	existing, err := r.Scheme.New(gvk)
	if err != nil {
		return err
	}
	existingObj, ok := existing.(client.Object)
	if !ok {
		return fmt.Errorf("%s is not a Kubernetes object", gvk.Kind)
	}
	switch err := r.Get(ctx, client.ObjectKeyFromObject(obj), existingObj); {
	case apierrors.IsNotFound(err):
	case err != nil:
		return err
	case !metav1.IsControlledBy(existingObj, queue):
		return fmt.Errorf("%s %s exists and is not controlled by this Queue", gvk.Kind, obj.GetName())
	}
	if err := controllerutil.SetControllerReference(queue, obj, r.Scheme); err != nil {
		return err
	}
	obj.GetObjectKind().SetGroupVersionKind(gvk)

	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}
	applied := &unstructured.Unstructured{Object: content}
	unstructured.RemoveNestedField(applied.Object, "status")
	unstructured.RemoveNestedField(applied.Object, "metadata", "creationTimestamp")
	if err := r.Apply(ctx, client.ApplyConfigurationFromUnstructured(applied),
		client.FieldOwner(fieldOwner), client.ForceOwnership); err != nil {
		return err
	}

	// The apply response is the stored object. The informer cache can still
	// hold the version before the apply.
	return runtime.DefaultUnstructuredConverter.FromUnstructured(applied.Object, obj)
}

// SetupWithManager sets up the controller with the Manager. A Queue is
// reconciled again when its spec changes, not when the operator writes its
// status. A NamespacesQueue or a RoleAssignment reconciles the owning Queue
// when its spec or its status changed, such as ASO reporting Ready. A change
// of the prefix annotation of a namespace reconciles every Queue in it. An
// ApplicationIdentity reconciles every Queue that can grant it a role, in any
// namespace. A new or deleted Queue reconciles the Queues with the same
// queue name. IndexQueues must run first.
func (r *QueueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&messagingv1alpha1.Queue{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		// spec.name is immutable, so only a new or a deleted Queue changes
		// which Queue holds a name.
		Watches(&messagingv1alpha1.Queue{}, handler.EnqueueRequestsFromMapFunc(r.sameNameQueues),
			builder.WithPredicates(predicate.Funcs{UpdateFunc: func(event.UpdateEvent) bool { return false }})).
		Owns(&servicebusv1.NamespacesQueue{}, builder.WithPredicates(asoResourceChanged())).
		Owns(&authorizationv1.RoleAssignment{}, builder.WithPredicates(asoResourceChanged())).
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(r.namespaceToQueues),
			builder.WithPredicates(prefixAnnotationChanged())).
		Watches(&identityv1alpha1.ApplicationIdentity{}, handler.EnqueueRequestsFromMapFunc(r.identityToQueues)).
		Named("queue").
		Complete(r)
}

// namespaceToQueues maps a Namespace to every Queue in it. A new prefix makes
// queue names valid or invalid, and no other event brings such a Queue back:
// spec.name is immutable, and an invalid Queue has no NamespacesQueue.
func (r *QueueReconciler) namespaceToQueues(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.queueRequests(ctx, obj, client.InNamespace(obj.GetName()))
}

// identityToQueues maps an ApplicationIdentity to the Queues that can grant
// it a role. A new principal ID gives the grant a new name.
func (r *QueueReconciler) identityToQueues(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.queueRequests(ctx, obj, client.MatchingFields{identityRefIndex: client.ObjectKeyFromObject(obj).String()})
}

// sameNameQueues maps a Queue to the Queues with the same queue name, in any
// namespace. A Queue created in the same second can take the name from a
// Queue that applied first, and a deleted Queue passes the name on.
func (r *QueueReconciler) sameNameQueues(ctx context.Context, obj client.Object) []ctrl.Request {
	queue, ok := obj.(*messagingv1alpha1.Queue)
	if !ok {
		return nil
	}

	return r.queueRequests(ctx, obj, client.MatchingFields{queueNameIndex: messaging.NameKey(queue.Spec.Name)})
}

// IndexQueues registers the Queue indexes that the reconcilers read. A field
// can be indexed only once, so the manager calls it once, before it sets up
// the reconcilers.
func IndexQueues(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &messagingv1alpha1.Queue{}, identityRefIndex, identityRefKeys); err != nil {
		return err
	}

	return indexer.IndexField(ctx, &messagingv1alpha1.Queue{}, queueNameIndex, queueNameKeys)
}

// queuesNamed returns the Queues in the cluster with the NameKey of
// queueName.
func queuesNamed(ctx context.Context, c client.Reader, queueName string) ([]messagingv1alpha1.Queue, error) {
	var queues messagingv1alpha1.QueueList
	err := c.List(ctx, &queues, client.MatchingFields{queueNameIndex: messaging.NameKey(queueName)})

	return queues.Items, err
}

// queueNameKeys returns the index key of a Queue for queueNameIndex.
func queueNameKeys(obj client.Object) []string {
	queue, ok := obj.(*messagingv1alpha1.Queue)
	if !ok {
		return nil
	}

	return []string{messaging.NameKey(queue.Spec.Name)}
}

// identityRefKeys returns the index keys of a Queue for identityRefIndex.
func identityRefKeys(obj client.Object) []string {
	queue, ok := obj.(*messagingv1alpha1.Queue)
	if !ok {
		return nil
	}

	identities := messaging.SplitPrincipals(queue).Identities
	keys := make([]string, 0, len(identities))
	for _, identity := range identities {
		keys = append(keys, client.ObjectKey{Namespace: identity.Ref.Namespace, Name: identity.Ref.Name}.String())
	}

	return keys
}

// queueRequests returns a request for each Queue that opts select. obj is
// the object of the event.
func (r *QueueReconciler) queueRequests(ctx context.Context, obj client.Object, opts ...client.ListOption) []ctrl.Request {
	var queues messagingv1alpha1.QueueList
	if err := r.List(ctx, &queues, opts...); err != nil {
		logf.FromContext(ctx).Error(err, "cannot list the Queues of an event", "object", client.ObjectKeyFromObject(obj))
		return nil
	}

	requests := make([]ctrl.Request, 0, len(queues.Items))
	for i := range queues.Items {
		requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&queues.Items[i])})
	}

	return requests
}

// prefixAnnotationChanged lets a Namespace event through when the prefix
// annotation was set, changed or removed. Every other change of a namespace
// leaves the queue names as they are. A new namespace has no Queues yet, and
// at start-up the Queue watch enqueues every Queue, so a Create event brings
// nothing. A deleted namespace has no Queues either.
func prefixAnnotationChanged() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return false },
		DeleteFunc: func(event.DeleteEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPrefix, oldSet := e.ObjectOld.GetAnnotations()[messaging.PrefixAnnotation]
			newPrefix, newSet := e.ObjectNew.GetAnnotations()[messaging.PrefixAnnotation]

			return oldSet != newSet || oldPrefix != newPrefix
		},
	}
}

// asoResourceChanged lets an update of an ASO resource through when its
// generation, its controller, its reconcile policy or its status changed. ASO
// writes a poller annotation on every poll of a running Azure operation, and
// each of those writes would cost a reconcile and an apply round trip. The
// reconcile policy is what keeps the Azure queue of a Retain Queue, so a
// policy removed by hand must bring the apply that restores it. A controller
// reference removed by hand makes the object foreign, and the Queue must
// report that instead of staying Ready.
func asoResourceChanged() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj, ok := e.ObjectOld.(genruntime.ARMMetaObject)
			if !ok {
				return true
			}
			newObj, ok := e.ObjectNew.(genruntime.ARMMetaObject)
			if !ok {
				return true
			}

			return oldObj.GetGeneration() != newObj.GetGeneration() ||
				oldObj.GetAnnotations()[annotations.ReconcilePolicy] != newObj.GetAnnotations()[annotations.ReconcilePolicy] ||
				!equality.Semantic.DeepEqual(metav1.GetControllerOf(oldObj), metav1.GetControllerOf(newObj)) ||
				!equality.Semantic.DeepEqual(oldObj.GetStatus(), newObj.GetStatus())
		},
	}
}
