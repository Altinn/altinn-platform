package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"text/template"

	corev1 "k8s.io/api/core/v1"
)

func initGenerator(userConfigFile, confDir, distDir, buildDir string) *K8sManifestGenerator {
	return &K8sManifestGenerator{
		UserConfigFile:            userConfigFile,
		ConfigDirectory:           confDir,
		DistDirectory:             distDir,
		BuildDirectory:            buildDir,
		DefaultScenariosDirectory: "/actions/generate-k6-manifests/default_scenarios",
		RepoRootDirectory:         "../../..",
	}
}

func validateConfigFolder(confDir string, testVersion string, t *testing.T) {
	_ = filepath.Walk(confDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			fmt.Printf("Validating %s\n", path)
			_, _, _, deployEnv := getInfoFromFilePath(path)
			if info.Name() == "expanded-configfile.yaml" {
				generatedFile, knownExpectedFile, equalContents := readFilesAndCompareContents(path, fmt.Sprintf("./expected_generated_files/%s/expanded-configfile.yaml", testVersion))
				if !equalContents {
					t.Errorf("generate %s: expected \n%s, actual \n%s", testVersion, knownExpectedFile, generatedFile)
				}
			} else if info.Name() == "tweaked-testconfig.json" {
				generatedFile, knownExpectedFile, equalContents := readFilesAndCompareContents(path, fmt.Sprintf("./expected_generated_files/%s/%s/tweaked-testconfig.json", testVersion, deployEnv))
				if !equalContents {
					t.Errorf("generate %s: expected \n%s, actual \n%s", testVersion, knownExpectedFile, generatedFile)
				}
			}
		}
		return nil
	})
}

func validateTestRun(path, testVersion, deployEnv, dirName string, t *testing.T) {
	manifestGenerationTimestamp := strings.Split(dirName, "-")[len(strings.Split(dirName, "-"))-1]
	generatedFile, knownExpectedFile, equalContents := readFileAndCompareWithTemplatedFile(
		path,
		fmt.Sprintf("./expected_generated_files/%s/%s/testrun.json.tmpl", testVersion, deployEnv),
		map[string]string{
			"UniqueName":                  dirName,
			"DeployEnv":                   deployEnv,
			"ManifestGenerationTimestamp": manifestGenerationTimestamp,
		},
	)
	if !equalContents {
		t.Errorf("generate %s: expected \n%s, actual \n%s", testVersion, knownExpectedFile, generatedFile)
	}
}

func validateConfigMap(path, testVersion, deployEnv, dirName string, t *testing.T) {
	generatedFile, knownExpectedFile, _ := readFileAndCompareWithTemplatedFile(
		path,
		fmt.Sprintf("./expected_generated_files/%s/%s/configmap.json.tmpl", testVersion, deployEnv),
		map[string]string{
			"UniqueName": dirName,
		},
	)
	var knownExpectedConfigMap corev1.ConfigMap
	var generatedConfigMap corev1.ConfigMap

	err := json.Unmarshal([]byte(knownExpectedFile), &knownExpectedConfigMap)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	err = json.Unmarshal([]byte(generatedFile), &generatedConfigMap)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if knownExpectedConfigMap.Name != generatedConfigMap.Name {
		t.Errorf("generate %s: expected \n%s, actual \n%s", testVersion, knownExpectedConfigMap.Name, generatedConfigMap.Name)
	}
	if knownExpectedConfigMap.Labels["testid"] != generatedConfigMap.Labels["testid"] {
		t.Errorf("generate %s: expected \n%s, actual \n%s", testVersion, knownExpectedConfigMap.Labels["testid"], generatedConfigMap.Labels["testid"])
	}
	if knownExpectedConfigMap.Labels["k6-test-configmap"] != generatedConfigMap.Labels["k6-test-configmap"] {
		t.Errorf("generate %s: expected \n%s, actual \n%s", testVersion, knownExpectedConfigMap.Labels["k6-test-configmap"], generatedConfigMap.Labels["k6-test-configmap"])
	}
	if knownExpectedConfigMap.Labels["k6-test"] != generatedConfigMap.Labels["k6-test"] {
		t.Errorf("generate %s: expected \n%s, actual \n%s", testVersion, knownExpectedConfigMap.Labels["k6-test"], generatedConfigMap.Labels["k6-test"])
	}
	if !(len(generatedConfigMap.Data["archive.tar"]) > 0) {
		t.Errorf("generate %s: expected length of data in key archive.tar to be over 0 , actual %d", testVersion, len(generatedConfigMap.Data["archive.tar"]))
	}
}

// TODO: Hacky just to test
func preTest(version string) {
	if version == "v7" {
		os.Setenv("INPUT_COMMAND_LINE_ARGS", "-e runFullTestSet=true -e tokenGeneratorUserName=olanordmenn -e orgNoRecipient=1234 -e resourceId=abcd")
	}
	if version == "v10" {
		os.Setenv("GITHUB_REPOSITORY", "octocat/Hello-World")
		os.Setenv("GITHUB_SERVER_URL", "https://github.com")
		os.Setenv("GITHUB_RUN_ID", "14965885066")
		os.Setenv("GITHUB_REF", "refs/heads/main")
	}
}

func postTest(version string) {
	if version == "v7" {
		os.Unsetenv("INPUT_COMMAND_LINE_ARGS")
	}
	if version == "v10" {
		os.Unsetenv("GITHUB_REPOSITORY")
		os.Unsetenv("GITHUB_SERVER_URL")
		os.Unsetenv("GITHUB_RUN_ID")
		os.Unsetenv("GITHUB_REF")
	}
}

var generateExamplesVersion = []string{"v1", "v2", "v3", "v4", "v5", "v6", "v7", "v8", "v9", "v10"}

func TestGenerate(t *testing.T) {
	for _, version := range generateExamplesVersion {
		preTest(version)
		fmt.Printf("Testing Generation for %s\n", version)
		testVersion := version
		distDir, confDir, buildDir := generateTempDirectories()
		var g Generator = initGenerator(fmt.Sprintf("./example_configfiles/%s.yaml", testVersion), confDir, distDir, buildDir)
		g.Generate()

		validateConfigFolder(confDir, testVersion, t)

		_ = filepath.Walk(distDir, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				fmt.Printf("Validating %s\n", path)
				dirName, _, _, deployEnv := getInfoFromFilePath(path)
				if info.Name() == "testrun.json" {
					validateTestRun(path, testVersion, deployEnv, dirName, t)
				} else if info.Name() == "configmap.json" {
					validateConfigMap(path, testVersion, deployEnv, dirName, t)
				} else if info.Name() == "slo.json" {
					// TODO
				}
			}
			return nil
		})
		postTest(version)
	}
}

func generateTempDirectories() (distDir string, confDir string, buildDir string) {
	distDir, err := os.MkdirTemp(os.TempDir(), ".dist")
	if err != nil {
		log.Fatal(err)
	}

	confDir, err = os.MkdirTemp(os.TempDir(), ".conf")
	if err != nil {
		log.Fatal(err)
	}

	buildDir, err = os.MkdirTemp(os.TempDir(), ".build")
	if err != nil {
		log.Fatal(err)
	}
	return
}

func readFilesAndCompareContents(filePath1 string, filePath2 string) (string, string, bool) {
	file1, err := os.ReadFile(filePath1)

	if err != nil {
		log.Fatal(err)
	}

	file2, err := os.ReadFile(filePath2)

	if err != nil {
		log.Fatal(err)
	}
	return string(file1), string(file2), string(file1) == string(file2)
}

func readFileAndCompareWithTemplatedFile(filePath1 string, filePath2 string, templateSubstitutions map[string]string) (string, string, bool) {
	file1, err := os.ReadFile(filePath1)

	if err != nil {
		log.Fatal(err)
	}

	tpl, err := template.ParseFiles(filePath2)
	if err != nil {
		log.Fatal(err)
	}
	buffer := bytes.Buffer{}
	err = tpl.Execute(&buffer, templateSubstitutions)
	if err != nil {
		log.Fatal(err)
	}
	file2 := buffer.String()

	return string(file1), file2, string(file1) == file2
}

func getTestScriptFileNameFromSplitString(split []string) (string, error) {
	if len(split) < 4 {
		return "", fmt.Errorf("expected split string to have at least 4 elements but got: %d", len(split))
	}
	tempString := split[1 : len(split)-2]
	return strings.Join(tempString, "-"), nil
}

var splitStrings = []struct {
	splitString []string // input
	expected    string   // expected result
}{
	{
		[]string{"at22", "k8s", "wrapper", "deployments", "0", "1741180011935"},
		"k8s-wrapper-deployments",
	},
}

func TestGetTestScriptFileNameFromSplitString(t *testing.T) {
	for _, tt := range splitStrings {
		actual, _ := getTestScriptFileNameFromSplitString(tt.splitString)
		if actual != tt.expected {
			t.Errorf("getTestScriptFileNameFromSplitString(%s): expected %s, actual %s", tt.splitString, tt.expected, actual)
		}
	}
}

func getInfoFromFilePath(path string) (dirName, fileName, testScriptName, deployEnv string) {
	var err error
	tempSplit := strings.Split(path, "/")

	if len(tempSplit) == 5 {
		dirName = tempSplit[3]
		fileName = tempSplit[4]
		tempString := strings.Split(dirName, "-")
		deployEnv = tempString[0]
		testScriptName, err = getTestScriptFileNameFromSplitString(tempString)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		fileName = tempSplit[3]
	}
	return
}

var splitPaths = []struct {
	path                   string // input
	expectedDirName        string
	expectedFileName       string
	expectedTestScriptName string
	expectedDeployEnv      string
}{
	{
		path:                   "/tmp/.conf1604560518/at22-k8s-wrapper-deployments-0-1742212333161/tweaked-testconfig.json",
		expectedDirName:        "at22-k8s-wrapper-deployments-0-1742212333161",
		expectedFileName:       "tweaked-testconfig.json",
		expectedTestScriptName: "k8s-wrapper-deployments", // NB: This does not necessarily represent the actual filename
		expectedDeployEnv:      "at22",
	},
	{
		path:                   "/tmp/.conf3755495154/expanded-configfile.yaml",
		expectedDirName:        "",
		expectedFileName:       "expanded-configfile.yaml",
		expectedTestScriptName: "",
		expectedDeployEnv:      "",
	},
}

func TestGetInfoFromFilePath(t *testing.T) {
	for _, tt := range splitPaths {
		actualDirName, actualFileName, actualTestScriptName, actualDeployEnv := getInfoFromFilePath(tt.path)
		if actualDirName != tt.expectedDirName {
			t.Errorf("getInfoFromFilePath(%s): expected %s, actual %s", tt.path, tt.expectedDirName, actualDirName)
		}
		if actualFileName != tt.expectedFileName {
			t.Errorf("getInfoFromFilePath(%s): expected %s, actual %s", tt.path, tt.expectedFileName, actualFileName)
		}
		if actualTestScriptName != tt.expectedTestScriptName {
			t.Errorf("getInfoFromFilePath(%s): expected %s, actual %s", tt.path, tt.expectedTestScriptName, actualTestScriptName)
		}
		if actualDeployEnv != tt.expectedDeployEnv {
			t.Errorf("getInfoFromFilePath(%s): expected %s, actual %s", tt.path, tt.expectedDeployEnv, actualDeployEnv)
		}
	}
}

var envVarOverrides = []struct {
	originalEnv map[string]string
	extraEnv    map[string]string
	mergedEnv   map[string]string
}{
	{
		originalEnv: map[string]string{},
		extraEnv:    map[string]string{},
		mergedEnv:   map[string]string{},
	},
	{
		originalEnv: map[string]string{"ENVFROMFILE1": "ENV1"},
		extraEnv:    map[string]string{},
		mergedEnv:   map[string]string{"ENVFROMFILE1": "ENV1"},
	},
	{
		originalEnv: map[string]string{},
		extraEnv:    map[string]string{"ENVFROMFILE1": "ENV1"},
		mergedEnv:   map[string]string{"ENVFROMFILE1": "ENV1"},
	},
	{
		originalEnv: map[string]string{"ENVFROMFILE1": "ENV1"},
		extraEnv:    map[string]string{"ENVFROMFILE2": "ENV2"},
		mergedEnv:   map[string]string{"ENVFROMFILE1": "ENV1", "ENVFROMFILE2": "ENV2"},
	},
	{
		originalEnv: map[string]string{"ENVFROMFILE1": "ENV1"},
		extraEnv:    map[string]string{"ENVFROMFILE1": "ENV2", "FOO": "BAR"},
		mergedEnv:   map[string]string{"ENVFROMFILE1": "ENV2", "FOO": "BAR"},
	},
}

func TestEnvOverride(t *testing.T) {
	for _, tt := range envVarOverrides {
		var original []*Env
		var extra []*Env
		for k, v := range tt.originalEnv {
			original = append(original, &Env{
				Name:  &k,
				Value: &v,
			})
		}
		for k, v := range tt.extraEnv {
			extra = append(extra, &Env{
				Name:  &k,
				Value: &v,
			})
		}
		result := handleExtraEnvVars(original, extra)
		resultMap := make(map[string]string)
		for _, v := range result {
			resultMap[*v.Name] = *v.Value
		}
		eq := reflect.DeepEqual(tt.mergedEnv, resultMap)
		if !eq {
			t.Errorf("TestEnvOverride: expected %v, actual %v", tt.mergedEnv, resultMap)
		}
	}
}
