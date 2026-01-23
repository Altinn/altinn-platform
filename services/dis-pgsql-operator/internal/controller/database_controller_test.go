package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	networkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20240601"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Database controller", func() {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	const ns = "default"

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancel()
	})

	It("allocates a subnet and writes it to status", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-subnet",
				Namespace: ns,
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
				Name:      "my-app-db-subnet",
				Namespace: ns,
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
				Name:      "db1-subnet",
				Namespace: ns,
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
				Name:      "db1-subnet",
				Namespace: ns,
			}, &updated)).To(Succeed())
			cidr1 = updated.Status.SubnetCIDR
			return cidr1
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			ShouldNot(BeEmpty())

		Eventually(func(g Gomega) bool {
			var list storagev1alpha1.DatabaseList
			g.Expect(k8sClient.List(ctx, &list)).To(Succeed())
			for _, item := range list.Items {
				if item.Name == db1.Name && item.Status.SubnetCIDR == cidr1 {
					return true
				}
			}
			return false
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).Should(BeTrue())

		db2 := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db2-subnet",
				Namespace: ns,
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
				Name:      "db2-subnet",
				Namespace: ns,
			}, &updated)).To(Succeed())
			cidr2 = updated.Status.SubnetCIDR
			return cidr2
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			ShouldNot(BeEmpty())

		Expect(cidr1).NotTo(BeEmpty())
		Expect(cidr2).NotTo(BeEmpty())
		Expect(cidr1).NotTo(Equal(cidr2))
	})

	// Database Private DNS Zone integration tests
	It("creates a Private DNS zone per Database", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-dns",
				Namespace: ns,
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: storagev1alpha1.DatabaseAuth{
					AdminAppIdentity: "admin-mi",
					UserAppIdentity:  "user-mi",
				},
			},
		}

		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		expectedZoneName := fmt.Sprintf("%s.private.postgres.database.azure.com", db.Name)

		Eventually(func(g Gomega) error {
			var zone networkv1.PrivateDnsZone
			key := types.NamespacedName{
				Name:      expectedZoneName,
				Namespace: ns,
			}
			err := k8sClient.Get(ctx, key, &zone)
			return err
		}).WithTimeout(20*time.Second).WithPolling(500*time.Millisecond).
			Should(Succeed(), "expected Private DNS zone for Database to be created by controller")

		// Inspect metadata of created Private DNS zone
		var zone networkv1.PrivateDnsZone
		key := types.NamespacedName{
			Name:      expectedZoneName,
			Namespace: ns,
		}
		Expect(k8sClient.Get(ctx, key, &zone)).To(Succeed())

		// Miminal expectations about the created zone
		Expect(zone.Name).To(Equal(expectedZoneName))
		Expect(zone.Namespace).To(Equal(ns))
	})

	It("creates Private DNS zone virtual network links for DB and AKS VNets", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: storagev1alpha1.DatabaseAuth{
					AdminAppIdentity: "admin-mi",
					UserAppIdentity:  "user-mi",
				},
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		zoneName := fmt.Sprintf("%s.private.postgres.database.azure.com", db.Name)

		// Wait for the zone
		Eventually(func(g Gomega) error {
			var zone networkv1.PrivateDnsZone
			key := types.NamespacedName{
				Name:      zoneName,
				Namespace: ns,
			}
			return k8sClient.Get(ctx, key, &zone)
		}).WithTimeout(20*time.Second).WithPolling(500*time.Millisecond).
			Should(Succeed(), "expected Private DNS zone for Database to be created")

		// Expect two VNet links
		expectedDBLinkName := vnetLinkNameForDB(db)
		expectedAKSLinkName := vnetLinkNameForAKS(db)

		Eventually(func(g Gomega) []string {
			var list networkv1.PrivateDnsZonesVirtualNetworkLinkList
			g.Expect(k8sClient.List(ctx, &list, client.InNamespace(ns))).To(Succeed())

			found := make(map[string]struct{})
			for _, link := range list.Items {
				if link.Name == expectedDBLinkName || link.Name == expectedAKSLinkName {
					found[link.Name] = struct{}{}
				}
			}

			out := make([]string, 0, len(found))
			for n := range found {
				out = append(out, n)
			}
			return out
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).
			Should(ConsistOf(expectedDBLinkName, expectedAKSLinkName))

		// Check one link object exists
		var dbLink networkv1.PrivateDnsZonesVirtualNetworkLink
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      expectedDBLinkName,
			Namespace: ns,
		}, &dbLink)).To(Succeed())
		Expect(dbLink.Namespace).To(Equal(ns))
	})

	// Database testing
	It("creates a FlexibleServer for the Database", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-psql",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: storagev1alpha1.DatabaseAuth{
					AdminAppIdentity: "admin-mi",
					UserAppIdentity:  "user-mi",
				},
			},
		}

		Expect(k8sClient.Create(ctx, db)).To(Succeed())
		expectedServerName := db.Name

		Eventually(func(g Gomega) error {
			var s dbforpostgresqlv1.FlexibleServer
			return k8sClient.Get(ctx, types.NamespacedName{
				Name:      expectedServerName,
				Namespace: db.Namespace,
			}, &s)
		}).WithTimeout(30*time.Second).WithPolling(500*time.Millisecond).
			Should(Succeed(), "expected FlexibleServer ASO resource to be created for Database")

		var s dbforpostgresqlv1.FlexibleServer
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      expectedServerName,
			Namespace: db.Namespace,
		}, &s)).To(Succeed())

		Expect(s.Name).To(Equal(expectedServerName))
		Expect(s.Namespace).To(Equal(db.Namespace))
		Expect(s.Labels["dis.altinn.cloud/database-name"]).To(Equal(db.Name))

		// Owner should be set and should use ARMID
		Expect(s.Spec.Owner).NotTo(BeNil())
		Expect(s.Spec.Owner.ARMID).NotTo(BeEmpty())

		// Storage defaults
		Expect(s.Spec.Storage).NotTo(BeNil())
		Expect(s.Spec.Storage.StorageSizeGB).NotTo(BeNil())
		Expect(*s.Spec.Storage.StorageSizeGB).To(Equal(32))

		Expect(s.Spec.Storage.AutoGrow).NotTo(BeNil())
		Expect(*s.Spec.Storage.AutoGrow).To(Equal(dbforpostgresqlv1.Storage_AutoGrow_Enabled))

		Expect(s.Spec.Storage.Tier).NotTo(BeNil())
		Expect(string(*s.Spec.Storage.Tier)).To(Equal("P10"))
	})

	It("creates a FlexibleServersAdministrator for the Database", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-admin",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: storagev1alpha1.DatabaseAuth{
					AdminAppIdentity: "admin",
					UserAppIdentity:  "user",
				},
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		adminName := fmt.Sprintf("%s-admin", db.Name)

		Eventually(func(g Gomega) error {
			var a dbforpostgresqlv1.FlexibleServersAdministrator
			return k8sClient.Get(ctx, types.NamespacedName{
				Name:      adminName,
				Namespace: db.Namespace,
			}, &a)
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Succeed())

		var a dbforpostgresqlv1.FlexibleServersAdministrator
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      adminName,
			Namespace: db.Namespace,
		}, &a)).To(Succeed())

		Expect(a.Spec.Owner).NotTo(BeNil())
		Expect(a.Spec.Owner.Name).To(Equal(db.Name))

		// PrincipalType sanity check
		Expect(a.Spec.PrincipalType).NotTo(BeNil())
		Expect(string(*a.Spec.PrincipalType)).To(Equal("ServicePrincipal"))

		// And that we used config refs
		Expect(a.Spec.PrincipalName).NotTo(BeNil())
		Expect(a.Spec.TenantId).NotTo(BeNil())
	})

})
