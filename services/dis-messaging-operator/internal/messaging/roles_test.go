package messaging

import (
	"errors"
	"testing"

	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	servicebusv1 "github.com/Azure/azure-service-operator/v2/api/servicebus/v1api20240101"
	"github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	messagingv1alpha1 "github.com/Altinn/altinn-platform/services/dis-messaging-operator/api/v1alpha1"
)

const (
	testSubscriptionID = "00000000-0000-0000-0000-000000000000"
	testIdentityID     = "11111111-1111-1111-1111-111111111111"
	testGroupID        = "22222222-2222-2222-2222-222222222222"

	servicePrincipalType = authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal
	groupType            = authorizationv1.RoleAssignmentProperties_PrincipalType_Group
)

var (
	identitySender   = Principal{ID: testIdentityID, Type: servicePrincipalType, Role: messagingv1alpha1.QueueAccessRoleSender}
	identityConsumer = Principal{ID: testIdentityID, Type: servicePrincipalType, Role: messagingv1alpha1.QueueAccessRoleConsumer}
	groupReceiver    = Principal{ID: testGroupID, Type: groupType, Role: messagingv1alpha1.QueueAccessRoleReceiver}
)

// The Azure names are fixed on purpose. A new seed gives every role
// assignment in Azure a new name.
var (
	wantIdentitySender   = wantRoleAssignment("74fd9b3c-d2e5-595d-be5f-6ffbf8a1d6cf", testIdentityID, servicePrincipalType, serviceBusDataSenderRoleID)
	wantIdentityReceiver = wantRoleAssignment("bb11dad8-1d33-5192-a05f-e8e4ea750167", testIdentityID, servicePrincipalType, serviceBusDataReceiverRoleID)
	wantGroupReceiver    = wantRoleAssignment("49c60cf2-3a5a-5021-8a3b-4b7dbda05365", testGroupID, groupType, serviceBusDataReceiverRoleID)
)

func wantRoleAssignment(azureName, principalID string, principalType authorizationv1.RoleAssignmentProperties_PrincipalType, roleID string) *authorizationv1.RoleAssignment {
	return &authorizationv1.RoleAssignment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-one-" + azureName,
			Namespace: "product-team-a",
			Labels: map[string]string{
				ManagedByLabel: ManagedByValue,
				QueueNameLabel: "app-one",
			},
			Annotations: map[string]string{
				annotations.ReconcilePolicy: string(annotations.ReconcilePolicyManage),
			},
		},
		Spec: authorizationv1.RoleAssignment_Spec{
			AzureName:     azureName,
			Owner:         &genruntime.ArbitraryOwnerReference{Group: "servicebus.azure.com", Kind: "NamespacesQueue", Name: "app-one"},
			PrincipalId:   ptr.To(principalID),
			PrincipalType: ptr.To(principalType),
			RoleDefinitionReference: &genruntime.WellKnownResourceReference{
				ResourceReference: genruntime.ResourceReference{
					ARMID: "/subscriptions/" + testSubscriptionID + "/providers/Microsoft.Authorization/roleDefinitions/" + roleID,
				},
			},
		},
	}
}

func testNamespacesQueue(t *testing.T, queue *messagingv1alpha1.Queue, namespaceResourceID string) *servicebusv1.NamespacesQueue {
	t.Helper()

	nsQueue, err := BuildNamespacesQueue(queue, namespaceResourceID)
	if err != nil {
		t.Fatalf("BuildNamespacesQueue: %v", err)
	}
	return nsQueue
}

func TestBuildRoleAssignments(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		principals []Principal
		want       []*authorizationv1.RoleAssignment
	}{
		"no principals":            {},
		"sender":                   {principals: []Principal{identitySender}, want: []*authorizationv1.RoleAssignment{wantIdentitySender}},
		"group receiver":           {principals: []Principal{groupReceiver}, want: []*authorizationv1.RoleAssignment{wantGroupReceiver}},
		"consumer gets both roles": {principals: []Principal{identityConsumer}, want: []*authorizationv1.RoleAssignment{wantIdentitySender, wantIdentityReceiver}},
		"one assignment per principal and Azure role": {
			principals: []Principal{identitySender, groupReceiver, identityConsumer},
			want:       []*authorizationv1.RoleAssignment{wantIdentitySender, wantGroupReceiver, wantIdentityReceiver},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			queue := testQueue(messagingv1alpha1.QueueSpec{})
			got, err := BuildRoleAssignments(queue, testNamespacesQueue(t, queue, testNamespaceResourceID), tt.principals, testSubscriptionID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("RoleAssignments mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildRoleAssignmentsUniqueAcrossClusters(t *testing.T) {
	t.Parallel()

	queue := testQueue(messagingv1alpha1.QueueSpec{})
	otherNamespaceID := "/subscriptions/" + testSubscriptionID + "/resourceGroups/rg-messaging/providers/Microsoft.ServiceBus/namespaces/sb-other"

	got, err := BuildRoleAssignments(queue, testNamespacesQueue(t, queue, otherNamespaceID), []Principal{groupReceiver}, testSubscriptionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Spec.AzureName == wantGroupReceiver.Spec.AzureName {
		t.Fatalf("AzureName %q is the same for two Service Bus namespaces", got[0].Spec.AzureName)
	}
}

func TestBuildRoleAssignmentsErrors(t *testing.T) {
	t.Parallel()

	queue := testQueue(messagingv1alpha1.QueueSpec{})
	nsQueue := testNamespacesQueue(t, queue, testNamespaceResourceID)
	withRole := func(role messagingv1alpha1.QueueAccessRole) []Principal {
		return []Principal{{ID: testGroupID, Type: groupType, Role: role}}
	}

	tests := map[string]struct {
		nsQueue        *servicebusv1.NamespacesQueue
		principals     []Principal
		subscriptionID string
		wantErr        error
	}{
		"owner role":            {nsQueue, withRole(messagingv1alpha1.QueueAccessRoleOwner), testSubscriptionID, ErrRoleNotAllowed},
		"unknown role":          {nsQueue, withRole("admin"), testSubscriptionID, ErrRoleNotAllowed},
		"empty principal ID":    {nsQueue, []Principal{{Type: groupType, Role: messagingv1alpha1.QueueAccessRoleSender}}, testSubscriptionID, nil},
		"empty subscription ID": {nsQueue, []Principal{identitySender}, "", nil},
		"no NamespacesQueue":    {nil, []Principal{identitySender}, testSubscriptionID, nil},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := BuildRoleAssignments(queue, tt.nsQueue, tt.principals, tt.subscriptionID)
			if err == nil {
				t.Fatalf("expected an error, got %v", got)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error %v does not wrap %v", err, tt.wantErr)
			}
		})
	}
}

func TestPrincipalType(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		principal messagingv1alpha1.QueueAccessPrincipalSpec
		want      authorizationv1.RoleAssignmentProperties_PrincipalType
	}{
		"identity": {
			principal: messagingv1alpha1.QueueAccessPrincipalSpec{IdentityRef: &messagingv1alpha1.ApplicationIdentityRef{Name: "app-one"}},
			want:      servicePrincipalType,
		},
		"group": {
			principal: messagingv1alpha1.QueueAccessPrincipalSpec{Group: &messagingv1alpha1.QueueGroupPrincipalSpec{Name: "team-a", PrincipalId: testGroupID}},
			want:      groupType,
		},
		"service principal": {
			principal: messagingv1alpha1.QueueAccessPrincipalSpec{ServicePrincipal: &messagingv1alpha1.QueueServicePrincipalSpec{Name: "app-two", PrincipalId: testIdentityID}},
			want:      servicePrincipalType,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := PrincipalType(tt.principal); got != tt.want {
				t.Fatalf("PrincipalType = %q, want %q", got, tt.want)
			}
		})
	}
}
