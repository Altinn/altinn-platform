package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/config"
	dbUtil "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/database"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	networkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20240601"
	"github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	asoconditions "github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("DatabaseServer controller", func() {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	const ns = "default"
	const sharedDelegatedSubnetResourceID = "/subscriptions/my-subscription-id/resourceGroups/rg-dis-dev-network/providers/Microsoft.Network/virtualNetworks/vnet-dis-dev-001/subnets/shared-postgres"
	const sharedPrivateDNSZoneResourceID = "/subscriptions/my-subscription-id/resourceGroups/rg-dis-dev-network/providers/Microsoft.Network/privateDnsZones/shared.private.postgres.database.azure.com"

	adminAuth := func(adminName, adminPrincipalID, adminServiceAccount string) storagev1alpha1.DatabaseAuth {
		return storagev1alpha1.DatabaseAuth{
			Admin: storagev1alpha1.AdminIdentitySpec{
				Identity: storagev1alpha1.IdentitySource{
					Name:        adminName,
					PrincipalId: adminPrincipalID,
				},
				ServiceAccountName: adminServiceAccount,
			},
		}
	}

	directAuth := func(adminName, adminPrincipalID, adminServiceAccount, userName, userPrincipalID string) storagev1alpha1.DatabaseAuth {
		auth := adminAuth(adminName, adminPrincipalID, adminServiceAccount)
		auth.User = &storagev1alpha1.UserIdentitySpec{
			Identity: storagev1alpha1.IdentitySource{
				Name:        userName,
				PrincipalId: userPrincipalID,
			},
		}
		return auth
	}

	adminIdentityRefAuth := func(adminRefName string) storagev1alpha1.DatabaseAuth {
		return storagev1alpha1.DatabaseAuth{
			Admin: storagev1alpha1.AdminIdentitySpec{
				Identity: storagev1alpha1.IdentitySource{
					IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: adminRefName},
				},
			},
		}
	}

	newDedicatedDatabaseServer := func(name string, auth storagev1alpha1.DatabaseAuth) *storagev1alpha1.Database {
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

	sharedNetwork := func() *storagev1alpha1.DatabaseNetworkSpec {
		return &storagev1alpha1.DatabaseNetworkSpec{
			DelegatedSubnetResourceID: sharedDelegatedSubnetResourceID,
			PrivateDNSZoneResourceID:  sharedPrivateDNSZoneResourceID,
		}
	}

	newSharedDatabaseServer := func(name string) *storagev1alpha1.Database {
		return &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Mode:       storagev1alpha1.DatabaseModeShared,
				Version:    17,
				ServerType: "dev",
				Network:    sharedNetwork(),
				Auth: storagev1alpha1.DatabaseAuth{
					Admin: storagev1alpha1.AdminIdentitySpec{
						Identity: storagev1alpha1.IdentitySource{
							Name:        "admin-mi",
							PrincipalId: "admin-mi-id",
						},
						ServiceAccountName: "admin-mi",
					},
				},
			},
		}
	}

	newLogicalDatabase := func(name, serverName string) *storagev1alpha1.LogicalDatabase {
		return &storagev1alpha1.LogicalDatabase{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: storagev1alpha1.LogicalDatabaseSpec{
				Name: name,
				Server: storagev1alpha1.LogicalDatabaseServerSpec{
					Name: serverName,
				},
				Access: storagev1alpha1.LogicalDatabaseAccessSpec{
					App: storagev1alpha1.LogicalDatabasePrincipalSpec{
						Name:        "myproduct-router-dev",
						PrincipalId: "myproduct-router-dev-principal-id",
					},
					Owner: storagev1alpha1.LogicalDatabasePrincipalSpec{
						Name:        "my-team-db-owners",
						PrincipalId: "my-team-db-owners-principal-id",
					},
				},
			},
		}
	}

	expectedLogicalDatabaseName := func(logicalDatabase *storagev1alpha1.LogicalDatabase) string {
		return dbUtil.LogicalDatabaseName(logicalDatabase.Spec.Name)
	}

	ensureNamespace := func(ctx context.Context, namespace string) {
		nsObject := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		if err := k8sClient.Create(ctx, nsObject); apierrors.IsAlreadyExists(err) {
			return
		} else {
			Expect(err).NotTo(HaveOccurred())
		}
	}

	waitForLogicalAccessJob := func(ctx context.Context, logicalDatabaseName, namespace string) batchv1.Job {
		var job batchv1.Job
		Eventually(func(g Gomega) string {
			var jobs batchv1.JobList
			g.Expect(k8sClient.List(ctx, &jobs,
				client.InNamespace(namespace),
				client.MatchingLabels(map[string]string{
					logicalDatabaseLabelKey:           logicalDatabaseName,
					"dis.altinn.cloud/user-provision": "true",
				}),
			)).To(Succeed())
			if len(jobs.Items) != 1 {
				return ""
			}
			job = jobs.Items[0]
			return job.Name
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			ShouldNot(BeEmpty())
		return job
	}

	createApplicationIdentity := func(ctx context.Context, name, namespace, managedName, principalID string) {
		appIdentity := &identityv1alpha1.ApplicationIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: identityv1alpha1.ApplicationIdentitySpec{},
		}
		Expect(k8sClient.Create(ctx, appIdentity)).To(Succeed())
		managed := managedName
		principal := principalID
		appIdentity.Status.ManagedIdentityName = &managed
		appIdentity.Status.PrincipalID = &principal
		Expect(k8sClient.Status().Update(ctx, appIdentity)).To(Succeed())
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
			host := fmt.Sprintf("%s.postgres.database.azure.com", serverName)
			server.Status.FullyQualifiedDomainName = &host
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

	markLogicalDatabaseASOReady := func(ctx context.Context, logicalDatabase *storagev1alpha1.LogicalDatabase) {
		databaseName := expectedLogicalDatabaseName(logicalDatabase)
		resourceName := logicalDatabaseASOResourceName(logicalDatabase.Spec.Server.Name, databaseName)
		Eventually(func() error {
			var server dbforpostgresqlv1.FlexibleServer
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      logicalDatabase.Spec.Server.Name,
				Namespace: logicalDatabase.Namespace,
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
			host := fmt.Sprintf("%s.postgres.database.azure.com", logicalDatabase.Spec.Server.Name)
			server.Status.FullyQualifiedDomainName = &host
			return k8sClient.Status().Update(ctx, &server)
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Succeed())

		Eventually(func() error {
			var asoDatabase dbforpostgresqlv1.FlexibleServersDatabase
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: logicalDatabase.Namespace,
			}, &asoDatabase); err != nil {
				return err
			}
			asoDatabase.Status.Conditions = []asoconditions.Condition{
				{
					Type:               asoconditions.ConditionTypeReady,
					Status:             metav1.ConditionTrue,
					Reason:             "Ready",
					LastTransitionTime: metav1.Now(),
					ObservedGeneration: asoDatabase.Generation,
				},
			}
			return k8sClient.Status().Update(ctx, &asoDatabase)
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Succeed())
	}

	cleanupNamespacedTestResources := func(ctx context.Context, namespace string) {
		propagationPolicy := metav1.DeletePropagationBackground
		deleteAll := func(obj client.Object) {
			Expect(k8sClient.DeleteAllOf(ctx, obj,
				client.InNamespace(namespace),
				client.PropagationPolicy(propagationPolicy),
			)).To(Succeed())
		}

		// Delete reconciling parents first so they cannot recreate children while
		// this cleanup is draining the namespace.
		deleteAll(&storagev1alpha1.LogicalDatabase{})
		deleteAll(&storagev1alpha1.Database{})

		deleteAll(&batchv1.Job{})
		deleteAll(&dbforpostgresqlv1.FlexibleServersDatabase{})
		deleteAll(&dbforpostgresqlv1.FlexibleServersAdministrator{})
		deleteAll(&dbforpostgresqlv1.FlexibleServersConfiguration{})
		deleteAll(&dbforpostgresqlv1.FlexibleServer{})
		deleteAll(&networkv1.PrivateDnsZonesVirtualNetworkLink{})
		deleteAll(&networkv1.PrivateDnsZone{})
		deleteAll(&identityv1alpha1.ApplicationIdentity{})

		Eventually(func(g Gomega) int {
			var list storagev1alpha1.DatabaseList
			g.Expect(k8sClient.List(ctx, &list, client.InNamespace(namespace))).To(Succeed())
			return len(list.Items)
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(0))

		Eventually(func(g Gomega) int {
			var list storagev1alpha1.LogicalDatabaseList
			g.Expect(k8sClient.List(ctx, &list, client.InNamespace(namespace))).To(Succeed())
			return len(list.Items)
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(0))
	}

	listLogicalDatabaseASOChildren := func(g Gomega, logicalDatabaseName string) []dbforpostgresqlv1.FlexibleServersDatabase {
		var logicalDatabases dbforpostgresqlv1.FlexibleServersDatabaseList
		g.Expect(k8sClient.List(ctx, &logicalDatabases,
			client.InNamespace(ns),
			client.MatchingLabels(map[string]string{
				logicalDatabaseLabelKey: logicalDatabaseName,
			}),
		)).To(Succeed())
		return logicalDatabases.Items
	}

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		ensureNamespace(ctx, ns)
	})

	AfterEach(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		cleanupNamespacedTestResources(cleanupCtx, ns)
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
				Auth: directAuth(
					"my-admin-app-identity",
					"my-admin-app-identity-id",
					"my-admin-app-identity",
					"my-app-identity",
					"my-app-identity-id",
				),
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

	It("allocates different /28 blocks for two database servers", func() {
		db1 := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db1-subnet",
				Namespace: ns,
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: directAuth(
					"adminidentity",
					"adminidentity-id",
					"adminidentity",
					"user1",
					"user1-id",
				),
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
				Auth: directAuth(
					"adminidentity",
					"adminidentity-id",
					"adminidentity",
					"user2",
					"user2-id",
				),
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

	// Database server Private DNS Zone integration tests
	It("creates a Private DNS zone per database server", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-dns",
				Namespace: ns,
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
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
			Should(Succeed(), "expected Private DNS zone for database server to be created by controller")

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
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
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
			Should(Succeed(), "expected Private DNS zone for database server to be created")

		// Expect two VNet links
		expectedDBLinkName := dbVNetLinkNameForDatabaseServer(db)
		expectedAKSLinkName := aksVNetLinkNameForDatabaseServer(db)

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

	It("reconciles existing Private DNS AKS VNet link when ARM ID drifts", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-vnet-drift",
				Namespace: ns,
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		expectedAKSLinkName := aksVNetLinkNameForDatabaseServer(db)
		expectedAKSVNetARMID := "/subscriptions/my-subscription-id/resourceGroups/aks-vnet-rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-dis-dev-001"

		Eventually(func(g Gomega) string {
			var link networkv1.PrivateDnsZonesVirtualNetworkLink
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      expectedAKSLinkName,
				Namespace: ns,
			}, &link)).To(Succeed())
			if link.Spec.VirtualNetwork == nil || link.Spec.VirtualNetwork.Reference == nil {
				return ""
			}
			return link.Spec.VirtualNetwork.Reference.ARMID
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(expectedAKSVNetARMID))

		var link networkv1.PrivateDnsZonesVirtualNetworkLink
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      expectedAKSLinkName,
			Namespace: ns,
		}, &link)).To(Succeed())
		link.Spec.VirtualNetwork = &networkv1.SubResource{
			Reference: &genruntime.ResourceReference{
				ARMID: "/subscriptions/another-sub/resourceGroups/wrong-rg/providers/Microsoft.Network/virtualNetworks/wrong-vnet",
			},
		}
		Expect(k8sClient.Update(ctx, &link)).To(Succeed())

		Eventually(func(g Gomega) string {
			var current networkv1.PrivateDnsZonesVirtualNetworkLink
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      expectedAKSLinkName,
				Namespace: ns,
			}, &current)).To(Succeed())
			if current.Spec.VirtualNetwork == nil || current.Spec.VirtualNetwork.Reference == nil {
				return ""
			}
			return current.Spec.VirtualNetwork.Reference.ARMID
		}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(expectedAKSVNetARMID))
	})

	It("allows dedicated database servers without user auth", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-dedicated-no-user",
				Namespace: ns,
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: storagev1alpha1.DatabaseAuth{
					Admin: storagev1alpha1.AdminIdentitySpec{
						Identity: storagev1alpha1.IdentitySource{
							Name:        "admin-mi",
							PrincipalId: "admin-mi-id",
						},
						ServiceAccountName: "admin-mi",
					},
				},
			},
		}

		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		Eventually(func(g Gomega) string {
			var server dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &server)).To(Succeed())
			return server.Name
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(db.Name))

		Consistently(func(g Gomega) int {
			var jobs batchv1.JobList
			g.Expect(k8sClient.List(ctx, &jobs,
				client.InNamespace(db.Namespace),
				client.MatchingLabels(map[string]string{
					"dis.altinn.cloud/database-name":  db.Name,
					"dis.altinn.cloud/user-provision": "true",
				}),
			)).To(Succeed())
			return len(jobs.Items)
		}).WithTimeout(3 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(0))
	})

	It("rejects dedicated database servers with shared network config", func() {
		db := newDedicatedDatabaseServer("my-app-db-dedicated-network", directAuth(
			"admin-mi",
			"admin-mi-id",
			"admin-mi",
			"user-mi",
			"user-mi-id",
		))
		db.Spec.Network = sharedNetwork()

		err := k8sClient.Create(ctx, db)
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected dedicated database server with spec.network to be rejected")
	})

	It("rejects shared database servers without network config", func() {
		db := newSharedDatabaseServer("my-app-db-shared-no-network")
		db.Spec.Network = nil

		err := k8sClient.Create(ctx, db)
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected shared database server without spec.network to be rejected")
	})

	DescribeTable("rejects shared network ARM IDs outside the allowed scope or expected type",
		func(mutate func(*storagev1alpha1.DatabaseNetworkSpec), expectedError string) {
			db := newSharedDatabaseServer("my-app-db-shared-invalid-network")
			mutate(db.Spec.Network)

			reconciler := DatabaseServerReconciler{
				Config: config.OperatorConfig{SubscriptionId: "my-subscription-id"},
			}
			_, err := reconciler.sharedPostgresNetworkConfig(db)

			Expect(err).To(MatchError(ContainSubstring(expectedError)))
		},
		Entry("subnet in a different subscription",
			func(network *storagev1alpha1.DatabaseNetworkSpec) {
				network.DelegatedSubnetResourceID = "/subscriptions/other-subscription/resourceGroups/rg-dis-dev-network/providers/Microsoft.Network/virtualNetworks/vnet-dis-dev-001/subnets/shared-postgres"
			},
			"spec.network.delegatedSubnetResourceId must be in subscription",
		),
		Entry("private DNS zone in a different subscription",
			func(network *storagev1alpha1.DatabaseNetworkSpec) {
				network.PrivateDNSZoneResourceID = "/subscriptions/other-subscription/resourceGroups/rg-dis-dev-network/providers/Microsoft.Network/privateDnsZones/shared.private.postgres.database.azure.com"
			},
			"spec.network.privateDnsZoneResourceId must be in subscription",
		),
		Entry("subnet reference with the wrong resource type",
			func(network *storagev1alpha1.DatabaseNetworkSpec) {
				network.DelegatedSubnetResourceID = "/subscriptions/my-subscription-id/resourceGroups/rg-dis-dev-network/providers/Microsoft.Network/virtualNetworks/vnet-dis-dev-001"
			},
			"spec.network.delegatedSubnetResourceId must reference Microsoft.Network/virtualNetworks/subnets",
		),
		Entry("private DNS zone reference with the wrong resource type",
			func(network *storagev1alpha1.DatabaseNetworkSpec) {
				network.PrivateDNSZoneResourceID = "/subscriptions/my-subscription-id/resourceGroups/rg-dis-dev-network/providers/Microsoft.Network/virtualNetworks/vnet-dis-dev-001/subnets/shared-postgres"
			},
			"spec.network.privateDnsZoneResourceId must reference Microsoft.Network/privateDnsZones",
		),
	)

	It("creates a shared FlexibleServer with existing network references and skips dedicated side effects", func() {
		db := newSharedDatabaseServer("my-app-db-shared")
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		Eventually(func(g Gomega) struct {
			subnetID string
			zoneID   string
		} {
			var server dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &server)).To(Succeed())
			g.Expect(server.Spec.Network).NotTo(BeNil())
			g.Expect(server.Spec.Network.DelegatedSubnetResourceReference).NotTo(BeNil())
			g.Expect(server.Spec.Network.PrivateDnsZoneArmResourceReference).NotTo(BeNil())
			return struct {
				subnetID string
				zoneID   string
			}{
				subnetID: server.Spec.Network.DelegatedSubnetResourceReference.ARMID,
				zoneID:   server.Spec.Network.PrivateDnsZoneArmResourceReference.ARMID,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				subnetID string
				zoneID   string
			}{
				subnetID: sharedDelegatedSubnetResourceID,
				zoneID:   sharedPrivateDNSZoneResourceID,
			}))

		Consistently(func(g Gomega) string {
			var updated storagev1alpha1.Database
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &updated)).To(Succeed())
			return updated.Status.SubnetCIDR
		}).WithTimeout(3 * time.Second).WithPolling(250 * time.Millisecond).
			Should(BeEmpty())

		Consistently(func() bool {
			var zone networkv1.PrivateDnsZone
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      zoneNameForDatabaseServer(db),
				Namespace: db.Namespace,
			}, &zone)
			return apierrors.IsNotFound(err)
		}).WithTimeout(3 * time.Second).WithPolling(250 * time.Millisecond).
			Should(BeTrue())

		Consistently(func(g Gomega) []string {
			var links networkv1.PrivateDnsZonesVirtualNetworkLinkList
			g.Expect(k8sClient.List(ctx, &links, client.InNamespace(db.Namespace))).To(Succeed())
			found := make([]string, 0)
			for _, link := range links.Items {
				if link.Name == dbVNetLinkNameForDatabaseServer(db) || link.Name == aksVNetLinkNameForDatabaseServer(db) {
					found = append(found, link.Name)
				}
			}
			return found
		}).WithTimeout(3 * time.Second).WithPolling(250 * time.Millisecond).
			Should(BeEmpty())

		Consistently(func(g Gomega) int {
			var jobs batchv1.JobList
			g.Expect(k8sClient.List(ctx, &jobs,
				client.InNamespace(db.Namespace),
				client.MatchingLabels(map[string]string{
					"dis.altinn.cloud/database-name":  db.Name,
					"dis.altinn.cloud/user-provision": "true",
				}),
			)).To(Succeed())
			return len(jobs.Items)
		}).WithTimeout(3 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(0))
	})

	It("reconciles shared server settings and parameters", func() {
		sizeGB := int32(64)
		tier := "P15"
		retentionDays := 21
		highAvailabilityEnabled := true
		db := newSharedDatabaseServer("my-app-db-shared-settings")
		db.Spec.Storage = &storagev1alpha1.DatabaseStorageSpec{
			SizeGB: &sizeGB,
			Tier:   &tier,
		}
		db.Spec.BackupRetentionDays = &retentionDays
		db.Spec.HighAvailabilityEnabled = &highAvailabilityEnabled
		db.Spec.EnableExtensions = []storagev1alpha1.DatabaseExtension{
			storagev1alpha1.DatabaseExtensionHstore,
			storagev1alpha1.DatabaseExtensionPgCron,
		}
		db.Spec.ServerParams = []storagev1alpha1.DatabaseServerParameter{
			{
				Name:  "autovacuum_naptime",
				Value: intstr.FromInt(15),
			},
		}

		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		Eventually(func(g Gomega) struct {
			sizeGB        int
			tier          string
			retentionDays int
			haMode        dbforpostgresqlv1.HighAvailability_Mode
		} {
			var server dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &server)).To(Succeed())
			g.Expect(server.Spec.Storage).NotTo(BeNil())
			g.Expect(server.Spec.Storage.StorageSizeGB).NotTo(BeNil())
			g.Expect(server.Spec.Storage.Tier).NotTo(BeNil())
			g.Expect(server.Spec.Backup).NotTo(BeNil())
			g.Expect(server.Spec.Backup.BackupRetentionDays).NotTo(BeNil())
			g.Expect(server.Spec.HighAvailability).NotTo(BeNil())
			g.Expect(server.Spec.HighAvailability.Mode).NotTo(BeNil())
			return struct {
				sizeGB        int
				tier          string
				retentionDays int
				haMode        dbforpostgresqlv1.HighAvailability_Mode
			}{
				sizeGB:        *server.Spec.Storage.StorageSizeGB,
				tier:          string(*server.Spec.Storage.Tier),
				retentionDays: *server.Spec.Backup.BackupRetentionDays,
				haMode:        *server.Spec.HighAvailability.Mode,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				sizeGB        int
				tier          string
				retentionDays int
				haMode        dbforpostgresqlv1.HighAvailability_Mode
			}{
				sizeGB:        64,
				tier:          "P15",
				retentionDays: 21,
				haMode:        dbforpostgresqlv1.HighAvailability_Mode_ZoneRedundant,
			}))

		expectedConfigurations := map[string]string{
			extensionsConfigResourceName(db.Name):                             "hstore,pg_cron",
			serverParameterConfigResourceName(db.Name, "autovacuum_naptime"):  "15",
			serverParameterConfigResourceName(db.Name, "pgbouncer.enabled"):   "true",
			serverParameterConfigResourceName(db.Name, "pgbouncer.pool_mode"): "transaction",
		}

		for resourceName, expectedValue := range expectedConfigurations {
			Eventually(func(g Gomega) string {
				var configuration dbforpostgresqlv1.FlexibleServersConfiguration
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      resourceName,
					Namespace: db.Namespace,
				}, &configuration)).To(Succeed())
				g.Expect(configuration.Spec.Value).NotTo(BeNil())
				return *configuration.Spec.Value
			}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
				Should(Equal(expectedValue))
		}
	})

	It("allows multiple shared database servers in one namespace without subnet allocation", func() {
		db1 := newSharedDatabaseServer("my-app-db-shared-one")
		db2 := newSharedDatabaseServer("my-app-db-shared-two")

		Expect(k8sClient.Create(ctx, db1)).To(Succeed())
		Expect(k8sClient.Create(ctx, db2)).To(Succeed())

		Eventually(func(g Gomega) []string {
			var servers dbforpostgresqlv1.FlexibleServerList
			g.Expect(k8sClient.List(ctx, &servers, client.InNamespace(ns))).To(Succeed())
			found := make([]string, 0, len(servers.Items))
			for _, server := range servers.Items {
				if server.Name == db1.Name || server.Name == db2.Name {
					found = append(found, server.Name)
				}
			}
			return found
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(ConsistOf(db1.Name, db2.Name))

		Consistently(func(g Gomega) []string {
			var dbList storagev1alpha1.DatabaseList
			g.Expect(k8sClient.List(ctx, &dbList, client.InNamespace(ns))).To(Succeed())
			subnets := make([]string, 0)
			for _, item := range dbList.Items {
				if item.Name == db1.Name || item.Name == db2.Name {
					subnets = append(subnets, item.Status.SubnetCIDR)
				}
			}
			return subnets
		}).WithTimeout(3 * time.Second).WithPolling(250 * time.Millisecond).
			Should(ConsistOf("", ""))
	})

	// Database server testing
	It("creates a FlexibleServer for the database server", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-psql",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
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
			Should(Succeed(), "expected FlexibleServer ASO resource to be created for database server")

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

		Expect(s.Spec.HighAvailability).NotTo(BeNil())
		Expect(s.Spec.HighAvailability.Mode).NotTo(BeNil())
		Expect(*s.Spec.HighAvailability.Mode).To(Equal(dbforpostgresqlv1.HighAvailability_Mode_Disabled))
		Expect(s.Spec.HighAvailability.StandbyAvailabilityZone).To(BeNil())

		Expect(s.Spec.Backup).NotTo(BeNil())
		Expect(s.Spec.Backup.BackupRetentionDays).NotTo(BeNil())
		Expect(*s.Spec.Backup.BackupRetentionDays).To(Equal(14))
		Expect(s.Spec.Backup.GeoRedundantBackup).NotTo(BeNil())
		Expect(*s.Spec.Backup.GeoRedundantBackup).To(Equal(dbforpostgresqlv1.Backup_GeoRedundantBackup_Disabled))
	})

	It("defaults highAvailabilityEnabled to true for prod server types", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-psql-ha-prod-default",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "prod",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
			},
		}

		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		Eventually(func(g Gomega) struct {
			mode        dbforpostgresqlv1.HighAvailability_Mode
			standbyZone string
		} {
			var s dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &s)).To(Succeed())
			g.Expect(s.Spec.HighAvailability).NotTo(BeNil())
			g.Expect(s.Spec.HighAvailability.Mode).NotTo(BeNil())
			g.Expect(s.Spec.HighAvailability.StandbyAvailabilityZone).NotTo(BeNil())
			return struct {
				mode        dbforpostgresqlv1.HighAvailability_Mode
				standbyZone string
			}{
				mode:        *s.Spec.HighAvailability.Mode,
				standbyZone: *s.Spec.HighAvailability.StandbyAvailabilityZone,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				mode        dbforpostgresqlv1.HighAvailability_Mode
				standbyZone string
			}{
				mode:        dbforpostgresqlv1.HighAvailability_Mode_ZoneRedundant,
				standbyZone: "2",
			}))
	})

	It("uses explicit highAvailabilityEnabled false when set", func() {
		highAvailabilityEnabled := false

		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-psql-ha-explicit-false",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:                 17,
				ServerType:              "prod",
				HighAvailabilityEnabled: &highAvailabilityEnabled,
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
			},
		}

		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		Eventually(func(g Gomega) struct {
			mode        dbforpostgresqlv1.HighAvailability_Mode
			standbyZone string
		} {
			var s dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &s)).To(Succeed())
			g.Expect(s.Spec.HighAvailability).NotTo(BeNil())
			g.Expect(s.Spec.HighAvailability.Mode).NotTo(BeNil())
			standbyZone := ""
			if s.Spec.HighAvailability.StandbyAvailabilityZone != nil {
				standbyZone = *s.Spec.HighAvailability.StandbyAvailabilityZone
			}
			return struct {
				mode        dbforpostgresqlv1.HighAvailability_Mode
				standbyZone string
			}{
				mode:        *s.Spec.HighAvailability.Mode,
				standbyZone: standbyZone,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				mode        dbforpostgresqlv1.HighAvailability_Mode
				standbyZone string
			}{
				mode:        dbforpostgresqlv1.HighAvailability_Mode_Disabled,
				standbyZone: "",
			}))
	})

	It("updates the FlexibleServer when highAvailabilityEnabled changes", func() {
		highAvailabilityEnabled := true

		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-psql-ha-update",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:                 17,
				ServerType:              "dev",
				HighAvailabilityEnabled: &highAvailabilityEnabled,
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
			},
		}

		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		Eventually(func(g Gomega) struct {
			mode        dbforpostgresqlv1.HighAvailability_Mode
			standbyZone string
		} {
			var s dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &s)).To(Succeed())
			g.Expect(s.Spec.HighAvailability).NotTo(BeNil())
			g.Expect(s.Spec.HighAvailability.Mode).NotTo(BeNil())
			g.Expect(s.Spec.HighAvailability.StandbyAvailabilityZone).NotTo(BeNil())
			return struct {
				mode        dbforpostgresqlv1.HighAvailability_Mode
				standbyZone string
			}{
				mode:        *s.Spec.HighAvailability.Mode,
				standbyZone: *s.Spec.HighAvailability.StandbyAvailabilityZone,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				mode        dbforpostgresqlv1.HighAvailability_Mode
				standbyZone string
			}{
				mode:        dbforpostgresqlv1.HighAvailability_Mode_ZoneRedundant,
				standbyZone: "2",
			}))

		highAvailabilityDisabled := false
		var updated storagev1alpha1.Database
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, &updated)).To(Succeed())
		updated.Spec.HighAvailabilityEnabled = &highAvailabilityDisabled
		Expect(k8sClient.Update(ctx, &updated)).To(Succeed())

		Eventually(func(g Gomega) struct {
			mode        dbforpostgresqlv1.HighAvailability_Mode
			standbyZone string
		} {
			var s dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &s)).To(Succeed())
			g.Expect(s.Spec.HighAvailability).NotTo(BeNil())
			g.Expect(s.Spec.HighAvailability.Mode).NotTo(BeNil())
			standbyZone := ""
			if s.Spec.HighAvailability.StandbyAvailabilityZone != nil {
				standbyZone = *s.Spec.HighAvailability.StandbyAvailabilityZone
			}
			return struct {
				mode        dbforpostgresqlv1.HighAvailability_Mode
				standbyZone string
			}{
				mode:        *s.Spec.HighAvailability.Mode,
				standbyZone: standbyZone,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				mode        dbforpostgresqlv1.HighAvailability_Mode
				standbyZone string
			}{
				mode:        dbforpostgresqlv1.HighAvailability_Mode_Disabled,
				standbyZone: "",
			}))
	})

	It("uses explicit backupRetentionDays when set", func() {
		requestedRetentionDays := 21
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-psql-backup-retention",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:             17,
				ServerType:          "dev",
				BackupRetentionDays: &requestedRetentionDays,
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
			},
		}

		Expect(k8sClient.Create(ctx, db)).To(Succeed())
		expectedServerName := db.Name

		Eventually(func(g Gomega) int {
			var s dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      expectedServerName,
				Namespace: db.Namespace,
			}, &s)).To(Succeed())
			g.Expect(s.Spec.Backup).NotTo(BeNil())
			g.Expect(s.Spec.Backup.BackupRetentionDays).NotTo(BeNil())
			return *s.Spec.Backup.BackupRetentionDays
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(requestedRetentionDays))
	})

	It("defaults backupRetentionDays to 30 for prod server types", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-psql-backup-retention-prod-default",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "prod",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
			},
		}

		Expect(k8sClient.Create(ctx, db)).To(Succeed())
		expectedServerName := db.Name

		Eventually(func(g Gomega) int {
			var s dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      expectedServerName,
				Namespace: db.Namespace,
			}, &s)).To(Succeed())
			g.Expect(s.Spec.Backup).NotTo(BeNil())
			g.Expect(s.Spec.Backup.BackupRetentionDays).NotTo(BeNil())
			return *s.Spec.Backup.BackupRetentionDays
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(30))
	})

	It("forces GeoRedundantBackup to Disabled when backupRetentionDays changes", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-psql-backup-geo",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
			},
		}

		Expect(k8sClient.Create(ctx, db)).To(Succeed())
		expectedServerName := db.Name

		Eventually(func(g Gomega) struct {
			retention int
			geo       dbforpostgresqlv1.Backup_GeoRedundantBackup
		} {
			var s dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      expectedServerName,
				Namespace: db.Namespace,
			}, &s)).To(Succeed())
			g.Expect(s.Spec.Backup).NotTo(BeNil())
			g.Expect(s.Spec.Backup.BackupRetentionDays).NotTo(BeNil())
			g.Expect(s.Spec.Backup.GeoRedundantBackup).NotTo(BeNil())
			return struct {
				retention int
				geo       dbforpostgresqlv1.Backup_GeoRedundantBackup
			}{
				retention: *s.Spec.Backup.BackupRetentionDays,
				geo:       *s.Spec.Backup.GeoRedundantBackup,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				retention int
				geo       dbforpostgresqlv1.Backup_GeoRedundantBackup
			}{
				retention: 14,
				geo:       dbforpostgresqlv1.Backup_GeoRedundantBackup_Disabled,
			}))

		var server dbforpostgresqlv1.FlexibleServer
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      expectedServerName,
			Namespace: db.Namespace,
		}, &server)).To(Succeed())
		enabled := dbforpostgresqlv1.Backup_GeoRedundantBackup_Enabled
		server.Spec.Backup.GeoRedundantBackup = &enabled
		Expect(k8sClient.Update(ctx, &server)).To(Succeed())

		requestedRetentionDays := 22
		var updated storagev1alpha1.Database
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, &updated)).To(Succeed())
		updated.Spec.BackupRetentionDays = &requestedRetentionDays
		Expect(k8sClient.Update(ctx, &updated)).To(Succeed())

		Eventually(func(g Gomega) struct {
			retention int
			geo       dbforpostgresqlv1.Backup_GeoRedundantBackup
		} {
			var s dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      expectedServerName,
				Namespace: db.Namespace,
			}, &s)).To(Succeed())
			g.Expect(s.Spec.Backup).NotTo(BeNil())
			g.Expect(s.Spec.Backup.BackupRetentionDays).NotTo(BeNil())
			g.Expect(s.Spec.Backup.GeoRedundantBackup).NotTo(BeNil())

			return struct {
				retention int
				geo       dbforpostgresqlv1.Backup_GeoRedundantBackup
			}{
				retention: *s.Spec.Backup.BackupRetentionDays,
				geo:       *s.Spec.Backup.GeoRedundantBackup,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				retention int
				geo       dbforpostgresqlv1.Backup_GeoRedundantBackup
			}{
				retention: requestedRetentionDays,
				geo:       dbforpostgresqlv1.Backup_GeoRedundantBackup_Disabled,
			}))
	})

	It("sets fixed server defaults on FlexibleServer", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-server-defaults",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "prod",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
			},
		}

		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		Eventually(func(g Gomega) struct {
			availabilityZone string
			standbyZone      string
			storageType      dbforpostgresqlv1.Storage_Type
			geoBackup        dbforpostgresqlv1.Backup_GeoRedundantBackup
			maintenanceDay   int
			maintenanceHour  int
			maintenanceMin   int
		} {
			var s dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &s)).To(Succeed())
			g.Expect(s.Spec.Storage).NotTo(BeNil())
			g.Expect(s.Spec.Storage.Type).NotTo(BeNil())
			g.Expect(s.Spec.Backup).NotTo(BeNil())
			g.Expect(s.Spec.Backup.GeoRedundantBackup).NotTo(BeNil())
			g.Expect(s.Spec.AvailabilityZone).NotTo(BeNil())
			g.Expect(s.Spec.HighAvailability).NotTo(BeNil())
			g.Expect(s.Spec.HighAvailability.StandbyAvailabilityZone).NotTo(BeNil())
			g.Expect(s.Spec.MaintenanceWindow).NotTo(BeNil())
			g.Expect(s.Spec.MaintenanceWindow.DayOfWeek).NotTo(BeNil())
			g.Expect(s.Spec.MaintenanceWindow.StartHour).NotTo(BeNil())
			g.Expect(s.Spec.MaintenanceWindow.StartMinute).NotTo(BeNil())
			return struct {
				availabilityZone string
				standbyZone      string
				storageType      dbforpostgresqlv1.Storage_Type
				geoBackup        dbforpostgresqlv1.Backup_GeoRedundantBackup
				maintenanceDay   int
				maintenanceHour  int
				maintenanceMin   int
			}{
				availabilityZone: *s.Spec.AvailabilityZone,
				standbyZone:      *s.Spec.HighAvailability.StandbyAvailabilityZone,
				storageType:      *s.Spec.Storage.Type,
				geoBackup:        *s.Spec.Backup.GeoRedundantBackup,
				maintenanceDay:   *s.Spec.MaintenanceWindow.DayOfWeek,
				maintenanceHour:  *s.Spec.MaintenanceWindow.StartHour,
				maintenanceMin:   *s.Spec.MaintenanceWindow.StartMinute,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				availabilityZone string
				standbyZone      string
				storageType      dbforpostgresqlv1.Storage_Type
				geoBackup        dbforpostgresqlv1.Backup_GeoRedundantBackup
				maintenanceDay   int
				maintenanceHour  int
				maintenanceMin   int
			}{
				availabilityZone: "1",
				standbyZone:      "2",
				storageType:      dbforpostgresqlv1.Storage_Type_Premium_LRS,
				geoBackup:        dbforpostgresqlv1.Backup_GeoRedundantBackup_Disabled,
				maintenanceDay:   0,
				maintenanceHour:  3,
				maintenanceMin:   0,
			}))
	})

	It("creates fixed and user-defined server parameter configurations", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-server-params",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
				ServerParams: []storagev1alpha1.DatabaseServerParameter{
					{
						Name:  "autovacuum_naptime",
						Value: intstr.FromInt(15),
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		maxConnections, err := dbUtil.ResolveMaxConnections(dbUtil.GetProfile("dev"))
		Expect(err).NotTo(HaveOccurred())

		expectedValues := map[string]string{
			dbUtil.ServerParameterPgBouncerEnabled:     "true",
			dbUtil.ServerParameterPgBouncerMaxPrepared: "5000",
			dbUtil.ServerParameterPgBouncerPoolMode:    "transaction",
			dbUtil.ServerParameterMaxConnections:       fmt.Sprintf("%d", maxConnections),
			"autovacuum_naptime":                       "15",
		}

		for parameterName, expectedValue := range expectedValues {
			resourceName := serverParameterConfigResourceName(db.Name, parameterName)

			Eventually(func(g Gomega) struct {
				azureName string
				value     string
			} {
				var configuration dbforpostgresqlv1.FlexibleServersConfiguration
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      resourceName,
					Namespace: db.Namespace,
				}, &configuration)).To(Succeed())
				g.Expect(configuration.Spec.Value).NotTo(BeNil())
				return struct {
					azureName string
					value     string
				}{
					azureName: configuration.Spec.AzureName,
					value:     *configuration.Spec.Value,
				}
			}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
				Should(Equal(struct {
					azureName string
					value     string
				}{
					azureName: parameterName,
					value:     expectedValue,
				}))
		}

		var updated storagev1alpha1.Database
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, &updated)).To(Succeed())
		updated.Spec.ServerParams = nil
		Expect(k8sClient.Update(ctx, &updated)).To(Succeed())

		customParamName := serverParameterConfigResourceName(db.Name, "autovacuum_naptime")
		Eventually(func() bool {
			var configuration dbforpostgresqlv1.FlexibleServersConfiguration
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      customParamName,
				Namespace: db.Namespace,
			}, &configuration)
			return apierrors.IsNotFound(err)
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(BeTrue())
	})

	It("writes ASO server parameter errors to database server status", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-server-params-status",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
				ServerParams: []storagev1alpha1.DatabaseServerParameter{
					{
						Name:  "autovacuum_naptime",
						Value: intstr.FromInt(15),
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		parameterName := "autovacuum_naptime"
		resourceName := serverParameterConfigResourceName(db.Name, parameterName)

		Eventually(func(g Gomega) {
			var configuration dbforpostgresqlv1.FlexibleServersConfiguration
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName,
				Namespace: db.Namespace,
			}, &configuration)).To(Succeed())

			configuration.Status.Conditions = []asoconditions.Condition{
				{
					Type:               asoconditions.ConditionTypeReady,
					Status:             metav1.ConditionFalse,
					Reason:             "InvalidParameterValue",
					Message:            "Parameter value is not valid",
					LastTransitionTime: metav1.Now(),
					ObservedGeneration: configuration.Generation,
				},
			}
			g.Expect(k8sClient.Status().Update(ctx, &configuration)).To(Succeed())
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Succeed())

		Eventually(func(g Gomega) struct {
			errorReason string
			errorMsg    string
			condReason  string
			condStatus  metav1.ConditionStatus
		} {
			var updated storagev1alpha1.Database
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &updated)).To(Succeed())

			var parameterError *storagev1alpha1.DatabaseServerParameterError
			for i := range updated.Status.ServerParameterErrors {
				if updated.Status.ServerParameterErrors[i].Name == parameterName {
					parameterError = &updated.Status.ServerParameterErrors[i]
					break
				}
			}
			g.Expect(parameterError).NotTo(BeNil())

			condition := meta.FindStatusCondition(updated.Status.Conditions, serverParametersReadyConditionType)
			g.Expect(condition).NotTo(BeNil())

			return struct {
				errorReason string
				errorMsg    string
				condReason  string
				condStatus  metav1.ConditionStatus
			}{
				errorReason: parameterError.Reason,
				errorMsg:    parameterError.Message,
				condReason:  condition.Reason,
				condStatus:  condition.Status,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				errorReason string
				errorMsg    string
				condReason  string
				condStatus  metav1.ConditionStatus
			}{
				errorReason: "InvalidParameterValue",
				errorMsg:    "Parameter value is not valid",
				condReason:  "ApplyFailed",
				condStatus:  metav1.ConditionFalse,
			}))
	})

	It("does not create FlexibleServersConfiguration resources when enableExtensions is omitted", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-no-extensions",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		extensionsName := extensionsConfigResourceName(db.Name)
		sharedPreloadName := sharedPreloadLibrariesConfigResourceName(db.Name)

		Consistently(func() bool {
			var configuration dbforpostgresqlv1.FlexibleServersConfiguration
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      extensionsName,
				Namespace: db.Namespace,
			}, &configuration)
			return apierrors.IsNotFound(err)
		}).WithTimeout(5 * time.Second).WithPolling(250 * time.Millisecond).
			Should(BeTrue())

		Consistently(func() bool {
			var configuration dbforpostgresqlv1.FlexibleServersConfiguration
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      sharedPreloadName,
				Namespace: db.Namespace,
			}, &configuration)
			return apierrors.IsNotFound(err)
		}).WithTimeout(5 * time.Second).WithPolling(250 * time.Millisecond).
			Should(BeTrue())
	})

	It("creates FlexibleServersConfiguration resources for enabled extensions", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-extensions",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
				EnableExtensions: []storagev1alpha1.DatabaseExtension{
					storagev1alpha1.DatabaseExtensionUUIDOSSP,
					storagev1alpha1.DatabaseExtensionPgCron,
					storagev1alpha1.DatabaseExtensionPgAudit,
					storagev1alpha1.DatabaseExtensionPgStatStatements,
					storagev1alpha1.DatabaseExtensionHstore,
				},
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		extensionsName := extensionsConfigResourceName(db.Name)
		sharedPreloadName := sharedPreloadLibrariesConfigResourceName(db.Name)

		Eventually(func(g Gomega) struct {
			azureName string
			value     string
		} {
			var configuration dbforpostgresqlv1.FlexibleServersConfiguration
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      extensionsName,
				Namespace: db.Namespace,
			}, &configuration)).To(Succeed())
			g.Expect(configuration.Spec.Value).NotTo(BeNil())
			return struct {
				azureName string
				value     string
			}{
				azureName: configuration.Spec.AzureName,
				value:     *configuration.Spec.Value,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				azureName string
				value     string
			}{
				azureName: "azure.extensions",
				value:     "hstore,pg_cron,pg_stat_statements,pgaudit,uuid-ossp",
			}))

		Eventually(func(g Gomega) struct {
			azureName string
			value     string
		} {
			var configuration dbforpostgresqlv1.FlexibleServersConfiguration
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      sharedPreloadName,
				Namespace: db.Namespace,
			}, &configuration)).To(Succeed())
			g.Expect(configuration.Spec.Value).NotTo(BeNil())
			return struct {
				azureName string
				value     string
			}{
				azureName: configuration.Spec.AzureName,
				value:     *configuration.Spec.Value,
			}
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(struct {
				azureName string
				value     string
			}{
				azureName: "shared_preload_libraries",
				value:     "pg_cron,pg_stat_statements,pgaudit",
			}))
	})

	It("updates FlexibleServersConfiguration resources when extensions change", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-extensions-update",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
				EnableExtensions: []storagev1alpha1.DatabaseExtension{
					storagev1alpha1.DatabaseExtensionHstore,
				},
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		extensionsName := extensionsConfigResourceName(db.Name)
		sharedPreloadName := sharedPreloadLibrariesConfigResourceName(db.Name)

		Eventually(func(g Gomega) string {
			var configuration dbforpostgresqlv1.FlexibleServersConfiguration
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      extensionsName,
				Namespace: db.Namespace,
			}, &configuration)).To(Succeed())
			g.Expect(configuration.Spec.Value).NotTo(BeNil())
			return *configuration.Spec.Value
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal("hstore"))

		Eventually(func(g Gomega) string {
			var configuration dbforpostgresqlv1.FlexibleServersConfiguration
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      sharedPreloadName,
				Namespace: db.Namespace,
			}, &configuration)).To(Succeed())
			g.Expect(configuration.Spec.Value).NotTo(BeNil())
			return *configuration.Spec.Value
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(BeEmpty())

		var updated storagev1alpha1.Database
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, &updated)).To(Succeed())
		updated.Spec.EnableExtensions = []storagev1alpha1.DatabaseExtension{
			storagev1alpha1.DatabaseExtensionHstore,
			storagev1alpha1.DatabaseExtensionPgCron,
		}
		Expect(k8sClient.Update(ctx, &updated)).To(Succeed())

		Eventually(func(g Gomega) string {
			var configuration dbforpostgresqlv1.FlexibleServersConfiguration
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      extensionsName,
				Namespace: db.Namespace,
			}, &configuration)).To(Succeed())
			g.Expect(configuration.Spec.Value).NotTo(BeNil())
			return *configuration.Spec.Value
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal("hstore,pg_cron"))

		Eventually(func(g Gomega) string {
			var configuration dbforpostgresqlv1.FlexibleServersConfiguration
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      sharedPreloadName,
				Namespace: db.Namespace,
			}, &configuration)).To(Succeed())
			g.Expect(configuration.Spec.Value).NotTo(BeNil())
			return *configuration.Spec.Value
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal("pg_cron"))

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: db.Name, Namespace: db.Namespace}, &updated)).To(Succeed())
		updated.Spec.EnableExtensions = nil
		Expect(k8sClient.Update(ctx, &updated)).To(Succeed())

		Eventually(func(g Gomega) string {
			var configuration dbforpostgresqlv1.FlexibleServersConfiguration
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      extensionsName,
				Namespace: db.Namespace,
			}, &configuration)).To(Succeed())
			g.Expect(configuration.Spec.Value).NotTo(BeNil())
			return *configuration.Spec.Value
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(BeEmpty())

		Eventually(func(g Gomega) string {
			var configuration dbforpostgresqlv1.FlexibleServersConfiguration
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      sharedPreloadName,
				Namespace: db.Namespace,
			}, &configuration)).To(Succeed())
			g.Expect(configuration.Spec.Value).NotTo(BeNil())
			return *configuration.Spec.Value
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(BeEmpty())
	})

	It("updates the FlexibleServer when database server storage spec changes", func() {
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
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
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

	It("clamps storage tier to the max supported for the requested size", func() {
		size := int32(32)
		requestedTier := "P80"
		expectedTier := "P50"

		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-psql-tier-clamp",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: directAuth(
					"admin-mi",
					"admin-mi-id",
					"admin-mi",
					"user-mi",
					"user-mi-id",
				),
				Storage: &storagev1alpha1.DatabaseStorageSpec{
					SizeGB: &size,
					Tier:   &requestedTier,
				},
			},
		}

		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		Eventually(func(g Gomega) string {
			var s dbforpostgresqlv1.FlexibleServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      db.Name,
				Namespace: db.Namespace,
			}, &s)).To(Succeed())
			g.Expect(s.Spec.Storage).NotTo(BeNil())
			g.Expect(s.Spec.Storage.Tier).NotTo(BeNil())
			return string(*s.Spec.Storage.Tier)
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Equal(expectedTier))
	})

	It("creates a FlexibleServersAdministrator for the database server", func() {
		db := &storagev1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-db-admin",
				Namespace: "default",
			},
			Spec: storagev1alpha1.DatabaseSpec{
				Version:    17,
				ServerType: "dev",
				Auth: directAuth(
					"admin",
					"admin-id",
					"admin",
					"user",
					"user-id",
				),
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
				Auth: directAuth(
					"admin-old",
					"admin-old-id",
					"admin-old",
					"user",
					"user-id",
				),
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
		updated.Spec.Auth.Admin.Identity.Name = "admin-new"
		updated.Spec.Auth.Admin.Identity.PrincipalId = "admin-new-id"
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

	It("does not create a database server-owned user provisioning Job from legacy user auth", func() {
		db := newDedicatedDatabaseServer("my-app-db-user-job-ignored", directAuth(
			"admin-mi",
			"admin-mi-id",
			"admin-mi",
			"user-mi",
			"user-mi-id",
		))
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		markASOReady(ctx, db)

		Consistently(func(g Gomega) int {
			var jobs batchv1.JobList
			g.Expect(k8sClient.List(ctx, &jobs,
				client.InNamespace(db.Namespace),
				client.MatchingLabels(map[string]string{
					"dis.altinn.cloud/database-name":  db.Name,
					"dis.altinn.cloud/user-provision": "true",
				}),
			)).To(Succeed())
			return len(jobs.Items)
		}).WithTimeout(3 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(0))
	})

	It("resolves ApplicationIdentity references for server admin", func() {
		createApplicationIdentity(ctx, "adminidentity", ns, "admin-mi", "admin-mi-id")

		db := newDedicatedDatabaseServer("my-app-db-appid-ref", adminIdentityRefAuth("adminidentity"))
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		adminName := fmt.Sprintf("%s-admin", db.Name)

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
				azureName:     "admin-mi-id",
				principalName: "admin-mi",
			}))
	})

	It("creates a FlexibleServersDatabase and publishes LogicalDatabase status", func() {
		db := newSharedDatabaseServer("shared-db-logical-valid")
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		logicalDatabase := newLogicalDatabase("router-valid", db.Name)
		Expect(k8sClient.Create(ctx, logicalDatabase)).To(Succeed())

		expectedDatabaseName := expectedLogicalDatabaseName(logicalDatabase)
		expectedResourceName := logicalDatabaseASOResourceName(db.Name, expectedDatabaseName)
		Expect(expectedResourceName).To(Equal(fmt.Sprintf("%s-%s", db.Name, expectedDatabaseName)))

		Eventually(func(g Gomega) dbforpostgresqlv1.FlexibleServersDatabase_Spec {
			var asoDatabase dbforpostgresqlv1.FlexibleServersDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      expectedResourceName,
				Namespace: logicalDatabase.Namespace,
			}, &asoDatabase)).To(Succeed())
			g.Expect(asoDatabase.Spec.AzureName).To(Equal(expectedDatabaseName))
			g.Expect(asoDatabase.Spec.Owner).NotTo(BeNil())
			g.Expect(asoDatabase.Spec.Owner.Name).To(Equal(db.Name))
			g.Expect(asoDatabase.Spec.Charset).To(BeNil())
			g.Expect(asoDatabase.Spec.Collation).To(BeNil())
			g.Expect(asoDatabase.Labels).To(HaveKeyWithValue(databaseNameLabelKey, db.Name))
			g.Expect(asoDatabase.Labels).To(HaveKeyWithValue(logicalDatabaseLabelKey, logicalDatabase.Name))
			g.Expect(asoDatabase.Annotations).To(HaveKeyWithValue(
				annotations.ReconcilePolicy,
				string(annotations.ReconcilePolicyDetachOnDelete),
			))
			return asoDatabase.Spec
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			ShouldNot(BeZero())

		markLogicalDatabaseASOReady(ctx, logicalDatabase)

		Eventually(func(g Gomega) []storagev1alpha1.LogicalDatabaseValidationError {
			var updated storagev1alpha1.LogicalDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      logicalDatabase.Name,
				Namespace: logicalDatabase.Namespace,
			}, &updated)).To(Succeed())
			g.Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))
			g.Expect(updated.Status.DatabaseName).To(Equal(expectedDatabaseName))
			g.Expect(updated.Status.Host).To(Equal(fmt.Sprintf("%s.postgres.database.azure.com", db.Name)))
			g.Expect(updated.Status.Port).To(Equal(logicalDatabasePort))

			ready := meta.FindStatusCondition(updated.Status.Conditions, logicalDatabaseConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(logicalDatabaseReasonProvisioning))

			databaseReady := meta.FindStatusCondition(updated.Status.Conditions, logicalDatabaseConditionDatabaseReady)
			g.Expect(databaseReady).NotTo(BeNil())
			g.Expect(databaseReady.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(databaseReady.Reason).To(Equal(logicalDatabaseReasonDatabaseReady))

			accessReady := meta.FindStatusCondition(updated.Status.Conditions, logicalDatabaseConditionAccessReady)
			g.Expect(accessReady).NotTo(BeNil())
			g.Expect(accessReady.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(accessReady.Reason).To(Equal(logicalDatabaseReasonProvisioning))

			return updated.Status.ValidationErrors
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(BeEmpty())

		job := waitForLogicalAccessJob(ctx, logicalDatabase.Name, logicalDatabase.Namespace)
		Expect(job.Labels).To(HaveKeyWithValue(databaseNameLabelKey, db.Name))
		Expect(job.Labels).To(HaveKeyWithValue(logicalDatabaseLabelKey, logicalDatabase.Name))
		Expect(job.Spec.Template.Labels["azure.workload.identity/use"]).To(Equal("true"))
		Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(db.Spec.Auth.Admin.ServiceAccountName))
		Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_DATABASE_NAME", Value: db.Name},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_DB_HOST", Value: fmt.Sprintf("%s.postgres.database.azure.com", db.Name)},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_DB_NAME", Value: expectedDatabaseName},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_DB_SCHEMA", Value: expectedDatabaseName},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_APP_IDENTITY_NAME", Value: logicalDatabase.Spec.Access.App.Name},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_APP_IDENTITY_ID", Value: logicalDatabase.Spec.Access.App.PrincipalId},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_OWNER_IDENTITY_NAME", Value: logicalDatabase.Spec.Access.Owner.Name},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_OWNER_IDENTITY_ID", Value: logicalDatabase.Spec.Access.Owner.PrincipalId},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_REVOKE_PUBLIC_CONNECT", Value: "1"},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_DB_SEARCH_PATH_SCOPE", Value: "database"},
		))
	})

	It("reports NotFound when LogicalDatabase server does not exist", func() {
		logicalDatabase := newLogicalDatabase("router-missing-server", "missing-shared-db")
		Expect(k8sClient.Create(ctx, logicalDatabase)).To(Succeed())

		Eventually(func(g Gomega) string {
			var updated storagev1alpha1.LogicalDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      logicalDatabase.Name,
				Namespace: logicalDatabase.Namespace,
			}, &updated)).To(Succeed())
			g.Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))

			ready := meta.FindStatusCondition(updated.Status.Conditions, logicalDatabaseConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(logicalDatabaseReasonValidationFailed))

			for _, validationError := range updated.Status.ValidationErrors {
				if validationError.Field == logicalDatabaseValidationFieldServerName {
					return validationError.Reason
				}
			}
			return ""
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(logicalDatabaseValidationReasonNotFound))

		Consistently(func(g Gomega) []dbforpostgresqlv1.FlexibleServersDatabase {
			return listLogicalDatabaseASOChildren(g, logicalDatabase.Name)
		}).WithTimeout(2 * time.Second).WithPolling(250 * time.Millisecond).
			Should(BeEmpty())
	})

	It("creates a LogicalDatabase and access Job on a dedicated database server", func() {
		db := newDedicatedDatabaseServer("dedicated-db-logical-valid", adminAuth(
			"admin-mi",
			"admin-mi-id",
			"admin-mi",
		))
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		logicalDatabase := newLogicalDatabase("router-dedicated-server", db.Name)
		Expect(k8sClient.Create(ctx, logicalDatabase)).To(Succeed())

		expectedDatabaseName := expectedLogicalDatabaseName(logicalDatabase)
		expectedResourceName := logicalDatabaseASOResourceName(db.Name, expectedDatabaseName)

		Eventually(func(g Gomega) dbforpostgresqlv1.FlexibleServersDatabase_Spec {
			var asoDatabase dbforpostgresqlv1.FlexibleServersDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      expectedResourceName,
				Namespace: logicalDatabase.Namespace,
			}, &asoDatabase)).To(Succeed())
			g.Expect(asoDatabase.Spec.AzureName).To(Equal(expectedDatabaseName))
			g.Expect(asoDatabase.Spec.Owner).NotTo(BeNil())
			g.Expect(asoDatabase.Spec.Owner.Name).To(Equal(db.Name))
			g.Expect(asoDatabase.Labels).To(HaveKeyWithValue(databaseNameLabelKey, db.Name))
			g.Expect(asoDatabase.Labels).To(HaveKeyWithValue(logicalDatabaseLabelKey, logicalDatabase.Name))
			return asoDatabase.Spec
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			ShouldNot(BeZero())

		markLogicalDatabaseASOReady(ctx, logicalDatabase)

		job := waitForLogicalAccessJob(ctx, logicalDatabase.Name, logicalDatabase.Namespace)
		Expect(job.Labels).To(HaveKeyWithValue(databaseNameLabelKey, db.Name))
		Expect(job.Labels).To(HaveKeyWithValue(logicalDatabaseLabelKey, logicalDatabase.Name))
		Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(db.Spec.Auth.Admin.ServiceAccountName))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_DATABASE_NAME", Value: db.Name},
		))
		Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "DISPG_DB_NAME", Value: expectedDatabaseName},
		))
	})

	It("rejects LogicalDatabase name and server references with surrounding whitespace", func() {
		db := newSharedDatabaseServer("shared-db-logical-whitespace")
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		logicalDatabase := newLogicalDatabase("router-whitespace", db.Name)
		logicalDatabase.Spec.Name = " router-whitespace "
		logicalDatabase.Spec.Server.Name = " " + db.Name + " "
		Expect(k8sClient.Create(ctx, logicalDatabase)).To(Succeed())

		Eventually(func(g Gomega) map[string]string {
			var updated storagev1alpha1.LogicalDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      logicalDatabase.Name,
				Namespace: logicalDatabase.Namespace,
			}, &updated)).To(Succeed())

			reasons := make(map[string]string, len(updated.Status.ValidationErrors))
			for _, validationError := range updated.Status.ValidationErrors {
				reasons[validationError.Field] = validationError.Reason
			}
			return reasons
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(HaveKeyWithValue(logicalDatabaseValidationFieldSpecName, logicalDatabaseValidationReasonInvalid))

		Eventually(func(g Gomega) map[string]string {
			var updated storagev1alpha1.LogicalDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      logicalDatabase.Name,
				Namespace: logicalDatabase.Namespace,
			}, &updated)).To(Succeed())

			reasons := make(map[string]string, len(updated.Status.ValidationErrors))
			for _, validationError := range updated.Status.ValidationErrors {
				reasons[validationError.Field] = validationError.Reason
			}
			return reasons
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(HaveKeyWithValue(logicalDatabaseValidationFieldServerName, logicalDatabaseValidationReasonInvalid))

		Consistently(func(g Gomega) []dbforpostgresqlv1.FlexibleServersDatabase {
			return listLogicalDatabaseASOChildren(g, logicalDatabase.Name)
		}).WithTimeout(2 * time.Second).WithPolling(250 * time.Millisecond).
			Should(BeEmpty())
	})

	It("revalidates LogicalDatabase when the referenced Database server is created later", func() {
		const serverName = "shared-db-created-later"
		logicalDatabase := newLogicalDatabase("router-late-server", serverName)
		Expect(k8sClient.Create(ctx, logicalDatabase)).To(Succeed())

		Eventually(func(g Gomega) string {
			var updated storagev1alpha1.LogicalDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      logicalDatabase.Name,
				Namespace: logicalDatabase.Namespace,
			}, &updated)).To(Succeed())
			for _, validationError := range updated.Status.ValidationErrors {
				if validationError.Field == logicalDatabaseValidationFieldServerName {
					return validationError.Reason
				}
			}
			return ""
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(logicalDatabaseValidationReasonNotFound))

		Expect(k8sClient.Create(ctx, newSharedDatabaseServer(serverName))).To(Succeed())

		Eventually(func(g Gomega) []storagev1alpha1.LogicalDatabaseValidationError {
			var updated storagev1alpha1.LogicalDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      logicalDatabase.Name,
				Namespace: logicalDatabase.Namespace,
			}, &updated)).To(Succeed())
			return updated.Status.ValidationErrors
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(BeEmpty())

		Eventually(func(g Gomega) int {
			return len(listLogicalDatabaseASOChildren(g, logicalDatabase.Name))
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(1))
	})

	It("allows the same database name on different shared database servers", func() {
		db1 := newSharedDatabaseServer("shared-db-logical-one")
		db2 := newSharedDatabaseServer("shared-db-logical-two")
		Expect(k8sClient.Create(ctx, db1)).To(Succeed())
		Expect(k8sClient.Create(ctx, db2)).To(Succeed())

		logicalDatabase1 := newLogicalDatabase("router-one", db1.Name)
		logicalDatabase2 := newLogicalDatabase("router-two", db2.Name)
		logicalDatabase2.Spec.Name = logicalDatabase1.Spec.Name
		Expect(k8sClient.Create(ctx, logicalDatabase1)).To(Succeed())
		Expect(k8sClient.Create(ctx, logicalDatabase2)).To(Succeed())

		expectedDatabaseName1 := expectedLogicalDatabaseName(logicalDatabase1)
		expectedDatabaseName2 := expectedLogicalDatabaseName(logicalDatabase2)
		Eventually(func(g Gomega) map[string]string {
			logicalDatabases := append(
				listLogicalDatabaseASOChildren(g, logicalDatabase1.Name),
				listLogicalDatabaseASOChildren(g, logicalDatabase2.Name)...,
			)
			azureNamesByResource := make(map[string]string, len(logicalDatabases))
			for _, asoDatabase := range logicalDatabases {
				azureNamesByResource[asoDatabase.Name] = asoDatabase.Spec.AzureName
			}
			return azureNamesByResource
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(map[string]string{
				logicalDatabaseASOResourceName(db1.Name, expectedDatabaseName1): expectedDatabaseName1,
				logicalDatabaseASOResourceName(db2.Name, expectedDatabaseName2): expectedDatabaseName2,
			}))
	})

	It("reports Conflict when another LogicalDatabase manages the same database on the same server", func() {
		db := newSharedDatabaseServer("shared-db-logical-owner-guard")
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		firstLogicalDatabase := newLogicalDatabase("router-owner-one", db.Name)
		Expect(k8sClient.Create(ctx, firstLogicalDatabase)).To(Succeed())

		secondLogicalDatabase := newLogicalDatabase("router-owner-two", db.Name)
		secondLogicalDatabase.Spec.Name = firstLogicalDatabase.Spec.Name
		expectedDatabaseName := expectedLogicalDatabaseName(secondLogicalDatabase)
		expectedResourceName := logicalDatabaseASOResourceName(db.Name, expectedDatabaseName)

		var firstUpdated storagev1alpha1.LogicalDatabase
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      firstLogicalDatabase.Name,
				Namespace: firstLogicalDatabase.Namespace,
			}, &firstUpdated)).To(Succeed())
			var cachedASODatabase dbforpostgresqlv1.FlexibleServersDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      expectedResourceName,
				Namespace: firstLogicalDatabase.Namespace,
			}, &cachedASODatabase)).To(Succeed())
			g.Expect(metav1.IsControlledBy(&cachedASODatabase, &firstUpdated)).To(BeTrue())
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Succeed())

		Expect(k8sClient.Create(ctx, secondLogicalDatabase)).To(Succeed())

		Eventually(func(g Gomega) string {
			var secondUpdated storagev1alpha1.LogicalDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      secondLogicalDatabase.Name,
				Namespace: secondLogicalDatabase.Namespace,
			}, &secondUpdated)).To(Succeed())
			g.Expect(secondUpdated.Status.DatabaseName).To(BeEmpty())

			ready := meta.FindStatusCondition(secondUpdated.Status.Conditions, logicalDatabaseConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(logicalDatabaseReasonValidationFailed))

			for _, validationError := range secondUpdated.Status.ValidationErrors {
				if validationError.Field == logicalDatabaseValidationFieldSpecName {
					g.Expect(validationError.Message).To(ContainSubstring(firstLogicalDatabase.Name))
					return validationError.Reason
				}
			}
			return ""
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(logicalDatabaseValidationReasonConflict))

		var existingASODatabase dbforpostgresqlv1.FlexibleServersDatabase
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      expectedResourceName,
			Namespace: firstLogicalDatabase.Namespace,
		}, &existingASODatabase)).To(Succeed())
		Expect(existingASODatabase.Labels).To(HaveKeyWithValue(logicalDatabaseLabelKey, firstLogicalDatabase.Name))
		Expect(metav1.IsControlledBy(&existingASODatabase, &firstUpdated)).To(BeTrue())

		fixedDatabaseName := "router-owner-two-fixed"
		Eventually(func() error {
			var secondUpdated storagev1alpha1.LogicalDatabase
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      secondLogicalDatabase.Name,
				Namespace: secondLogicalDatabase.Namespace,
			}, &secondUpdated); err != nil {
				return err
			}
			secondUpdated.Spec.Name = fixedDatabaseName
			return k8sClient.Update(ctx, &secondUpdated)
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Succeed())

		Eventually(func(g Gomega) []storagev1alpha1.LogicalDatabaseValidationError {
			var secondUpdated storagev1alpha1.LogicalDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      secondLogicalDatabase.Name,
				Namespace: secondLogicalDatabase.Namespace,
			}, &secondUpdated)).To(Succeed())
			g.Expect(secondUpdated.Status.DatabaseName).To(Equal(fixedDatabaseName))
			return secondUpdated.Status.ValidationErrors
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(BeEmpty())

		var secondASODatabase dbforpostgresqlv1.FlexibleServersDatabase
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      logicalDatabaseASOResourceName(db.Name, fixedDatabaseName),
			Namespace: secondLogicalDatabase.Namespace,
		}, &secondASODatabase)).To(Succeed())
		Expect(secondASODatabase.Spec.AzureName).To(Equal(fixedDatabaseName))
		Expect(secondASODatabase.Labels).To(HaveKeyWithValue(logicalDatabaseLabelKey, secondLogicalDatabase.Name))
	})

	It("sets LogicalDatabase Ready after the access provisioning Job completes", func() {
		db := newSharedDatabaseServer("shared-db-logical-access-ready")
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		logicalDatabase := newLogicalDatabase("router-access-ready", db.Name)
		Expect(k8sClient.Create(ctx, logicalDatabase)).To(Succeed())

		Eventually(func(g Gomega) int {
			return len(listLogicalDatabaseASOChildren(g, logicalDatabase.Name))
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(1))

		markLogicalDatabaseASOReady(ctx, logicalDatabase)

		job := waitForLogicalAccessJob(ctx, logicalDatabase.Name, logicalDatabase.Namespace)

		Eventually(func() error {
			var accessJob batchv1.Job
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      job.Name,
				Namespace: job.Namespace,
			}, &accessJob); err != nil {
				return err
			}
			now := metav1.Now()
			accessJob.Status.StartTime = &now
			accessJob.Status.CompletionTime = &now
			accessJob.Status.Succeeded = 1
			accessJob.Status.Conditions = []batchv1.JobCondition{
				{
					Type:               batchv1.JobSuccessCriteriaMet,
					Status:             corev1.ConditionTrue,
					Reason:             "Completed",
					LastTransitionTime: now,
				},
				{
					Type:               batchv1.JobComplete,
					Status:             corev1.ConditionTrue,
					Reason:             "Completed",
					LastTransitionTime: now,
				},
			}
			return k8sClient.Status().Update(ctx, &accessJob)
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Succeed())

		Eventually(func(g Gomega) metav1.ConditionStatus {
			var updated storagev1alpha1.LogicalDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      logicalDatabase.Name,
				Namespace: logicalDatabase.Namespace,
			}, &updated)).To(Succeed())

			accessReady := meta.FindStatusCondition(updated.Status.Conditions, logicalDatabaseConditionAccessReady)
			g.Expect(accessReady).NotTo(BeNil())
			g.Expect(accessReady.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(accessReady.Reason).To(Equal(logicalDatabaseReasonReady))

			ready := meta.FindStatusCondition(updated.Status.Conditions, logicalDatabaseConditionReady)
			g.Expect(ready).NotTo(BeNil())
			return ready.Status
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(metav1.ConditionTrue))
	})

	It("recreates the LogicalDatabase access provisioning Job when the current Job is failed", func() {
		db := newSharedDatabaseServer("shared-db-logical-access-job-failed")
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		logicalDatabase := newLogicalDatabase("router-access-job-failed", db.Name)
		Expect(k8sClient.Create(ctx, logicalDatabase)).To(Succeed())

		Eventually(func(g Gomega) int {
			return len(listLogicalDatabaseASOChildren(g, logicalDatabase.Name))
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(1))

		markLogicalDatabaseASOReady(ctx, logicalDatabase)

		oldJob := waitForLogicalAccessJob(ctx, logicalDatabase.Name, logicalDatabase.Namespace)
		oldUID := oldJob.UID

		Eventually(func() error {
			var failedJob batchv1.Job
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      oldJob.Name,
				Namespace: oldJob.Namespace,
			}, &failedJob); err != nil {
				return err
			}
			now := metav1.Now()
			failedJob.Status.StartTime = &now
			failedJob.Status.CompletionTime = nil
			failedJob.Status.Failed = 1
			failedJob.Status.Conditions = []batchv1.JobCondition{
				{
					Type:               batchv1.JobFailureTarget,
					Status:             corev1.ConditionTrue,
					Reason:             "BackoffLimitExceeded",
					LastTransitionTime: now,
				},
				{
					Type:               batchv1.JobFailed,
					Status:             corev1.ConditionTrue,
					Reason:             "BackoffLimitExceeded",
					LastTransitionTime: now,
				},
			}
			return k8sClient.Status().Update(ctx, &failedJob)
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			Should(Succeed())

		Eventually(func(g Gomega) types.UID {
			var recreatedJob batchv1.Job
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      oldJob.Name,
				Namespace: oldJob.Namespace,
			}, &recreatedJob)).To(Succeed())
			return recreatedJob.UID
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).
			ShouldNot(Equal(oldUID))
	})

	It("fails validation instead of creating a second database when spec.name changes", func() {
		db := newSharedDatabaseServer("shared-db-logical-name-change")
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		logicalDatabase := newLogicalDatabase("router", db.Name)
		Expect(k8sClient.Create(ctx, logicalDatabase)).To(Succeed())

		Eventually(func(g Gomega) string {
			var updated storagev1alpha1.LogicalDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      logicalDatabase.Name,
				Namespace: logicalDatabase.Namespace,
			}, &updated)).To(Succeed())
			return updated.Status.DatabaseName
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(expectedLogicalDatabaseName(logicalDatabase)))

		Eventually(func() error {
			var updated storagev1alpha1.LogicalDatabase
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      logicalDatabase.Name,
				Namespace: logicalDatabase.Namespace,
			}, &updated); err != nil {
				return err
			}
			updated.Spec.Name = "renamed-db"
			return k8sClient.Update(ctx, &updated)
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Succeed())

		Eventually(func(g Gomega) string {
			var updated storagev1alpha1.LogicalDatabase
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      logicalDatabase.Name,
				Namespace: logicalDatabase.Namespace,
			}, &updated)).To(Succeed())
			for _, validationError := range updated.Status.ValidationErrors {
				if validationError.Field == logicalDatabaseValidationFieldDatabaseName {
					return validationError.Reason
				}
			}
			return ""
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(logicalDatabaseValidationReasonImmutable))

		Eventually(func(g Gomega) []dbforpostgresqlv1.FlexibleServersDatabase {
			return listLogicalDatabaseASOChildren(g, logicalDatabase.Name)
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(HaveLen(1))
	})

	It("defaults LogicalDatabase deletionPolicy to Retain", func() {
		db := newSharedDatabaseServer("shared-db-logical-default-policy")
		Expect(k8sClient.Create(ctx, db)).To(Succeed())

		logicalDatabase := newLogicalDatabase("router-default-policy", db.Name)
		Expect(logicalDatabase.Spec.DeletionPolicy).To(BeEmpty())
		Expect(k8sClient.Create(ctx, logicalDatabase)).To(Succeed())

		var updated storagev1alpha1.LogicalDatabase
		Eventually(func(g Gomega) storagev1alpha1.LogicalDatabaseDeletionPolicy {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      logicalDatabase.Name,
				Namespace: logicalDatabase.Namespace,
			}, &updated)).To(Succeed())
			return updated.Spec.DeletionPolicy
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).
			Should(Equal(storagev1alpha1.LogicalDatabaseDeletionPolicyRetain))
	})

})
