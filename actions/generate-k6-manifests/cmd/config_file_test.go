package cmd

import (
	"testing"

	yaml "gopkg.in/yaml.v3"
)

var validEnvironmentsTests = []struct {
	envName  string // input
	expected bool   // expected result
}{
	{"at21", false},
	{"at22", true},
	{"at23", true},
	{"at24", true},
	{"at25", false},
	{"", false},
	{"yt01", true},
	{"prod", true},
	{"custom", true},
	{"randomstring", false},
	{"tt02", true},
}

func TestIsValidDeploymentEnvironment(t *testing.T) {
	configFile := ConfigFile{
		ValidEnvironmentValues: []string{
			"at22",
			"at23",
			"at24",
			"tt02",
			"yt01",
			"prod",
			"custom",
		},
	}

	for _, tt := range validEnvironmentsTests {
		actual := configFile.IsValidDeploymentEnvironment(tt.envName)
		if actual != tt.expected {
			t.Errorf("isValidDeploymentEnvironment(%s): expected %t, actual %t", tt.envName, tt.expected, actual)
		}
	}
}

func TestDefaults(t *testing.T) {
	var bareMinimumConfig = `
namespace: platform
test_definitions:
  - test_file: services/k6/first_test.js
    contexts:
      - environment: yt01
`
	configFile := ConfigFile{
		ValidEnvironmentValues: []string{
			"at22",
			"at23",
			"at24",
			"yt01",
			"prod",
		},
		ValidTestTypes: []string{
			"smoke",
			"soak",
			"spike",
			"breakpoint",
			"browser",
			"custom",
		},
	}

	err := yaml.Unmarshal([]byte(bareMinimumConfig), &configFile)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if !configFile.IsValid() {
		t.Errorf("Expected config file to be valid, but it's not")
	}

	configFile.SetDefaults()

	if configFile.Namespace != "platform" {
		t.Errorf("setDefaults: expected %s, actual %s", "platform", configFile.Namespace)
	}
	if configFile.TestDefinitions[0].TestFile != "services/k6/first_test.js" {
		t.Errorf("setDefaults: expected %s, actual %s", "services/k6/first_test.js", configFile.TestDefinitions[0].TestFile)
	}
	if configFile.TestDefinitions[0].Contexts[0].Environment != "yt01" {
		t.Errorf("setDefaults: expected %s, actual %s", "yt01", configFile.TestDefinitions[0].Contexts[0].Environment)
	}
	if *configFile.TestDefinitions[0].Contexts[0].NodeType != "spot" {
		t.Errorf("setDefaults: expected %s, actual %s", "spot", *configFile.TestDefinitions[0].Contexts[0].NodeType)
	}
	if *configFile.TestDefinitions[0].Contexts[0].TestTypeDefinition.Type != "functional" {
		t.Errorf("setDefaults: expected %s, actual %s", "functional", *configFile.TestDefinitions[0].Contexts[0].TestTypeDefinition.Type)
	}
	if configFile.TestDefinitions[0].Contexts[0].TestTypeDefinition.Enabled != true {
		t.Errorf("setDefaults: expected %t, actual %t", true, configFile.TestDefinitions[0].Contexts[0].TestTypeDefinition.Enabled)
	}
	if *configFile.TestDefinitions[0].Contexts[0].TestRun.Name != "first-test" {
		t.Errorf("setDefaults: expected %s, actual %s", "first-test", *configFile.TestDefinitions[0].Contexts[0].TestRun.Name)
	}
	if *configFile.TestDefinitions[0].Contexts[0].TestRun.Parallelism != 1 {
		t.Errorf("setDefaults: expected %d, actual %d", 1, *configFile.TestDefinitions[0].Contexts[0].TestRun.Parallelism)
	}
	if *configFile.TestDefinitions[0].Contexts[0].TestRun.Resources.Requests.Memory != "200Mi" {
		t.Errorf("setDefaults: expected %s, actual %s", "200Mi", *configFile.TestDefinitions[0].Contexts[0].TestRun.Resources.Requests.Memory)
	}
	if *configFile.TestDefinitions[0].Contexts[0].TestRun.Resources.Requests.Cpu != "250m" {
		t.Errorf("setDefaults: expected %s, actual %s", "250m", *configFile.TestDefinitions[0].Contexts[0].TestRun.Resources.Requests.Cpu)
	}
}

func TestTwoTests(t *testing.T) {
	var bareMinimumConfig = `
namespace: platform

test_definitions:
  - test_file: services/k6/test_k8s_wrapper_get_deployments.js
    contexts:
      - environment: at22
  - test_file: services/k6/test_k8s_wrapper_get_daemonsets.js
    contexts:
      - environment: at22
`
	configFile := ConfigFile{
		ValidEnvironmentValues: []string{
			"at22",
			"at23",
			"at24",
			"yt01",
			"prod",
		},
		ValidTestTypes: []string{
			"smoke",
			"soak",
			"spike",
			"breakpoint",
			"browser",
			"custom",
		},
	}

	err := yaml.Unmarshal([]byte(bareMinimumConfig), &configFile)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if !configFile.IsValid() {
		t.Errorf("Expected config file to be valid, but it's not")
	}

	configFile.SetDefaults()

	if configFile.Namespace != "platform" {
		t.Errorf("setDefaults: expected %s, actual %s", "platform", configFile.Namespace)
	}
	if configFile.TestDefinitions[0].TestFile != "services/k6/test_k8s_wrapper_get_deployments.js" {
		t.Errorf("setDefaults: expected %s, actual %s", "services/k6/test_k8s_wrapper_get_deployments.js", configFile.TestDefinitions[0].TestFile)
	}
	if configFile.TestDefinitions[0].Contexts[0].Environment != "at22" {
		t.Errorf("setDefaults: expected %s, actual %s", "at22", configFile.TestDefinitions[0].Contexts[0].Environment)
	}
	if *configFile.TestDefinitions[0].Contexts[0].NodeType != "spot" {
		t.Errorf("setDefaults: expected %s, actual %s", "spot", *configFile.TestDefinitions[0].Contexts[0].NodeType)
	}
	if *configFile.TestDefinitions[0].Contexts[0].TestTypeDefinition.Type != "functional" {
		t.Errorf("setDefaults: expected %s, actual %s", "functional", *configFile.TestDefinitions[0].Contexts[0].TestTypeDefinition.Type)
	}
	if configFile.TestDefinitions[0].Contexts[0].TestTypeDefinition.Enabled != true {
		t.Errorf("setDefaults: expected %t, actual %t", true, configFile.TestDefinitions[0].Contexts[0].TestTypeDefinition.Enabled)
	}
	if *configFile.TestDefinitions[0].Contexts[0].TestRun.Name != "test-k8s-wrapper-get-deployments" {
		t.Errorf("setDefaults: expected %s, actual %s", "test-k8s-wrapper-get-deployments", *configFile.TestDefinitions[0].Contexts[0].TestRun.Name)
	}
	if *configFile.TestDefinitions[0].Contexts[0].TestRun.Parallelism != 1 {
		t.Errorf("setDefaults: expected %d, actual %d", 1, *configFile.TestDefinitions[0].Contexts[0].TestRun.Parallelism)
	}
	if *configFile.TestDefinitions[0].Contexts[0].TestRun.Resources.Requests.Memory != "200Mi" {
		t.Errorf("setDefaults: expected %s, actual %s", "200Mi", *configFile.TestDefinitions[0].Contexts[0].TestRun.Resources.Requests.Memory)
	}
	if *configFile.TestDefinitions[0].Contexts[0].TestRun.Resources.Requests.Cpu != "250m" {
		t.Errorf("setDefaults: expected %s, actual %s", "250m", *configFile.TestDefinitions[0].Contexts[0].TestRun.Resources.Requests.Cpu)
	}
}
