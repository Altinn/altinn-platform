package main

import (
	"path/filepath"
	"strings"
)

func (cFile *ConfigFile) isValidDeploymentEnvironment(environmentName string) bool {
	for _, i := range cFile.validEnvironmentValues {
		if i == environmentName {
			return true
		}
	}
	return false
}

func (cFile *ConfigFile) isValidTestType(testType string) bool {
	for _, i := range cFile.validTestTypes {
		if i == testType {
			return true
		}
	}
	return false
}

func (cFile *ConfigFile) hasValidTestTypes() bool {
	for _, td := range cFile.TestDefinitions {
		for _, c := range td.Contexts {
			if c.TestTypeDefinition != nil && !cFile.isValidTestType(*c.TestTypeDefinition.Type) {
				return false
			}
		}
	}
	return true
}

func (cFile *ConfigFile) hasValidDeploymentEnvironments() bool {
	for _, td := range cFile.TestDefinitions {
		for _, c := range td.Contexts {
			if !cFile.isValidDeploymentEnvironment(c.Environment) {
				return false
			}
		}
	}
	return true
}

// Validates the config file the user generated
func (cFile *ConfigFile) isValid() bool {
	// Validate envs are correct, that test files exist, that node types are valid, etc.
	return cFile.hasValidDeploymentEnvironments() && cFile.hasValidTestTypes()
}

// Sets defaults for things that the user did not configure.
func (cFile *ConfigFile) setDefaults() {
	for _, td := range cFile.TestDefinitions {
		for _, c := range td.Contexts {
			// Use Spot nodes by default.
			if c.NodeType == nil || *c.NodeType == "" {
				spot := "spot"
				c.NodeType = &spot
			}
			// If no test types are configured, add a smoke test definition by default.
			if c.TestTypeDefinition == nil {
				smoke := "smoke"
				c.TestTypeDefinition = &TestTypeDefinition{
					Type:    &smoke,
					Enabled: true,
				}
			}

			// If TestRun Name isn't passed, default to path to file.
			// TODO: Append test type eventually
			if c.TestRun == nil {
				c.TestRun = &TestRun{}
			}
			if c.TestRun.Name == nil || *c.TestRun.Name == "" {
				tempString := strings.ReplaceAll(td.TestFile, "/", "-")
				tempString = strings.ReplaceAll(tempString, "_", "-")
				// Remove .js or .ts
				tempString = strings.TrimSuffix(tempString, filepath.Ext(tempString))
				c.TestRun.Name = &tempString
			}
			if c.TestRun.Parallelism == nil || *c.TestRun.Parallelism <= 0 {
				tempInt := 1
				c.TestRun.Parallelism = &tempInt
			}
			if c.TestRun.Resources == nil {
				c.TestRun.Resources = &Resources{
					Requests: &Requests{},
				}
			}
			if c.TestRun.Resources.Requests.Memory == nil || *c.TestRun.Resources.Requests.Memory == "" {
				defaultMemoryRequests := "200Mi"
				c.TestRun.Resources.Requests.Memory = &defaultMemoryRequests
			}
			if c.TestRun.Resources.Requests.Cpu == nil || *c.TestRun.Resources.Requests.Cpu == "" {
				defaultCpuRequests := "250m"
				c.TestRun.Resources.Requests.Cpu = &defaultCpuRequests
			}
		}
	}
}
