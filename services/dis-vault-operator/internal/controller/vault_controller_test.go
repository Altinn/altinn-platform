package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/config"
	vaultpkg "github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/vault"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	keyvaultv1 "github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
		deleteAll(&identityv1alpha1.ApplicationIdentity{})

		waitUntilEmpty(&authorizationv1.RoleAssignmentList{})
		waitUntilEmpty(&keyvaultv1.VaultList{})
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
					TenantID:       "tenant-123",
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
})
