package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ConfigFile struct {
	Namespace              string            `yaml:"namespace"`
	TestDefinitions        []*TestDefinition `yaml:"test_definitions"`
	ValidEnvironmentValues []string          `yaml:"-"`
	ValidTestTypes         []string          `yaml:"-"`
}

type TestDefinition struct {
	TestFile   string         `yaml:"test_file"`
	ConfigFile string         `yaml:"config_file"`
	EnvFile    string         `yaml:"env_file"`
	Contexts   []*TestContext `yaml:"contexts"`
}

type TestContext struct {
	Environment        string              `yaml:"environment"`
	NodeType           *string             `yaml:"node_type"`
	TestTypeDefinition *TestTypeDefinition `yaml:"test_type"`
	TestRun            *TestRun            `yaml:"test_run"`
}

type TestTypeDefinition struct {
	Type       *string `yaml:"type"`
	Enabled    bool    `yaml:"enabled"`
	ConfigFile string  `yaml:"config_file"`
}

type TestRun struct {
	Name             *string    `yaml:"name"` // Use the path to the file by default?
	Parallelism      *int       `yaml:"parallelism"`
	Resources        *Resources `yaml:"resources"`
	Env              []*Env     `yaml:"env"`
	SecretReferences []*string  `yaml:"secrets"`
}

type Resources struct {
	Requests *Requests `yaml:"requests"`
}

type Requests struct {
	Memory *string `yaml:"memory"`
	Cpu    *string `yaml:"cpu"`
}

type Env struct {
	Name  *string `yaml:"name"`
	Value *string `yaml:"value"`
}

func (cFile *ConfigFile) IsValidDeploymentEnvironment(environmentName string) bool {
	for _, i := range cFile.ValidEnvironmentValues {
		if i == environmentName {
			return true
		}
	}
	return false
}

func (cFile *ConfigFile) IsValidTestType(testType string) bool {
	for _, i := range cFile.ValidTestTypes {
		if i == testType {
			return true
		}
	}
	return false
}

func (cFile *ConfigFile) HasOnlyValidTestTypes() bool {
	for _, td := range cFile.TestDefinitions {
		for _, c := range td.Contexts {
			if c.TestTypeDefinition != nil && c.TestTypeDefinition.Type != nil && !cFile.IsValidTestType(*c.TestTypeDefinition.Type) {
				fmt.Printf("detected invalid test type: %s\n", *c.TestTypeDefinition.Type)
				return false
			}
		}
	}
	return true
}

func (cFile *ConfigFile) HasOnlyValidDeploymentEnvironments() bool {
	for _, td := range cFile.TestDefinitions {
		for _, c := range td.Contexts {
			if !cFile.IsValidDeploymentEnvironment(c.Environment) {
				fmt.Printf("detected invalid environment: %s\n", c.Environment)
				return false
			}
		}
	}
	return true
}

// Validates the config file the user generated
func (cFile *ConfigFile) IsValid() bool {
	// Validate envs are correct, that test files exist, that node types are valid, etc.
	return cFile.HasOnlyValidDeploymentEnvironments() && cFile.HasOnlyValidTestTypes()
}

// Sets defaults for things that the user did not configure.
func (cFile *ConfigFile) SetDefaults() {
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
				//tempString := strings.ReplaceAll(td.TestFile, "/", "-")

				tempString := filepath.Base(td.TestFile)
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
