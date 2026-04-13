package controller

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/config"
	vaultpkg "github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/vault"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	keyvaultv1 "github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	asoconditions "github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	esov1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type noMatchSecretStoreClient struct {
	client.Client
}

func (c noMatchSecretStoreClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*esov1.SecretStore); ok {
		return &meta.NoKindMatchError{
			GroupKind: schema.GroupKind{Group: esov1.Group, Kind: esov1.SecretStoreKind},
		}
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

var _ = Describe("Vault controller", func() {
	var (
		testCtx context.Context
		cancel  context.CancelFunc
	)

	const ns = "default"

	cleanupNamespacedTestResources := func(ctx context.Context, namespace string) {
		deleteAll := func(obj client.Object) {
			Expect(k8sClient.DeleteAllOf(ctx, obj, client.InNamespace(namespace))).To(Succeed())
		}
		waitUntilEmpty := func(list client.ObjectList) {
			Eventually(func(g Gomega) int {
				g.Expect(k8sClient.List(ctx, list, client.InNamespace(namespace))).To(Succeed())
				switch typed := list.(type) {
				case *vaultv1alpha1.VaultList:
					return len(typed.Items)
				case *keyvaultv1.VaultList:
					return len(typed.Items)
				case *authorizationv1.RoleAssignmentList:
					return len(typed.Items)
				case *esov1.SecretStoreList:
					return len(typed.Items)
				case *identityv1alpha1.ApplicationIdentityList:
					return len(typed.Items)
				default:
					panic("unsupported list type in cleanup")
				}
			}).WithTimeout(10 * time.Second).WithPolling(200 * time.Millisecond).Should(Equal(0))
		}

		// Delete owners first to avoid controller re-creating dependents during cleanup.
		deleteAll(&vaultv1alpha1.Vault{})
		waitUntilEmpty(&vaultv1alpha1.VaultList{})

		deleteAll(&authorizationv1.RoleAssignment{})
		deleteAll(&keyvaultv1.Vault{})
		deleteAll(&esov1.SecretStore{})
		deleteAll(&identityv1alpha1.ApplicationIdentity{})

		waitUntilEmpty(&authorizationv1.RoleAssignmentList{})
		waitUntilEmpty(&keyvaultv1.VaultList{})
		waitUntilEmpty(&esov1.SecretStoreList{})
		waitUntilEmpty(&identityv1alpha1.ApplicationIdentityList{})
	}

	newVault := func(name, identityRef string) *vaultv1alpha1.Vault {
		return &vaultv1alpha1.Vault{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: vaultv1alpha1.VaultSpec{
				IdentityRef: vaultv1alpha1.ApplicationIdentityRef{Name: identityRef},
			},
		}
	}

	newVaultWithGroupObjectID := func(name, identityRef, groupObjectID string) *vaultv1alpha1.Vault {
		vaultObj := newVault(name, identityRef)
		vaultObj.Spec.GroupObjectID = groupObjectID
		return vaultObj
	}

	createIdentity := func(ctx context.Context, name string, ready bool) {
		appIdentity := &identityv1alpha1.ApplicationIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: identityv1alpha1.ApplicationIdentitySpec{},
		}
		Expect(k8sClient.Create(ctx, appIdentity)).To(Succeed())

		if ready {
			managedName := name + "-managed"
			principalID := name + "-principal"
			appIdentity.Status.ManagedIdentityName = &managedName
			appIdentity.Status.PrincipalID = &principalID
			meta.SetStatusCondition(&appIdentity.Status.Conditions, metav1.Condition{
				Type:   string(identityv1alpha1.ConditionReady),
				Status: metav1.ConditionTrue,
				Reason: "Ready",
			})
			Expect(k8sClient.Status().Update(ctx, appIdentity)).To(Succeed())
		}
	}

	setKeyVaultReadyStatus := func(ctx context.Context, name, resourceID, vaultURI string) {
		Eventually(func(g Gomega) bool {
			var keyVault keyvaultv1.Vault
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &keyVault)).To(Succeed())

			keyVault.Status.Id = &resourceID
			keyVault.Status.Properties = &keyvaultv1.VaultProperties_STATUS{
				VaultUri: &vaultURI,
			}
			keyVault.Status.Conditions = []asoconditions.Condition{{
				Type:               asoconditions.ConditionTypeReady,
				Status:             metav1.ConditionTrue,
				Severity:           asoconditions.ConditionSeverityNone,
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: keyVault.Generation,
				Reason:             "Ready",
				Message:            "Provisioned",
			}}

			if err := k8sClient.Status().Update(ctx, &keyVault); err != nil {
				if apierrors.IsConflict(err) {
					return false
				}
				g.Expect(err).NotTo(HaveOccurred())
			}
			return true
		}).WithTimeout(10 * time.Second).WithPolling(300 * time.Millisecond).Should(BeTrue())
	}

	setRoleAssignmentReadyStatus := func(ctx context.Context, name, resourceID string) {
		Eventually(func(g Gomega) bool {
			var roleAssignment authorizationv1.RoleAssignment
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &roleAssignment)).To(Succeed())

			roleAssignment.Status.Id = &resourceID
			roleAssignment.Status.Conditions = []asoconditions.Condition{{
				Type:               asoconditions.ConditionTypeReady,
				Status:             metav1.ConditionTrue,
				Severity:           asoconditions.ConditionSeverityNone,
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: roleAssignment.Generation,
				Reason:             "Ready",
				Message:            "Assigned",
			}}

			if err := k8sClient.Status().Update(ctx, &roleAssignment); err != nil {
				if apierrors.IsConflict(err) {
					return false
				}
				g.Expect(err).NotTo(HaveOccurred())
			}
			return true
		}).WithTimeout(10 * time.Second).WithPolling(300 * time.Millisecond).Should(BeTrue())
	}

	BeforeEach(func() {
		testCtx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		cleanupNamespacedTestResources(cleanupCtx, ns)
		cancel()
	})

	It("creates ASO Vault and RoleAssignment for a ready ApplicationIdentity", func() {
		createIdentity(testCtx, "my-app-identity", true)
		Expect(k8sClient.Create(testCtx, newVault("my-app-vault", "my-app-identity"))).To(Succeed())

		Eventually(func(g Gomega) int {
			var list keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &list, client.InNamespace(ns))).To(Succeed())
			return len(list.Items)
		}).WithTimeout(20*time.Second).WithPolling(500*time.Millisecond).
			Should(Equal(1), "expected controller to create one ASO Key Vault for Vault CR")

		Eventually(func(g Gomega) int {
			var list authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &list, client.InNamespace(ns))).To(Succeed())
			return len(list.Items)
		}).WithTimeout(20*time.Second).WithPolling(500*time.Millisecond).
			Should(Equal(1), "expected controller to create one owner RoleAssignment for Vault CR")
	})

	It("sets IdentityReady=False and blocks ASO resources when identity is not ready", func() {
		createIdentity(testCtx, "my-app-identity", false)
		Expect(k8sClient.Create(testCtx, newVault("my-app-vault-unready", "my-app-identity"))).To(Succeed())

		Eventually(func(g Gomega) metav1.ConditionStatus {
			var v vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: "my-app-vault-unready", Namespace: ns}, &v)).To(Succeed())
			cond := meta.FindStatusCondition(v.Status.Conditions, string(vaultv1alpha1.ConditionIdentityReady))
			if cond == nil {
				return metav1.ConditionUnknown
			}
			return cond.Status
		}).WithTimeout(20*time.Second).WithPolling(500*time.Millisecond).
			Should(Equal(metav1.ConditionFalse), "expected IdentityReady=False while ApplicationIdentity is not ready")

		Consistently(func(g Gomega) int {
			var list keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &list, client.InNamespace(ns))).To(Succeed())
			return len(list.Items)
		}, 3*time.Second, 500*time.Millisecond).Should(Equal(0))
	})

	It("reconciles dependent Vault after ApplicationIdentity becomes ready", func() {
		createIdentity(testCtx, "my-app-identity-update", false)
		Expect(k8sClient.Create(testCtx, newVault("my-app-vault-update", "my-app-identity-update"))).To(Succeed())

		Eventually(func(g Gomega) bool {
			var identity identityv1alpha1.ApplicationIdentity
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: "my-app-identity-update", Namespace: ns}, &identity)).To(Succeed())

			managedName := identity.Name + "-managed"
			principalID := identity.Name + "-principal"
			identity.Status.ManagedIdentityName = &managedName
			identity.Status.PrincipalID = &principalID
			meta.SetStatusCondition(&identity.Status.Conditions, metav1.Condition{
				Type:   string(identityv1alpha1.ConditionReady),
				Status: metav1.ConditionTrue,
				Reason: "Ready",
			})
			if err := k8sClient.Status().Update(testCtx, &identity); err != nil {
				if apierrors.IsConflict(err) {
					return false
				}
				g.Expect(err).NotTo(HaveOccurred())
			}
			return true
		}).WithTimeout(10 * time.Second).WithPolling(300 * time.Millisecond).Should(BeTrue())

		Eventually(func(g Gomega) int {
			var list keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &list, client.InNamespace(ns))).To(Succeed())
			return len(list.Items)
		}).WithTimeout(20*time.Second).WithPolling(500*time.Millisecond).
			Should(Equal(1), "expected identity update to enqueue and reconcile dependent Vault")
	})

	It("updates existing ASO Vault when Vault spec changes", func() {
		createIdentity(testCtx, "my-app-identity-propagation", true)

		initialPurgeProtection := false
		v := newVault("my-app-vault-propagation", "my-app-identity-propagation")
		v.Spec.SKU = vaultv1alpha1.VaultSKUStandard
		v.Spec.Tags = map[string]string{
			"team":   "apps",
			"remove": "old-value",
		}
		v.Spec.SoftDeleteRetentionDays = 7
		v.Spec.PurgeProtectionEnabled = &initialPurgeProtection
		Expect(k8sClient.Create(testCtx, v)).To(Succeed())

		var createdKeyVaultUID types.UID
		Eventually(func(g Gomega) {
			var list keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &list, client.InNamespace(ns))).To(Succeed())
			g.Expect(list.Items).To(HaveLen(1))

			keyVault := list.Items[0]
			createdKeyVaultUID = keyVault.UID
			g.Expect(createdKeyVaultUID).NotTo(BeEmpty())
			g.Expect(keyVault.Spec.Properties).NotTo(BeNil())
			g.Expect(keyVault.Spec.Properties.Sku).NotTo(BeNil())
			g.Expect(keyVault.Spec.Properties.Sku.Name).NotTo(BeNil())
			g.Expect(*keyVault.Spec.Properties.Sku.Name).To(Equal(keyvaultv1.Sku_Name_Standard))
			g.Expect(keyVault.Spec.Properties.SoftDeleteRetentionInDays).NotTo(BeNil())
			g.Expect(*keyVault.Spec.Properties.SoftDeleteRetentionInDays).To(Equal(7))
			g.Expect(keyVault.Spec.Properties.EnablePurgeProtection).NotTo(BeNil())
			g.Expect(*keyVault.Spec.Properties.EnablePurgeProtection).To(BeFalse())
			g.Expect(keyVault.Spec.Tags).To(Equal(map[string]string{
				"team":   "apps",
				"remove": "old-value",
			}))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Eventually(func(g Gomega) bool {
			var current vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: "my-app-vault-propagation", Namespace: ns}, &current)).To(Succeed())

			updatedPurgeProtection := true
			current.Spec.SKU = vaultv1alpha1.VaultSKUPremium
			current.Spec.Tags = map[string]string{
				"team": "platform",
				"env":  "prod",
			}
			current.Spec.SoftDeleteRetentionDays = 30
			current.Spec.PurgeProtectionEnabled = &updatedPurgeProtection

			if err := k8sClient.Update(testCtx, &current); err != nil {
				if apierrors.IsConflict(err) {
					return false
				}
				g.Expect(err).NotTo(HaveOccurred())
			}
			return true
		}).WithTimeout(10 * time.Second).WithPolling(300 * time.Millisecond).Should(BeTrue())

		Eventually(func(g Gomega) {
			var list keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &list, client.InNamespace(ns))).To(Succeed())
			g.Expect(list.Items).To(HaveLen(1))

			keyVault := list.Items[0]
			g.Expect(keyVault.UID).To(Equal(createdKeyVaultUID))
			g.Expect(keyVault.Spec.Properties).NotTo(BeNil())
			g.Expect(keyVault.Spec.Properties.Sku).NotTo(BeNil())
			g.Expect(keyVault.Spec.Properties.Sku.Name).NotTo(BeNil())
			g.Expect(*keyVault.Spec.Properties.Sku.Name).To(Equal(keyvaultv1.Sku_Name_Premium))
			g.Expect(keyVault.Spec.Properties.SoftDeleteRetentionInDays).NotTo(BeNil())
			g.Expect(*keyVault.Spec.Properties.SoftDeleteRetentionInDays).To(Equal(30))
			g.Expect(keyVault.Spec.Properties.EnablePurgeProtection).NotTo(BeNil())
			g.Expect(*keyVault.Spec.Properties.EnablePurgeProtection).To(BeTrue())
			g.Expect(keyVault.Spec.Tags).To(Equal(map[string]string{
				"team": "platform",
				"env":  "prod",
			}))
			g.Expect(keyVault.Spec.Tags).NotTo(HaveKey("remove"))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())
	})

	It("updates owner RoleAssignment when Vault identityRef changes", func() {
		const (
			identityA = "identity-owner-a"
			identityB = "identity-owner-b"
			vaultName = "my-app-vault-identity-switch"
		)

		createIdentity(testCtx, identityA, true)
		createIdentity(testCtx, identityB, true)
		Expect(k8sClient.Create(testCtx, newVault(vaultName, identityA))).To(Succeed())

		var roleAssignmentUID types.UID
		var initialRoleAssignmentAzureName string
		Eventually(func(g Gomega) {
			var list authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &list, client.InNamespace(ns))).To(Succeed())
			g.Expect(list.Items).To(HaveLen(1))

			roleAssignment := list.Items[0]
			roleAssignmentUID = roleAssignment.UID
			initialRoleAssignmentAzureName = roleAssignment.Spec.AzureName

			g.Expect(roleAssignmentUID).NotTo(BeEmpty())
			g.Expect(initialRoleAssignmentAzureName).NotTo(BeEmpty())
			g.Expect(roleAssignment.Spec.PrincipalId).NotTo(BeNil())
			g.Expect(*roleAssignment.Spec.PrincipalId).To(Equal(identityA + "-principal"))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Eventually(func(g Gomega) bool {
			var current vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &current)).To(Succeed())

			current.Spec.IdentityRef.Name = identityB
			if err := k8sClient.Update(testCtx, &current); err != nil {
				if apierrors.IsConflict(err) {
					return false
				}
				g.Expect(err).NotTo(HaveOccurred())
			}
			return true
		}).WithTimeout(10 * time.Second).WithPolling(300 * time.Millisecond).Should(BeTrue())

		expectedPrincipalID := identityB + "-principal"
		Eventually(func(g Gomega) {
			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			g.Expect(roleAssignments.Items).To(HaveLen(1))
			roleAssignment := roleAssignments.Items[0]

			g.Expect(roleAssignment.UID).To(Equal(roleAssignmentUID))
			g.Expect(roleAssignment.Spec.PrincipalId).NotTo(BeNil())
			g.Expect(*roleAssignment.Spec.PrincipalId).To(Equal(expectedPrincipalID))
			g.Expect(roleAssignment.Spec.AzureName).NotTo(Equal(initialRoleAssignmentAzureName))

			var currentVault vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &currentVault)).To(Succeed())

			var keyVaults keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &keyVaults, client.InNamespace(ns))).To(Succeed())
			g.Expect(keyVaults.Items).To(HaveLen(1))

			expectedRoleAssignment, err := vaultpkg.BuildOwnerRoleAssignmentResource(
				&currentVault,
				&keyVaults.Items[0],
				expectedPrincipalID,
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(roleAssignment.Spec.AzureName).To(Equal(expectedRoleAssignment.Spec.AzureName))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Consistently(func(g Gomega) int {
			var list authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &list, client.InNamespace(ns))).To(Succeed())
			return len(list.Items)
		}, 3*time.Second, 500*time.Millisecond).Should(Equal(1), "expected no duplicate owner RoleAssignments")
	})

	It("clears owner role assignment status when Vault identityRef changes to an unready identity", func() {
		const (
			identityReady   = "identity-owner-ready"
			identityPending = "identity-owner-pending"
			vaultName       = "my-app-vault-identity-pending"
		)

		createIdentity(testCtx, identityReady, true)
		createIdentity(testCtx, identityPending, false)
		Expect(k8sClient.Create(testCtx, newVault(vaultName, identityReady))).To(Succeed())

		var keyVaultName string
		var roleAssignmentName string
		Eventually(func(g Gomega) {
			var keyVaults keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &keyVaults, client.InNamespace(ns))).To(Succeed())
			g.Expect(keyVaults.Items).To(HaveLen(1))
			keyVaultName = keyVaults.Items[0].Name

			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			g.Expect(roleAssignments.Items).To(HaveLen(1))
			roleAssignmentName = roleAssignments.Items[0].Name
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		resourceID := "/subscriptions/sub-123/resourceGroups/rg-dis-dev/providers/Microsoft.KeyVault/vaults/" + vaultName
		vaultURI := "https://" + vaultName + ".vault.azure.net"
		roleAssignmentID := resourceID + "/providers/Microsoft.Authorization/roleAssignments/role-identity-ready"
		setKeyVaultReadyStatus(testCtx, keyVaultName, resourceID, vaultURI)
		setRoleAssignmentReadyStatus(testCtx, roleAssignmentName, roleAssignmentID)

		Eventually(func(g Gomega) {
			var current vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &current)).To(Succeed())
			g.Expect(current.Status.OwnerPrincipalID).To(Equal(identityReady + "-principal"))
			g.Expect(current.Status.OwnerRoleAssignmentID).To(Equal(roleAssignmentID))

			roleCondition := meta.FindStatusCondition(current.Status.Conditions, string(vaultv1alpha1.ConditionRoleAssignmentReady))
			g.Expect(roleCondition).NotTo(BeNil())
			g.Expect(roleCondition.Status).To(Equal(metav1.ConditionTrue))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Eventually(func(g Gomega) bool {
			var current vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &current)).To(Succeed())

			current.Spec.IdentityRef.Name = identityPending
			if err := k8sClient.Update(testCtx, &current); err != nil {
				if apierrors.IsConflict(err) {
					return false
				}
				g.Expect(err).NotTo(HaveOccurred())
			}
			return true
		}).WithTimeout(10 * time.Second).WithPolling(300 * time.Millisecond).Should(BeTrue())

		Eventually(func(g Gomega) {
			var current vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &current)).To(Succeed())

			identityCondition := meta.FindStatusCondition(current.Status.Conditions, string(vaultv1alpha1.ConditionIdentityReady))
			g.Expect(identityCondition).NotTo(BeNil())
			g.Expect(identityCondition.Status).To(Equal(metav1.ConditionFalse))

			roleCondition := meta.FindStatusCondition(current.Status.Conditions, string(vaultv1alpha1.ConditionRoleAssignmentReady))
			g.Expect(roleCondition).NotTo(BeNil())
			g.Expect(roleCondition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(roleCondition.Reason).To(Equal("IdentityNotReady"))

			g.Expect(current.Status.OwnerPrincipalID).To(BeEmpty())
			g.Expect(current.Status.OwnerRoleAssignmentID).To(BeEmpty())
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())
	})

	It("marks group role assignment ready as NotConfigured when none is specified", func() {
		const (
			identityName = "identity-no-groups"
			vaultName    = "my-app-vault-no-groups"
		)

		createIdentity(testCtx, identityName, true)
		Expect(k8sClient.Create(testCtx, newVault(vaultName, identityName))).To(Succeed())

		var keyVaultName string
		var roleAssignmentName string
		Eventually(func(g Gomega) {
			var keyVaults keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &keyVaults, client.InNamespace(ns))).To(Succeed())
			g.Expect(keyVaults.Items).To(HaveLen(1))
			keyVaultName = keyVaults.Items[0].Name

			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			g.Expect(roleAssignments.Items).To(HaveLen(1))
			roleAssignmentName = roleAssignments.Items[0].Name
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		resourceID := "/subscriptions/sub-123/resourceGroups/rg-dis-dev/providers/Microsoft.KeyVault/vaults/" + vaultName
		vaultURI := "https://" + vaultName + ".vault.azure.net"
		roleAssignmentID := resourceID + "/providers/Microsoft.Authorization/roleAssignments/role-none"
		setKeyVaultReadyStatus(testCtx, keyVaultName, resourceID, vaultURI)
		setRoleAssignmentReadyStatus(testCtx, roleAssignmentName, roleAssignmentID)

		Eventually(func(g Gomega) {
			var vaultObj vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &vaultObj)).To(Succeed())

			groupCondition := meta.FindStatusCondition(vaultObj.Status.Conditions, string(vaultv1alpha1.ConditionGroupRoleAssignment))
			g.Expect(groupCondition).NotTo(BeNil())
			g.Expect(groupCondition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(groupCondition.Reason).To(Equal("NotConfigured"))

			ready := meta.FindStatusCondition(vaultObj.Status.Conditions, string(vaultv1alpha1.ConditionReady))
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())
	})

	It("rejects group object IDs that are not canonical lowercase UUIDs", func() {
		const (
			identityName = "identity-invalid-group-id"
			vaultName    = "my-app-vault-invalid-group-id"
		)

		createIdentity(testCtx, identityName, true)
		err := k8sClient.Create(testCtx, newVaultWithGroupObjectID(
			vaultName,
			identityName,
			"AAAAAAAA-1111-1111-1111-111111111111",
		))
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
	})

	It("creates a single group role assignment", func() {
		const (
			identityName          = "identity-one-group"
			vaultName             = "my-app-vault-one-group"
			groupObjectID         = "11111111-1111-1111-1111-111111111111"
			expectedWellKnownRole = "Key Vault Secrets Officer"
		)

		createIdentity(testCtx, identityName, true)
		Expect(k8sClient.Create(testCtx, newVaultWithGroupObjectID(
			vaultName,
			identityName,
			groupObjectID,
		))).To(Succeed())

		var keyVaultName string
		Eventually(func(g Gomega) {
			var keyVaults keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &keyVaults, client.InNamespace(ns))).To(Succeed())
			g.Expect(keyVaults.Items).To(HaveLen(1))
			keyVaultName = keyVaults.Items[0].Name
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Eventually(func(g Gomega) {
			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			g.Expect(roleAssignments.Items).To(HaveLen(2))

			foundGroup := false
			for i := range roleAssignments.Items {
				assignment := roleAssignments.Items[i]
				if assignment.Spec.PrincipalId == nil || *assignment.Spec.PrincipalId != groupObjectID {
					continue
				}
				foundGroup = true
				g.Expect(assignment.Spec.PrincipalType).NotTo(BeNil())
				g.Expect(*assignment.Spec.PrincipalType).To(Equal(authorizationv1.RoleAssignmentProperties_PrincipalType_Group))
				g.Expect(assignment.Spec.RoleDefinitionReference).NotTo(BeNil())
				g.Expect(assignment.Spec.RoleDefinitionReference.WellKnownName).To(Equal(expectedWellKnownRole))
			}
			g.Expect(foundGroup).To(BeTrue(), "expected one group role assignment to be reconciled")
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		resourceID := "/subscriptions/sub-123/resourceGroups/rg-dis-dev/providers/Microsoft.KeyVault/vaults/" + vaultName
		vaultURI := "https://" + vaultName + ".vault.azure.net"
		Eventually(func(g Gomega) {
			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			for i := range roleAssignments.Items {
				assignment := roleAssignments.Items[i]
				setRoleAssignmentReadyStatus(
					testCtx,
					assignment.Name,
					resourceID+"/providers/Microsoft.Authorization/roleAssignments/"+assignment.Name,
				)
			}
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())
		setKeyVaultReadyStatus(testCtx, keyVaultName, resourceID, vaultURI)

		Eventually(func(g Gomega) {
			var vaultObj vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &vaultObj)).To(Succeed())

			groupCondition := meta.FindStatusCondition(vaultObj.Status.Conditions, string(vaultv1alpha1.ConditionGroupRoleAssignment))
			g.Expect(groupCondition).NotTo(BeNil())
			g.Expect(groupCondition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(groupCondition.Reason).To(Equal("Ready"))

			ready := meta.FindStatusCondition(vaultObj.Status.Conditions, string(vaultv1alpha1.ConditionReady))
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())
	})

	It("updates and removes the configured group role assignment", func() {
		const (
			identityName = "identity-group-switch"
			vaultName    = "my-app-vault-group-switch"
			groupOneID   = "11111111-1111-1111-1111-111111111111"
			groupTwoID   = "22222222-2222-2222-2222-222222222222"
		)

		createIdentity(testCtx, identityName, true)
		Expect(k8sClient.Create(testCtx, newVaultWithGroupObjectID(
			vaultName,
			identityName,
			groupOneID,
		))).To(Succeed())

		var keyVaultName string
		Eventually(func(g Gomega) {
			var keyVaults keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &keyVaults, client.InNamespace(ns))).To(Succeed())
			g.Expect(keyVaults.Items).To(HaveLen(1))
			keyVaultName = keyVaults.Items[0].Name
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		resourceID := "/subscriptions/sub-123/resourceGroups/rg-dis-dev/providers/Microsoft.KeyVault/vaults/" + vaultName
		vaultURI := "https://" + vaultName + ".vault.azure.net"
		Eventually(func(g Gomega) {
			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			g.Expect(roleAssignments.Items).To(HaveLen(2))
			for i := range roleAssignments.Items {
				setRoleAssignmentReadyStatus(
					testCtx,
					roleAssignments.Items[i].Name,
					resourceID+"/providers/Microsoft.Authorization/roleAssignments/"+roleAssignments.Items[i].Name,
				)
			}
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())
		setKeyVaultReadyStatus(testCtx, keyVaultName, resourceID, vaultURI)

		Eventually(func(g Gomega) {
			var vaultObj vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &vaultObj)).To(Succeed())
			groupCondition := meta.FindStatusCondition(vaultObj.Status.Conditions, string(vaultv1alpha1.ConditionGroupRoleAssignment))
			g.Expect(groupCondition).NotTo(BeNil())
			g.Expect(groupCondition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(groupCondition.Reason).To(Equal("Ready"))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Eventually(func(g Gomega) bool {
			var current vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &current)).To(Succeed())
			current.Spec.GroupObjectID = groupTwoID
			if err := k8sClient.Update(testCtx, &current); err != nil {
				if apierrors.IsConflict(err) {
					return false
				}
				g.Expect(err).NotTo(HaveOccurred())
			}
			return true
		}).WithTimeout(10 * time.Second).WithPolling(300 * time.Millisecond).Should(BeTrue())

		Eventually(func(g Gomega) {
			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			g.Expect(roleAssignments.Items).To(HaveLen(2))

			principalIDs := make([]string, 0, len(roleAssignments.Items))
			for i := range roleAssignments.Items {
				if roleAssignments.Items[i].Spec.PrincipalId != nil {
					principalIDs = append(principalIDs, *roleAssignments.Items[i].Spec.PrincipalId)
				}
			}
			g.Expect(principalIDs).NotTo(ContainElement(groupOneID))
			g.Expect(principalIDs).To(ContainElement(groupTwoID))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Eventually(func(g Gomega) {
			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			for i := range roleAssignments.Items {
				setRoleAssignmentReadyStatus(
					testCtx,
					roleAssignments.Items[i].Name,
					resourceID+"/providers/Microsoft.Authorization/roleAssignments/"+roleAssignments.Items[i].Name,
				)
			}
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Eventually(func(g Gomega) bool {
			var current vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &current)).To(Succeed())
			current.Spec.GroupObjectID = ""
			if err := k8sClient.Update(testCtx, &current); err != nil {
				if apierrors.IsConflict(err) {
					return false
				}
				g.Expect(err).NotTo(HaveOccurred())
			}
			return true
		}).WithTimeout(10 * time.Second).WithPolling(300 * time.Millisecond).Should(BeTrue())

		Eventually(func(g Gomega) {
			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			g.Expect(roleAssignments.Items).To(HaveLen(1))
			g.Expect(roleAssignments.Items[0].Spec.PrincipalId).NotTo(BeNil())
			g.Expect(*roleAssignments.Items[0].Spec.PrincipalId).To(Equal(identityName + "-principal"))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Eventually(func(g Gomega) {
			var vaultObj vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &vaultObj)).To(Succeed())
			groupCondition := meta.FindStatusCondition(vaultObj.Status.Conditions, string(vaultv1alpha1.ConditionGroupRoleAssignment))
			g.Expect(groupCondition).NotTo(BeNil())
			g.Expect(groupCondition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(groupCondition.Reason).To(Equal("NotConfigured"))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())
	})

	It("recreates owned ASO resources when children are deleted", func() {
		const (
			identityName = "identity-drift-healing"
			vaultName    = "my-app-vault-drift-healing"
		)

		createIdentity(testCtx, identityName, true)

		purgeProtectionEnabled := false
		vaultObj := newVault(vaultName, identityName)
		vaultObj.Spec.SKU = vaultv1alpha1.VaultSKUPremium
		vaultObj.Spec.Tags = map[string]string{
			"team": "platform",
		}
		vaultObj.Spec.SoftDeleteRetentionDays = 14
		vaultObj.Spec.PurgeProtectionEnabled = &purgeProtectionEnabled
		Expect(k8sClient.Create(testCtx, vaultObj)).To(Succeed())

		var initialKeyVault keyvaultv1.Vault
		var initialRoleAssignment authorizationv1.RoleAssignment
		Eventually(func(g Gomega) {
			var keyVaults keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &keyVaults, client.InNamespace(ns))).To(Succeed())
			g.Expect(keyVaults.Items).To(HaveLen(1))
			initialKeyVault = keyVaults.Items[0]
			g.Expect(initialKeyVault.UID).NotTo(BeEmpty())

			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			g.Expect(roleAssignments.Items).To(HaveLen(1))
			initialRoleAssignment = roleAssignments.Items[0]
			g.Expect(initialRoleAssignment.UID).NotTo(BeEmpty())
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Expect(k8sClient.Delete(testCtx, &initialRoleAssignment)).To(Succeed())
		Expect(k8sClient.Delete(testCtx, &initialKeyVault)).To(Succeed())

		hasVaultControllerOwnerRef := func(ownerRefs []metav1.OwnerReference, owner *vaultv1alpha1.Vault) bool {
			for _, ref := range ownerRefs {
				if ref.APIVersion != vaultv1alpha1.GroupVersion.String() {
					continue
				}
				if ref.Kind != "Vault" || ref.Name != owner.Name || ref.UID != owner.UID {
					continue
				}
				if ref.Controller != nil && *ref.Controller {
					return true
				}
			}
			return false
		}

		Eventually(func(g Gomega) {
			var currentVault vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &currentVault)).To(Succeed())

			var keyVaults keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &keyVaults, client.InNamespace(ns))).To(Succeed())
			g.Expect(keyVaults.Items).To(HaveLen(1))
			recreatedKeyVault := keyVaults.Items[0]
			g.Expect(recreatedKeyVault.UID).NotTo(Equal(initialKeyVault.UID))
			g.Expect(hasVaultControllerOwnerRef(recreatedKeyVault.OwnerReferences, &currentVault)).To(BeTrue())

			expectedAzureName := vaultpkg.DeterministicAzureVaultName(currentVault.Namespace, currentVault.Name, "dev")
			expectedKeyVault, err := vaultpkg.BuildASOKeyVaultResource(
				&currentVault,
				config.OperatorConfig{
					SubscriptionID: "sub-123",
					ResourceGroup:  "rg-dis-dev",
					TenantID:       "00000000-0000-0000-0000-000000000000",
					Location:       "westeurope",
					Environment:    "dev",
					AKSSubnetIDs: []string{
						"/subscriptions/sub-123/resourceGroups/rg-net/providers/Microsoft.Network/virtualNetworks/vnet/subnets/aks-1",
					},
				},
				expectedAzureName,
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(recreatedKeyVault.Spec).To(Equal(expectedKeyVault.Spec))

			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			g.Expect(roleAssignments.Items).To(HaveLen(1))
			recreatedRoleAssignment := roleAssignments.Items[0]
			g.Expect(recreatedRoleAssignment.UID).NotTo(Equal(initialRoleAssignment.UID))
			g.Expect(hasVaultControllerOwnerRef(recreatedRoleAssignment.OwnerReferences, &currentVault)).To(BeTrue())

			expectedRoleAssignment, err := vaultpkg.BuildOwnerRoleAssignmentResource(
				&currentVault,
				&recreatedKeyVault,
				identityName+"-principal",
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(recreatedRoleAssignment.Spec).To(Equal(expectedRoleAssignment.Spec))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())
	})

	It("projects dependent resource readiness and identifiers onto Vault status", func() {
		createIdentity(testCtx, "my-app-identity-status", true)
		Expect(k8sClient.Create(testCtx, newVault("my-app-vault-status", "my-app-identity-status"))).To(Succeed())

		var keyVaultName string
		var roleAssignmentName string
		Eventually(func(g Gomega) {
			var keyVaults keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &keyVaults, client.InNamespace(ns))).To(Succeed())
			g.Expect(keyVaults.Items).To(HaveLen(1))
			keyVaultName = keyVaults.Items[0].Name

			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			g.Expect(roleAssignments.Items).To(HaveLen(1))
			roleAssignmentName = roleAssignments.Items[0].Name
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		resourceID := "/subscriptions/sub-123/resourceGroups/rg-dis-dev/providers/Microsoft.KeyVault/vaults/my-app-vault-status"
		vaultURI := "https://my-app-vault-status.vault.azure.net"
		roleAssignmentID := resourceID + "/providers/Microsoft.Authorization/roleAssignments/role-123"
		setKeyVaultReadyStatus(testCtx, keyVaultName, resourceID, vaultURI)
		setRoleAssignmentReadyStatus(testCtx, roleAssignmentName, roleAssignmentID)

		Eventually(func(g Gomega) {
			var vaultObj vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: "my-app-vault-status", Namespace: ns}, &vaultObj)).To(Succeed())

			g.Expect(meta.FindStatusCondition(vaultObj.Status.Conditions, string(vaultv1alpha1.ConditionVaultReady))).NotTo(BeNil())
			g.Expect(meta.FindStatusCondition(vaultObj.Status.Conditions, string(vaultv1alpha1.ConditionRoleAssignmentReady))).NotTo(BeNil())
			g.Expect(meta.FindStatusCondition(vaultObj.Status.Conditions, string(vaultv1alpha1.ConditionReady))).NotTo(BeNil())
			g.Expect(vaultObj.Status.ResourceID).To(Equal(resourceID))
			g.Expect(vaultObj.Status.VaultURI).To(Equal(vaultURI))
			g.Expect(vaultObj.Status.OwnerRoleAssignmentID).To(Equal(roleAssignmentID))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())
	})

	It("manages a SecretStore lifecycle when external secrets integration is enabled", func() {
		const (
			identityName = "my-app-identity-secretstore"
			vaultName    = "my-app-vault-secretstore"
		)

		createIdentity(testCtx, identityName, true)
		vaultObj := newVault(vaultName, identityName)
		vaultObj.Spec.ExternalSecrets = true
		Expect(k8sClient.Create(testCtx, vaultObj)).To(Succeed())

		var keyVaultName string
		var roleAssignmentName string
		Eventually(func(g Gomega) {
			var keyVaults keyvaultv1.VaultList
			g.Expect(k8sClient.List(testCtx, &keyVaults, client.InNamespace(ns))).To(Succeed())
			g.Expect(keyVaults.Items).To(HaveLen(1))
			keyVaultName = keyVaults.Items[0].Name

			var roleAssignments authorizationv1.RoleAssignmentList
			g.Expect(k8sClient.List(testCtx, &roleAssignments, client.InNamespace(ns))).To(Succeed())
			g.Expect(roleAssignments.Items).To(HaveLen(1))
			roleAssignmentName = roleAssignments.Items[0].Name
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Consistently(func(g Gomega) int {
			var stores esov1.SecretStoreList
			g.Expect(k8sClient.List(testCtx, &stores, client.InNamespace(ns))).To(Succeed())
			return len(stores.Items)
		}, 2*time.Second, 300*time.Millisecond).Should(Equal(0))

		resourceID := "/subscriptions/sub-123/resourceGroups/rg-dis-dev/providers/Microsoft.KeyVault/vaults/" + vaultName
		vaultURI := "https://" + vaultName + ".vault.azure.net"
		roleAssignmentID := resourceID + "/providers/Microsoft.Authorization/roleAssignments/role-123"
		setKeyVaultReadyStatus(testCtx, keyVaultName, resourceID, vaultURI)
		setRoleAssignmentReadyStatus(testCtx, roleAssignmentName, roleAssignmentID)

		expectedStoreName := vaultpkg.DeterministicSecretStoreName(vaultName)
		Eventually(func(g Gomega) {
			var store esov1.SecretStore
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: expectedStoreName, Namespace: ns}, &store)).To(Succeed())
			g.Expect(store.Labels).To(HaveKeyWithValue("vault.dis.altinn.cloud/name", vaultName))
			g.Expect(store.Spec.Provider).NotTo(BeNil())
			g.Expect(store.Spec.Provider.AzureKV).NotTo(BeNil())
			g.Expect(store.Spec.Provider.AzureKV.AuthType).NotTo(BeNil())
			g.Expect(*store.Spec.Provider.AzureKV.AuthType).To(Equal(esov1.AzureWorkloadIdentity))
			g.Expect(store.Spec.Provider.AzureKV.VaultURL).NotTo(BeNil())
			g.Expect(*store.Spec.Provider.AzureKV.VaultURL).To(Equal(vaultURI))
			g.Expect(store.Spec.Provider.AzureKV.TenantID).NotTo(BeNil())
			g.Expect(*store.Spec.Provider.AzureKV.TenantID).To(Equal("00000000-0000-0000-0000-000000000000"))
			g.Expect(store.Spec.Provider.AzureKV.ServiceAccountRef).NotTo(BeNil())
			g.Expect(store.Spec.Provider.AzureKV.ServiceAccountRef.Name).To(Equal(identityName))
			g.Expect(metav1.IsControlledBy(&store, vaultObj)).To(BeTrue())
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Eventually(func(g Gomega) {
			var current vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &current)).To(Succeed())

			external := meta.FindStatusCondition(current.Status.Conditions, string(vaultv1alpha1.ConditionExternalSecretsReady))
			g.Expect(external).NotTo(BeNil())
			g.Expect(external.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(external.Reason).To(Equal("Ready"))
			g.Expect(current.Status.ExternalSecretStoreName).To(Equal(expectedStoreName))

			ready := meta.FindStatusCondition(current.Status.Conditions, string(vaultv1alpha1.ConditionReady))
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Eventually(func(g Gomega) bool {
			var current vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &current)).To(Succeed())
			current.Spec.ExternalSecrets = false
			if err := k8sClient.Update(testCtx, &current); err != nil {
				if apierrors.IsConflict(err) {
					return false
				}
				g.Expect(err).NotTo(HaveOccurred())
			}
			return true
		}).WithTimeout(10 * time.Second).WithPolling(300 * time.Millisecond).Should(BeTrue())

		Eventually(func() bool {
			var store esov1.SecretStore
			err := k8sClient.Get(testCtx, types.NamespacedName{Name: expectedStoreName, Namespace: ns}, &store)
			return apierrors.IsNotFound(err)
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(BeTrue())

		Eventually(func(g Gomega) {
			var current vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &current)).To(Succeed())

			external := meta.FindStatusCondition(current.Status.Conditions, string(vaultv1alpha1.ConditionExternalSecretsReady))
			g.Expect(external).NotTo(BeNil())
			g.Expect(external.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(external.Reason).To(Equal("Disabled"))
			g.Expect(current.Status.ExternalSecretStoreName).To(BeEmpty())

			ready := meta.FindStatusCondition(current.Status.Conditions, string(vaultv1alpha1.ConditionReady))
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())
	})

	It("surfaces SecretStore name conflicts without overwriting non-owned resources", func() {
		const (
			identityName = "my-app-identity-conflict"
			vaultName    = "my-app-vault-conflict"
		)

		createIdentity(testCtx, identityName, true)

		conflictOwner := newVault("other-vault", identityName)
		conflictStore, err := vaultpkg.BuildManagedSecretStore(conflictOwner, "00000000-0000-0000-0000-000000000000", "https://other-vault.vault.azure.net")
		Expect(err).NotTo(HaveOccurred())
		conflictStore.Name = vaultpkg.DeterministicSecretStoreName(vaultName)
		conflictStore.Namespace = ns
		conflictStore.OwnerReferences = nil
		Expect(k8sClient.Create(testCtx, conflictStore)).To(Succeed())

		vaultObj := newVault(vaultName, identityName)
		vaultObj.Spec.ExternalSecrets = true
		Expect(k8sClient.Create(testCtx, vaultObj)).To(Succeed())

		Eventually(func(g Gomega) {
			var current vaultv1alpha1.Vault
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: vaultName, Namespace: ns}, &current)).To(Succeed())

			external := meta.FindStatusCondition(current.Status.Conditions, string(vaultv1alpha1.ConditionExternalSecretsReady))
			g.Expect(external).NotTo(BeNil())
			g.Expect(external.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(external.Reason).To(Equal("NameConflict"))
			g.Expect(current.Status.ExternalSecretStoreName).To(BeEmpty())
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

		Consistently(func(g Gomega) {
			var store esov1.SecretStore
			g.Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: conflictStore.Name, Namespace: ns}, &store)).To(Succeed())
			g.Expect(store.OwnerReferences).To(BeEmpty())
		}, 3*time.Second, 300*time.Millisecond).Should(Succeed())
	})
})

func TestReconcileManagedSecretStoreReturnsCRDNotInstalledWhenSecretStoreCRDIsMissing(t *testing.T) {
	t.Parallel()

	scheme := newControllerUnitTestScheme(t)
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &VaultReconciler{
		Client: noMatchSecretStoreClient{Client: baseClient},
		Scheme: scheme,
		Config: config.OperatorConfig{TenantID: "00000000-0000-0000-0000-000000000000"},
	}

	vaultObj := &vaultv1alpha1.Vault{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "vault-sample",
			Namespace:  "default",
			Generation: 7,
		},
		Spec: vaultv1alpha1.VaultSpec{
			IdentityRef:     vaultv1alpha1.ApplicationIdentityRef{Name: "app-identity-sample"},
			ExternalSecrets: true,
		},
	}

	result, err := reconciler.reconcileManagedSecretStore(context.Background(), vaultObj, nil)
	if err != nil {
		t.Fatalf("expected missing SecretStore CRD to surface as status, got error: %v", err)
	}
	if result.Name != "" {
		t.Fatalf("expected no managed SecretStore name, got %q", result.Name)
	}
	if result.Condition.Type != string(vaultv1alpha1.ConditionExternalSecretsReady) {
		t.Fatalf("expected ExternalSecretsReady condition, got %q", result.Condition.Type)
	}
	if result.Condition.Status != metav1.ConditionFalse || result.Condition.Reason != "CRDNotInstalled" {
		t.Fatalf("expected ExternalSecretsReady=False/CRDNotInstalled, got %s/%s", result.Condition.Status, result.Condition.Reason)
	}
}

func TestBuildNetworkPolicyCondition(t *testing.T) {
	t.Parallel()

	cfg := config.OperatorConfig{
		SubscriptionID: "sub-123",
		ResourceGroup:  "rg-dis-dev",
		Location:       "westeurope",
		Environment:    "dev",
		AKSSubnetIDs: []string{
			"/subscriptions/sub-123/resourceGroups/rg-net/providers/Microsoft.Network/virtualNetworks/vnet/subnets/aks-1",
		},
	}

	vaultObj := &vaultv1alpha1.Vault{
		ObjectMeta: metav1.ObjectMeta{Name: "vault-sample", Namespace: "default"},
		Spec: vaultv1alpha1.VaultSpec{
			IdentityRef: vaultv1alpha1.ApplicationIdentityRef{Name: "app-identity-sample"},
		},
	}

	desired, err := vaultpkg.BuildASOKeyVaultResource(vaultObj, cfg, "vault-sample-akv")
	if err != nil {
		t.Fatalf("expected key vault builder to succeed, got error: %v", err)
	}

	ready := buildNetworkPolicyCondition(3, desired, cfg)
	if ready.Status != metav1.ConditionTrue || ready.Reason != "Ready" {
		t.Fatalf("expected network policy condition to be ready, got %s/%s", ready.Status, ready.Reason)
	}

	mismatched := buildNetworkPolicyCondition(3, desired, config.OperatorConfig{
		AKSSubnetIDs: []string{
			"/subscriptions/sub-123/resourceGroups/rg-net/providers/Microsoft.Network/virtualNetworks/vnet/subnets/aks-1",
			"/subscriptions/sub-123/resourceGroups/rg-net/providers/Microsoft.Network/virtualNetworks/vnet/subnets/aks-2",
		},
	})
	if mismatched.Status != metav1.ConditionFalse || mismatched.Reason != "InvalidPolicy" {
		t.Fatalf("expected network policy mismatch to be InvalidPolicy, got %s/%s", mismatched.Status, mismatched.Reason)
	}
}

func newControllerUnitTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := vaultv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add Vault scheme: %v", err)
	}
	if err := esov1.AddToScheme(scheme); err != nil {
		t.Fatalf("add SecretStore scheme: %v", err)
	}
	if err := keyvaultv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add Key Vault scheme: %v", err)
	}
	return scheme
}
