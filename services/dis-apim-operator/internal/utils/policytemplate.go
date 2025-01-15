package utils

import (
	"strings"
	"text/template"
)

// GeneratePolicyFromTemplate generates a policy from a template
func GeneratePolicyFromTemplate(templateContent string, data interface{}) (string, error) {
	tmpl, err := template.New("policy").Parse(templateContent)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	tmpl.Option("missingkey=error")
	err = tmpl.Execute(&sb, data)
	if err != nil {
		return "", err
	}

	return sb.String(), nil
}
