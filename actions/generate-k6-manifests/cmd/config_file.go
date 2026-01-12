package cmd

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

type ConfigFile struct {
	Namespace              string            `yaml:"namespace"`
	BaseDir                string            `yaml:"base_dir"`
	TestDefinitions        []*TestDefinition `yaml:"test_definitions"`
	ValidEnvironmentValues []string          `yaml:"-"`
	ValidTestTypes         []string          `yaml:"-"`
}

type TestDefinition struct {
	TestScope  string         `yaml:"test_scope"`
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
	Name             *string    `yaml:"name"`
	Id               *string    `yaml:"id,omitempty"`
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
	return slices.Contains(cFile.ValidEnvironmentValues, environmentName)
}

func (cFile *ConfigFile) IsValidTestType(testType string) bool {
	return slices.Contains(cFile.ValidTestTypes, testType)
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

		if cFile.BaseDir != "" {
			if !strings.HasPrefix(td.TestFile, cFile.BaseDir) {
				td.TestFile = filepath.Join(cFile.BaseDir, td.TestFile)
			}
			if td.ConfigFile != "" && !strings.HasPrefix(td.ConfigFile, cFile.BaseDir) {
				td.ConfigFile = filepath.Join(cFile.BaseDir, td.ConfigFile)
			}
			if td.EnvFile != "" && !strings.HasPrefix(td.EnvFile, cFile.BaseDir) {
				td.EnvFile = filepath.Join(cFile.BaseDir, td.EnvFile)
			}
		}

		if td.TestScope == "" {
			td.TestScope = strings.Split(td.TestFile, "/")[len(strings.Split(td.TestFile, "/"))-2]
		}
		td.TestScope = strings.ReplaceAll(td.TestScope, "_", "-")

		for _, c := range td.Contexts {
			// If no test types are configured, add a functional test definition by default.
			if c.TestTypeDefinition == nil {
				functional := "functional"
				c.TestTypeDefinition = &TestTypeDefinition{
					Type:    &functional,
					Enabled: true,
				}
			}
			if c.TestRun == nil {
				c.TestRun = &TestRun{}
			}
			if c.TestRun.Name == nil || *c.TestRun.Name == "" {
				c.TestRun.Name = sanitizeNameFromTestFileName(td.TestFile)
			}
			if c.TestRun.Id == nil || *c.TestRun.Id == "" {
				suffix := ""
				switch *c.TestTypeDefinition.Type {
				case "breakpoint":
					suffix = "-break"
				case "smoke":
					suffix = "-smoke"
				}

				tempString := sanitizeNameFromTestFileName(td.TestFile)
				defaultId := fmt.Sprintf("%s-%s%s", c.Environment, *tempString, suffix)
				c.TestRun.Id = &defaultId
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
				if *c.TestTypeDefinition.Type == "breakpoint" {
					defaultMemoryRequests = "4Gi"
				}
				c.TestRun.Resources.Requests.Memory = &defaultMemoryRequests
			}
			if c.TestRun.Resources.Requests.Cpu == nil || *c.TestRun.Resources.Requests.Cpu == "" {
				defaultCpuRequests := "250m"
				if *c.TestTypeDefinition.Type == "breakpoint" {
					defaultCpuRequests = "2"
				}
				c.TestRun.Resources.Requests.Cpu = &defaultCpuRequests
			}
			found := false
			tokenGeneratorCreds := "token-generator-creds"
			for i := range c.TestRun.SecretReferences {
				if *c.TestRun.SecretReferences[i] == tokenGeneratorCreds {
					found = true
				}
			}
			if !found {
				c.TestRun.SecretReferences = append(c.TestRun.SecretReferences, &tokenGeneratorCreds)
			}
			if c.NodeType == nil || *c.NodeType == "" {
				// As of now we have enough capacity in the default node pool to run functional and smoke tests there.
				nodeType := "default"
				if *c.TestTypeDefinition.Type == "breakpoint" {
					nodeType = "spot"
				}
				c.NodeType = &nodeType
			}
		}
	}
}
