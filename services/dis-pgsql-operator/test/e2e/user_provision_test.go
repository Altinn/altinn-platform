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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/test/utils"
)

var _ = Describe("User provisioning", Ordered, func() {
	const (
		dbName          = "e2e-user-provision"
		namespace       = "default"
		adminIdentity   = "admin1"
		adminPrincipal  = "admin1-principal-id"
		userIdentity    = "user-e2e"
		userPrincipalId = "user-e2e-principal-id"
	)

	var manifestPath string

	BeforeAll(func() {
		manifestPath = writeDatabaseManifest(dbName, namespace, adminIdentity, adminPrincipal, userIdentity, userPrincipalId)
		By("creating a Database custom resource")
		cmd := exec.Command("kubectl", "apply", "-f", manifestPath)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply Database manifest")
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
		cmd := exec.Command(
			"kubectl", "wait",
			"--for=condition=complete",
			"job",
			"-l", fmt.Sprintf("dis.altinn.cloud/database-name=%s,dis.altinn.cloud/user-provision=true", dbName),
			"-n", namespace,
			"--timeout=5m",
		)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "User provisioning job did not complete")

		By("verifying the role exists in Postgres")
		output := runPostgresQuery("SELECT 1 FROM pg_roles WHERE rolname = '" + userIdentity + "';")
		Expect(strings.TrimSpace(output)).To(Equal("1"))

		By("verifying the schema exists in Postgres")
		output = runPostgresQuery("SELECT 1 FROM pg_namespace WHERE nspname = '" + dbName + "';")
		Expect(strings.TrimSpace(output)).To(Equal("1"))

		By("verifying the user can create tables in its schema")
		tableName := fmt.Sprintf("%s.e2e_check", quoteIdentifier(dbName))
		_, err = runPostgresQueryAsUser(userIdentity, fmt.Sprintf(
			"CREATE TABLE %s (id int); DROP TABLE %s;",
			tableName,
			tableName,
		))
		Expect(err).NotTo(HaveOccurred())
	})
})

func writeDatabaseManifest(dbName, namespace, adminIdentity, adminPrincipalId, userIdentity, userPrincipalId string) string {
	content := fmt.Sprintf(`apiVersion: storage.dis.altinn.cloud/v1alpha1
kind: Database
metadata:
  name: %s
  namespace: %s
spec:
  version: 17
  serverType: dev
  auth:
    adminAppIdentity: %s
    adminAppPrincipalId: %s
    adminServiceAccountName: %s
    userAppIdentity: %s
    userAppPrincipalId: %s
`, dbName, namespace, adminIdentity, adminPrincipalId, adminIdentity, userIdentity, userPrincipalId)

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
