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

	servicebusv1 "github.com/Azure/azure-service-operator/v2/api/servicebus/v1api20240101"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-messaging-operator/internal/messaging"
)

// fieldOwner is the server-side apply field manager of this operator.
const fieldOwner = "dis-messaging-operator"

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
// +kubebuilder:rbac:groups=servicebus.azure.com,resources=namespacesqueues,verbs=get;list;watch;create;patch

// Reconcile checks the queue name against the prefix of the namespace,
// applies the ASO NamespacesQueue, and mirrors its Ready condition into the
// Queue status. There is no finalizer: the owner reference deletes the
// NamespacesQueue, and its reconcile policy decides if Azure keeps the queue.
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
	// Only a change of the Queue or the namespace fixes the name, so an
	// invalid name is not requeued.
	if err := messaging.ValidateName(namespace.Name, namespace.Annotations, queue.Spec.Name); err != nil {
		return ctrl.Result{}, r.writeStatus(ctx, &queue, nil, newCondition(messagingv1alpha1.ConditionQueueReady,
			queue.Generation, metav1.ConditionFalse, messaging.ReasonInvalidName, err.Error()))
	}

	nsQueue, err := r.applyNamespacesQueue(ctx, &queue)
	if err != nil {
		return ctrl.Result{}, r.failed(ctx, &queue, err)
	}

	// The NamespacesQueue watch brings the next reconcile when ASO reports.
	return ctrl.Result{}, r.writeStatus(ctx, &queue, nsQueue, queueReadyCondition(queue.Generation, nsQueue))
}

// applyNamespacesQueue writes the NamespacesQueue with server-side apply and
// the Queue as its controller, and returns the stored object with its status.
// Server-side apply also removes the reconcile-policy annotation when the
// deletion policy changes from Retain to Delete, and it keeps the annotations
// that ASO writes.
func (r *QueueReconciler) applyNamespacesQueue(
	ctx context.Context,
	queue *messagingv1alpha1.Queue,
) (*servicebusv1.NamespacesQueue, error) {
	desired, err := messaging.BuildNamespacesQueue(queue, r.Config.NamespaceResourceID)
	if err != nil {
		return nil, err
	}
	// The operator does not take over an ASO resource it does not own. With
	// ForceOwnership the apply would adopt a NamespacesQueue that a team
	// created by hand, so a foreign object of the same name stops the apply.
	existing := &servicebusv1.NamespacesQueue{}
	switch err := r.Get(ctx, client.ObjectKeyFromObject(desired), existing); {
	case apierrors.IsNotFound(err):
	case err != nil:
		return nil, err
	case !metav1.IsControlledBy(existing, queue):
		return nil, fmt.Errorf("NamespacesQueue %s exists and is not controlled by this Queue", desired.Name)
	}
	if err := controllerutil.SetControllerReference(queue, desired, r.Scheme); err != nil {
		return nil, err
	}
	gvk, err := apiutil.GVKForObject(desired, r.Scheme)
	if err != nil {
		return nil, err
	}
	desired.SetGroupVersionKind(gvk)

	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(desired)
	if err != nil {
		return nil, err
	}
	applied := &unstructured.Unstructured{Object: content}
	unstructured.RemoveNestedField(applied.Object, "status")
	unstructured.RemoveNestedField(applied.Object, "metadata", "creationTimestamp")
	if err := r.Apply(ctx, client.ApplyConfigurationFromUnstructured(applied),
		client.FieldOwner(fieldOwner), client.ForceOwnership); err != nil {
		return nil, err
	}

	// The apply response is the stored object. The informer cache can still
	// hold the version before the apply.
	current := &servicebusv1.NamespacesQueue{}
	return current, runtime.DefaultUnstructuredConverter.FromUnstructured(applied.Object, current)
}

// SetupWithManager sets up the controller with the Manager. A Queue is
// reconciled again when its spec changes, not when the operator writes its
// status. A NamespacesQueue event, such as ASO writing its status, reconciles
// the owning Queue.
func (r *QueueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&messagingv1alpha1.Queue{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&servicebusv1.NamespacesQueue{}).
		Named("queue").
		Complete(r)
}
