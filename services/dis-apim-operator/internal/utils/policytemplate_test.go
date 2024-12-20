package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const ValidTemplate = "Hello, {{.Name}}!"

var _ = Describe("GeneratePolicyFromTemplate", func() {

	Context("with valid template and data", func() {

		It("should generate the correct policy", func() {
			expected := "Hello, World!"
			templateContent := ValidTemplate
			data := map[string]string{"Name": "World"}
			result, err := GeneratePolicyFromTemplate(templateContent, data)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expected))
		})
	})

	Context("handles quotes", func() {
		It("should generate the correct policy", func() {
			expected := `Hello, "World"!`
			templateContent := `Hello, "{{.Name}}"!`
			data := map[string]string{"Name": "World"}
			result, err := GeneratePolicyFromTemplate(templateContent, data)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expected))
		})
	})

	Context("with template missing data", func() {
		It("should return an error", func() {
			expected := ""
			templateContent := ValidTemplate
			data := map[string]string{}
			result, err := GeneratePolicyFromTemplate(templateContent, data)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(expected))
		})
	})

	Context("with template data nil", func() {
		It("should return an error", func() {
			expected := ""
			templateContent := ValidTemplate
			result, err := GeneratePolicyFromTemplate(templateContent, nil)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(expected))
		})
	})

	Context("with invalid template syntax", func() {
		It("should return an error", func() {
			expected := ""
			templateContent := "Hello, {{.Name"
			data := map[string]string{"Name": "World"}
			result, err := GeneratePolicyFromTemplate(templateContent, data)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(expected))
		})
	})

	Context("with empty template content", func() {
		It("should return empty string", func() {
			expected := ""
			templateContent := ""
			data := map[string]string{"Name": "World"}
			result, err := GeneratePolicyFromTemplate(templateContent, data)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expected))
		})
	})
})
