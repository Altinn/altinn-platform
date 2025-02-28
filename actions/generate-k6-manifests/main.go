package main

import (
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"os"
	"os/exec"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v3"
)

func main() {
	fmt.Println("Generating K6 Manifests")
	userConfigFile, ok := os.LookupEnv("INPUT_CONFIG_FILE")
	if !ok {
		log.Fatal("INPUT_CONFIG_FILE is mandatory")
	}
	cf := Initialize(userConfigFile)

	d, err := yaml.Marshal(&cf)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	err = os.WriteFile(".conf/expanded-configfile.yaml", d, 0644)
	if err != nil {
		fmt.Printf("Failed to create expanded config file\n err: %s", err)
		os.Exit(1)
	}

	for _, td := range cf.TestDefinitions {
		for i, c := range td.Contexts {
			if c.TestTypeDefinition.Enabled {
				var testConfigFileToUse = td.ConfigFile
				if *c.TestTypeDefinition.Type != "custom" {
					testConfigFileToUse = handleConfigFile(td.ConfigFile, *c.TestTypeDefinition.Type)
					if c.TestTypeDefinition.ConfigFile != "" {
						testConfigFileToUse = handleConfigFileOverride(testConfigFileToUse, c.TestTypeDefinition.ConfigFile)
					}
				}

				callK6Archive(testConfigFileToUse, td.TestFile)

				uniqName := fmt.Sprintf("%s-%d-%s-%d", *c.TestRun.Name, i, *c.TestTypeDefinition.Type, time.Now().Unix())

				callKubectl(uniqName, cf.Namespace)

				extraEnvVars, err := yaml.Marshal(c.TestRun.Env)
				if err != nil {
					log.Fatalf("error: %v", err)
				}
				resources, err := yaml.Marshal(c.TestRun.Resources)
				if err != nil {
					log.Fatalf("error: %v", err)
				}

				callJsonnet(uniqName, cf.Namespace, c.Environment, *c.TestRun.Parallelism, *c.NodeType, "", extraEnvVars, resources)
			}
		}
	}
}

func handleConfigFileOverride(defaultConfigFile string, overrideConfigFile string) string {
	base := map[string]interface{}{}
	override := map[string]interface{}{}

	defaultConfig, err := os.ReadFile(defaultConfigFile)
	if err != nil {
		log.Fatal(err)
	}
	overrideConfig, err := os.ReadFile(overrideConfigFile)
	if err != nil {
		log.Fatal(err)
	}

	json.Unmarshal([]byte(defaultConfig), &base)
	json.Unmarshal([]byte(overrideConfig), &override)
	maps.Copy(base, override)

	mergedOutMarshalled, err := json.Marshal(base)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(".conf/tweaked-testconfig.json", mergedOutMarshalled, 0644)
	if err != nil {
		log.Fatal(err)
	}
	return ".conf/tweaked-testconfig.json"
}

func handleConfigFile(defaultConfigFile string, testType string) string {
	mergedOut := map[string]interface{}{}

	if defaultConfigFile != "" {
		testConfig, err := os.ReadFile(defaultConfigFile)
		if err != nil {
			log.Fatal(err)
		}
		json.Unmarshal([]byte(testConfig), &mergedOut)
	}

	_, okst := mergedOut["stages"]
	_, oksc := mergedOut["scenarios"]
	// Don't override if users configured it.
	if !(okst || oksc) {
		defaultScenario, err := os.ReadFile(fmt.Sprintf("./actions/generate-k6-manifests/default_scenarios/%s.json", testType))
		if err != nil {
			log.Fatal(err)
		}
		temp := map[string]interface{}{}
		json.Unmarshal([]byte(defaultScenario), &temp)
		maps.Copy(mergedOut, temp)

		mergedOutMarshalled, err := json.Marshal(mergedOut)
		if err != nil {
			log.Fatal(err)
		}

		err = os.WriteFile(".conf/tweaked-testconfig.json", mergedOutMarshalled, 0644)
		if err != nil {
			log.Fatal(err)
		}
		return ".conf/tweaked-testconfig.json"
	}
	// handle override
	return defaultConfigFile
}

func callK6Archive(testConfigFileToUse string, testFile string) {
	var cmd *exec.Cmd
	if testConfigFileToUse != "" {
		cmd = exec.Command("k6",
			"archive",
			"--config",
			testConfigFileToUse,
			testFile,
			"-O",
			".build/archive.tar",
		)
	} else {
		cmd = exec.Command("k6",
			"archive",
			testFile,
			"-O",
			".build/archive.tar",
		)
	}

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Failed to call k6 archive --config %s %s\n err: %s", testConfigFileToUse, testFile, err)
		os.Exit(1)
	}
}

func callKubectl(uniqName string, namespace string) {
	cmd := exec.Command("kubectl",
		"create",
		"configmap",
		uniqName,
		"--from-file=.build/archive.tar",
		"-o", "json",
		"-n", namespace,
		"--dry-run=client",
	)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Failed to call kubectl create configmap %s --from-file=archive.tar -o json -n %s --dry-run=client\n err: %s", uniqName, namespace, err)
		os.Exit(1)
	}
	// TODO: Add labels to ConfigMap; at least k6-test with unique identifier
	err = os.WriteFile(fmt.Sprintf(".dist/configmap-%s.json", uniqName), []byte(out.String()), 0644)
	if err != nil {
		fmt.Printf("Failed to create configmap file: %s\n err: %s", uniqName, err)
		os.Exit(1)
	}
}

func callJsonnet(uniqName string, namespace string, environment string, parallelism int, nodeType string, sealedSecretName string, extraEnvVars []byte, resources []byte) {
	var errb strings.Builder
	k6ClusterConfigFile, err := os.ReadFile("./actions/generate-k6-manifests/infra/k6_cluster_conf.yaml")
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command("jsonnet",
		"--jpath", "vendor",
		"--ext-str", fmt.Sprintf("unique_name=%s", uniqName),
		"--ext-str", fmt.Sprintf("namespace=%s", namespace),
		"--ext-str", fmt.Sprintf("deploy_env=%s", environment),
		"--ext-str", fmt.Sprintf("parallelism=%d", parallelism),
		"--ext-str", fmt.Sprintf("node_type=%s", nodeType),
		"--ext-str", fmt.Sprintf("sealed_secret_name=%s", sealedSecretName),
		"--ext-str", fmt.Sprintf("extra_env_vars=%s", extraEnvVars),
		"--ext-str", fmt.Sprintf("resources=%s", resources),
		"--ext-str", fmt.Sprintf("extra_cli_args=%s", os.Getenv("INPUT_COMMAND_LINE_ARGS")),
		"--ext-str", fmt.Sprintf("k6clusterconfig=%s", k6ClusterConfigFile),
		"--multi", ".dist", "/github/workspace/actions/generate-k6-manifests/main.jsonnet",
	)
	cmd.Stderr = &errb
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Failed to generate resources via Jsonnet\nerr:%s", errb.String())
		os.Exit(1)
	}
}
