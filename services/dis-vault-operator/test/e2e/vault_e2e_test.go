//go:build e2e
// +build e2e

package e2e

import (
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Altinn/altinn-platform/services/dis-vault-operator/test/utils"
)

var _ = Describe("Vault e2e", Ordered, func() {
	const (
		sampleNamespace        = "default"
		sampleVaultName        = "vault-sample"
		expectedASOVaultName   = "vault-sample-akv"
		expectedRoleAssignName = "vault-sample-owner-ra"
	)

	BeforeAll(func() {
		By("ensuring no stale Vault sample resources are present")
		cmd := exec.Command("kubectl", "delete", "-k", "config/samples", "--ignore-not-found=true")
		_, _ = utils.Run(cmd)

		// TODO: Remove explicit ASO cleanup when dis-vault handles cascading cleanup for Vault-owned resources.
		// Expected future behavior: deleting the Vault CR should be sufficient for ASO child resource cleanup.
		cmd = exec.Command("kubectl", "delete", "vaults.keyvault.azure.com", expectedASOVaultName, "-n", sampleNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
		cmd = exec.Command("kubectl", "delete", "roleassignments.authorization.azure.com", expectedRoleAssignName, "-n", sampleNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
	})

	AfterAll(func() {
		By("cleaning up Vault sample resources")
		cmd := exec.Command("kubectl", "delete", "-k", "config/samples", "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
		// TODO: Remove explicit ASO cleanup when dis-vault implements reconciliation-driven deletion
		// of resources that follow a Vault.
		cmd = exec.Command("kubectl", "delete", "vaults.keyvault.azure.com", expectedASOVaultName, "-n", sampleNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
		cmd = exec.Command("kubectl", "delete", "roleassignments.authorization.azure.com", expectedRoleAssignName, "-n", sampleNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
	})

	It("applies Vault sample from config and creates ASO resources", func() {
		By("applying the Vault sample manifest from config/samples")
		cmd := exec.Command("kubectl", "apply", "-k", "config/samples")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply config/samples")

		By("waiting for Vault status IdentityReady=True")
		Eventually(func() (string, error) {
			checkCmd := exec.Command(
				"kubectl", "get", "vaults.vault.dis.altinn.cloud", sampleVaultName,
				"-n", sampleNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='IdentityReady')].status}",
			)
			out, err := utils.Run(checkCmd)
			return strings.TrimSpace(out), err
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Equal("True"))

		By("verifying ASO Key Vault was created")
		Eventually(func() error {
			checkCmd := exec.Command("kubectl", "get", "vaults.keyvault.azure.com", expectedASOVaultName, "-n", sampleNamespace)
			_, err := utils.Run(checkCmd)
			return err
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

		By("verifying owner RoleAssignment was created")
		Eventually(func() error {
			checkCmd := exec.Command("kubectl", "get", "roleassignments.authorization.azure.com", expectedRoleAssignName, "-n", sampleNamespace)
			_, err := utils.Run(checkCmd)
			return err
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())
	})
})
