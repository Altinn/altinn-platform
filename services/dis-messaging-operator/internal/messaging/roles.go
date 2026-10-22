package messaging

import (
	"errors"
	"fmt"
	"strings"

	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	servicebusv1 "github.com/Azure/azure-service-operator/v2/api/servicebus/v1api20240101"
	"github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
)

const (
	// ReasonRoleNotAllowed is the AccessReady condition reason for a Queue
	// with a role that the operator does not grant.
	ReasonRoleNotAllowed = "RoleNotAllowed"

	// Role definition IDs of the Azure built-in roles. See
	// https://learn.microsoft.com/azure/role-based-access-control/built-in-roles/integration
	// The operator never grants Azure Service Bus Data Owner.
	serviceBusDataSenderRoleID   = "69a216fc-b8fb-44d8-bc22-1f3c2cd27a39"
	serviceBusDataReceiverRoleID = "4f6d3b9b-027b-4f4c-9142-0e5a2a2247e0"
)

// ErrRoleNotAllowed is wrapped by the errors for a role that the operator does
// not grant. The reconciler sets reason ReasonRoleNotAllowed for it.
var ErrRoleNotAllowed = errors.New("the operator does not grant this role on a queue")

// Principal is a principal of a Queue with a known Entra object ID.
type Principal struct {
	// ID is the Entra object ID.
	ID string
	// Type is the Azure principal type. See PrincipalType.
	Type authorizationv1.RoleAssignmentProperties_PrincipalType
	// Role is the role of the principal on the queue.
	Role messagingv1alpha1.QueueAccessRole
}

// PrincipalType returns the Azure principal type of a Queue principal: Group
// for a group, ServicePrincipal for an identity or a service principal.
func PrincipalType(p messagingv1alpha1.QueueAccessPrincipalSpec) authorizationv1.RoleAssignmentProperties_PrincipalType {
	if p.Group != nil {
		return authorizationv1.RoleAssignmentProperties_PrincipalType_Group
	}
	return authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal
}

// AzureRoleIDs returns the role definition IDs of the Azure built-in roles
// that a role gives on a queue. The owner role, and a role that this version
// does not know, give an error that wraps ErrRoleNotAllowed.
func AzureRoleIDs(role messagingv1alpha1.QueueAccessRole) ([]string, error) {
	switch role {
	case messagingv1alpha1.QueueAccessRoleSender:
		return []string{serviceBusDataSenderRoleID}, nil
	case messagingv1alpha1.QueueAccessRoleReceiver:
		return []string{serviceBusDataReceiverRoleID}, nil
	case messagingv1alpha1.QueueAccessRoleConsumer:
		return []string{serviceBusDataSenderRoleID, serviceBusDataReceiverRoleID}, nil
	default:
		return nil, fmt.Errorf("role %q: %w", role, ErrRoleNotAllowed)
	}
}

// BuildRoleAssignments returns the ASO RoleAssignments that give each
// principal its role on the queue, one per principal and Azure role. nsQueue
// is the NamespacesQueue of the queue from BuildNamespacesQueue. It is the
// owner, and so the scope, of every RoleAssignment. subscriptionID is the
// subscription of the role definitions. When two principals give the same
// principal ID the same Azure role, the first one wins.
//
// Every RoleAssignment has the reconcile policy manage, so the Azure role
// assignment goes when the Queue goes. This is also true for the deletion
// policy Retain: the Azure queue keeps its messages, but nobody keeps access
// to them. The annotation stops a reconcile policy on the namespace or an ASO
// default from changing this.
func BuildRoleAssignments(
	queue *messagingv1alpha1.Queue,
	nsQueue *servicebusv1.NamespacesQueue,
	principals []Principal,
	subscriptionID string,
) ([]*authorizationv1.RoleAssignment, error) {
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID must not be empty")
	}
	if nsQueue == nil || nsQueue.Spec.AzureName == "" || nsQueue.Spec.Owner == nil || nsQueue.Spec.Owner.ARMID == "" {
		return nil, fmt.Errorf("the NamespacesQueue must have an Azure name and an owner ARM ID")
	}

	// A role assignment name must be unique in the Entra tenant, not only at
	// its scope. The namespace ID makes the queue path unique across clusters.
	queuePath := nsQueue.Spec.Owner.ARMID + "/queues/" + nsQueue.Spec.AzureName

	var assignments []*authorizationv1.RoleAssignment
	seen := map[string]bool{}
	for _, principal := range principals {
		if principal.ID == "" {
			return nil, fmt.Errorf("principal with role %q has no principal ID", principal.Role)
		}
		roleIDs, err := AzureRoleIDs(principal.Role)
		if err != nil {
			return nil, err
		}

		for _, roleID := range roleIDs {
			seed := strings.Join([]string{queue.Namespace, queuePath, principal.ID, roleID}, "/")
			azureName := uuid.NewSHA1(uuid.NameSpaceURL, []byte(seed)).String()
			if seen[azureName] {
				continue
			}
			seen[azureName] = true

			assignments = append(assignments, &authorizationv1.RoleAssignment{
				ObjectMeta: metav1.ObjectMeta{
					// The Azure name holds every field that ASO cannot
					// change after creation, so a change gives a new object.
					// The CRD keeps the Queue name to MaxQueueNameLength, so
					// the name has at most 100 characters.
					Name:      queue.Name + "-" + azureName,
					Namespace: queue.Namespace,
					Labels: map[string]string{
						ManagedByLabel: ManagedByValue,
						QueueNameLabel: queue.Name,
					},
					Annotations: map[string]string{
						annotations.ReconcilePolicy: string(annotations.ReconcilePolicyManage),
					},
				},
				Spec: authorizationv1.RoleAssignment_Spec{
					AzureName: azureName,
					Owner: &genruntime.ArbitraryOwnerReference{
						Group: servicebusv1.GroupVersion.Group,
						Kind:  "NamespacesQueue",
						Name:  nsQueue.Name,
					},
					PrincipalId:   ptr.To(principal.ID),
					PrincipalType: ptr.To(principal.Type),
					RoleDefinitionReference: &genruntime.WellKnownResourceReference{
						ResourceReference: genruntime.ResourceReference{
							ARMID: "/subscriptions/" + subscriptionID + "/providers/Microsoft.Authorization/roleDefinitions/" + roleID,
						},
					},
				},
			})
		}
	}
	return assignments, nil
}
