package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

type JsonnetParameters struct {
	dirName                     string
	configMapName               string
	uniqName                    string
	testName                    string
	manifestGenerationTimestamp string
	namespace                   string
	environment                 string
	parallelism                 int
	nodeType                    string
	secretReferences            []byte
	extraEnvVars                []byte
	resources                   []byte
	imageName                   string
	testid                      string
	testScope                   string
}

type Generator interface {
	Initialize(filePath string) *ConfigFile
	Generate(cf ConfigFile)
	HandleConfigFile(defaultConfigFile string, testType string) map[string]any
	HandleConfigFileOverride(base map[string]any, overrideConfigFile string) map[string]any
	CallK6Archive(dirName string, testConfigFileToUse string, testFile string, k6ArchiveArgs []string)
	CallKubectl(dirName string, configMapName string, uniqName string, testId, testName, testScope, namespace string)
	CallJsonnet(jsonnetParameters JsonnetParameters)
}

type K8sManifestGenerator struct {
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

	requiredDirs := []string{
		r.ConfigDirectory,
		r.BuildDirectory,
		r.DistDirectory,
	}

	for _, d := range requiredDirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			err := os.Mkdir(d, 0775)
			if err != nil {
				log.Fatalf("unable to create the %s directory, ensure it exists and try again: %v", d, err)
			}
		}
	}

	d, err := yaml.Marshal(&cf)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	err = os.WriteFile(fmt.Sprintf("%s/expanded-configfile.yaml", r.ConfigDirectory), d, 0644)
	if err != nil {
		fmt.Printf("Failed to create expanded config file\n err: %s", err)
		os.Exit(1)
	}

	return &cf
}

func generate(td *TestDefinition, c *TestContext, r K8sManifestGenerator, cf ConfigFile, envFileSlice []*Env, envOptions []*Env) {
	manifestGenerationTimestamp := time.Now().UnixMilli()

	mergedEnvs := handleExtraEnvVars(c.TestRun.Env, envFileSlice)
	mergedEnvs = handleExtraEnvVars(mergedEnvs, getGithubRelatedVars())
	mergedEnvs = handleExtraEnvVars(mergedEnvs, envOptions)

	k6ArchiveArgs := []string{
		"--env", fmt.Sprintf("%s=%s", "K6_NO_USAGE_REPORT", "true"),
		"--env", fmt.Sprintf("%s=%s", "K6_PROMETHEUS_RW_SERVER_URL", "http://kube-prometheus-stack-prometheus.monitoring:9090/api/v1/write"),
		"--env", fmt.Sprintf("%s=%s", "K6_PROMETHEUS_RW_TREND_STATS", "avg,min,med,max,count,p(95),p(99),p(99.5),p(99.9)"),

		"--env", fmt.Sprintf("%s=%s", "ENVIRONMENT", c.Environment),
		"--env", fmt.Sprintf("%s=%s", "NAMESPACE", cf.Namespace),
		"--env", fmt.Sprintf("%s=%d", "MANIFEST_GENERATION_TIMESTAMP", manifestGenerationTimestamp),
		"--env", fmt.Sprintf("%s=%s", "TESTID", *c.TestRun.Id),
		"--env", fmt.Sprintf("%s=%s", "TEST_NAME", *c.TestRun.Name),
	}
	for _, env := range mergedEnvs {
		k6ArchiveArgs = append(k6ArchiveArgs, "--env", fmt.Sprintf("%s=%s", *env.Name, *env.Value))
	}

	var configFile map[string]any
	if *c.TestTypeDefinition.Type != "custom" {
		if *c.TestTypeDefinition.Type == "breakpoint" {
			configFile = r.HandleBreakpointConfigFile(mergedEnvs)
		} else {
			configFile = r.HandleConfigFile(td.ConfigFile, *c.TestTypeDefinition.Type)
			if c.TestTypeDefinition.ConfigFile != "" {
				configFile = r.HandleConfigFileOverride(configFile, c.TestTypeDefinition.ConfigFile)
			}
		}
	}

	uniqName := fmt.Sprintf("%s-%d-%s", c.Environment, manifestGenerationTimestamp, randomString(5))
	testScope := td.TestScope
	dirName := fmt.Sprintf("%s", *c.TestRun.Id)
	newpath := filepath.Join(r.ConfigDirectory, dirName)
	testConfigFileToUse := fmt.Sprintf("%s/tweaked-testconfig.json", newpath)

	marshalledConfigFile, err := json.MarshalIndent(configFile, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	if err = os.MkdirAll(newpath, os.ModePerm); err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(testConfigFileToUse, marshalledConfigFile, 0644)
	if err != nil {
		log.Fatal(err)
	}

	r.CallK6Archive(dirName, testConfigFileToUse, fmt.Sprintf("%s/%s", r.RepoRootDirectory, td.TestFile), k6ArchiveArgs)

	configMapName := fmt.Sprintf("%s-%s", td.TestScope, *c.TestRun.Id)

	r.CallKubectl(dirName, configMapName, uniqName, *c.TestRun.Id, *c.TestRun.Name, testScope, cf.Namespace)

	// Jsonnet related things
	slackChannel := chooseCorrectSlackChannel(c.Environment)
	secretReferences, err := yaml.Marshal(append(c.TestRun.SecretReferences, &slackChannel))
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	resources, err := yaml.Marshal(c.TestRun.Resources)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	imageName := "ghcr.io/altinn/altinn-platform/k6-image:latest"
	if *c.TestTypeDefinition.Type == "browser" {
		imageName = "grafana/k6:master-with-browser"
	}

	mergedEnvsMarshalled, err := yaml.Marshal(mergedEnvs)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	JsonnetParameters := JsonnetParameters{
		dirName:                     dirName,
		configMapName:               configMapName,
		uniqName:                    uniqName,
		testName:                    *c.TestRun.Name,
		manifestGenerationTimestamp: strconv.FormatInt(manifestGenerationTimestamp, 10),
		namespace:                   cf.Namespace,
		environment:                 c.Environment,
		parallelism:                 *c.TestRun.Parallelism,
		nodeType:                    *c.NodeType,
		secretReferences:            secretReferences,
		extraEnvVars:                mergedEnvsMarshalled,
		resources:                   resources,
		imageName:                   imageName,
		testid:                      *c.TestRun.Id,
		testScope:                   testScope,
	}
	r.CallJsonnet(JsonnetParameters)
}

func (r K8sManifestGenerator) Generate(cf ConfigFile) {
	// Command line args, used mainly with ad-hoc tests
	var envOptions []*Env
	if cliArgs, ok := os.LookupEnv("INPUT_COMMAND_LINE_ARGS"); ok {
		envOptions = getEnvVarsFromCliArgs(cliArgs)
	}

	wg := &sync.WaitGroup{}
	for _, td := range cf.TestDefinitions {
		var envFileSlice []*Env
		if td.EnvFile != "" {
			envFileSlice = parseEnvFile(fmt.Sprintf("%s/%s", r.RepoRootDirectory, td.EnvFile))
		}

		for _, c := range td.Contexts {
			if c.TestTypeDefinition.Enabled {
				wg.Go(func() { generate(td, c, r, cf, envFileSlice, envOptions) })
			}
		}

	}
	wg.Wait()
}

func (r K8sManifestGenerator) HandleConfigFileOverride(base map[string]any, overrideConfigFile string) map[string]any {
	override := map[string]any{}

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

func (r K8sManifestGenerator) HandleBreakpointConfigFile(envSlice []*Env) map[string]any {
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
	var gInterface map[string]any
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

func (r K8sManifestGenerator) HandleConfigFile(defaultConfigFile string, testType string) map[string]any {
	mergedOut := map[string]any{}

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
	_, okit := mergedOut["iterations"]
	_, okvus := mergedOut["vus"]
	_, okduration := mergedOut["duration"]
	// Don't override if users configured it.
	if !(okst || oksc || okit || okvus || okduration) {
		defaultScenario, err := os.ReadFile(fmt.Sprintf("%s/%s.json", r.DefaultScenariosDirectory, testType))
		if err != nil {
			log.Fatal(err)
		}
		temp := map[string]any{}
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
}

func (r K8sManifestGenerator) CallKubectl(dirName string, configMapName string, uniqName string, testId, testName, testScope, namespace string) {
	cmd := exec.Command("kubectl",
		"create",
		"configmap",
		configMapName,
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
		"generated-by": "k6-action-image",
		"uniq_name":    uniqName,
	})
	temp.SetAnnotations(map[string]string{
		"k6-action-image/test_name":  testName,
		"k6-action-image/test_scope": testScope,
		"k6-action-image/testid":     testId,
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

func (r K8sManifestGenerator) CallJsonnet(jp JsonnetParameters) {
	var errb strings.Builder
	k6ClusterConfigFile, err := os.ReadFile("/actions/generate-k6-manifests/infra/k6_cluster_conf.yaml")
	if err != nil {
		log.Fatal(err)
	}
	newpath := filepath.Join(r.DistDirectory, jp.dirName)
	err = os.MkdirAll(newpath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command("jsonnet",
		"--jpath", "/jsonnet/vendor",
		"--ext-str", fmt.Sprintf("unique_name=%s", jp.uniqName),
		"--ext-str", fmt.Sprintf("testid=%s", jp.testid),
		"--ext-str", fmt.Sprintf("test_scope=%s", jp.testScope),
		"--ext-str", fmt.Sprintf("configmap_name=%s", jp.configMapName),
		"--ext-str", fmt.Sprintf("test_name=%s", jp.testName),
		"--ext-str", fmt.Sprintf("manifest_generation_timestamp=%s", jp.manifestGenerationTimestamp),
		"--ext-str", fmt.Sprintf("namespace=%s", jp.namespace),
		"--ext-str", fmt.Sprintf("deploy_env=%s", jp.environment),
		"--ext-str", fmt.Sprintf("parallelism=%d", jp.parallelism),
		"--ext-str", fmt.Sprintf("node_type=%s", jp.nodeType),
		"--ext-str", fmt.Sprintf("extra_env_vars=%s", jp.extraEnvVars),
		"--ext-str", fmt.Sprintf("secret_references=%s", jp.secretReferences),
		"--ext-str", fmt.Sprintf("resources=%s", jp.resources),
		"--ext-str", fmt.Sprintf("image_name=%s", jp.imageName),
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
