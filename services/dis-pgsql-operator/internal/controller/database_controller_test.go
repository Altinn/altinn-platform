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
	asoconditions "github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Database controller", func() {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	const ns = "default"

	newDatabaseForJob := func(name string, auth storagev1alpha1.DatabaseAuth) *storagev1alpha1.Database {
		return &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth:       auth,
			},
		}
	}

	waitForJob := func(ctx context.Context, name, namespace string) batchv1.Job {
		var job batchv1.Job
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			}, &job)
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Succeed())
		return job
	}

	markASOReady := func(ctx context.Context, db *storagev1alpha1.Database) {
		serverName := db.Name
		adminName := fmt.Sprintf("%s-admin", db.Name)

		Eventually(func() error {
			var server dbforpostgresqlv1.FlexibleServer
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      serverName,
				Namespace: db.Namespace,
			}, &server); err != nil {
				return err
			}
			server.Status.Conditions = []asoconditions.Condition{
				{
					Type:               asoconditions.ConditionTypeReady,
					Status:             metav1.ConditionTrue,
					Reason:             "Ready",
					LastTransitionTime: metav1.Now(),
					ObservedGeneration: server.Generation,
				},
			}
			return k8sClient.Status().Update(ctx, &server)
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Succeed())

		Eventually(func() error {
			var admin dbforpostgresqlv1.FlexibleServersAdministrator
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      adminName,
				Namespace: db.Namespace,
			}, &admin); err != nil {
				return err
			}
			admin.Status.Conditions = []asoconditions.Condition{
				{
					Type:               asoconditions.ConditionTypeReady,
					Status:             metav1.ConditionTrue,
					Reason:             "Ready",
					LastTransitionTime: metav1.Now(),
					ObservedGeneration: admin.Generation,
				},
			}
			return k8sClient.Status().Update(ctx, &admin)
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Succeed())
	}

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
					AdminAppIdentity:        "my-admin-app-identity",
					AdminAppPrincipalId:     "my-admin-app-identity-id",
					AdminServiceAccountName: "my-admin-app-identity",
					UserAppIdentity:         "my-app-identity",
					UserAppPrincipalId:      "my-app-identity-id",
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
					AdminAppIdentity:        "admin1",
					AdminAppPrincipalId:     "admin1-id",
					AdminServiceAccountName: "admin1",
					UserAppIdentity:         "user1",
					UserAppPrincipalId:      "user1-id",
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
					AdminAppIdentity:        "admin2",
					AdminAppPrincipalId:     "admin2-id",
					AdminServiceAccountName: "admin2",
					UserAppIdentity:         "user2",
					UserAppPrincipalId:      "user2-id",
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
					AdminAppIdentity:        "admin-mi",
					AdminAppPrincipalId:     "admin-mi-id",
					AdminServiceAccountName: "admin-mi",
					UserAppIdentity:         "user-mi",
					UserAppPrincipalId:      "user-mi-id",
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
					AdminAppIdentity:        "admin-mi",
					AdminAppPrincipalId:     "admin-mi-id",
					AdminServiceAccountName: "admin-mi",
					UserAppIdentity:         "user-mi",
					UserAppPrincipalId:      "user-mi-id",
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
					AdminAppIdentity:        "admin-mi",
					AdminAppPrincipalId:     "admin-mi-id",
					AdminServiceAccountName: "admin-mi",
					UserAppIdentity:         "user-mi",
					UserAppPrincipalId:      "user-mi-id",
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

	It("updates the FlexibleServer when Database storage spec changes", func() {
		initialSize := int32(32)
		initialTier := "P10"
		updatedSize := int32(64)
		updatedTier := "P15"

		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-psql-update",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: storagev1alpha1.DatabaseAuth{
					AdminAppIdentity:        "admin-mi",
					AdminAppPrincipalId:     "admin-mi-id",
					AdminServiceAccountName: "admin-mi",
					UserAppIdentity:         "user-mi",
					UserAppPrincipalId:      "user-mi-id",
				},
				Storage: &storagev1alpha1.DatabaseStorageSpec{
					SizeGB: &initialSize,
					Tier:   &initialTier,
				},
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		Eventually(func(g Gomega) int {
			var s dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &s)).To(Succeed())
			g.Expect(s.Spec.Storage).NotTo(BeNil())
			g.Expect(s.Spec.Storage.StorageSizeGB).NotTo(BeNil())
			g.Expect(s.Spec.Storage.Tier).NotTo(BeNil())
			return *s.Spec.Storage.StorageSizeGB
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(int(initialSize)))

		var updated storagev1alpha1.Database
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, &updated)).To(Succeed())
		updated.Spec.Storage = &storagev1alpha1.DatabaseStorageSpec{
			SizeGB: &updatedSize,
			Tier:   &updatedTier,
		}
		Expect(k8sClient.Update(ctx, &updated)).To(Succeed())

		Eventually(func(g Gomega) struct {
			size int
			tier string
		} {
			var s dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &s)).To(Succeed())
			g.Expect(s.Spec.Storage).NotTo(BeNil())
			g.Expect(s.Spec.Storage.StorageSizeGB).NotTo(BeNil())
			g.Expect(s.Spec.Storage.Tier).NotTo(BeNil())
			return struct {
				size int
				tier string
			}{
				size: *s.Spec.Storage.StorageSizeGB,
				tier: string(*s.Spec.Storage.Tier),
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				size int
				tier string
			}{
				size: int(updatedSize),
				tier: updatedTier,
			}))
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
					AdminAppIdentity:        "admin",
					AdminAppPrincipalId:     "admin-id",
					AdminServiceAccountName: "admin",
					UserAppIdentity:         "user",
					UserAppPrincipalId:      "user-id",
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

	It("updates the FlexibleServersAdministrator when admin identity changes", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-admin-update",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: storagev1alpha1.DatabaseAuth{
					AdminAppIdentity:        "admin-old",
					AdminAppPrincipalId:     "admin-old-id",
					AdminServiceAccountName: "admin-old",
					UserAppIdentity:         "user",
					UserAppPrincipalId:      "user-id",
				},
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		adminName := fmt.Sprintf("%s-admin", db.Name)

		Eventually(func(g Gomega) string {
			var a dbforpostgresqlv1.FlexibleServersAdministrator
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      adminName,
				Namespace: db.Namespace,
			}, &a)).To(Succeed())
			g.Expect(a.Spec.PrincipalName).NotTo(BeNil())
			return *a.Spec.PrincipalName
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal("admin-old"))

		var updated storagev1alpha1.Database
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, &updated)).To(Succeed())
		updated.Spec.Auth.AdminAppIdentity = "admin-new"
		updated.Spec.Auth.AdminAppPrincipalId = "admin-new-id"
		Expect(k8sClient.Update(ctx, &updated)).To(Succeed())

		Eventually(func(g Gomega) struct {
			azureName     string
			principalName string
		} {
			var a dbforpostgresqlv1.FlexibleServersAdministrator
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      adminName,
				Namespace: db.Namespace,
			}, &a)).To(Succeed())
			g.Expect(a.Spec.PrincipalName).NotTo(BeNil())
			return struct {
				azureName     string
				principalName string
			}{
				azureName:     a.Spec.AzureName,
				principalName: *a.Spec.PrincipalName,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				azureName     string
				principalName string
			}{
				azureName:     "admin-new-id",
				principalName: "admin-new",
			}))
	})

	It("creates a Job to provision the normal database user", func() {
		db := newDatabaseForJob("my-app-db-user-job", storagev1alpha1.DatabaseAuth{
			AdminAppIdentity:        "admin-mi",
			AdminAppPrincipalId:     "admin-mi-id",
			AdminServiceAccountName: "admin-mi",
			UserAppIdentity:         "user-mi",
			UserAppPrincipalId:      "user-mi-id",
		})
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		markASOReady(ctx, db)

		jobName := userProvisionJobName(db)

		job := waitForJob(ctx, jobName, db.Namespace)

		Expect(job.Labels["dis.altinn.cloud/database-name"]).To(Equal(db.Name))
		Expect(job.Spec.Template.Labels["azure.workload.identity/use"]).To(Equal("true"))
		Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(db.Spec.Auth.AdminServiceAccountName))
		Expect(job.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyOnFailure))
		Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
		Expect(job.Spec.Template.Spec.Containers[0].Args).To(ContainElement("--provision-user"))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_USER_APP_IDENTITY", Value: db.Spec.Auth.UserAppIdentity},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_USER_APP_PRINCIPAL_ID", Value: db.Spec.Auth.UserAppPrincipalId},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_ADMIN_APP_IDENTITY", Value: db.Spec.Auth.AdminAppIdentity},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_DATABASE_NAME", Value: db.Name},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_DB_SCHEMA", Value: db.Name},
		))
	})

	It("recreates the user provisioning Job when the spec changes", func() {
		db := newDatabaseForJob("my-app-db-user-job-update", storagev1alpha1.DatabaseAuth{
			AdminAppIdentity:        "admin-mi",
			AdminAppPrincipalId:     "admin-mi-id",
			AdminServiceAccountName: "admin-mi",
			UserAppIdentity:         "user-mi",
			UserAppPrincipalId:      "user-mi-id",
		})
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		markASOReady(ctx, db)

		oldJobName := userProvisionJobName(db)

		waitForJob(ctx, oldJobName, db.Namespace)

		var updated storagev1alpha1.Database
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      db.Name,
			Namespace: db.Namespace,
		}, &updated)).To(Succeed())
		updated.Spec.Auth.UserAppIdentity = "user-mi-2"
		updated.Spec.Auth.UserAppPrincipalId = "user-mi-2-id"
		Expect(k8sClient.Update(ctx, &updated)).To(Succeed())

		newJobName := userProvisionJobName(&updated)
		Expect(newJobName).NotTo(Equal(oldJobName))

		waitForJob(ctx, newJobName, db.Namespace)

		Eventually(func() error {
			var job batchv1.Job
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      oldJobName,
				Namespace: db.Namespace,
			}, &job)
			if err == nil {
				// envtest doesn't run the Job controller, so deletion may hang on finalizers.
				// Treat a deletion timestamp as "deleted" for test purposes.
				if job.DeletionTimestamp != nil {
					return nil
				}
				return fmt.Errorf("old job still exists")
			}
			if !apierrors.IsNotFound(err) {
				return err
			}
			return nil
		}).WithTimeout(30*time.Second).WithPolling(500*time.Millisecond).
			Should(Succeed(), "expected old user-provisioning Job to be deleted")
	})

})
