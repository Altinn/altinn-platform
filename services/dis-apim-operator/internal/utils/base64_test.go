package utils

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Base64FromContent", func() {
	Context("with a valid URL", func() {
		It("should return the correct Base64 string", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintln(w, "test content")
			}))
			defer server.Close()
			expectedHash := "dGVzdCBjb250ZW50Cg=="
			hash, err := Base64FromContent(context.Background(), &server.URL)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash).To(BeEquivalentTo(expectedHash))
		})
	})

	Context("with a valid content string", func() {
		It("should return the correct Base64 string", func() {
			content := "test content"
			expectedHash := "dGVzdCBjb250ZW50"
			hash, err := Base64FromContent(context.Background(), &content)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash).To(BeEquivalentTo(expectedHash))
		})
	})

	Context("handle nil and empty string", func() {
		It("should return empty string when nil", func() {
			expectedHash := ""
			hash, err := Base64FromContent(context.Background(), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash).To(BeEquivalentTo(expectedHash))
		})
		It("should return empty string when empty string content", func() {
			expectedHash := ""
			emptyString := ""
			hash, err := Base64FromContent(context.Background(), &emptyString)
			Expect(err).NotTo(HaveOccurred())
			Expect(hash).To(BeEquivalentTo(expectedHash))
		})
	})

	Context("with an invalid URL", func() {
		It("should return an error", func() {
			invalidUrl := "http://invalid-url"
			_, err := Base64FromContent(context.Background(), &invalidUrl)
			Expect(err).To(HaveOccurred())
		})
	})
})
