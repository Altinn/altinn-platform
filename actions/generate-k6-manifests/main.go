package main

import (
	"log"
	"os"

	"github.com/Altinn/altinn-platform/actions/generate-k6-manifests/cmd"
)

func main() {
	userConfigFile, ok := os.LookupEnv("INPUT_CONFIG_FILE")
	if !ok {
		log.Fatal("INPUT_CONFIG_FILE is mandatory")
	}

	var g cmd.Generator = cmd.K8sManifestGenerator{
		ConfigDirectory:           ".conf",
		DistDirectory:             ".dist",
		BuildDirectory:            ".build",
		DefaultScenariosDirectory: "/actions/generate-k6-manifests/default_scenarios",
		RepoRootDirectory:         ".",
	}
	cf := g.Initialize(userConfigFile)
	g.Generate(*cf)
}
