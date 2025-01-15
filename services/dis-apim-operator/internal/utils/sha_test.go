package utils

import (
	"context"
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
				_, _ = fmt.Fprint(w, "test content")
			}))
			defer server.Close()
			expectedHash := "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72"
			hash, err := Sha256FromContent(context.Background(), &server.URL)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash).To(BeEquivalentTo(expectedHash))
		})
	})

	Context("with a valid content string", func() {
		It("should return the correct SHA256 hash", func() {
			content := "test content"
			expectedHash := "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72"
			hash, err := Sha256FromContent(context.Background(), &content)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash).To(BeEquivalentTo(expectedHash))
		})
	})

	Context("handle nil and empty string", func() {
		It("should return empty string when nil", func() {
			expectedHash := ""
			hash, err := Sha256FromContent(context.Background(), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash).To(BeEquivalentTo(expectedHash))
		})
		It("should return correct SHA256  when empty string content", func() {
			expectedHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
			emptyString := ""
			hash, err := Sha256FromContent(context.Background(), &emptyString)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash).To(BeEquivalentTo(expectedHash))
		})
	})

	Context("with an invalid URL", func() {
		It("should return an error", func() {
			invalidUrl := "http://invalid-url"
			_, err := Sha256FromContent(context.Background(), &invalidUrl)
			Expect(err).To(HaveOccurred())
		})
	})
})
