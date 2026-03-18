//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
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

	waitForJSONPath := func(resource, name, jsonPath, expected string) {
		Eventually(func() (string, error) {
			checkCmd := utils.Kubectl(
				"get", resource, name,
				"-n", sampleNamespace,
				"-o", fmt.Sprintf("jsonpath=%s", jsonPath),
			)
			out, err := utils.Run(checkCmd)
			return strings.TrimSpace(out), err
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Equal(expected))
	}

	getJSONPath := func(resource, name, jsonPath string) (string, error) {
		checkCmd := utils.Kubectl(
			"get", resource, name,
			"-n", sampleNamespace,
			"-o", fmt.Sprintf("jsonpath=%s", jsonPath),
		)
		out, err := utils.Run(checkCmd)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	}

	mustGetJSONPath := func(resource, name, jsonPath string) string {
		out, err := getJSONPath(resource, name, jsonPath)
		Expect(err).NotTo(HaveOccurred())
		return out
	}

	waitForResourceNotFound := func(resource, name string) {
		Eventually(func() bool {
			checkCmd := utils.Kubectl("get", resource, name, "-n", sampleNamespace)
			_, err := utils.Run(checkCmd)
			if err == nil {
				return false
			}
			return strings.Contains(err.Error(), "NotFound")
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(BeTrue(), "expected %s/%s to be deleted", resource, name)
	}

	BeforeAll(func() {
		By("ensuring no stale Vault sample resources are present")
		cmd := utils.Kubectl("delete", "-k", "config/samples", "--ignore-not-found=true")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete config/samples in test setup")

		By("verifying ASO child resources are absent after deleting sample Vault")
		waitForResourceNotFound("vaults.vault.dis.altinn.cloud", sampleVaultName)
		waitForResourceNotFound("vaults.keyvault.azure.com", expectedASOVaultName)
		waitForResourceNotFound("roleassignments.authorization.azure.com", expectedRoleAssignName)
	})

	AfterAll(func() {
		By("cleaning up Vault sample resources")
		cmd := utils.Kubectl("delete", "-k", "config/samples", "--ignore-not-found=true")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete config/samples during cleanup")

		By("verifying Vault and ASO child resources are deleted")
		waitForResourceNotFound("vaults.vault.dis.altinn.cloud", sampleVaultName)
		waitForResourceNotFound("vaults.keyvault.azure.com", expectedASOVaultName)
		waitForResourceNotFound("roleassignments.authorization.azure.com", expectedRoleAssignName)
	})

	It("applies Vault sample from config and creates ASO resources", func() {
		By("applying the Vault sample manifest from config/samples")
		cmd := utils.Kubectl("apply", "-k", "config/samples")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply config/samples")

		By("waiting for Vault status IdentityReady=True")
		Eventually(func() (string, error) {
			checkCmd := utils.Kubectl(
				"get", "vaults.vault.dis.altinn.cloud", sampleVaultName,
				"-n", sampleNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='IdentityReady')].status}",
			)
			out, err := utils.Run(checkCmd)
			return strings.TrimSpace(out), err
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Equal("True"))

		By("verifying ASO Key Vault was created")
		Eventually(func() error {
			checkCmd := utils.Kubectl("get", "vaults.keyvault.azure.com", expectedASOVaultName, "-n", sampleNamespace)
			_, err := utils.Run(checkCmd)
			return err
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

		By("verifying owner RoleAssignment was created")
		Eventually(func() error {
			checkCmd := utils.Kubectl("get", "roleassignments.authorization.azure.com", expectedRoleAssignName, "-n", sampleNamespace)
			_, err := utils.Run(checkCmd)
			return err
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())
	})

	It("updates Key Vault spec when Vault spec changes", func() {
		By("patching Vault sample with updated spec values")
		patchPayload := `{"spec":{"sku":"premium","softDeleteRetentionDays":30,"purgeProtectionEnabled":true,"tags":{"team":"platform","env":"prod"}}}`
		cmd := utils.Kubectl(
			"patch", "vaults.vault.dis.altinn.cloud", sampleVaultName,
			"-n", sampleNamespace,
			"--type=merge",
			"-p", patchPayload,
		)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch Vault sample spec")

		By("verifying ASO Key Vault spec reflects updated Vault spec")
		waitForJSONPath("vaults.keyvault.azure.com", expectedASOVaultName, "{.spec.properties.sku.name}", "Premium")
		waitForJSONPath("vaults.keyvault.azure.com", expectedASOVaultName, "{.spec.properties.softDeleteRetentionInDays}", "30")
		waitForJSONPath("vaults.keyvault.azure.com", expectedASOVaultName, "{.spec.properties.enablePurgeProtection}", "true")
		waitForJSONPath("vaults.keyvault.azure.com", expectedASOVaultName, "{.spec.tags.team}", "platform")
		waitForJSONPath("vaults.keyvault.azure.com", expectedASOVaultName, "{.spec.tags.env}", "prod")
		waitForJSONPath("vaults.keyvault.azure.com", expectedASOVaultName, "{.spec.tags.remove}", "")
	})

	It("recreates deleted ASO child resources", func() {
		By("capturing child resource UIDs before deletion")
		originalVaultUID := mustGetJSONPath("vaults.keyvault.azure.com", expectedASOVaultName, "{.metadata.uid}")
		originalRoleAssignUID := mustGetJSONPath("roleassignments.authorization.azure.com", expectedRoleAssignName, "{.metadata.uid}")

		By("deleting ASO child resources")
		cmd := utils.Kubectl("delete", "vaults.keyvault.azure.com", expectedASOVaultName, "-n", sampleNamespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete ASO Key Vault")

		cmd = utils.Kubectl("delete", "roleassignments.authorization.azure.com", expectedRoleAssignName, "-n", sampleNamespace)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete owner RoleAssignment")

		By("verifying deleted children are recreated by reconciliation")
		Eventually(func() (string, error) {
			return getJSONPath("vaults.keyvault.azure.com", expectedASOVaultName, "{.metadata.uid}")
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).ShouldNot(Equal(originalVaultUID))

		Eventually(func() (string, error) {
			return getJSONPath("roleassignments.authorization.azure.com", expectedRoleAssignName, "{.metadata.uid}")
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).ShouldNot(Equal(originalRoleAssignUID))
	})

	It("deletes Vault sample and cascades deletion to ASO child resources", func() {
		By("deleting the Vault sample manifest from config/samples")
		cmd := utils.Kubectl("delete", "-k", "config/samples", "--ignore-not-found=true")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete config/samples")

		By("verifying Vault CR was deleted")
		waitForResourceNotFound("vaults.vault.dis.altinn.cloud", sampleVaultName)

		By("verifying ASO Key Vault was deleted via ownerReference garbage collection")
		waitForResourceNotFound("vaults.keyvault.azure.com", expectedASOVaultName)

		By("verifying owner RoleAssignment was deleted via ownerReference garbage collection")
		waitForResourceNotFound("roleassignments.authorization.azure.com", expectedRoleAssignName)
	})
})
