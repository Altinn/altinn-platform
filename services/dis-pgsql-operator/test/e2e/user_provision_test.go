//go:build e2e
// +build e2e

/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"
)

var _ = Describe("User provisioning", Ordered, func() {
	const (
		dbName                     = "e2e-user-provision"
		explicitRetentionDBName    = "e2e-backup-retention-explicit"
		explicitHADBName           = "e2e-ha-explicit"
		explicitServerParamsDBName = "e2e-server-params-explicit"
		namespace                  = "default"
		adminIdentityRef           = "adminidentity"
		adminIdentity              = "adminidentity"
		adminPrincipal             = "adminidentity-principal-id"
		userIdentityRef            = "useridentity"
		userIdentity               = "user1"
		userPrincipalId            = "user1-principal-id"
		explicitBackupRetentionDay = 21
		explicitCustomServerParam  = "autovacuum_naptime"
		explicitCustomServerValue  = "15"
		logicalSharedDBName        = "e2e-shared-logical"
		logicalResourceName        = "e2e-router"
		logicalAppIdentity         = "e2e-logical-app"
		logicalAppPrincipalID      = "e2e-logical-app-principal-id"
		logicalOwnerIdentity       = "e2e-logical-owner"
		logicalOwnerPrincipalID    = "e2e-logical-owner-principal-id"
	)

	var manifestPath string

	BeforeAll(func() {
		By("cleaning up stale Database and Job resources from previous runs")
		deleteDatabaseAndProvisionJobs(dbName, namespace)
		deleteDatabaseAndProvisionJobs(explicitRetentionDBName, namespace)
		deleteDatabaseAndProvisionJobs(explicitHADBName, namespace)
		deleteDatabaseAndProvisionJobs(explicitServerParamsDBName, namespace)

		manifestPath = writeTestManifest(
			dbName,
			namespace,
			adminIdentityRef,
			userIdentityRef,
		)
		By("creating a Database custom resource with identity prerequisites")
		applyManifestWithIdentityPrerequisites(
			manifestPath,
			namespace,
			adminIdentityRef,
			adminIdentity,
			adminPrincipal,
			userIdentityRef,
			userIdentity,
			userPrincipalId,
			"Failed to apply Database manifest",
		)

	})

	AfterAll(func() {
		if manifestPath == "" {
			return
		}
		By("deleting the Database custom resource")
		cmd := exec.Command("kubectl", "delete", "-f", manifestPath, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
		_ = os.Remove(manifestPath)
	})

	It("provisions the user and schema in Postgres", func() {
		By("waiting for the user provisioning job to complete")
		labelSelector := fmt.Sprintf("dis.altinn.cloud/database-name=%s,dis.altinn.cloud/user-provision=true", dbName)
		Eventually(func() error {
			cmd := exec.Command(
				"kubectl", "wait",
				"--for=condition=complete",
				"job",
				"-l", labelSelector,
				"-n", namespace,
				"--timeout=20s",
			)
			_, err := utils.Run(cmd)
			return err
		}).WithTimeout(5*time.Minute).WithPolling(2*time.Second).
			Should(Succeed(), "User provisioning job did not complete")

		By("verifying the role exists in Postgres")
		output := runPostgresQuery("SELECT 1 FROM pg_roles WHERE rolname = '" + userIdentity + "';")
		Expect(strings.TrimSpace(output)).To(Equal("1"))

		By("verifying the schema exists in Postgres")
		output = runPostgresQuery("SELECT 1 FROM pg_namespace WHERE nspname = '" + dbName + "';")
		Expect(strings.TrimSpace(output)).To(Equal("1"))

		By("verifying the user can create tables in its schema")
		tableName := fmt.Sprintf("%s.e2e_check", quoteIdentifier(dbName))
		_, err := runPostgresQueryAsUser(userIdentity, fmt.Sprintf(
			"CREATE TABLE %s (id int); DROP TABLE %s;",
			tableName,
			tableName,
		))
		Expect(err).NotTo(HaveOccurred())
	})

	It("provisions LogicalDatabase app and owner access in Postgres", func() {
		expectedDatabaseName := logicalResourceName
		manifestPath := writeLogicalDatabaseTestManifest(
			logicalSharedDBName,
			logicalResourceName,
			namespace,
			adminIdentityRef,
			logicalAppIdentity,
			logicalAppPrincipalID,
			logicalOwnerIdentity,
			logicalOwnerPrincipalID,
		)

		defer func() {
			deleteLogicalDatabaseAndProvisionJobs(logicalResourceName, namespace)
			deleteDatabaseAndProvisionJobs(logicalSharedDBName, namespace)
			_ = os.Remove(manifestPath)
			cleanupLogicalPostgresResources(expectedDatabaseName, logicalAppIdentity, logicalOwnerIdentity)
		}()

		By("cleaning up stale LogicalDatabase resources from previous runs")
		deleteLogicalDatabaseAndProvisionJobs(logicalResourceName, namespace)
		deleteDatabaseAndProvisionJobs(logicalSharedDBName, namespace)
		cleanupLogicalPostgresResources(expectedDatabaseName, logicalAppIdentity, logicalOwnerIdentity)

		By("creating a shared Database and LogicalDatabase")
		cmd := exec.Command("kubectl", "apply", "-f", manifestPath)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply LogicalDatabase manifest")
		patchApplicationIdentityStatus(adminIdentityRef, namespace, adminIdentity, adminPrincipal)

		By("waiting for the ASO logical database child")
		asoDatabaseName := waitForLogicalDatabaseASOResource(logicalResourceName, namespace)

		By("creating the real logical database in local Postgres")
		runPostgresQuery(fmt.Sprintf(
			"CREATE DATABASE %s;",
			quoteIdentifier(expectedDatabaseName),
		))

		By("marking ASO resources ready for the local Postgres stand-in")
		patchFlexibleServerReady(logicalSharedDBName, namespace, "postgres.default.svc")
		patchFlexibleServersDatabaseReady(asoDatabaseName, namespace)

		By("waiting for the LogicalDatabase access provisioning job to complete")
		labelSelector := fmt.Sprintf("dis.altinn.cloud/logical-database-name=%s,dis.altinn.cloud/user-provision=true", logicalResourceName)
		Eventually(func() error {
			cmd := exec.Command(
				"kubectl", "wait",
				"--for=condition=complete",
				"job",
				"-l", labelSelector,
				"-n", namespace,
				"--timeout=20s",
			)
			_, err := utils.Run(cmd)
			return err
		}).WithTimeout(5*time.Minute).WithPolling(2*time.Second).
			Should(Succeed(), "LogicalDatabase access provisioning job did not complete")

		By("verifying LogicalDatabase Ready status")
		Eventually(func(g Gomega) string {
			cmd := exec.Command(
				"kubectl", "get",
				"logicaldatabases.storage.dis.altinn.cloud",
				logicalResourceName,
				"-n", namespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			return strings.TrimSpace(output)
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(Equal("True"))

		By("verifying app and owner roles exist in Postgres")
		output := runPostgresQuery("SELECT 1 FROM pg_roles WHERE rolname = '" + logicalAppIdentity + "';")
		Expect(strings.TrimSpace(output)).To(Equal("1"))
		output = runPostgresQuery("SELECT 1 FROM pg_roles WHERE rolname = '" + logicalOwnerIdentity + "';")
		Expect(strings.TrimSpace(output)).To(Equal("1"))

		By("verifying the app can create tables in the logical database schema")
		_, err = runPostgresQueryAsUserInDatabase(logicalAppIdentity, expectedDatabaseName,
			"CREATE TABLE e2e_app_table (id int PRIMARY KEY, value text); INSERT INTO e2e_app_table VALUES (1, 'app');")
		Expect(err).NotTo(HaveOccurred())

		By("verifying the owner can use app-created objects and create objects in the schema")
		_, err = runPostgresQueryAsUserInDatabase(logicalOwnerIdentity, expectedDatabaseName,
			"INSERT INTO e2e_app_table VALUES (2, 'owner'); CREATE TABLE e2e_owner_table (id int); DROP TABLE e2e_owner_table;")
		Expect(err).NotTo(HaveOccurred())
		output, err = runPostgresQueryAsUserInDatabase(logicalOwnerIdentity, expectedDatabaseName,
			"SELECT count(*) FROM e2e_app_table;")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(output)).To(Equal("2"))

		By("verifying public connect was revoked")
		runPostgresQuery(`DROP ROLE IF EXISTS "e2e-logical-intruder"; CREATE ROLE "e2e-logical-intruder" LOGIN;`)
		_, err = runPostgresQueryAsUserInDatabase("e2e-logical-intruder", expectedDatabaseName, "SELECT 1;")
		Expect(err).To(HaveOccurred())
	})

	It("applies storage settings to the FlexibleServer", func() {
		By("verifying the FlexibleServer storage size and tier")
		Eventually(func(g Gomega) struct {
			size string
			tier string
		} {
			cmd := exec.Command(
				"kubectl", "get",
				"flexibleservers.dbforpostgresql.azure.com",
				dbName,
				"-n", namespace,
				"-o", "jsonpath={.spec.storage.storageSizeGB},{.spec.storage.tier}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())

			parts := strings.Split(strings.TrimSpace(output), ",")
			g.Expect(parts).To(HaveLen(2))

			return struct {
				size string
				tier string
			}{
				size: parts[0],
				tier: parts[1],
			}
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(Equal(struct {
				size string
				tier string
			}{
				size: "32",
				tier: "P50",
			}))
	})

	It("defaults backupRetentionDays to 14 for dev server types", func() {
		By("verifying the FlexibleServer backup retention default")
		Eventually(func(g Gomega) string {
			cmd := exec.Command(
				"kubectl", "get",
				"flexibleservers.dbforpostgresql.azure.com",
				dbName,
				"-n", namespace,
				"-o", "jsonpath={.spec.backup.backupRetentionDays}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			return strings.TrimSpace(output)
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(Equal("14"))
	})

	It("applies explicit backupRetentionDays when set on Database", func() {
		manifestPath := writeTestManifestWithBackupRetention(
			explicitRetentionDBName,
			namespace,
			adminIdentityRef,
			userIdentityRef,
			explicitBackupRetentionDay,
		)

		defer func() {
			deleteDatabaseAndProvisionJobs(explicitRetentionDBName, namespace)
			_ = os.Remove(manifestPath)
		}()

		By("creating a Database custom resource with explicit backup retention")
		applyManifestWithIdentityPrerequisites(
			manifestPath,
			namespace,
			adminIdentityRef,
			adminIdentity,
			adminPrincipal,
			userIdentityRef,
			userIdentity,
			userPrincipalId,
			"Failed to apply Database manifest with explicit backup retention",
		)

		By("verifying the Database spec keeps the explicit backup retention")
		Eventually(func(g Gomega) string {
			cmd := exec.Command(
				"kubectl", "get",
				"databases.storage.dis.altinn.cloud",
				explicitRetentionDBName,
				"-n", namespace,
				"-o", "jsonpath={.spec.backupRetentionDays}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			return strings.TrimSpace(output)
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(Equal(fmt.Sprintf("%d", explicitBackupRetentionDay)))

		By("verifying the explicit backup retention is applied to FlexibleServer")
		Eventually(func(g Gomega) string {
			cmd := exec.Command(
				"kubectl", "get",
				"flexibleservers.dbforpostgresql.azure.com",
				explicitRetentionDBName,
				"-n", namespace,
				"-o", "jsonpath={.spec.backup.backupRetentionDays}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			return strings.TrimSpace(output)
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(Equal(fmt.Sprintf("%d", explicitBackupRetentionDay)))
	})

	It("applies explicit highAvailabilityEnabled to the FlexibleServer", func() {
		manifestPath := writeTestManifestWithHighAvailability(
			explicitHADBName,
			namespace,
			adminIdentityRef,
			userIdentityRef,
			true,
		)

		defer func() {
			deleteDatabaseAndProvisionJobs(explicitHADBName, namespace)
			_ = os.Remove(manifestPath)
		}()

		By("creating a Database custom resource with explicit HA enabled")
		applyManifestWithIdentityPrerequisites(
			manifestPath,
			namespace,
			adminIdentityRef,
			adminIdentity,
			adminPrincipal,
			userIdentityRef,
			userIdentity,
			userPrincipalId,
			"Failed to apply Database manifest with explicit HA",
		)

		By("verifying the Database spec keeps explicit highAvailabilityEnabled")
		Eventually(func(g Gomega) string {
			cmd := exec.Command(
				"kubectl", "get",
				"databases.storage.dis.altinn.cloud",
				explicitHADBName,
				"-n", namespace,
				"-o", "jsonpath={.spec.highAvailabilityEnabled}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			return strings.TrimSpace(output)
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(Equal("true"))

		By("verifying explicit HA maps to ZoneRedundant mode and standby zone 2")
		Eventually(func(g Gomega) struct {
			mode        string
			standbyZone string
		} {
			cmd := exec.Command(
				"kubectl", "get",
				"flexibleservers.dbforpostgresql.azure.com",
				explicitHADBName,
				"-n", namespace,
				"-o", "jsonpath={.spec.highAvailability.mode},{.spec.highAvailability.standbyAvailabilityZone}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())

			parts := strings.Split(strings.TrimSpace(output), ",")
			g.Expect(parts).To(HaveLen(2))

			return struct {
				mode        string
				standbyZone string
			}{
				mode:        parts[0],
				standbyZone: parts[1],
			}
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(Equal(struct {
				mode        string
				standbyZone string
			}{
				mode:        "ZoneRedundant",
				standbyZone: "2",
			}))
	})

	It("creates fixed and user-defined server parameter configurations", func() {
		manifestPath := writeTestManifestWithServerParameters(
			explicitServerParamsDBName,
			namespace,
			adminIdentityRef,
			userIdentityRef,
		)

		defer func() {
			deleteDatabaseAndProvisionJobs(explicitServerParamsDBName, namespace)
			_ = os.Remove(manifestPath)
		}()

		By("creating a Database custom resource with explicit server parameters")
		applyManifestWithIdentityPrerequisites(
			manifestPath,
			namespace,
			adminIdentityRef,
			adminIdentity,
			adminPrincipal,
			userIdentityRef,
			userIdentity,
			userPrincipalId,
			"Failed to apply Database manifest with explicit server parameters",
		)

		By("verifying fixed and explicit server parameter configurations are created")
		expected := map[string]string{
			"pgbouncer.enabled":                 "true",
			"pgbouncer.max_prepared_statements": "5000",
			"pgbouncer.pool_mode":               "transaction",
			"max_connections":                   "50",
			explicitCustomServerParam:           explicitCustomServerValue,
		}

		Eventually(func(g Gomega) {
			cmd := exec.Command(
				"kubectl", "get",
				"flexibleserversconfigurations.dbforpostgresql.azure.com",
				"-n", namespace,
				"-l",
				fmt.Sprintf(
					"dis.altinn.cloud/database-name=%s,dis.altinn.cloud/configuration-kind=server-parameter",
					explicitServerParamsDBName,
				),
				"-o",
				"jsonpath={range .items[*]}{.spec.azureName}={.spec.value}{\"\\n\"}{end}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())

			actual := parseAzureConfigurationValues(output)
			g.Expect(actual).To(HaveLen(len(expected)))
			for name, value := range expected {
				g.Expect(actual).To(HaveKeyWithValue(name, value))
			}
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(Succeed())
	})
})

func writeTestManifest(dbName, namespace, adminIdentityRef, userIdentityRef string) string {
	return writeTestManifestWithBackupRetention(dbName, namespace, adminIdentityRef, userIdentityRef, 0)
}

func writeTestManifestWithBackupRetention(
	dbName, namespace, adminIdentityRef, userIdentityRef string,
	backupRetentionDays int,
) string {
	sizeGB := int32(32)
	tier := "P80"

	database := &storagev1alpha1.Database{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "Database",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.DatabaseSpec{
			Version:    17,
			ServerType: "dev",
			Storage: &storagev1alpha1.DatabaseStorageSpec{
				SizeGB: &sizeGB,
				Tier:   &tier,
			},
			Auth: storagev1alpha1.DatabaseAuth{
				Admin: storagev1alpha1.AdminIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: adminIdentityRef},
					},
				},
				User: &storagev1alpha1.UserIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: userIdentityRef},
					},
				},
			},
		},
	}
	if backupRetentionDays > 0 {
		database.Spec.BackupRetentionDays = &backupRetentionDays
	}

	return writeManifestWithApplicationIdentities(database, namespace, adminIdentityRef, userIdentityRef)
}

func writeTestManifestWithHighAvailability(
	dbName, namespace, adminIdentityRef, userIdentityRef string,
	highAvailabilityEnabled bool,
) string {
	sizeGB := int32(32)
	tier := "P80"

	database := &storagev1alpha1.Database{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "Database",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.DatabaseSpec{
			Version:                 17,
			ServerType:              "dev",
			HighAvailabilityEnabled: &highAvailabilityEnabled,
			Storage: &storagev1alpha1.DatabaseStorageSpec{
				SizeGB: &sizeGB,
				Tier:   &tier,
			},
			Auth: storagev1alpha1.DatabaseAuth{
				Admin: storagev1alpha1.AdminIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: adminIdentityRef},
					},
				},
				User: &storagev1alpha1.UserIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: userIdentityRef},
					},
				},
			},
		},
	}

	return writeManifestWithApplicationIdentities(database, namespace, adminIdentityRef, userIdentityRef)
}

func writeTestManifestWithServerParameters(
	dbName, namespace, adminIdentityRef, userIdentityRef string,
) string {
	sizeGB := int32(32)
	tier := "P80"

	database := &storagev1alpha1.Database{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "Database",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.DatabaseSpec{
			Version:    17,
			ServerType: "dev",
			ServerParams: []storagev1alpha1.DatabaseServerParameter{
				{
					Name:  "autovacuum_naptime",
					Value: intstr.FromInt(15),
				},
			},
			Storage: &storagev1alpha1.DatabaseStorageSpec{
				SizeGB: &sizeGB,
				Tier:   &tier,
			},
			Auth: storagev1alpha1.DatabaseAuth{
				Admin: storagev1alpha1.AdminIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: adminIdentityRef},
					},
				},
				User: &storagev1alpha1.UserIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: userIdentityRef},
					},
				},
			},
		},
	}

	return writeManifestWithApplicationIdentities(database, namespace, adminIdentityRef, userIdentityRef)
}

func writeManifestWithApplicationIdentities(
	database *storagev1alpha1.Database,
	namespace, adminIdentityRef, userIdentityRef string,
) string {

	adminIdentity := &identityv1alpha1.ApplicationIdentity{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "application.dis.altinn.cloud/v1alpha1",
			Kind:       "ApplicationIdentity",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      adminIdentityRef,
			Namespace: namespace,
		},
		Spec: identityv1alpha1.ApplicationIdentitySpec{},
	}

	userIdentity := &identityv1alpha1.ApplicationIdentity{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "application.dis.altinn.cloud/v1alpha1",
			Kind:       "ApplicationIdentity",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      userIdentityRef,
			Namespace: namespace,
		},
		Spec: identityv1alpha1.ApplicationIdentitySpec{},
	}

	resources := []interface{}{adminIdentity, userIdentity, database}
	docs := make([]string, 0, len(resources))
	for i := range resources {
		content, err := yaml.Marshal(resources[i])
		Expect(err).NotTo(HaveOccurred(), "Failed to marshal test manifest resource")
		docs = append(docs, string(content))
	}

	content := strings.Join(docs, "---\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	dir := os.TempDir()
	path := filepath.Join(dir, fmt.Sprintf("db-%s.yaml", database.Name))
	err := os.WriteFile(path, []byte(content), 0o600)
	Expect(err).NotTo(HaveOccurred(), "Failed to write temp manifest")
	return path
}

func writeLogicalDatabaseTestManifest(
	sharedDBName,
	logicalResourceName,
	namespace,
	adminIdentityRef,
	appIdentity,
	appPrincipalID,
	ownerIdentity,
	ownerPrincipalID string,
) string {
	adminIdentity := &identityv1alpha1.ApplicationIdentity{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "application.dis.altinn.cloud/v1alpha1",
			Kind:       "ApplicationIdentity",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      adminIdentityRef,
			Namespace: namespace,
		},
		Spec: identityv1alpha1.ApplicationIdentitySpec{},
	}

	sharedDatabase := &storagev1alpha1.Database{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "Database",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      sharedDBName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.DatabaseSpec{
			Mode:       storagev1alpha1.DatabaseModeShared,
			Version:    17,
			ServerType: "dev",
			Network: &storagev1alpha1.DatabaseNetworkSpec{
				DelegatedSubnetResourceID: "/subscriptions/fake-subscription/resourceGroups/rg-dis-admin-network/providers/Microsoft.Network/virtualNetworks/vnet-dis-admin-dbs/subnets/snet-postgres-shared",
				PrivateDNSZoneResourceID:  "/subscriptions/fake-subscription/resourceGroups/rg-dis-admin-network/providers/Microsoft.Network/privateDnsZones/shared.private.postgres.database.azure.com",
			},
			Auth: storagev1alpha1.DatabaseAuth{
				Admin: storagev1alpha1.AdminIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: adminIdentityRef},
					},
				},
			},
		},
	}

	logicalDatabase := &storagev1alpha1.LogicalDatabase{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "LogicalDatabase",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      logicalResourceName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.LogicalDatabaseSpec{
			Name: logicalResourceName,
			Server: storagev1alpha1.LogicalDatabaseServerSpec{
				Name: sharedDBName,
			},
			Access: storagev1alpha1.LogicalDatabaseAccessSpec{
				App: storagev1alpha1.LogicalDatabasePrincipalSpec{
					Name:        appIdentity,
					PrincipalId: appPrincipalID,
				},
				Owner: storagev1alpha1.LogicalDatabasePrincipalSpec{
					Name:        ownerIdentity,
					PrincipalId: ownerPrincipalID,
				},
			},
		},
	}

	resources := []interface{}{adminIdentity, sharedDatabase, logicalDatabase}
	docs := make([]string, 0, len(resources))
	for i := range resources {
		content, err := yaml.Marshal(resources[i])
		Expect(err).NotTo(HaveOccurred(), "Failed to marshal test manifest resource")
		docs = append(docs, string(content))
	}

	content := strings.Join(docs, "---\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	dir := os.TempDir()
	path := filepath.Join(dir, fmt.Sprintf("logical-db-%s.yaml", logicalResourceName))
	err := os.WriteFile(path, []byte(content), 0o600)
	Expect(err).NotTo(HaveOccurred(), "Failed to write temp manifest")
	return path
}

func patchApplicationIdentityStatus(identityRef, namespace, managedIdentityName, principalID string) {
	cmd := exec.Command(
		"kubectl", "patch",
		"applicationidentity", identityRef,
		"-n", namespace,
		"--subresource=status",
		"--type=merge",
		"-p", fmt.Sprintf(`{"status":{"managedIdentityName":"%s","principalId":"%s"}}`, managedIdentityName, principalID),
	)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to patch ApplicationIdentity status for %s", identityRef)
}

func applyManifestWithIdentityPrerequisites(
	manifestPath, namespace,
	adminIdentityRef, adminManagedIdentityName, adminPrincipalID,
	userIdentityRef, userManagedIdentityName, userPrincipalID,
	applyFailureMessage string,
) {
	cmd := exec.Command("kubectl", "apply", "-f", manifestPath)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), applyFailureMessage)

	patchApplicationIdentityStatus(adminIdentityRef, namespace, adminManagedIdentityName, adminPrincipalID)
	patchApplicationIdentityStatus(userIdentityRef, namespace, userManagedIdentityName, userPrincipalID)
}

func waitForLogicalDatabaseASOResource(logicalResourceName, namespace string) string {
	Eventually(func(g Gomega) string {
		cmd := exec.Command(
			"kubectl", "get",
			"flexibleserversdatabases.dbforpostgresql.azure.com",
			"-n", namespace,
			"-l", fmt.Sprintf("dis.altinn.cloud/logical-database-name=%s", logicalResourceName),
			"-o", "jsonpath={.items[0].metadata.name}",
		)
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		return strings.TrimSpace(output)
	}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
		ShouldNot(BeEmpty())

	cmd := exec.Command(
		"kubectl", "get",
		"flexibleserversdatabases.dbforpostgresql.azure.com",
		"-n", namespace,
		"-l", fmt.Sprintf("dis.altinn.cloud/logical-database-name=%s", logicalResourceName),
		"-o", "jsonpath={.items[0].metadata.name}",
	)
	output, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(output)
}

func patchFlexibleServerReady(serverName, namespace, host string) {
	transitionTime := time.Now().UTC().Format(time.RFC3339)
	patch := fmt.Sprintf(
		`{"status":{"fullyQualifiedDomainName":"%s","conditions":[{"type":"Ready","status":"True","reason":"Ready","message":"ready for e2e","lastTransitionTime":"%s"}]}}`,
		host,
		transitionTime,
	)
	Eventually(func() error {
		cmd := exec.Command(
			"kubectl", "patch",
			"flexibleservers.dbforpostgresql.azure.com", serverName,
			"-n", namespace,
			"--subresource=status",
			"--type=merge",
			"-p", patch,
		)
		_, err := utils.Run(cmd)
		return err
	}).WithTimeout(2*time.Minute).WithPolling(2*time.Second).
		Should(Succeed(), "Failed to patch FlexibleServer status for %s", serverName)
}

func patchFlexibleServersDatabaseReady(databaseResourceName, namespace string) {
	transitionTime := time.Now().UTC().Format(time.RFC3339)
	patch := fmt.Sprintf(
		`{"status":{"conditions":[{"type":"Ready","status":"True","reason":"Ready","message":"ready for e2e","lastTransitionTime":"%s"}]}}`,
		transitionTime,
	)
	Eventually(func() error {
		cmd := exec.Command(
			"kubectl", "patch",
			"flexibleserversdatabases.dbforpostgresql.azure.com", databaseResourceName,
			"-n", namespace,
			"--subresource=status",
			"--type=merge",
			"-p", patch,
		)
		_, err := utils.Run(cmd)
		return err
	}).WithTimeout(2*time.Minute).WithPolling(2*time.Second).
		Should(Succeed(), "Failed to patch FlexibleServersDatabase status for %s", databaseResourceName)
}

func runPostgresQuery(query string) string {
	output, err := runPostgresQueryAsUser("postgres", query)
	Expect(err).NotTo(HaveOccurred(), "Failed to run Postgres query")
	return output
}

func runPostgresQueryAsUser(user, query string) (string, error) {
	return runPostgresQueryAsUserInDatabase(user, "postgres", query)
}

func runPostgresQueryAsUserInDatabase(user, databaseName, query string) (string, error) {
	cmd := exec.Command("kubectl", "get", "pods", "-l", "app=postgres", "-n", "default", "-o", "jsonpath={.items[0].metadata.name}")
	podName, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to get Postgres pod name")
	podName = strings.TrimSpace(podName)

	cmd = exec.Command("kubectl", "exec", "-n", "default", podName, "--",
		"psql", "-v", "ON_ERROR_STOP=1", "-U", user, "-d", databaseName, "-tAc", query)
	output, err := utils.Run(cmd)
	if err != nil {
		return "", err
	}
	return output, nil
}

func quoteIdentifier(value string) string {
	escaped := strings.ReplaceAll(value, `"`, `""`)
	return `"` + escaped + `"`
}

func parseAzureConfigurationValues(output string) map[string]string {
	result := map[string]string{}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result
}

func deleteDatabaseAndProvisionJobs(dbName, namespace string) {
	cmd := exec.Command(
		"kubectl", "delete",
		"databases.storage.dis.altinn.cloud",
		dbName,
		"-n", namespace,
		"--ignore-not-found=true",
	)
	_, _ = utils.Run(cmd)

	labelSelector := fmt.Sprintf("dis.altinn.cloud/database-name=%s,dis.altinn.cloud/user-provision=true", dbName)
	cmd = exec.Command(
		"kubectl", "delete",
		"job",
		"-l", labelSelector,
		"-n", namespace,
		"--ignore-not-found=true",
	)
	_, _ = utils.Run(cmd)

	Eventually(func() string {
		cmd = exec.Command(
			"kubectl", "get",
			"job",
			"-l", labelSelector,
			"-n", namespace,
			"-o", "name",
		)
		output, err := utils.Run(cmd)
		if err != nil {
			return "error"
		}
		return strings.TrimSpace(output)
	}).WithTimeout(30 * time.Second).WithPolling(2 * time.Second).
		Should(BeEmpty())
}

func deleteLogicalDatabaseAndProvisionJobs(logicalDatabaseName, namespace string) {
	logicalDatabaseLabelSelector := fmt.Sprintf("dis.altinn.cloud/logical-database-name=%s", logicalDatabaseName)

	cmd := exec.Command(
		"kubectl", "delete",
		"logicaldatabases.storage.dis.altinn.cloud",
		logicalDatabaseName,
		"-n", namespace,
		"--ignore-not-found=true",
	)
	_, _ = utils.Run(cmd)

	jobLabelSelector := fmt.Sprintf("%s,dis.altinn.cloud/user-provision=true", logicalDatabaseLabelSelector)
	cmd = exec.Command(
		"kubectl", "delete",
		"job",
		"-l", jobLabelSelector,
		"-n", namespace,
		"--ignore-not-found=true",
	)
	_, _ = utils.Run(cmd)

	Eventually(func() string {
		cmd = exec.Command(
			"kubectl", "get",
			"logicaldatabases.storage.dis.altinn.cloud",
			logicalDatabaseName,
			"-n", namespace,
			"--ignore-not-found=true",
			"-o", "name",
		)
		output, err := utils.Run(cmd)
		if err != nil {
			return "error"
		}
		return strings.TrimSpace(output)
	}).WithTimeout(30 * time.Second).WithPolling(2 * time.Second).
		Should(BeEmpty())

	Eventually(func() string {
		cmd = exec.Command(
			"kubectl", "get",
			"flexibleserversdatabases.dbforpostgresql.azure.com",
			"-l", logicalDatabaseLabelSelector,
			"-n", namespace,
			"-o", "name",
		)
		output, err := utils.Run(cmd)
		if err != nil {
			return "error"
		}
		return strings.TrimSpace(output)
	}).WithTimeout(30 * time.Second).WithPolling(2 * time.Second).
		Should(BeEmpty())

	Eventually(func() string {
		cmd = exec.Command(
			"kubectl", "get",
			"job",
			"-l", jobLabelSelector,
			"-n", namespace,
			"-o", "name",
		)
		output, err := utils.Run(cmd)
		if err != nil {
			return "error"
		}
		return strings.TrimSpace(output)
	}).WithTimeout(30 * time.Second).WithPolling(2 * time.Second).
		Should(BeEmpty())
}

func cleanupLogicalPostgresResources(databaseName, appIdentity, ownerIdentity string) {
	_, _ = runPostgresQueryAsUser("postgres", fmt.Sprintf(
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s';",
		strings.ReplaceAll(databaseName, "'", "''"),
	))
	_, _ = runPostgresQueryAsUser("postgres", fmt.Sprintf("DROP DATABASE IF EXISTS %s;", quoteIdentifier(databaseName)))
	_, _ = runPostgresQueryAsUser("postgres", fmt.Sprintf("DROP ROLE IF EXISTS %s;", quoteIdentifier(appIdentity)))
	_, _ = runPostgresQueryAsUser("postgres", fmt.Sprintf("DROP ROLE IF EXISTS %s;", quoteIdentifier(ownerIdentity)))
	_, _ = runPostgresQueryAsUser("postgres", `DROP ROLE IF EXISTS "e2e-logical-intruder";`)
}
