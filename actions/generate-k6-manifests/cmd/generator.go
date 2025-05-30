package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	orderedmap "github.com/elliotchance/orderedmap/v3"
	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

type Generator interface {
	Initialize(filePath string) *ConfigFile
	Generate()
	HandleConfigFile(defaultConfigFile string, testType string) map[string]interface{}
	HandleConfigFileOverride(base map[string]interface{}, overrideConfigFile string) map[string]interface{}
	CallK6Archive(uniqName string, testConfigFileToUse string, testFile string)
	CallKubectl(uniqName string, namespace string)
	CallJsonnet(uniqName string, manifestGenerationTimestamp string, namespace string, environment string, parallelism int, nodeType string, secretReferences []byte, extraEnvVars []byte, resources []byte)
}

type K8sManifestGenerator struct {
	UserConfigFile            string
	ConfigDirectory           string
	DistDirectory             string
	BuildDirectory            string
	DefaultScenariosDirectory string
	RepoRootDirectory         string
}

func (r K8sManifestGenerator) Initialize(filePath string) *ConfigFile {
	yfile, err := os.ReadFile(filePath)

	if err != nil {
		log.Fatal(err)
	}

	cf := ConfigFile{
		ValidEnvironmentValues: []string{
			"at22",
			"at23",
			"at24",
			"tt02",
			"yt01",
			"prod",
		},
		ValidTestTypes: []string{
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

	if !cf.IsValid() {
		log.Fatal("Config file is not valid.")
	}
	cf.SetDefaults()
	return &cf
}

func parseEnvFile(filePath string) []*Env {
	envFromFile := []*Env{}
	tempFile, err := os.Open(filePath)

	if err != nil {
		log.Fatalf("error opening file: %s", err)
	}
	defer tempFile.Close()

	scanner := bufio.NewScanner(tempFile)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines and comments
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		keyValue := strings.Split(line, "=")
		if len(keyValue) != 2 {
			log.Fatalf("expected %s to have the format KEY=VALUE", keyValue)
		}
		envFromFile = append(envFromFile, &Env{
			Name:  &keyValue[0],
			Value: &keyValue[1],
		})
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("error reading from file: %s", err)
	}
	return envFromFile
}

func handleExtraEnvVars(original []*Env, extra []*Env) []*Env {
	newEnv := []*Env{}
	m := orderedmap.NewOrderedMap[string, string]()
	for _, o := range original {
		m.Set(*o.Name, *o.Value)
	}
	for _, e := range extra {
		m.Set(*e.Name, *e.Value)
	}
	for k, v := range m.AllFromFront() {
		newEnv = append(newEnv, &Env{
			Name:  &k,
			Value: &v,
		})
	}
	return newEnv
}

func chooseCorrectSlackChannel(deploy_env string) string {
	branch, ok := os.LookupEnv("GITHUB_REF")

	if ok && branch == "refs/heads/main" {
		aux := map[string]string{
			"at22": "slack-dev",
			"at23": "slack-dev",
			"at24": "slack-dev",
			"yt01": "slack-dev",

			"tt02": "slack-prod",
			"prod": "slack-prod",
		}
		return aux[deploy_env]
	}

	return "slack-test"
}

func (r K8sManifestGenerator) Generate() {
	fmt.Println("Generating K6 Manifests")

	cf := r.Initialize(r.UserConfigFile)

	d, err := yaml.Marshal(&cf)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	err = os.WriteFile(fmt.Sprintf("%s/expanded-configfile.yaml", r.ConfigDirectory), d, 0644)
	if err != nil {
		fmt.Printf("Failed to create expanded config file\n err: %s", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote config file into: %s/expanded-configfile.yaml\n", r.ConfigDirectory)

	for _, td := range cf.TestDefinitions {
		var envFileSlice []*Env
		if td.EnvFile != "" {
			envFileSlice = parseEnvFile(fmt.Sprintf("%s/%s", r.RepoRootDirectory, td.EnvFile))
		}
		for i, c := range td.Contexts {
			if c.TestTypeDefinition.Enabled {
				var configFile map[string]interface{}
				if *c.TestTypeDefinition.Type != "custom" {
					configFile = r.HandleConfigFile(td.ConfigFile, *c.TestTypeDefinition.Type)
					if c.TestTypeDefinition.ConfigFile != "" {
						configFile = r.HandleConfigFileOverride(configFile, c.TestTypeDefinition.ConfigFile)
					}
				}

				manifestGenerationTimestamp := time.Now().UnixMilli()
				uniqName := fmt.Sprintf("%s-%s-%d-%d", c.Environment, *c.TestRun.Name, i, manifestGenerationTimestamp)

				marshalledConfigFile, err := json.MarshalIndent(configFile, "", "  ")
				if err != nil {
					log.Fatal(err)
				}

				newpath := filepath.Join(r.ConfigDirectory, uniqName)
				err = os.MkdirAll(newpath, os.ModePerm)
				if err != nil {
					log.Fatal(err)
				}

				testConfigFileToUse := fmt.Sprintf("%s/tweaked-testconfig.json", newpath)
				err = os.WriteFile(testConfigFileToUse, marshalledConfigFile, 0644)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Printf("Wrote k6 test config file into: %s\n", testConfigFileToUse)

				r.CallK6Archive(uniqName, testConfigFileToUse, fmt.Sprintf("%s/%s", r.RepoRootDirectory, td.TestFile))

				if utf8.RuneCountInString(uniqName) > 51 {
					log.Fatalf("Automatic generated name is too big: %s. Provide a default name such that the generated name does not go over 51 characters", uniqName)
				}

				r.CallKubectl(uniqName, cf.Namespace)
				// merge env file with overrides.
				githubRepositoryEnvName := "GITHUB_REPOSITORY"
				githubServerUrlEnvName := "GITHUB_SERVER_URL"
				githubRunIdEnvName := "GITHUB_RUN_ID"

				githubRepositoryEnvValue := os.Getenv(githubRepositoryEnvName)
				githubServerUrlEnvValue := os.Getenv(githubServerUrlEnvName)
				githubRunIdEnvValue := os.Getenv(githubRunIdEnvName)

				githubRelatedEnvVars := []*Env{
					&Env{
						Name:  &githubRepositoryEnvName,
						Value: &githubRepositoryEnvValue,
					},
					&Env{
						Name:  &githubServerUrlEnvName,
						Value: &githubServerUrlEnvValue,
					},
					&Env{
						Name:  &githubRunIdEnvName,
						Value: &githubRunIdEnvValue,
					},
				}

				testRunEnvWithGithubContext := handleExtraEnvVars(c.TestRun.Env, githubRelatedEnvVars)
				mergedEnvs := handleExtraEnvVars(envFileSlice, testRunEnvWithGithubContext)
				mergedEnvsMarshalled, err := yaml.Marshal(mergedEnvs)
				if err != nil {
					log.Fatalf("error: %v", err)
				}

				slackChannel := chooseCorrectSlackChannel(c.Environment)
				secretReferences, err := yaml.Marshal(append(c.TestRun.SecretReferences, &slackChannel))
				if err != nil {
					log.Fatalf("error: %v", err)
				}

				resources, err := yaml.Marshal(c.TestRun.Resources)
				if err != nil {
					log.Fatalf("error: %v", err)
				}
				r.CallJsonnet(uniqName, strconv.FormatInt(manifestGenerationTimestamp, 10), cf.Namespace, c.Environment, *c.TestRun.Parallelism, *c.NodeType, secretReferences, mergedEnvsMarshalled, resources)
			}
		}
	}
}

func (r K8sManifestGenerator) HandleConfigFileOverride(base map[string]interface{}, overrideConfigFile string) map[string]interface{} {
	override := map[string]interface{}{}

	overrideConfig, err := os.ReadFile(fmt.Sprintf("%s/%s", r.RepoRootDirectory, overrideConfigFile))
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal([]byte(overrideConfig), &override)
	if err != nil {
		log.Fatal(err)
	}
	maps.Copy(base, override)

	return base
}

func (r K8sManifestGenerator) HandleConfigFile(defaultConfigFile string, testType string) map[string]interface{} {
	mergedOut := map[string]interface{}{}

	if defaultConfigFile != "" {
		testConfig, err := os.ReadFile(fmt.Sprintf("%s/%s", r.RepoRootDirectory, defaultConfigFile))
		if err != nil {
			log.Fatal(err)
		}
		err = json.Unmarshal([]byte(testConfig), &mergedOut)
		if err != nil {
			log.Fatal(err)
		}
	}

	_, okst := mergedOut["stages"]
	_, oksc := mergedOut["scenarios"]
	// Don't override if users configured it.
	if !(okst || oksc) {
		defaultScenario, err := os.ReadFile(fmt.Sprintf("%s/%s.json", r.DefaultScenariosDirectory, testType))
		if err != nil {
			log.Fatal(err)
		}
		temp := map[string]interface{}{}
		err = json.Unmarshal([]byte(defaultScenario), &temp)
		if err != nil {
			log.Fatal(err)
		}
		maps.Copy(mergedOut, temp)
	}
	return mergedOut
}

func (r K8sManifestGenerator) CallK6Archive(uniqName string, testConfigFileToUse string, testFile string) {
	newpath := filepath.Join(r.BuildDirectory, uniqName)
	err := os.MkdirAll(newpath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	var cmd *exec.Cmd
	if testConfigFileToUse != "" {
		cmd = exec.Command("k6",
			"archive",
			"--config",
			testConfigFileToUse,
			testFile,
			"-O",
			fmt.Sprintf("%s/archive.tar", newpath),
		)
	} else {
		cmd = exec.Command("k6",
			"archive",
			testFile,
			"-O",
			fmt.Sprintf("%s/archive.tar", newpath),
		)
	}

	err = cmd.Run()
	if err != nil {
		fmt.Printf("Failed to call k6 archive --config %s %s\n err: %s", testConfigFileToUse, testFile, err)
		os.Exit(1)
	}
	fmt.Printf("Wrote archive.tar into: %s/archive.tar\n", newpath)
}

func (r K8sManifestGenerator) CallKubectl(uniqName string, namespace string) {
	cmd := exec.Command("kubectl",
		"create",
		"configmap",
		uniqName,
		fmt.Sprintf("--from-file=archive.tar=%s/%s/archive.tar", r.BuildDirectory, uniqName),
		"-o", "json",
		"-n", namespace,
		"--dry-run=client",
	)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Failed to call kubectl create configmap %s --from-file=archive.tar=%s/%s/archive.tar -o json -n %s --dry-run=client\n err: %s", uniqName, r.BuildDirectory, uniqName, namespace, errb.String())
		os.Exit(1)
	}

	var temp corev1.ConfigMap

	err = json.Unmarshal([]byte(out.String()), &temp)
	if err != nil {
		log.Fatal(err)
	}
	temp.SetLabels(map[string]string{
		"testid":            uniqName,
		"k6-test":           uniqName,
		"k6-test-configmap": "true",
	})

	tempMarshalled, err := json.MarshalIndent(temp, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	newpath := filepath.Join(r.DistDirectory, uniqName)
	err = os.MkdirAll(newpath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(fmt.Sprintf("%s/configmap.json", newpath), tempMarshalled, 0644)
	if err != nil {
		fmt.Printf("Failed to create configmap file: %s\n err: %s", uniqName, err)
		os.Exit(1)
	}
}

func (r K8sManifestGenerator) CallJsonnet(uniqName string, manifestGenerationTimestamp string, namespace string, environment string, parallelism int, nodeType string, secretReferences []byte, extraEnvVars []byte, resources []byte) {
	var errb strings.Builder
	k6ClusterConfigFile, err := os.ReadFile("/actions/generate-k6-manifests/infra/k6_cluster_conf.yaml")
	if err != nil {
		log.Fatal(err)
	}
	newpath := filepath.Join(r.DistDirectory, uniqName)
	err = os.MkdirAll(newpath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command("jsonnet",
		"--jpath", "/jsonnet/vendor",
		"--ext-str", fmt.Sprintf("unique_name=%s", uniqName),
		"--ext-str", fmt.Sprintf("manifest_generation_timestamp=%s", manifestGenerationTimestamp),
		"--ext-str", fmt.Sprintf("namespace=%s", namespace),
		"--ext-str", fmt.Sprintf("deploy_env=%s", environment),
		"--ext-str", fmt.Sprintf("parallelism=%d", parallelism),
		"--ext-str", fmt.Sprintf("node_type=%s", nodeType),
		"--ext-str", fmt.Sprintf("secret_references=%s", secretReferences),
		"--ext-str", fmt.Sprintf("extra_env_vars=%s", extraEnvVars),
		"--ext-str", fmt.Sprintf("resources=%s", resources),
		"--ext-str", fmt.Sprintf("extra_cli_args=%s", os.Getenv("INPUT_COMMAND_LINE_ARGS")),
		"--ext-str", fmt.Sprintf("k6clusterconfig=%s", k6ClusterConfigFile),
		"--multi", newpath, "/actions/generate-k6-manifests/jsonnet/main.jsonnet",
	)
	cmd.Stderr = &errb
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Failed to generate resources via Jsonnet\nerr:%s", errb.String())
		os.Exit(1)
	}
}
