package main

import (
	"log"
	"os"

	yaml "gopkg.in/yaml.v3"
)

type ConfigFile struct {
	Namespace              string            `yaml:"namespace"`
	TestDefinitions        []*TestDefinition `yaml:"test_definitions"`
	validEnvironmentValues []string
	validTestTypes         []string
}

func Initialize(filePath string) *ConfigFile {
	yfile, err := os.ReadFile(filePath)

	if err != nil {
		log.Fatal(err)
	}

	cf := ConfigFile{
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

	err = yaml.Unmarshal(yfile, &cf)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	if !cf.isValid() {
		log.Fatal("Config file is not valid.")
	}
	cf.setDefaults()
	return &cf
}

type TestDefinition struct {
	TestFile   string         `yaml:"test_file"`
	ConfigFile string         `yaml:"config_file"`
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
	Name        *string    `yaml:"name"` // Use the path to the file by default?
	Parallelism *int       `yaml:"parallelism"`
	Resources   *Resources `yaml:"resources"`
	Env         []*Env     `yaml:"env"`
}

type Resources struct {
	Requests *Requests `yaml:"requests"`
	// Limits   Limits  `yaml:"limits"`
}

type Requests struct {
	Memory *string `yaml:"memory"`
	Cpu    *string `yaml:"cpu"`
}

/*
	type Limits struct {
		Memory string
		Cpu    string
	}
*/

type Env struct {
	Name  *string `yaml:"name"`
	Value *string `yaml:"value"`
}
