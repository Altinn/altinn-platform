package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Utils", func() {
	Context("isUrl", func() {
		It("should return true for valid URLs", func() {
			Expect(isUrl("http://example.com")).To(BeTrue())
			Expect(isUrl("https://example.com")).To(BeTrue())
			Expect(isUrl("http://example.com:8080")).To(BeTrue())
		})

		It("should return false for invalid URLs", func() {
			Expect(isUrl("example.com")).To(BeFalse())
			Expect(isUrl("/example/com")).To(BeFalse())
			Expect(isUrl("ftp://example.com")).To(BeFalse())
			Expect(isUrl("http://")).To(BeFalse())
		})
	})

	Context("getContentURL", func() {
		It("should return content for a valid URL", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprint(w, "test content")
			}))
			defer server.Close()

			resp, err := getContentURL(context.Background(), server.URL)
			Expect(err).NotTo(HaveOccurred())
			defer closeIgnoreError(resp.Body)

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("test content"))
		})

		It("should return an error for an invalid URL", func() {
			_, err := getContentURL(context.Background(), "http://invalid-url")
			Expect(err).To(HaveOccurred())
		})

		It("should return an error for content exceeding max size", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", maxContentSize+1))
				_, _ = fmt.Fprintln(w, "test content")
			}))
			defer server.Close()

			_, err := getContentURL(context.Background(), server.URL)
			Expect(err).To(HaveOccurred())
		})
		It("should return an error for non-200 status codes", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			_, err := getContentURL(context.Background(), server.URL)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected status code 404"))
		})
	})
})
