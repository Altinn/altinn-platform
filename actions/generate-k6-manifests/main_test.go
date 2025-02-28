package main

import (
	"fmt"
	"log"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

var validEnvironmentsTests = []struct {
	envName  string // input
	expected bool   // expected result
}{
	{"at21", true},
	{"at22", true},
	{"at23", true},
	{"at24", true},
	{"at25", false},
	{"", false},
	{"yt01", true},
	{"prod", true},
	{"custom", true},
	{"randomstring", false},
}

func TestIsValidDeploymentEnvironment(t *testing.T) {
	configFile := ConfigFile{
		validEnvironmentValues: []string{
			"at21",
			"at22",
			"at23",
			"at24",
			"yt01",
			"prod",
			"custom",
		},
	}

	for _, tt := range validEnvironmentsTests {
		actual := configFile.isValidDeploymentEnvironment(tt.envName)
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
		validEnvironmentValues: []string{
			"at21",
			"at22",
			"at23",
			"at24",
			"yt01",
			"prod",
		},
		validTestTypes: []string{
			"smoke",
			"soak",
			"spike",
			"breakpoint",
			"custom",
		},
	}

	err := yaml.Unmarshal([]byte(bareMinimumConfig), &configFile)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	if !configFile.isValid() {
		fmt.Println("Config file is not valid.")
	}

	configFile.setDefaults()

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
	if *configFile.TestDefinitions[0].Contexts[0].TestTypeDefinition.Type != "smoke" {
		t.Errorf("setDefaults: expected %s, actual %s", "smoke", *configFile.TestDefinitions[0].Contexts[0].TestTypeDefinition.Type)
	}
	if configFile.TestDefinitions[0].Contexts[0].TestTypeDefinition.Enabled != true {
		t.Errorf("setDefaults: expected %t, actual %t", true, configFile.TestDefinitions[0].Contexts[0].TestTypeDefinition.Enabled)
	}
	if *configFile.TestDefinitions[0].Contexts[0].TestRun.Name != "services-k6-first-test" {
		t.Errorf("setDefaults: expected %s, actual %s", "services-k6-first-test", *configFile.TestDefinitions[0].Contexts[0].TestRun.Name)
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
