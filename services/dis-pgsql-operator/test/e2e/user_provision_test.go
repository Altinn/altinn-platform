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
	"encoding/json"
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"
)

// provisionAdminUser is the non-superuser Postgres role the operator's
// provisioning Job connects as in Kind (useAzFakes). It must match
// kindProvisionAdminUser in internal/controller and the role bootstrapped by
// config/kind/postgres.yaml. App databases are owned by this role so the
// non-superuser admin can provision them, mirroring Azure's Entra admin.
const provisionAdminUser = "dispg_admin"

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
		userIdentity               = "user1"
		userPrincipalId            = "user1-principal-id"
		userOwnerIdentity          = "e2e-user-owner"
		userOwnerPrincipalID       = "11111111-1111-1111-1111-111111111111"
		explicitBackupRetentionDay = 21
		explicitCustomServerParam  = "autovacuum_naptime"
		explicitCustomServerValue  = "15"
		sharedDatabaseServerName   = "e2e-shared-database"
		databaseResourceName       = "e2e-router"
		databaseAppIdentity        = "e2e-database-app"
		databaseAppPrincipalID     = "e2e-database-app-principal-id"
		databaseOwnerIdentity      = "e2e-database-owner"
		databaseOwnerPrincipalID   = "22222222-2222-2222-2222-222222222222"
	)

	var manifestPath string

	BeforeAll(func() {
		By("cleaning up stale DatabaseServer and Job resources from previous runs")
		deleteDatabaseAndProvisionJobs(dbName, namespace)
		deleteDatabaseServerAndProvisionJobs(dbName, namespace)
		deleteDatabaseServerAndProvisionJobs(explicitRetentionDBName, namespace)
		deleteDatabaseServerAndProvisionJobs(explicitHADBName, namespace)
		deleteDatabaseServerAndProvisionJobs(explicitServerParamsDBName, namespace)

		manifestPath = writeTestManifest(
			dbName,
			namespace,
			adminIdentityRef,
		)
		By("creating a DatabaseServer custom resource with identity prerequisites")
		applyManifestWithIdentityPrerequisites(
			manifestPath,
			namespace,
			adminIdentityRef,
			adminIdentity,
			adminPrincipal,
			"Failed to apply DatabaseServer manifest",
		)

	})

	AfterAll(func() {
		if manifestPath == "" {
			return
		}
		By("deleting the DatabaseServer custom resource")
		cmd := exec.Command("kubectl", "delete", "-f", manifestPath, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
		_ = os.Remove(manifestPath)
	})

	It("provisions single-tenant Database app and owner access in Postgres", func() {
		expectedDatabaseName := dbName
		databaseManifestPath := writeDatabaseOnlyTestManifest(
			dbName,
			dbName,
			namespace,
			userIdentity,
			userPrincipalId,
			userOwnerIdentity,
			userOwnerPrincipalID,
		)

		defer func() {
			deleteDatabaseAndProvisionJobs(dbName, namespace)
			_ = os.Remove(databaseManifestPath)
			cleanupDatabasePostgresResources(expectedDatabaseName, userIdentity, userOwnerIdentity)
		}()

		By("cleaning up stale single-tenant Database resources from previous runs")
		deleteDatabaseAndProvisionJobs(dbName, namespace)
		cleanupDatabasePostgresResources(expectedDatabaseName, userIdentity, userOwnerIdentity)

		By("creating the child Database")
		cmd := exec.Command("kubectl", "apply", "-f", databaseManifestPath)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply Database manifest")
		patchApplicationIdentityStatus(userIdentity, namespace, userIdentity, userPrincipalId)

		By("waiting for the ASO database child")
		asoDatabaseName := waitForDatabaseASOResource(dbName, namespace)

		By("creating the real app database in local Postgres")
		runPostgresQuery(fmt.Sprintf(
			"CREATE DATABASE %s OWNER %s;",
			quoteIdentifier(expectedDatabaseName),
			quoteIdentifier(provisionAdminUser),
		))

		By("marking ASO resources ready for the local Postgres stand-in")
		patchFlexibleServerReady(dbName, namespace, "postgres.default.svc")
		patchFlexibleServersDatabaseReady(asoDatabaseName, namespace)

		By("waiting for the Database access provisioning job to complete")
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
			Should(Succeed(), "Database access provisioning job did not complete")

		By("verifying the role exists in Postgres")
		output := runPostgresQuery("SELECT 1 FROM pg_roles WHERE rolname = '" + userIdentity + "';")
		Expect(strings.TrimSpace(output)).To(Equal("1"))

		By("verifying the schema exists in the app database")
		output, err = runPostgresQueryAsUserInDatabase("postgres", expectedDatabaseName, fmt.Sprintf(
			"SELECT 1 FROM pg_namespace WHERE nspname = '%s';",
			strings.ReplaceAll(dbName, "'", "''"),
		))
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(output)).To(Equal("1"))

		By("verifying the user can create tables in its schema")
		tableName := fmt.Sprintf("%s.e2e_check", quoteIdentifier(dbName))
		_, err = runPostgresQueryAsUserInDatabase(userIdentity, expectedDatabaseName, fmt.Sprintf(
			"CREATE TABLE %s (id int); DROP TABLE %s;",
			tableName,
			tableName,
		))
		Expect(err).NotTo(HaveOccurred())
	})

	It("provisions Database app and owner access in Postgres", func() {
		expectedDatabaseName := databaseResourceName
		manifestPath := writeDatabaseTestManifest(
			sharedDatabaseServerName,
			databaseResourceName,
			namespace,
			adminIdentityRef,
			databaseAppIdentity,
			databaseAppPrincipalID,
			databaseOwnerIdentity,
			databaseOwnerPrincipalID,
		)

		defer func() {
			deleteDatabaseAndProvisionJobs(databaseResourceName, namespace)
			deleteDatabaseServerAndProvisionJobs(sharedDatabaseServerName, namespace)
			_ = os.Remove(manifestPath)
			cleanupDatabasePostgresResources(expectedDatabaseName, databaseAppIdentity, databaseOwnerIdentity)
		}()

		By("cleaning up stale Database resources from previous runs")
		deleteDatabaseAndProvisionJobs(databaseResourceName, namespace)
		deleteDatabaseServerAndProvisionJobs(sharedDatabaseServerName, namespace)
		cleanupDatabasePostgresResources(expectedDatabaseName, databaseAppIdentity, databaseOwnerIdentity)

		By("creating a shared DatabaseServer and Database")
		cmd := exec.Command("kubectl", "apply", "-f", manifestPath)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply Database manifest")
		patchApplicationIdentityStatus(adminIdentityRef, namespace, adminIdentity, adminPrincipal)
		patchApplicationIdentityStatus(databaseAppIdentity, namespace, databaseAppIdentity, databaseAppPrincipalID)

		By("waiting for the ASO database child")
		asoDatabaseName := waitForDatabaseASOResource(databaseResourceName, namespace)

		By("creating the real database in local Postgres")
		runPostgresQuery(fmt.Sprintf(
			"CREATE DATABASE %s OWNER %s;",
			quoteIdentifier(expectedDatabaseName),
			quoteIdentifier(provisionAdminUser),
		))

		By("marking ASO resources ready for the local Postgres stand-in")
		patchFlexibleServerReady(sharedDatabaseServerName, namespace, "postgres.default.svc")
		patchFlexibleServersDatabaseReady(asoDatabaseName, namespace)

		By("waiting for the Database access provisioning job to complete")
		labelSelector := fmt.Sprintf("dis.altinn.cloud/database-name=%s,dis.altinn.cloud/user-provision=true", databaseResourceName)
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
			Should(Succeed(), "Database access provisioning job did not complete")

		By("verifying Database Ready status")
		Eventually(func(g Gomega) string {
			cmd := exec.Command(
				"kubectl", "get",
				"databases.storage.dis.altinn.cloud",
				databaseResourceName,
				"-n", namespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			return strings.TrimSpace(output)
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(Equal("True"))

		By("verifying the connection ConfigMap is published for the app (service) principal")
		connConfigMapName := fmt.Sprintf("%s-%s-dis-pgsql", databaseResourceName, databaseAppIdentity)
		expectedURI := fmt.Sprintf(
			"postgresql://%s@postgres.default.svc:5432/%s?sslmode=require",
			databaseAppIdentity, expectedDatabaseName,
		)
		Eventually(func(g Gomega) {
			get := func(jsonpath string) string {
				cmd := exec.Command("kubectl", "get", "configmap", connConfigMapName, "-n", namespace, "-o", jsonpath)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				return strings.TrimSpace(output)
			}
			g.Expect(get("jsonpath={.data.host}")).To(Equal("postgres.default.svc"))
			g.Expect(get("jsonpath={.data.port}")).To(Equal("5432"))
			g.Expect(get("jsonpath={.data.dbname}")).To(Equal(expectedDatabaseName))
			g.Expect(get("jsonpath={.data.user}")).To(Equal(databaseAppIdentity))
			g.Expect(get("jsonpath={.data.sslmode}")).To(Equal("require"))
			g.Expect(get("jsonpath={.data.uri}")).To(Equal(expectedURI))
			g.Expect(get("jsonpath={.metadata.labels.pgsql\\.dis\\.altinn\\.cloud/principal}")).To(Equal(databaseAppIdentity))
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(Succeed())

		By("verifying the Owner group principal did not get a ConfigMap")
		Eventually(func(g Gomega) string {
			cmd := exec.Command(
				"kubectl", "get", "configmaps",
				"-n", namespace,
				"-l", "pgsql.dis.altinn.cloud/component=connection,pgsql.dis.altinn.cloud/database="+databaseResourceName,
				"-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			return strings.TrimSpace(output)
		}).WithTimeout(30 * time.Second).WithPolling(2 * time.Second).
			Should(Equal(connConfigMapName))

		By("verifying app and owner roles exist in Postgres")
		output := runPostgresQuery("SELECT 1 FROM pg_roles WHERE rolname = '" + databaseAppIdentity + "';")
		Expect(strings.TrimSpace(output)).To(Equal("1"))
		output = runPostgresQuery("SELECT 1 FROM pg_roles WHERE rolname = '" + databaseOwnerIdentity + "';")
		Expect(strings.TrimSpace(output)).To(Equal("1"))

		By("verifying the Writer app cannot create tables")
		_, err = runPostgresQueryAsUserInDatabase(databaseAppIdentity, expectedDatabaseName,
			"CREATE TABLE e2e_app_table (id int PRIMARY KEY, value text);")
		Expect(err).To(HaveOccurred())

		By("verifying the Owner group can create objects in the schema")
		_, err = runPostgresQueryAsUserInDatabase(databaseOwnerIdentity, expectedDatabaseName,
			"CREATE TABLE e2e_app_table (id int PRIMARY KEY, value text);")
		Expect(err).NotTo(HaveOccurred())

		By("verifying the Writer app can use owner-created objects")
		_, err = runPostgresQueryAsUserInDatabase(databaseAppIdentity, expectedDatabaseName,
			"INSERT INTO e2e_app_table VALUES (1, 'app');")
		Expect(err).NotTo(HaveOccurred())

		By("verifying the Owner group can use app-written objects")
		_, err = runPostgresQueryAsUserInDatabase(databaseOwnerIdentity, expectedDatabaseName,
			"INSERT INTO e2e_app_table VALUES (2, 'owner'); CREATE TABLE e2e_owner_table (id int); DROP TABLE e2e_owner_table;")
		Expect(err).NotTo(HaveOccurred())
		output, err = runPostgresQueryAsUserInDatabase(databaseOwnerIdentity, expectedDatabaseName,
			"SELECT count(*) FROM e2e_app_table;")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(output)).To(Equal("2"))

		By("verifying public connect was revoked")
		runPostgresQuery(`DROP ROLE IF EXISTS "e2e-database-intruder"; CREATE ROLE "e2e-database-intruder" LOGIN;`)
		_, err = runPostgresQueryAsUserInDatabase("e2e-database-intruder", expectedDatabaseName, "SELECT 1;")
		Expect(err).To(HaveOccurred())

		By("verifying the connection ConfigMap is garbage-collected when the Database is deleted")
		deleteDatabaseAndProvisionJobs(databaseResourceName, namespace)
		Eventually(func(g Gomega) string {
			cmd := exec.Command(
				"kubectl", "get", "configmap", connConfigMapName,
				"-n", namespace, "--ignore-not-found", "-o", "name",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			return strings.TrimSpace(output)
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(BeEmpty())
	})

	It("re-provisions without revoking the connecting admin (regression for SQLSTATE 2BP01)", func() {
		const (
			reproServerName      = "e2e-reprovision-server"
			reproDatabaseName    = "e2e-reprovision"
			reproOwnerA          = "e2e-reprovision-owner-a"
			reproOwnerAPrincipal = "33333333-3333-3333-3333-333333333333"
			reproOwnerB          = "e2e-reprovision-owner-b"
			reproOwnerBPrincipal = "44444444-4444-4444-4444-444444444444"
		)

		manifestOwnerA := writeReprovisionManifest(
			reproServerName, reproDatabaseName, namespace, adminIdentityRef, reproOwnerA, reproOwnerAPrincipal)
		manifestOwnerB := writeReprovisionManifest(
			reproServerName, reproDatabaseName, namespace, adminIdentityRef, reproOwnerB, reproOwnerBPrincipal)

		defer func() {
			deleteDatabaseAndProvisionJobs(reproDatabaseName, namespace)
			deleteDatabaseServerAndProvisionJobs(reproServerName, namespace)
			_ = os.Remove(manifestOwnerA)
			_ = os.Remove(manifestOwnerB)
			cleanupDatabasePostgresResources(reproDatabaseName, reproOwnerA, reproOwnerB)
		}()

		By("cleaning up stale re-provision resources from previous runs")
		deleteDatabaseAndProvisionJobs(reproDatabaseName, namespace)
		deleteDatabaseServerAndProvisionJobs(reproServerName, namespace)
		cleanupDatabasePostgresResources(reproDatabaseName, reproOwnerA, reproOwnerB)

		waitForAccessJobComplete := func(failure string) {
			labelSelector := fmt.Sprintf(
				"dis.altinn.cloud/database-name=%s,dis.altinn.cloud/user-provision=true", reproDatabaseName)
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
				Should(Succeed(), failure)
		}

		By("provisioning the database with owner A")
		cmd := exec.Command("kubectl", "apply", "-f", manifestOwnerA)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply re-provision manifest (owner A)")
		patchApplicationIdentityStatus(adminIdentityRef, namespace, adminIdentity, adminPrincipal)

		By("waiting for the ASO database child")
		asoDatabaseName := waitForDatabaseASOResource(reproDatabaseName, namespace)

		By("creating the app database owned by the provisioning admin")
		runPostgresQuery(fmt.Sprintf(
			"CREATE DATABASE %s OWNER %s;",
			quoteIdentifier(reproDatabaseName),
			quoteIdentifier(provisionAdminUser),
		))

		By("marking ASO resources ready for the local Postgres stand-in")
		patchFlexibleServerReady(reproServerName, namespace, "postgres.default.svc")
		patchFlexibleServersDatabaseReady(asoDatabaseName, namespace)

		By("waiting for the first provisioning job to complete")
		waitForAccessJobComplete("First access provisioning job did not complete")

		By("re-provisioning with owner B so the reconcile must revoke the removed owner A member")
		cmd = exec.Command("kubectl", "apply", "-f", manifestOwnerB)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply re-provision manifest (owner B)")

		// The connecting admin is a non-superuser that PostgreSQL auto-grants into
		// every managed role it created. Before the fix, the reconcile treated that
		// membership as revocable; the revoke failed with "dependent privileges
		// exist" (SQLSTATE 2BP01) and the second Job never completed. Owner B's role
		// is only created once the second Job gets past schema/role setup, so its
		// presence plus Job completion proves the admin was not revoked.
		By("waiting until owner B is provisioned by the second job")
		Eventually(func(g Gomega) string {
			out, err := runPostgresQueryAsUser("postgres",
				"SELECT 1 FROM pg_roles WHERE rolname = '"+reproOwnerB+"';")
			g.Expect(err).NotTo(HaveOccurred())
			return strings.TrimSpace(out)
		}).WithTimeout(5*time.Minute).WithPolling(2*time.Second).
			Should(Equal("1"),
				"owner B role was not created: the second provisioning job likely failed before reaching it (regression for SQLSTATE 2BP01)")

		By("waiting for the second provisioning job to complete")
		waitForAccessJobComplete(
			"Second access provisioning job did not complete: the reconcile likely tried to revoke the connecting admin (SQLSTATE 2BP01)")
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

	It("appends the cluster-id to the FlexibleServer AzureName for global uniqueness", func() {
		// The Kind manager patch sets DISPG_CLUSTER_ID=kind; the operator must
		// suffix every new FlexibleServer's Spec.AzureName with that value so
		// two clusters can host a DatabaseServer of the same CR name without
		// colliding on Azure's global PostgreSQL server-name registry. The
		// DatabaseServer.status.serverName field mirrors the AzureName for
		// out-of-cluster consumers.
		By("verifying the FlexibleServer Spec.AzureName carries the cluster-id suffix")
		Eventually(func(g Gomega) string {
			cmd := exec.Command(
				"kubectl", "get",
				"flexibleservers.dbforpostgresql.azure.com",
				dbName,
				"-n", namespace,
				"-o", "jsonpath={.spec.azureName}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			return strings.TrimSpace(output)
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(Equal(dbName + "-kind"))

		By("verifying DatabaseServer.status.serverName mirrors the AzureName")
		Eventually(func(g Gomega) string {
			cmd := exec.Command(
				"kubectl", "get",
				"databaseservers.storage.dis.altinn.cloud",
				dbName,
				"-n", namespace,
				"-o", "jsonpath={.status.serverName}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			return strings.TrimSpace(output)
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).
			Should(Equal(dbName + "-kind"))
	})

	It("applies explicit backupRetentionDays when set on DatabaseServer", func() {
		manifestPath := writeTestManifestWithBackupRetention(
			explicitRetentionDBName,
			namespace,
			adminIdentityRef,
			explicitBackupRetentionDay,
		)

		defer func() {
			deleteDatabaseServerAndProvisionJobs(explicitRetentionDBName, namespace)
			_ = os.Remove(manifestPath)
		}()

		By("creating a DatabaseServer custom resource with explicit backup retention")
		applyManifestWithIdentityPrerequisites(
			manifestPath,
			namespace,
			adminIdentityRef,
			adminIdentity,
			adminPrincipal,
			"Failed to apply DatabaseServer manifest with explicit backup retention",
		)

		By("verifying the DatabaseServer spec keeps the explicit backup retention")
		Eventually(func(g Gomega) string {
			cmd := exec.Command(
				"kubectl", "get",
				"databaseservers.storage.dis.altinn.cloud",
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
			true,
		)

		defer func() {
			deleteDatabaseServerAndProvisionJobs(explicitHADBName, namespace)
			_ = os.Remove(manifestPath)
		}()

		By("creating a DatabaseServer custom resource with explicit HA enabled")
		applyManifestWithIdentityPrerequisites(
			manifestPath,
			namespace,
			adminIdentityRef,
			adminIdentity,
			adminPrincipal,
			"Failed to apply DatabaseServer manifest with explicit HA",
		)

		By("verifying the DatabaseServer spec keeps explicit highAvailabilityEnabled")
		Eventually(func(g Gomega) string {
			cmd := exec.Command(
				"kubectl", "get",
				"databaseservers.storage.dis.altinn.cloud",
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
		)

		defer func() {
			deleteDatabaseServerAndProvisionJobs(explicitServerParamsDBName, namespace)
			_ = os.Remove(manifestPath)
		}()

		By("creating a DatabaseServer custom resource with explicit server parameters")
		applyManifestWithIdentityPrerequisites(
			manifestPath,
			namespace,
			adminIdentityRef,
			adminIdentity,
			adminPrincipal,
			"Failed to apply DatabaseServer manifest with explicit server parameters",
		)

		By("verifying fixed and explicit server parameter configurations are created")
		// The dev server runs on the Burstable tier, which does not support the
		// built-in PgBouncer, so only max_connections and the user-defined
		// parameter are expected (no pgbouncer.* parameters).
		expected := map[string]string{
			"max_connections":         "50",
			explicitCustomServerParam: explicitCustomServerValue,
		}

		Eventually(func(g Gomega) {
			cmd := exec.Command(
				"kubectl", "get",
				"flexibleserversconfigurations.dbforpostgresql.azure.com",
				"-n", namespace,
				"-l",
				fmt.Sprintf(
					"dis.altinn.cloud/database-server-name=%s,dis.altinn.cloud/configuration-kind=server-parameter",
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

func writeTestManifest(dbName, namespace, adminIdentityRef string) string {
	return writeTestManifestWithBackupRetention(dbName, namespace, adminIdentityRef, 0)
}

func writeTestManifestWithBackupRetention(
	dbName, namespace, adminIdentityRef string,
	backupRetentionDays int,
) string {
	sizeGB := int32(32)
	tier := "P80"

	databaseServer := &storagev1alpha1.DatabaseServer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "DatabaseServer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.DatabaseServerSpec{
			Version:    17,
			ServerType: "dev",
			Storage: &storagev1alpha1.DatabaseServerStorageSpec{
				SizeGB: &sizeGB,
				Tier:   &tier,
			},
			Auth: storagev1alpha1.DatabaseServerAuth{
				Admin: storagev1alpha1.AdminIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: adminIdentityRef},
					},
				},
			},
		},
	}
	if backupRetentionDays > 0 {
		databaseServer.Spec.BackupRetentionDays = &backupRetentionDays
	}

	return writeManifestWithAdminIdentity(databaseServer, namespace, adminIdentityRef)
}

func writeTestManifestWithHighAvailability(
	dbName, namespace, adminIdentityRef string,
	highAvailabilityEnabled bool,
) string {
	sizeGB := int32(32)
	tier := "P80"

	databaseServer := &storagev1alpha1.DatabaseServer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "DatabaseServer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.DatabaseServerSpec{
			Version:                 17,
			ServerType:              "dev",
			HighAvailabilityEnabled: &highAvailabilityEnabled,
			Storage: &storagev1alpha1.DatabaseServerStorageSpec{
				SizeGB: &sizeGB,
				Tier:   &tier,
			},
			Auth: storagev1alpha1.DatabaseServerAuth{
				Admin: storagev1alpha1.AdminIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: adminIdentityRef},
					},
				},
			},
		},
	}

	return writeManifestWithAdminIdentity(databaseServer, namespace, adminIdentityRef)
}

func writeTestManifestWithServerParameters(
	dbName, namespace, adminIdentityRef string,
) string {
	sizeGB := int32(32)
	tier := "P80"

	databaseServer := &storagev1alpha1.DatabaseServer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "DatabaseServer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.DatabaseServerSpec{
			Version:    17,
			ServerType: "dev",
			ServerParams: []storagev1alpha1.DatabaseServerParameter{
				{
					Name:  "autovacuum_naptime",
					Value: intstr.FromInt(15),
				},
			},
			Storage: &storagev1alpha1.DatabaseServerStorageSpec{
				SizeGB: &sizeGB,
				Tier:   &tier,
			},
			Auth: storagev1alpha1.DatabaseServerAuth{
				Admin: storagev1alpha1.AdminIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: adminIdentityRef},
					},
				},
			},
		},
	}

	return writeManifestWithAdminIdentity(databaseServer, namespace, adminIdentityRef)
}

func writeManifestWithAdminIdentity(
	databaseServer *storagev1alpha1.DatabaseServer,
	namespace, adminIdentityRef string,
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

	resources := []interface{}{adminIdentity, databaseServer}
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
	path := filepath.Join(dir, fmt.Sprintf("db-%s.yaml", databaseServer.Name))
	err := os.WriteFile(path, []byte(content), 0o600)
	Expect(err).NotTo(HaveOccurred(), "Failed to write temp manifest")
	return path
}

func writeDatabaseOnlyTestManifest(
	serverName,
	databaseResourceName,
	namespace,
	appIdentity,
	appPrincipalID,
	ownerIdentity,
	ownerPrincipalID string,
) string {
	appIdentityResource := &identityv1alpha1.ApplicationIdentity{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "application.dis.altinn.cloud/v1alpha1",
			Kind:       "ApplicationIdentity",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      appIdentity,
			Namespace: namespace,
		},
		Spec: identityv1alpha1.ApplicationIdentitySpec{},
	}

	database := &storagev1alpha1.Database{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "Database",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      databaseResourceName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.DatabaseSpec{
			Name: databaseResourceName,
			Server: storagev1alpha1.DatabaseServerReference{
				Name: serverName,
			},
			Access: storagev1alpha1.DatabaseAccessSpec{
				Principals: []storagev1alpha1.DatabaseAccessPrincipalSpec{
					{
						Role: storagev1alpha1.DatabaseAccessRoleOwner,
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{
							Name: appIdentity,
						},
					},
					{
						Role: storagev1alpha1.DatabaseAccessRoleOwner,
						Group: &storagev1alpha1.DatabaseGroupPrincipalSpec{
							Name:        ownerIdentity,
							PrincipalId: ownerPrincipalID,
						},
					},
				},
			},
		},
	}

	resources := []interface{}{appIdentityResource, database}
	docs := make([]string, 0, len(resources))
	for i := range resources {
		content, err := yaml.Marshal(resources[i])
		Expect(err).NotTo(HaveOccurred(), "Failed to marshal Database test manifest resource")
		docs = append(docs, string(content))
	}

	content := strings.Join(docs, "---\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	dir := os.TempDir()
	path := filepath.Join(dir, fmt.Sprintf("database-%s.yaml", databaseResourceName))
	err := os.WriteFile(path, []byte(content), 0o600)
	Expect(err).NotTo(HaveOccurred(), "Failed to write temp manifest")
	return path
}

func writeDatabaseTestManifest(
	sharedDBName,
	databaseResourceName,
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
	appIdentityResource := &identityv1alpha1.ApplicationIdentity{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "application.dis.altinn.cloud/v1alpha1",
			Kind:       "ApplicationIdentity",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      appIdentity,
			Namespace: namespace,
		},
		Spec: identityv1alpha1.ApplicationIdentitySpec{},
	}

	sharedDatabase := &storagev1alpha1.DatabaseServer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "DatabaseServer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      sharedDBName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.DatabaseServerSpec{
			Mode:       storagev1alpha1.DatabaseServerModeShared,
			Version:    17,
			ServerType: "dev",
			Network: &storagev1alpha1.DatabaseServerNetworkSpec{
				DelegatedSubnetResourceID: "/subscriptions/fake-subscription/resourceGroups/rg-dis-admin-network/providers/Microsoft.Network/virtualNetworks/vnet-dis-admin-dbs/subnets/snet-postgres-shared",
				PrivateDNSZoneResourceID:  "/subscriptions/fake-subscription/resourceGroups/rg-dis-admin-network/providers/Microsoft.Network/privateDnsZones/shared.private.postgres.database.azure.com",
			},
			Auth: storagev1alpha1.DatabaseServerAuth{
				Admin: storagev1alpha1.AdminIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: adminIdentityRef},
					},
				},
			},
		},
	}

	database := &storagev1alpha1.Database{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "Database",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      databaseResourceName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.DatabaseSpec{
			Name: databaseResourceName,
			Server: storagev1alpha1.DatabaseServerReference{
				Name: sharedDBName,
			},
			Access: storagev1alpha1.DatabaseAccessSpec{
				Principals: []storagev1alpha1.DatabaseAccessPrincipalSpec{
					{
						Role: storagev1alpha1.DatabaseAccessRoleWriter,
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{
							Name: appIdentity,
						},
					},
					{
						Role: storagev1alpha1.DatabaseAccessRoleOwner,
						Group: &storagev1alpha1.DatabaseGroupPrincipalSpec{
							Name:        ownerIdentity,
							PrincipalId: ownerPrincipalID,
						},
					},
				},
			},
		},
	}

	resources := []interface{}{adminIdentity, appIdentityResource, sharedDatabase, database}
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
	path := filepath.Join(dir, fmt.Sprintf("database-%s.yaml", databaseResourceName))
	err := os.WriteFile(path, []byte(content), 0o600)
	Expect(err).NotTo(HaveOccurred(), "Failed to write temp manifest")
	return path
}

func writeReprovisionManifest(
	serverName,
	databaseResourceName,
	namespace,
	adminIdentityRef,
	ownerName,
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

	sharedDatabase := &storagev1alpha1.DatabaseServer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "DatabaseServer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.DatabaseServerSpec{
			Mode:       storagev1alpha1.DatabaseServerModeShared,
			Version:    17,
			ServerType: "dev",
			Network: &storagev1alpha1.DatabaseServerNetworkSpec{
				DelegatedSubnetResourceID: "/subscriptions/fake-subscription/resourceGroups/rg-dis-admin-network/providers/Microsoft.Network/virtualNetworks/vnet-dis-admin-dbs/subnets/snet-postgres-shared",
				PrivateDNSZoneResourceID:  "/subscriptions/fake-subscription/resourceGroups/rg-dis-admin-network/providers/Microsoft.Network/privateDnsZones/shared.private.postgres.database.azure.com",
			},
			Auth: storagev1alpha1.DatabaseServerAuth{
				Admin: storagev1alpha1.AdminIdentitySpec{
					Identity: storagev1alpha1.IdentitySource{
						IdentityRef: &storagev1alpha1.ApplicationIdentityRef{Name: adminIdentityRef},
					},
				},
			},
		},
	}

	database := &storagev1alpha1.Database{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "Database",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      databaseResourceName,
			Namespace: namespace,
		},
		Spec: storagev1alpha1.DatabaseSpec{
			Name: databaseResourceName,
			Server: storagev1alpha1.DatabaseServerReference{
				Name: serverName,
			},
			Access: storagev1alpha1.DatabaseAccessSpec{
				Principals: []storagev1alpha1.DatabaseAccessPrincipalSpec{
					{
						Role: storagev1alpha1.DatabaseAccessRoleOwner,
						Group: &storagev1alpha1.DatabaseGroupPrincipalSpec{
							Name:        ownerName,
							PrincipalId: ownerPrincipalID,
						},
					},
				},
			},
		},
	}

	resources := []interface{}{adminIdentity, sharedDatabase, database}
	docs := make([]string, 0, len(resources))
	for i := range resources {
		content, err := yaml.Marshal(resources[i])
		Expect(err).NotTo(HaveOccurred(), "Failed to marshal re-provision manifest resource")
		docs = append(docs, string(content))
	}

	content := strings.Join(docs, "---\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	dir := os.TempDir()
	path := filepath.Join(dir, fmt.Sprintf("reprovision-%s-%s.yaml", databaseResourceName, ownerName))
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
	applyFailureMessage string,
) {
	cmd := exec.Command("kubectl", "apply", "-f", manifestPath)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), applyFailureMessage)

	patchApplicationIdentityStatus(adminIdentityRef, namespace, adminManagedIdentityName, adminPrincipalID)
}

func waitForDatabaseASOResource(databaseResourceName, namespace string) string {
	Eventually(func(g Gomega) string {
		cmd := exec.Command(
			"kubectl", "get",
			"flexibleserversdatabases.dbforpostgresql.azure.com",
			"-n", namespace,
			"-l", fmt.Sprintf("dis.altinn.cloud/database-name=%s", databaseResourceName),
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
		"-l", fmt.Sprintf("dis.altinn.cloud/database-name=%s", databaseResourceName),
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
	podName := waitForReadyPostgresPod()

	cmd := exec.Command("kubectl", "exec", "-n", "default", podName, "--",
		"psql", "-v", "ON_ERROR_STOP=1", "-U", user, "-d", databaseName, "-tAc", query)
	output, err := utils.Run(cmd)
	if err != nil {
		return "", err
	}
	return output, nil
}

func waitForReadyPostgresPod() string {
	var podName string
	Eventually(func(g Gomega) string {
		cmd := exec.Command("kubectl", "get", "pods", "-l", "app=postgres", "-n", "default", "-o", "json")
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred(), "Failed to get Postgres pods")

		var pods corev1.PodList
		g.Expect(json.Unmarshal([]byte(output), &pods)).To(Succeed(), "Failed to parse Postgres pods")
		for i := range pods.Items {
			pod := pods.Items[i]
			if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning {
				continue
			}
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					podName = pod.Name
					return podName
				}
			}
		}
		return ""
	}).WithTimeout(2*time.Minute).WithPolling(2*time.Second).
		ShouldNot(BeEmpty(), "expected a ready Postgres pod")
	return podName
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

func deleteDatabaseServerAndProvisionJobs(dbName, namespace string) {
	cmd := exec.Command(
		"kubectl", "delete",
		"databaseservers.storage.dis.altinn.cloud",
		dbName,
		"-n", namespace,
		"--ignore-not-found=true",
	)
	_, _ = utils.Run(cmd)

	labelSelector := fmt.Sprintf("dis.altinn.cloud/database-server-name=%s,dis.altinn.cloud/user-provision=true", dbName)
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

func deleteDatabaseAndProvisionJobs(databaseName, namespace string) {
	databaseLabelSelector := fmt.Sprintf("dis.altinn.cloud/database-name=%s", databaseName)

	cmd := exec.Command(
		"kubectl", "delete",
		"databases.storage.dis.altinn.cloud",
		databaseName,
		"-n", namespace,
		"--ignore-not-found=true",
	)
	_, _ = utils.Run(cmd)

	jobLabelSelector := fmt.Sprintf("%s,dis.altinn.cloud/user-provision=true", databaseLabelSelector)
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
			"databases.storage.dis.altinn.cloud",
			databaseName,
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
			"-l", databaseLabelSelector,
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

func cleanupDatabasePostgresResources(databaseName, appIdentity, ownerIdentity string) {
	_, _ = runPostgresQueryAsUser("postgres", fmt.Sprintf(
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s';",
		strings.ReplaceAll(databaseName, "'", "''"),
	))
	_, _ = runPostgresQueryAsUser("postgres", fmt.Sprintf("DROP DATABASE IF EXISTS %s;", quoteIdentifier(databaseName)))
	_, _ = runPostgresQueryAsUser("postgres", fmt.Sprintf("DROP ROLE IF EXISTS %s;", quoteIdentifier(appIdentity)))
	_, _ = runPostgresQueryAsUser("postgres", fmt.Sprintf("DROP ROLE IF EXISTS %s;", quoteIdentifier(ownerIdentity)))
	_, _ = runPostgresQueryAsUser("postgres", `DROP ROLE IF EXISTS "e2e-database-intruder";`)
}
