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
	"sigs.k8s.io/yaml"
)

var _ = Describe("User provisioning", Ordered, func() {
	const (
		dbName                     = "e2e-user-provision"
		explicitRetentionDBName    = "e2e-backup-retention-explicit"
		namespace                  = "default"
		adminIdentityRef           = "adminidentity"
		adminIdentity              = "adminidentity"
		adminPrincipal             = "adminidentity-principal-id"
		userIdentityRef            = "useridentity"
		userIdentity               = "user1"
		userPrincipalId            = "user1-principal-id"
		explicitBackupRetentionDay = 21
	)

	var manifestPath string

	BeforeAll(func() {
		By("cleaning up stale Database and Job resources from previous runs")
		deleteDatabaseAndProvisionJobs(dbName, namespace)
		deleteDatabaseAndProvisionJobs(explicitRetentionDBName, namespace)

		manifestPath = writeTestManifest(
			dbName,
			namespace,
			adminIdentityRef,
			userIdentityRef,
		)
		By("creating a Database custom resource")
		cmd := exec.Command("kubectl", "apply", "-f", manifestPath)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply Database manifest")

		By("patching ApplicationIdentity status fields")
		cmd = exec.Command(
			"kubectl", "patch",
			"applicationidentity", adminIdentityRef,
			"-n", namespace,
			"--subresource=status",
			"--type=merge",
			"-p", fmt.Sprintf(`{"status":{"managedIdentityName":"%s","principalId":"%s"}}`, adminIdentity, adminPrincipal),
		)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch admin ApplicationIdentity status")

		cmd = exec.Command(
			"kubectl", "patch",
			"applicationidentity", userIdentityRef,
			"-n", namespace,
			"--subresource=status",
			"--type=merge",
			"-p", fmt.Sprintf(`{"status":{"managedIdentityName":"%s","principalId":"%s"}}`, userIdentity, userPrincipalId),
		)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch user ApplicationIdentity status")

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
			cmd := exec.Command("kubectl", "delete", "-f", manifestPath, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			_ = os.Remove(manifestPath)
			deleteDatabaseAndProvisionJobs(explicitRetentionDBName, namespace)
		}()

		By("creating a Database custom resource with explicit backup retention")
		cmd := exec.Command("kubectl", "apply", "-f", manifestPath)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply Database manifest with explicit backup retention")

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
				User: storagev1alpha1.UserIdentitySpec{
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
	path := filepath.Join(dir, fmt.Sprintf("db-%s.yaml", dbName))
	err := os.WriteFile(path, []byte(content), 0o600)
	Expect(err).NotTo(HaveOccurred(), "Failed to write temp manifest")
	return path
}

func runPostgresQuery(query string) string {
	output, err := runPostgresQueryAsUser("postgres", query)
	Expect(err).NotTo(HaveOccurred(), "Failed to run Postgres query")
	return output
}

func runPostgresQueryAsUser(user, query string) (string, error) {
	cmd := exec.Command("kubectl", "get", "pods", "-l", "app=postgres", "-n", "default", "-o", "jsonpath={.items[0].metadata.name}")
	podName, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to get Postgres pod name")
	podName = strings.TrimSpace(podName)

	cmd = exec.Command("kubectl", "exec", "-n", "default", podName, "--",
		"psql", "-v", "ON_ERROR_STOP=1", "-U", user, "-d", "postgres", "-tAc", query)
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
