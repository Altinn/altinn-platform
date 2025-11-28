package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
)

var _ = Describe("Database controller", func() {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancel()
	})

	It("allocates a subnet and writes it to status", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: storagev1alpha1.DatabaseAuth{
					AdminAppIdentity: "my-admin-app-identity",
					UserAppIdentity:  "my-app-identity",
				},
				Environment: "dev",
				Team:        "team-a",
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		// Wait for status to be updated
		var subnetCIDR string
		Eventually(func(g Gomega) string {
			var updated storagev1alpha1.Database
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "my-app-db",
				Namespace: "default",
			}, &updated)).To(Succeed())
			subnetCIDR = updated.Status.SubnetCIDR
			return subnetCIDR
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			ShouldNot(BeEmpty())

		// With the test SubnetCatalog we injected in the suite, the first free subnet is:
		Expect(subnetCIDR).To(Equal("10.100.1.0/28"))
	})

	It("allocates different /28 blocks for two databases", func() {
		db1 := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db1",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:     17,
				ServerType:  "dev",
				Environment: "dev",
				Team:        "team-a",
				Auth: storagev1alpha1.DatabaseAuth{
					AdminAppIdentity: "admin1",
					UserAppIdentity:  "user1",
				},
			},
		}
		db2 := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db2",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:     17,
				ServerType:  "dev",
				Environment: "dev",
				Team:        "team-a",
				Auth: storagev1alpha1.DatabaseAuth{
					AdminAppIdentity: "admin2",
					UserAppIdentity:  "user2",
				},
			},
		}

		Expect(k8sClient.Create(ctx, db1)).To(Succeed())
		Expect(k8sClient.Create(ctx, db2)).To(Succeed())

		var cidr1, cidr2 string

		Eventually(func() bool {
			var a, b storagev1alpha1.Database
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name: "db1", Namespace: "default",
			}, &a); err != nil {
				return false
			}
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name: "db2", Namespace: "default",
			}, &b); err != nil {
				return false
			}
			cidr1 = a.Status.SubnetCIDR
			cidr2 = b.Status.SubnetCIDR
			if cidr1 == "" || cidr2 == "" {
				return false
			}
			return cidr1 != cidr2
		}).WithTimeout(15 * time.Second).WithPolling(250 * time.Millisecond).
			Should(BeTrue())

		Expect(cidr1).NotTo(BeEmpty())
		Expect(cidr2).NotTo(BeEmpty())
		Expect(cidr1).NotTo(Equal(cidr2))
	})
})
