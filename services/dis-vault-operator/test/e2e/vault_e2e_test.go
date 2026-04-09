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
		sampleNamespace                     = "default"
		sampleVaultName                     = "vault-sample"
		expectedASOVaultName                = "vault-sample-akv"
		expectedRoleAssignName              = "vault-sample-owner-ra"
		expectedVPNExitNodeSubnetID         = "/subscriptions/fake-subscription/resourceGroups/fake-network-rg/providers/Microsoft.Network/virtualNetworks/fake-vnet/subnets/fake-vpn-exit-subnet"
		externalSecretsSamplePath           = "config/samples/vault_v1alpha1_external_secrets_vault.yaml"
		externalSecretsVaultName            = "vault-es-sample"
		expectedExternalSecretsASOVaultName = "vault-es-sample-akv"
		expectedExternalSecretsRoleAssign   = "vault-es-sample-owner-ra"
		expectedSecretStoreName             = "vault-es-sample-secret-store"
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

	waitForJSONPathContains := func(resource, name, jsonPath, expected string) {
		Eventually(func() (string, error) {
			checkCmd := utils.Kubectl(
				"get", resource, name,
				"-n", sampleNamespace,
				"-o", fmt.Sprintf("jsonpath=%s", jsonPath),
			)
			out, err := utils.Run(checkCmd)
			return strings.TrimSpace(out), err
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(ContainSubstring(expected))
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

	resourceExists := func(resource, name string) (bool, error) {
		checkCmd := utils.Kubectl(
			"get", resource, name,
			"-n", sampleNamespace,
			"--ignore-not-found",
			"-o", "name",
		)
		out, err := utils.Run(checkCmd)
		if err != nil {
			return false, err
		}
		return strings.TrimSpace(out) != "", nil
	}

	waitForResourceNotFound := func(resource, name string) {
		Eventually(func() (bool, error) {
			return resourceExists(resource, name)
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(BeFalse(), "expected %s/%s to be deleted", resource, name)
	}

	ensureResourceNotCreatedYet := func(resource, name string) {
		Consistently(func() (bool, error) {
			return resourceExists(resource, name)
		}).WithTimeout(15 * time.Second).WithPolling(2 * time.Second).Should(BeFalse(), "expected %s/%s to remain absent", resource, name)
	}

	applyManifest := func(path string) {
		cmd := utils.Kubectl("apply", "-f", path)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply %s", path)
	}

	deleteManifest := func(path string) {
		cmd := utils.Kubectl("delete", "-f", path, "--ignore-not-found=true")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete %s", path)
	}

	patchKeyVaultReadyStatus := func(name, resourceID, vaultURI string) {
		payload := fmt.Sprintf(
			`{"status":{"id":%q,"properties":{"vaultUri":%q},"conditions":[{"type":"Ready","status":"True","reason":"Ready","message":"Provisioned","lastTransitionTime":"2026-01-01T00:00:00Z"}]}}`,
			resourceID,
			vaultURI,
		)
		cmd := utils.Kubectl(
			"patch", "vaults.keyvault.azure.com", name,
			"-n", sampleNamespace,
			"--subresource=status",
			"--type=merge",
			"-p", payload,
		)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch Key Vault status")
	}

	patchRoleAssignmentReadyStatus := func(name, resourceID string) {
		payload := fmt.Sprintf(
			`{"status":{"id":%q,"conditions":[{"type":"Ready","status":"True","reason":"Ready","message":"Assigned","lastTransitionTime":"2026-01-01T00:00:00Z"}]}}`,
			resourceID,
		)
		cmd := utils.Kubectl(
			"patch", "roleassignments.authorization.azure.com", name,
			"-n", sampleNamespace,
			"--subresource=status",
			"--type=merge",
			"-p", payload,
		)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch RoleAssignment status")
	}

	BeforeAll(func() {
		By("ensuring no stale Vault sample resources are present")
		cmd := utils.Kubectl("delete", "-k", "config/samples", "--ignore-not-found=true")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete config/samples in test setup")
		deleteManifest(externalSecretsSamplePath)

		By("verifying ASO child resources are absent after deleting sample Vault")
		waitForResourceNotFound("vaults.vault.dis.altinn.cloud", sampleVaultName)
		waitForResourceNotFound("vaults.keyvault.azure.com", expectedASOVaultName)
		waitForResourceNotFound("roleassignments.authorization.azure.com", expectedRoleAssignName)
		waitForResourceNotFound("vaults.vault.dis.altinn.cloud", externalSecretsVaultName)
		waitForResourceNotFound("vaults.keyvault.azure.com", expectedExternalSecretsASOVaultName)
		waitForResourceNotFound("roleassignments.authorization.azure.com", expectedExternalSecretsRoleAssign)
		waitForResourceNotFound("secretstores.external-secrets.io", expectedSecretStoreName)
	})

	AfterAll(func() {
		By("cleaning up Vault sample resources")
		cmd := utils.Kubectl("delete", "-k", "config/samples", "--ignore-not-found=true")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete config/samples during cleanup")
		deleteManifest(externalSecretsSamplePath)

		By("verifying Vault and ASO child resources are deleted")
		waitForResourceNotFound("vaults.vault.dis.altinn.cloud", sampleVaultName)
		waitForResourceNotFound("vaults.keyvault.azure.com", expectedASOVaultName)
		waitForResourceNotFound("roleassignments.authorization.azure.com", expectedRoleAssignName)
		waitForResourceNotFound("vaults.vault.dis.altinn.cloud", externalSecretsVaultName)
		waitForResourceNotFound("vaults.keyvault.azure.com", expectedExternalSecretsASOVaultName)
		waitForResourceNotFound("roleassignments.authorization.azure.com", expectedExternalSecretsRoleAssign)
		waitForResourceNotFound("secretstores.external-secrets.io", expectedSecretStoreName)
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

		By("patching dependent ASO resources to Ready so parent status can be projected")
		resourceID := "/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/Microsoft.KeyVault/vaults/vault-sample"
		vaultURI := "https://vault-sample.vault.azure.net"
		roleAssignmentID := resourceID + "/providers/Microsoft.Authorization/roleAssignments/role-123"
		patchKeyVaultReadyStatus(expectedASOVaultName, resourceID, vaultURI)
		patchRoleAssignmentReadyStatus(expectedRoleAssignName, roleAssignmentID)

		By("verifying Vault status projects dependent readiness and identifiers")
		waitForJSONPath("vaults.vault.dis.altinn.cloud", sampleVaultName, "{.status.conditions[?(@.type=='VaultReady')].status}", "True")
		waitForJSONPath("vaults.vault.dis.altinn.cloud", sampleVaultName, "{.status.conditions[?(@.type=='RoleAssignmentReady')].status}", "True")
		waitForJSONPath("vaults.vault.dis.altinn.cloud", sampleVaultName, "{.status.conditions[?(@.type=='Ready')].status}", "True")
		waitForJSONPath("vaults.vault.dis.altinn.cloud", sampleVaultName, "{.status.resourceId}", resourceID)
		waitForJSONPath("vaults.vault.dis.altinn.cloud", sampleVaultName, "{.status.vaultUri}", vaultURI)
		waitForJSONPath("vaults.vault.dis.altinn.cloud", sampleVaultName, "{.status.ownerRoleAssignmentId}", roleAssignmentID)
		waitForJSONPathContains(
			"vaults.keyvault.azure.com",
			expectedASOVaultName,
			"{.spec.properties.networkAcls.virtualNetworkRules[*].reference.armId}",
			expectedVPNExitNodeSubnetID,
		)
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
			waitForJSONPath("vaults.keyvault.azure.com", expectedASOVaultName, "{.spec.properties.sku.name}", "premium")
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

	It("manages a SecretStore for the external secrets sample", func() {
		By("applying the external secrets Vault sample")
		applyManifest(externalSecretsSamplePath)

		By("waiting for Azure child resources to be created")
		Eventually(func() error {
			checkCmd := utils.Kubectl("get", "vaults.keyvault.azure.com", expectedExternalSecretsASOVaultName, "-n", sampleNamespace)
			_, err := utils.Run(checkCmd)
			return err
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())
		Eventually(func() error {
			checkCmd := utils.Kubectl("get", "roleassignments.authorization.azure.com", expectedExternalSecretsRoleAssign, "-n", sampleNamespace)
			_, err := utils.Run(checkCmd)
			return err
		}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).Should(Succeed())

		By("verifying the SecretStore is not created before the Vault URI is known")
		ensureResourceNotCreatedYet("secretstores.external-secrets.io", expectedSecretStoreName)

		By("patching dependent ASO resources to Ready")
		resourceID := "/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/Microsoft.KeyVault/vaults/vault-es-sample"
		vaultURI := "https://vault-es-sample.vault.azure.net"
		roleAssignmentID := resourceID + "/providers/Microsoft.Authorization/roleAssignments/role-123"
		patchKeyVaultReadyStatus(expectedExternalSecretsASOVaultName, resourceID, vaultURI)
		patchRoleAssignmentReadyStatus(expectedExternalSecretsRoleAssign, roleAssignmentID)

		By("verifying the managed SecretStore is created with workload identity settings")
		waitForJSONPath("secretstores.external-secrets.io", expectedSecretStoreName, "{.spec.provider.azurekv.authType}", "WorkloadIdentity")
		waitForJSONPath("secretstores.external-secrets.io", expectedSecretStoreName, "{.spec.provider.azurekv.serviceAccountRef.name}", "app-identity-sample")
		waitForJSONPath("secretstores.external-secrets.io", expectedSecretStoreName, "{.spec.provider.azurekv.vaultUrl}", vaultURI)
		waitForJSONPath("secretstores.external-secrets.io", expectedSecretStoreName, "{.spec.provider.azurekv.tenantId}", "00000000-0000-0000-0000-000000000000")

		By("verifying Vault status reports External Secrets readiness")
		waitForJSONPath("vaults.vault.dis.altinn.cloud", externalSecretsVaultName, "{.status.conditions[?(@.type=='ExternalSecretsReady')].status}", "True")
		waitForJSONPath("vaults.vault.dis.altinn.cloud", externalSecretsVaultName, "{.status.externalSecretStoreName}", expectedSecretStoreName)
		waitForJSONPath("vaults.vault.dis.altinn.cloud", externalSecretsVaultName, "{.status.conditions[?(@.type=='Ready')].status}", "True")

		By("disabling external secrets integration")
		patchPayload := `{"spec":{"externalSecrets":false}}`
		cmd := utils.Kubectl(
			"patch", "vaults.vault.dis.altinn.cloud", externalSecretsVaultName,
			"-n", sampleNamespace,
			"--type=merge",
			"-p", patchPayload,
		)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch external secrets sample")

		By("verifying the managed SecretStore is deleted and status reflects Disabled")
		waitForResourceNotFound("secretstores.external-secrets.io", expectedSecretStoreName)
		waitForJSONPath("vaults.vault.dis.altinn.cloud", externalSecretsVaultName, "{.status.conditions[?(@.type=='ExternalSecretsReady')].status}", "False")
		waitForJSONPath("vaults.vault.dis.altinn.cloud", externalSecretsVaultName, "{.status.conditions[?(@.type=='ExternalSecretsReady')].reason}", "Disabled")
		waitForJSONPath("vaults.vault.dis.altinn.cloud", externalSecretsVaultName, "{.status.externalSecretStoreName}", "")
		waitForJSONPath("vaults.vault.dis.altinn.cloud", externalSecretsVaultName, "{.status.conditions[?(@.type=='Ready')].status}", "True")

		By("cleaning up the external secrets sample")
		deleteManifest(externalSecretsSamplePath)
		waitForResourceNotFound("vaults.vault.dis.altinn.cloud", externalSecretsVaultName)
		waitForResourceNotFound("vaults.keyvault.azure.com", expectedExternalSecretsASOVaultName)
		waitForResourceNotFound("roleassignments.authorization.azure.com", expectedExternalSecretsRoleAssign)
	})
})
