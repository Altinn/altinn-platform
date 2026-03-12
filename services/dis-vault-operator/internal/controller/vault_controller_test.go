package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
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
})
