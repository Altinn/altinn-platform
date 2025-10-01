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
	CallK6Archive(dirName string, testConfigFileToUse string, testFile string, k6ArchiveArgs []string)
	CallKubectl(dirName string, uniqName string, namespace string)
	CallJsonnet(dirName string, uniqName string, testName string, manifestGenerationTimestamp string, namespace string, environment string, parallelism int, nodeType string, secretReferences []byte, extraEnvVars []byte, resources []byte, isBrowserTest bool, testid string)
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
			"functional",
			"smoke",
			"soak",
			"spike",
			"breakpoint",
			"browser",
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

	requiredDirs := []string{".conf", ".build", ".dist"}

	for _, d := range requiredDirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			err := os.Mkdir(d, 0775)
			if err != nil {
				log.Fatalf("unable to create the %s directory, ensure it exists and try again: %v", d, err)
			}
		}
	}

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
					if *c.TestTypeDefinition.Type == "breakpoint" {
						configFile = r.HandleBreakpointConfigFile(c.TestRun.Env)
					} else {
						configFile = r.HandleConfigFile(td.ConfigFile, *c.TestTypeDefinition.Type)
						if c.TestTypeDefinition.ConfigFile != "" {
							configFile = r.HandleConfigFileOverride(configFile, c.TestTypeDefinition.ConfigFile)
						}
					}
				}

				manifestGenerationTimestamp := time.Now().UnixMilli()
				uniqName := fmt.Sprintf("%s-%s-%d-%d", c.Environment, *c.TestRun.Name, manifestGenerationTimestamp, i)

				marshalledConfigFile, err := json.MarshalIndent(configFile, "", "  ")
				if err != nil {
					log.Fatal(err)
				}
				// TODO: I'm still not sure what the best way to do this is. But I guess this approach is a bit less noisy than the previous.
				// Will keep revisiting as we start adding more tests over time.
				dirName := fmt.Sprintf("%s-%s", c.Environment, *c.TestRun.Name)
				newpath := filepath.Join(r.ConfigDirectory, dirName)
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

				// Add Env Vars to archive
				mergedEnvs := handleExtraEnvVars(envFileSlice, c.TestRun.Env)
				k6ArchiveArgs := []string{"--env", fmt.Sprintf("%s=%s", "ENVIRONMENT", c.Environment)}
				for _, env := range mergedEnvs {
					// --env MY_USER_AGENT="hello"
					k6ArchiveArgs = append(k6ArchiveArgs, "--env", fmt.Sprintf("%s=%s", *env.Name, *env.Value))
				}

				r.CallK6Archive(dirName, testConfigFileToUse, fmt.Sprintf("%s/%s", r.RepoRootDirectory, td.TestFile), k6ArchiveArgs)

				if utf8.RuneCountInString(uniqName) > 51 {
					log.Fatalf("Automatic generated name is too big: %s. Provide a default name such that the generated name does not go over 51 characters", uniqName)
				}

				r.CallKubectl(dirName, uniqName, cf.Namespace)
				// merge env file with overrides.
				githubRepositoryEnvName := "GITHUB_REPOSITORY"
				githubServerUrlEnvName := "GITHUB_SERVER_URL"
				githubRunIdEnvName := "GITHUB_RUN_ID"

				var githubRelatedEnvVars []*Env

				githubRepositoryEnvValue, ok := os.LookupEnv(githubRepositoryEnvName)
				if ok {
					githubRelatedEnvVars = append(githubRelatedEnvVars, &Env{
						Name:  &githubRepositoryEnvName,
						Value: &githubRepositoryEnvValue,
					})
				}
				githubServerUrlEnvValue, ok := os.LookupEnv(githubServerUrlEnvName)
				if ok {
					githubRelatedEnvVars = append(githubRelatedEnvVars, &Env{
						Name:  &githubServerUrlEnvName,
						Value: &githubServerUrlEnvValue,
					})
				}
				githubRunIdEnvValue, ok := os.LookupEnv(githubRunIdEnvName)
				if ok {
					githubRelatedEnvVars = append(githubRelatedEnvVars, &Env{
						Name:  &githubRunIdEnvName,
						Value: &githubRunIdEnvValue,
					})
				}

				mergedEnvs = handleExtraEnvVars(mergedEnvs, githubRelatedEnvVars)
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
				isBrowserTest := false
				if *c.TestTypeDefinition.Type == "browser" {
					isBrowserTest = true
				}
				if c.TestRun.Id != nil {
					r.CallJsonnet(dirName, uniqName, *c.TestRun.Name, strconv.FormatInt(manifestGenerationTimestamp, 10), cf.Namespace, c.Environment, *c.TestRun.Parallelism, *c.NodeType, secretReferences, mergedEnvsMarshalled, resources, isBrowserTest, *c.TestRun.Id)
				} else {
					r.CallJsonnet(dirName, uniqName, *c.TestRun.Name, strconv.FormatInt(manifestGenerationTimestamp, 10), cf.Namespace, c.Environment, *c.TestRun.Parallelism, *c.NodeType, secretReferences, mergedEnvsMarshalled, resources, isBrowserTest, "")
				}

				grafanaDashboard, ok := os.LookupEnv("GRAFANA_DASHBOARD")
				if !ok {
					grafanaDashboard = "d/ccbb2351-2ae2-462f-ae0e-f2c893ad1028/k6-prometheus"
				}

				fmt.Printf("\nTo run the test '%s' in '%s' run\n\tkubectl --context k6tests-cluster apply --server-side -f %s", *c.TestRun.Name, c.Environment, filepath.Join(r.DistDirectory, dirName))
				fmt.Printf("\nTo check the logs run\n\tkubectl --context k6tests-cluster -n %s logs -f --tail=-1 -l \"k6-test=%s,runner=true\"", cf.Namespace, uniqName)
				fmt.Printf("\nGrafana URL: \n\t%s/%s?orgId=1&var-DS_PROMETHEUS=%s&var-namespace=%s&var-testid=%s&from=%s&to=now&refresh=1m\n\n",
					"https://grafana.altinn.cloud",
					grafanaDashboard,
					"k6tests-amw",
					cf.Namespace,
					uniqName,
					strconv.FormatInt(manifestGenerationTimestamp, 10),
				)

				if githubOutputFilePath, ok := os.LookupEnv("GITHUB_OUTPUT"); ok {
					f, err := os.OpenFile(githubOutputFilePath, os.O_APPEND|os.O_WRONLY, 0644)
					if err != nil {
						log.Fatal(err)
					}
					if _, err := f.Write([]byte(fmt.Sprintf("%s-%s=%s\n", c.Environment, *c.TestRun.Name, uniqName))); err != nil {
						log.Fatal(err)
					}
					fmt.Printf("You can interact with k8s resources using the unique id, e.g.\n\tkubectl get pods -l \"k6-test=${{ steps.<step_id>.outputs.%s-%s }}\" -o name\n", c.Environment, *c.TestRun.Name)
					if err := f.Close(); err != nil {
						log.Fatal(err)
					}
				}
			}
		}
	}
	fmt.Printf("\nTo run all tests run:\n\tkubectl --context k6tests-cluster apply --server-side -f .dist/ -R\n")
	fmt.Printf("\nTo abort all tests run:\n\tkubectl --context k6tests-cluster delete -f .dist/ -R\n")
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

func (r K8sManifestGenerator) HandleBreakpointConfigFile(envSlice []*Env) map[string]interface{} {
	env := map[string]string{}
	for _, e := range envSlice {
		if strings.HasPrefix(*e.Name, "BREAKPOINT_") {
			env[*e.Name] = *e.Value
		}
	}
	var breakPointConf BreakpointConfig

	defaultScenario, err := os.ReadFile(fmt.Sprintf("%s/%s.json", r.DefaultScenariosDirectory, "breakpoint"))
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal([]byte(defaultScenario), &breakPointConf)
	if err != nil {
		log.Fatal(err)
	}

	duration, ok := env["BREAKPOINT_STAGE_DURATION"]
	if ok {
		breakPointConf.Stages[0].Duration = duration
	}

	target, ok := env["BREAKPOINT_STAGE_TARGET"]
	if ok {
		i, err := strconv.Atoi(target)
		if err != nil {
			log.Fatal(err)
		}
		breakPointConf.Stages[0].Target = i
	}

	abortOnFail, ok := env["BREAKPOINT_STAGE_ABORTONFAIL"]
	if ok {
		b, err := strconv.ParseBool(abortOnFail)
		if err != nil {
			log.Fatal(err)
		}
		for k := range breakPointConf.Thresholds {
			for idx := range breakPointConf.Thresholds[k] {
				breakPointConf.Thresholds[k][idx].AbortOnFail = b
			}
		}
	}

	// TODO: Hacky, make a better interface as we are likely to do something similar with smoke tests, etc.
	var gInterface map[string]interface{}
	breakPointConfMarshalled, err := json.Marshal(breakPointConf)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(breakPointConfMarshalled, &gInterface)
	if err != nil {
		log.Fatal(err)
	}

	return gInterface
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

func (r K8sManifestGenerator) CallK6Archive(dirName string, testConfigFileToUse string, testFile string, k6ArchiveArgs []string) {
	newpath := filepath.Join(r.BuildDirectory, dirName)
	err := os.MkdirAll(newpath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	// https://grafana.com/docs/k6/latest/using-k6/environment-variables/#-the--e-flag-does-not-configure-options
	// https://grafana.com/docs/k6/latest/reference/archive/#how-to-create-and-run-an-archive
	// https://grafana.com/docs/k6/latest/using-k6/test-lifecycle/#the-init-stage
	var cmd *exec.Cmd
	var out, errb strings.Builder
	k6Args := []string{"archive"}
	k6Args = append(k6Args, k6ArchiveArgs...)

	if testConfigFileToUse != "" {
		k6Args = append(k6Args, "--config", testConfigFileToUse)
	}
	k6Args = append(k6Args, testFile, "-O", fmt.Sprintf("%s/archive.tar", newpath))

	cmd = exec.Command("k6", k6Args...)
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Failed to call %s\nerr: %s", cmd.String(), errb.String())
		os.Exit(1)
	}
	fmt.Printf("Wrote archive.tar into: %s/archive.tar\n", newpath)
}

func (r K8sManifestGenerator) CallKubectl(dirName string, uniqName string, namespace string) {
	cmd := exec.Command("kubectl",
		"create",
		"configmap",
		dirName,
		fmt.Sprintf("--from-file=archive.tar=%s/%s/archive.tar", r.BuildDirectory, dirName),
		"-o", "json",
		"-n", namespace,
		"--dry-run=client",
	)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Failed to call kubectl create configmap %s --from-file=archive.tar=%s/%s/archive.tar -o json -n %s --dry-run=client\n err: %s", dirName, r.BuildDirectory, dirName, namespace, errb.String())
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
		"generated-by":      "k6-action-image",
	})

	tempMarshalled, err := json.MarshalIndent(temp, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	newpath := filepath.Join(r.DistDirectory, dirName)
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

func (r K8sManifestGenerator) CallJsonnet(dirName string, uniqName string, testName string, manifestGenerationTimestamp string, namespace string, environment string, parallelism int, nodeType string, secretReferences []byte, extraEnvVars []byte, resources []byte, isBrowserTest bool, testid string) {
	var errb strings.Builder
	k6ClusterConfigFile, err := os.ReadFile("/actions/generate-k6-manifests/infra/k6_cluster_conf.yaml")
	if err != nil {
		log.Fatal(err)
	}
	newpath := filepath.Join(r.DistDirectory, dirName)
	err = os.MkdirAll(newpath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command("jsonnet",
		"--jpath", "/jsonnet/vendor",
		"--ext-str", fmt.Sprintf("unique_name=%s", uniqName),
		"--ext-str", fmt.Sprintf("testid=%s", testid),
		"--ext-str", fmt.Sprintf("dir_name=%s", dirName),
		"--ext-str", fmt.Sprintf("test_name=%s", testName),
		"--ext-str", fmt.Sprintf("manifest_generation_timestamp=%s", manifestGenerationTimestamp),
		"--ext-str", fmt.Sprintf("namespace=%s", namespace),
		"--ext-str", fmt.Sprintf("deploy_env=%s", environment),
		"--ext-str", fmt.Sprintf("parallelism=%d", parallelism),
		"--ext-str", fmt.Sprintf("node_type=%s", nodeType),
		"--ext-str", fmt.Sprintf("secret_references=%s", secretReferences),
		"--ext-str", fmt.Sprintf("extra_env_vars=%s", extraEnvVars),
		"--ext-str", fmt.Sprintf("resources=%s", resources),
		"--ext-str", fmt.Sprintf("is_browser_test=%t", isBrowserTest),
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
