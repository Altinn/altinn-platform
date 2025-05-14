package v1alpha1

import (
	"fmt"

	managedidentity "github.com/Azure/azure-service-operator/v2/api/managedidentity/v1api20230131"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Altinn/altinn-platform/services/dis-identity-operator/internal/utils"
)

const managedByDisIdentityTag = "managed-by:dis-identity-operator"

// GenerateUserAssignedIdentity generates a managedidentity.UserAssignedIdentity object based on the ApplicationIdentity instance.
func (a *ApplicationIdentity) GenerateUserAssignedIdentity(ownerARMID string) *managedidentity.UserAssignedIdentity {
	// Create a new UserAssignedIdentity object
	identity := &managedidentity.UserAssignedIdentity{
		TypeMeta: metav1.TypeMeta{
			Kind:       "UserAssignedIdentity",
			APIVersion: "managedidentity.azure.com/v1api20181130",
		},
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
			Tags: a.Spec.Tags,
		},
	}
	// Add the common tags to the identity
	identity.Spec.Tags[managedByDisIdentityTag] = "true"
	return identity
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
