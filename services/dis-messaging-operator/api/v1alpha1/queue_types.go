package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QueueAccessRole is the role a principal gets on the queue.
// +kubebuilder:validation:Enum=sender;receiver;consumer;owner
type QueueAccessRole string

const (
	// QueueAccessRoleSender grants Azure Service Bus Data Sender.
	QueueAccessRoleSender QueueAccessRole = "sender"

	// QueueAccessRoleReceiver grants Azure Service Bus Data Receiver.
	QueueAccessRoleReceiver QueueAccessRole = "receiver"

	// QueueAccessRoleConsumer grants Data Sender and Data Receiver. A listener
	// needs both, because retries re-send a scheduled copy to the same queue.
	QueueAccessRoleConsumer QueueAccessRole = "consumer"

	// QueueAccessRoleOwner is reserved. The operator does not grant it in this
	// API version.
	QueueAccessRoleOwner QueueAccessRole = "owner"
)

// QueueDeletionPolicy controls what happens to the Azure queue when the Queue
// resource is deleted.
// +kubebuilder:validation:Enum=Retain;Delete
type QueueDeletionPolicy string

const (
	// QueueDeletionPolicyRetain keeps the Azure queue and its messages.
	QueueDeletionPolicyRetain QueueDeletionPolicy = "Retain"

	// QueueDeletionPolicyDelete deletes the Azure queue and its messages.
	QueueDeletionPolicyDelete QueueDeletionPolicy = "Delete"
)

// ISODuration is an ISO 8601 duration, for example PT1M or P14D.
// +kubebuilder:validation:MaxLength=64
// +kubebuilder:validation:Pattern=`^P(\d+D)?(T(\d+H)?(\d+M)?(\d+(\.\d+)?S)?)?$`
// +kubebuilder:validation:XValidation:rule="!self.endsWith('P') && !self.endsWith('T')",message="duration must have at least one value, for example PT1M"
type ISODuration string

// ApplicationIdentityRef points to an ApplicationIdentity.
type ApplicationIdentityRef struct {
	// name is the ApplicationIdentity name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// namespace is the ApplicationIdentity namespace. Defaults to the
	// namespace of the Queue.
	// +optional
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace,omitempty"`
}

// QueueGroupPrincipalSpec identifies an Entra group.
type QueueGroupPrincipalSpec struct {
	// name is the Entra group display name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// principalId is the Entra group object ID.
	// +kubebuilder:validation:MinLength=1
	PrincipalId string `json:"principalId"`
}

// QueueServicePrincipalSpec identifies an Entra service principal that has no
// ApplicationIdentity in the cluster, such as an Azure service or an app in
// another cluster.
type QueueServicePrincipalSpec struct {
	// name is a label for the principal, for example the managed identity name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// principalId is the Entra object ID of the service principal.
	// +kubebuilder:validation:MinLength=1
	PrincipalId string `json:"principalId"`
}

// QueueAccessPrincipalSpec describes one principal and the role it gets.
// +kubebuilder:validation:XValidation:rule="(has(self.identityRef) ? 1 : 0) + (has(self.group) ? 1 : 0) + (has(self.servicePrincipal) ? 1 : 0) == 1",message="Provide exactly one principal source: identityRef, group, or servicePrincipal."
type QueueAccessPrincipalSpec struct {
	// role is the role the principal gets on the queue.
	Role QueueAccessRole `json:"role"`

	// identityRef points to an ApplicationIdentity. The operator reads the
	// principal ID from its status.
	// +optional
	IdentityRef *ApplicationIdentityRef `json:"identityRef,omitempty"`

	// group identifies an Entra group.
	// +optional
	Group *QueueGroupPrincipalSpec `json:"group,omitempty"`

	// servicePrincipal identifies an Entra service principal by object ID.
	// +optional
	ServicePrincipal *QueueServicePrincipalSpec `json:"servicePrincipal,omitempty"`
}

// QueueAccessSpec lists the principals that get a role on the queue.
type QueueAccessSpec struct {
	// principals is the list of principals and their roles.
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=50
	Principals []QueueAccessPrincipalSpec `json:"principals"`
}

// QueueSpec defines the desired state of Queue.
type QueueSpec struct {
	// name is the Azure queue name. It must start with the product prefix.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=260
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9]([A-Za-z0-9._/-]*[A-Za-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// lockDuration is how long a received message stays locked for the
	// receiver. Use minutes and seconds only. Azure allows up to PT5M.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self.matches('^PT([0-9]+M)?([0-9]+([.][0-9]+)?S)?$') && (self.find('[0-9]+M') == '' ? 0.0 : double(self.find('[0-9]+M').replace('M', '')) * 60.0) + (self.find('[0-9.]+S') == '' ? 0.0 : double(self.find('[0-9.]+S').replace('S', ''))) <= 300.0",message="lockDuration must use minutes and seconds only and be at most PT5M, for example PT1M or PT30S"
	LockDuration *ISODuration `json:"lockDuration,omitempty"`

	// maxDeliveryCount is how many times Azure delivers a message before it
	// moves the message to the dead-letter queue.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=2000
	MaxDeliveryCount *int32 `json:"maxDeliveryCount,omitempty"`

	// defaultMessageTimeToLive is how long a message lives when the sender
	// does not set a time to live.
	// +optional
	DefaultMessageTimeToLive *ISODuration `json:"defaultMessageTimeToLive,omitempty"`

	// maxSizeInMegabytes is the maximum size of the queue.
	// +optional
	// +kubebuilder:validation:Enum=1024;2048;3072;4096;5120;10240;20480;40960;81920
	MaxSizeInMegabytes *int32 `json:"maxSizeInMegabytes,omitempty"`

	// deadLetteringOnMessageExpiration moves expired messages to the
	// dead-letter queue.
	// +optional
	DeadLetteringOnMessageExpiration *bool `json:"deadLetteringOnMessageExpiration,omitempty"`

	// requiresSession makes the queue session-aware. Azure fixes it at creation.
	// +optional
	// +kubebuilder:default=false
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="requiresSession is immutable"
	RequiresSession bool `json:"requiresSession,omitempty"`

	// requiresDuplicateDetection turns on duplicate detection. Azure fixes it
	// at creation.
	// +optional
	// +kubebuilder:default=false
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="requiresDuplicateDetection is immutable"
	RequiresDuplicateDetection bool `json:"requiresDuplicateDetection,omitempty"`

	// access lists the principals that get a role on the queue.
	// +optional
	Access *QueueAccessSpec `json:"access,omitempty"`

	// deletionPolicy controls what happens to the Azure queue when this
	// resource is deleted.
	// +optional
	// +kubebuilder:default=Retain
	DeletionPolicy QueueDeletionPolicy `json:"deletionPolicy,omitempty"`
}

// QueueStatus defines the observed state of Queue.
type QueueStatus struct {
	// name is the Azure queue name the operator manages.
	// +optional
	Name string `json:"name,omitempty"`

	// resourceId is the Azure resource ID of the queue.
	// +optional
	ResourceId string `json:"resourceId,omitempty"`

	// conditions represent the current state of this Queue.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the latest generation reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ConditionType represents status condition type names used by Queue.
type ConditionType string

const (
	// ConditionReady aggregates the other conditions.
	ConditionReady ConditionType = "Ready"

	// ConditionQueueReady reports the Azure queue.
	ConditionQueueReady ConditionType = "QueueReady"

	// ConditionAccessReady reports the role assignments on the queue.
	ConditionAccessReady ConditionType = "AccessReady"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Queue",type="string",JSONPath=".spec.name"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"

// Queue is the Schema for the queues API.
type Queue struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// Spec defines the desired state of Queue.
	// +required
	Spec QueueSpec `json:"spec"`

	// Status defines the observed state of Queue.
	// +optional
	Status QueueStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// QueueList contains a list of Queue.
type QueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Queue `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Queue{}, &QueueList{})
}
