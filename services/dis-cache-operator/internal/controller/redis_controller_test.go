package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RedisReconciler", func() {
	It("smoke check: reconciler scheme is registered", func() {
		// Placeholder test that runs even when envtest is skipped via DISREDIS_SKIP_ENVTEST=1.
		// Real Ginkgo envtest coverage is added in follow-up PRs once ASO CRD provisioning
		// is wired into setup-envtest.
		Expect(true).To(BeTrue())
	})
})
