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
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

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
				Version:    17,
				ServerType: "dev",
				Auth: storagev1alpha1.DatabaseAuth{
					AdminAppIdentity: "admin1",
					UserAppIdentity:  "user1",
				},
			},
		}
		Expect(k8sClient.Create(ctx, db1)).To(Succeed())

		// Wait until db1 has a subnet assigned
		var cidr1 string
		Eventually(func(g Gomega) string {
			var updated storagev1alpha1.Database
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "db1",
				Namespace: "default",
			}, &updated)).To(Succeed())
			cidr1 = updated.Status.SubnetCIDR
			return cidr1
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			ShouldNot(BeEmpty())

		db2 := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db2",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: storagev1alpha1.DatabaseAuth{
					AdminAppIdentity: "admin2",
					UserAppIdentity:  "user2",
				},
			},
		}
		Expect(k8sClient.Create(ctx, db2)).To(Succeed())

		// Wait until db2 has a subnet assigned
		var cidr2 string
		Eventually(func(g Gomega) string {
			var updated storagev1alpha1.Database
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "db2",
				Namespace: "default",
			}, &updated)).To(Succeed())
			cidr2 = updated.Status.SubnetCIDR
			return cidr2
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			ShouldNot(BeEmpty())

		Expect(cidr1).NotTo(BeEmpty())
		Expect(cidr2).NotTo(BeEmpty())
		Expect(cidr1).NotTo(Equal(cidr2))
	})
})
