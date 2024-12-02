package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sha256FromContent", func() {
	Context("with a valid URL", func() {
		It("should return the correct SHA256 hash", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, "test content")
			}))
			defer server.Close()
			expectedHash := "a1fff0ffefb9eace7230c24e50731f0a91c62f9cefdfe77121c2f607125dffae"
			hash, err := Sha256FromContent(server.URL)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash).To(BeEquivalentTo(expectedHash))
		})
	})

	Context("with a valid content string", func() {
		It("should return the correct SHA256 hash", func() {
			content := "test content"
			expectedHash := "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72"
			hash, err := Sha256FromContent(content)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash).To(BeEquivalentTo(expectedHash))
		})
	})

	Context("with an invalid URL", func() {
		It("should return an error", func() {
			_, err := Sha256FromContent("http://invalid-url")
			Expect(err).To(HaveOccurred())
		})
	})
})
