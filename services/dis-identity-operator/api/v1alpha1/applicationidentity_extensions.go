package v1alpha1

import (
	"fmt"
	"reflect"

	managedidentity "github.com/Azure/azure-service-operator/v2/api/managedidentity/v1api20230131"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Altinn/altinn-platform/services/dis-identity-operator/internal/utils"
)

const managedByDisIdentityTag = "managed-by:dis-identity-operator"

// GenerateUserAssignedIdentity generates a managedidentity.UserAssignedIdentity object based on the ApplicationIdentity instance.
func (a *ApplicationIdentity) GenerateUserAssignedIdentity(ownerARMID string) *managedidentity.UserAssignedIdentity {
	// Create a new UserAssignedIdentity object
	identity := &managedidentity.UserAssignedIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
		Spec: managedidentity.UserAssignedIdentity_Spec{
			AzureName: fmt.Sprintf("%s-%s", a.Namespace, a.Name),
			Location:  utils.ToPointer("norwayeast"),
			Owner: &genruntime.KnownResourceReference{
				ARMID: ownerARMID,
			},
			Tags: a.GetUserAssignedIdentityTags(),
		},
	}
	return identity
}

func (a *ApplicationIdentity) GetUserAssignedIdentityTags() map[string]string {
	result := make(map[string]string)
	for k, v := range a.Spec.Tags {
		result[k] = v
	}
	// Add the managed-by tag to the tags map
	result[managedByDisIdentityTag] = "true"
	return result
}

// GenerateFederatedCredentials generates a managedidentity.FederatedIdentityCredential object based on the ApplicationIdentity instance.
func (a *ApplicationIdentity) GenerateFederatedCredentials(issuer string) *managedidentity.FederatedIdentityCredential {
	subject := fmt.Sprintf("system:serviceaccount:%s:%s", a.Namespace, a.Name)
	// Create a new FederatedIdentityCredential object
	credential := &managedidentity.FederatedIdentityCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
		Spec: managedidentity.FederatedIdentityCredential_Spec{
			Audiences: a.Spec.AzureAudiences,
			AzureName: fmt.Sprintf("%s-%s", a.Namespace, a.Name),
			Issuer:    &issuer,
			Owner: &genruntime.KnownResourceReference{
				Name: a.Name,
			},
			Subject: &subject,
		},
	}

	return credential
}

func (a *ApplicationIdentity) ReplaceCondition(conditionType ConditionType, condition metav1.Condition) {
	for i, c := range a.Status.Conditions {
		if c.Type == string(conditionType) {
			a.Status.Conditions[i] = condition
			return
		}
	}
	a.Status.Conditions = append(a.Status.Conditions, condition)
}

func (a *ApplicationIdentity) OutdatedUserAssignedIdentity(identity *managedidentity.UserAssignedIdentity) bool {
	if identity == nil {
		return true
	}
	expectedTags := a.GetUserAssignedIdentityTags()
	return !reflect.DeepEqual(expectedTags, identity.Spec.Tags)
}

func (a *ApplicationIdentity) OutdatedFederatedCredentials(credential *managedidentity.FederatedIdentityCredential) bool {
	if credential == nil {
		return true
	}
	expectedAudiences := a.Spec.AzureAudiences
	return !reflect.DeepEqual(expectedAudiences, credential.Spec.Audiences)
}

func (a *ApplicationIdentity) OutdatedServiceAccount(serviceAccount *corev1.ServiceAccount) bool {
	if serviceAccount == nil {
		return true
	}
	id, ok := serviceAccount.Annotations["serviceaccount.azure.com/azure-identity"]
	if !ok && a.Status.ClientID != nil {
		return true
	}
	if a.Status.ClientID != nil && *a.Status.ClientID != id {
		return true
	}
	return false
}
