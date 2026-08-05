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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("DatabaseServer debug access (data plane)", Ordered, func() {
	const (
		serverName       = "e2e-debug-access"
		namespace        = "default"
		adminIdentityRef = "debug-admin-identity"
		adminIdentity    = "debug-admin-identity"
		adminPrincipal   = "debug-admin-principal-id"

		debugGroupName        = "e2e_debug_group"
		debugGroupPrincipalID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

		debugGroupName2        = "e2e_debug_group_2"
		debugGroupPrincipalID2 = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)

	var manifestPath string

	BeforeAll(func() {
		By("cleaning up stale DatabaseServer and Job resources from previous runs")
		deleteDatabaseServerAndProvisionJobs(serverName, namespace)
		dropRoleIfExists(debugGroupName)
		dropRoleIfExists(debugGroupName2)

		manifestPath = writeDebugAccessManifest(
			serverName,
			namespace,
			adminIdentityRef,
			debugGroupName,
			debugGroupPrincipalID,
		)

		By("creating a dedicated DatabaseServer with debugAccess and identity prerequisites")
		applyManifestWithIdentityPrerequisites(
			manifestPath,
			namespace,
			adminIdentityRef,
			adminIdentity,
			adminPrincipal,
			"Failed to apply DatabaseServer debug-access manifest",
		)

		By("marking the FlexibleServer ready for the local Postgres stand-in")
		patchFlexibleServerReady(serverName, namespace, "postgres.default.svc")
	})

	AfterAll(func() {
		if manifestPath != "" {
			By("deleting the DatabaseServer custom resource")
			cmd := exec.Command("kubectl", "delete", "-f", manifestPath, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			_ = os.Remove(manifestPath)
		}
		deleteDatabaseServerAndProvisionJobs(serverName, namespace)
		dropRoleIfExists(debugGroupName)
		dropRoleIfExists(debugGroupName2)
	})

	It("grants a debug principal read-only access via a managed pg_monitor/pg_read_all_data role", func() {
		By("waiting for the debug access provisioning job to complete")
		waitForDebugAccessJobComplete(serverName, namespace)

		By("verifying the debug principal role exists in Postgres")
		output := runPostgresQuery("SELECT 1 FROM pg_roles WHERE rolname = '" + debugGroupName + "';")
		Expect(strings.TrimSpace(output)).To(Equal("1"))

		By("verifying a managed debug role exists holding pg_monitor and pg_read_all_data")
		output = runPostgresQuery(managedDebugRoleBuiltinsQuery)
		Expect(strings.TrimSpace(output)).To(Equal("1"))

		By("verifying the principal is a member of the managed debug role with both built-in roles")
		output = runPostgresQuery(principalDebugBuiltinsQuery(debugGroupName))
		Expect(strings.TrimSpace(output)).To(Equal("pg_monitor,pg_read_all_data"))

		By("verifying the principal can CONNECT to databases on the server")
		output = runPostgresQuery(fmt.Sprintf(
			"SELECT has_database_privilege('%s', 'postgres', 'CONNECT');",
			debugGroupName,
		))
		Expect(strings.TrimSpace(output)).To(Equal("t"))
	})

	It("revokes a debug principal that is removed from the spec", func() {
		By("swapping the debug principal to a different group")
		patchDebugAccessGroupPrincipal(serverName, namespace, debugGroupName2, debugGroupPrincipalID2)

		By("waiting for the new debug access provisioning job to complete")
		waitForDebugAccessJobComplete(serverName, namespace)

		By("verifying the new principal is a member of the managed debug role")
		Eventually(func() string {
			return strings.TrimSpace(runPostgresQuery(principalDebugBuiltinsQuery(debugGroupName2)))
		}).WithTimeout(2 * time.Minute).WithPolling(3 * time.Second).
			Should(Equal("pg_monitor,pg_read_all_data"))

		By("verifying the removed principal is no longer a member of the managed debug role")
		Eventually(func() string {
			return strings.TrimSpace(runPostgresQuery(principalManagedDebugMembershipCountQuery(debugGroupName)))
		}).WithTimeout(2 * time.Minute).WithPolling(3 * time.Second).
			Should(Equal("0"))
	})
})

// managedDebugRoleBuiltinsQuery returns 1 when at least one managed debug role
// (dispg-*-debug-*) is a member of both pg_monitor and pg_read_all_data.
const managedDebugRoleBuiltinsQuery = `SELECT 1 WHERE EXISTS (
  SELECT 1
  FROM pg_roles managed
  JOIN pg_auth_members mm ON mm.member = managed.oid
  JOIN pg_roles builtin ON builtin.oid = mm.roleid
  WHERE managed.rolname LIKE 'dispg-%debug%'
    AND builtin.rolname IN ('pg_monitor','pg_read_all_data')
  GROUP BY managed.oid
  HAVING count(DISTINCT builtin.rolname) = 2
);`

// principalDebugBuiltinsQuery returns the comma-separated built-in roles a
// principal transitively holds through a managed debug role.
func principalDebugBuiltinsQuery(principal string) string {
	return fmt.Sprintf(`SELECT string_agg(DISTINCT builtin.rolname, ',' ORDER BY builtin.rolname)
FROM pg_auth_members mp
JOIN pg_roles managed ON managed.oid = mp.roleid
JOIN pg_roles principal ON principal.oid = mp.member
JOIN pg_auth_members mb ON mb.member = managed.oid
JOIN pg_roles builtin ON builtin.oid = mb.roleid
WHERE principal.rolname = '%s'
  AND managed.rolname LIKE 'dispg-%%debug%%'
  AND builtin.rolname IN ('pg_monitor','pg_read_all_data');`, strings.ReplaceAll(principal, "'", "''"))
}

// principalManagedDebugMembershipCountQuery counts how many managed debug roles a
// principal is a direct member of (0 once revoked).
func principalManagedDebugMembershipCountQuery(principal string) string {
	return fmt.Sprintf(`SELECT count(*)
FROM pg_auth_members mp
JOIN pg_roles managed ON managed.oid = mp.roleid
JOIN pg_roles principal ON principal.oid = mp.member
WHERE principal.rolname = '%s'
  AND managed.rolname LIKE 'dispg-%%debug%%';`, strings.ReplaceAll(principal, "'", "''"))
}

func waitForDebugAccessJobComplete(serverName, namespace string) {
	labelSelector := fmt.Sprintf(
		"dis.altinn.cloud/database-server-name=%s,dis.altinn.cloud/component=debug-access,dis.altinn.cloud/user-provision=true",
		serverName,
	)

	// Wait for the Job to exist first: kubectl wait errors immediately when the
	// label selector matches nothing, and a missing Job (operator never created
	// it) is a different failure than a Job that never completes.
	Eventually(func() error {
		cmd := exec.Command("kubectl", "get", "job", "-l", labelSelector, "-n", namespace, "-o", "name")
		output, err := utils.Run(cmd)
		if err != nil {
			return err
		}
		if strings.TrimSpace(output) == "" {
			return fmt.Errorf("no debug access provisioning job matches %q yet", labelSelector)
		}
		return nil
	}).WithTimeout(3*time.Minute).WithPolling(3*time.Second).
		Should(Succeed(), func() string {
			return "debug access provisioning job was never created\n" + debugAccessDiagnostics(namespace)
		})

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
	}).WithTimeout(3*time.Minute).WithPolling(3*time.Second).
		Should(Succeed(), func() string {
			return "debug access provisioning job did not complete\n" + debugAccessDiagnostics(namespace)
		})
}

// debugAccessDiagnostics collects the state needed to diagnose a debug access
// provisioning failure from CI output alone: the jobs with their labels and the
// operator manager logs.
func debugAccessDiagnostics(namespace string) string {
	var b strings.Builder

	b.WriteString("--- jobs (with labels) ---\n")
	cmd := exec.Command("kubectl", "get", "jobs", "-n", namespace, "--show-labels")
	if output, err := utils.Run(cmd); err == nil {
		b.WriteString(output)
	} else {
		fmt.Fprintf(&b, "failed to list jobs: %v\n", err)
	}

	b.WriteString("--- provisioning job pod logs ---\n")
	cmd = exec.Command("kubectl", "get", "job", "-n", namespace, "-o", "name")
	if output, err := utils.Run(cmd); err == nil {
		for _, jobRef := range strings.Fields(output) {
			fmt.Fprintf(&b, "%s:\n", jobRef)
			logsCmd := exec.Command("kubectl", "logs", jobRef, "-n", namespace, "--tail", "60")
			if logs, logsErr := utils.Run(logsCmd); logsErr == nil {
				b.WriteString(logs)
			} else {
				fmt.Fprintf(&b, "failed to fetch logs: %v\n", logsErr)
			}
		}
	} else {
		fmt.Fprintf(&b, "failed to list jobs for logs: %v\n", err)
	}

	b.WriteString("--- operator logs (tail) ---\n")
	cmd = exec.Command(
		"kubectl", "logs",
		"-n", "dis-pgsql-operator-system",
		"deploy/dis-pgsql-operator-controller-manager",
		"--tail", "100",
	)
	if output, err := utils.Run(cmd); err == nil {
		b.WriteString(output)
	} else {
		fmt.Fprintf(&b, "failed to fetch operator logs: %v\n", err)
	}

	return b.String()
}

func patchDebugAccessGroupPrincipal(serverName, namespace, groupName, groupPrincipalID string) {
	patch := fmt.Sprintf(
		`{"spec":{"debugAccess":{"principals":[{"group":{"name":"%s","principalId":"%s"}}]}}}`,
		groupName,
		groupPrincipalID,
	)
	cmd := exec.Command(
		"kubectl", "patch",
		"databaseservers.storage.dis.altinn.cloud", serverName,
		"-n", namespace,
		"--type=merge",
		"-p", patch,
	)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to patch DatabaseServer debugAccess principals")
}

func dropRoleIfExists(role string) {
	_, _ = runPostgresQueryAsUser("postgres", fmt.Sprintf("DROP ROLE IF EXISTS %s;", quoteIdentifier(role)))
}

func writeDebugAccessManifest(
	serverName, namespace, adminIdentityRef, debugGroupName, debugGroupPrincipalID string,
) string {
	sizeGB := int32(32)
	tier := "P80"

	databaseServer := &storagev1alpha1.DatabaseServer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Kind:       "DatabaseServer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverName,
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
			DebugAccess: &storagev1alpha1.DatabaseServerDebugAccessSpec{
				Principals: []storagev1alpha1.DebugAccessPrincipalSpec{
					{
						Group: &storagev1alpha1.DatabaseGroupPrincipalSpec{
							Name:        debugGroupName,
							PrincipalId: debugGroupPrincipalID,
						},
					},
				},
			},
		},
	}

	return writeManifestWithAdminIdentity(databaseServer, namespace, adminIdentityRef)
}
