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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func initGenerator(confDir, distDir, buildDir string) K8sManifestGenerator {
	return K8sManifestGenerator{
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
			_, deployEnv := getInfoFromFilePath(path)
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

func validateTestRun(path, dirName, testVersion, deployEnv string, t *testing.T) {
	uniqName, err := extractUniqueIdFromGeneratedTestRun(path)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	manifestGenerationTimestamp := strings.Split(uniqName, "-")[1]
	// Ugly hack, but let's go with it until I refactor further.
	var testName string
	testName = strings.TrimPrefix(dirName, fmt.Sprintf("%s-", deployEnv))
	testName = strings.TrimSuffix(testName, "-breakpoint")
	testName = strings.TrimSuffix(testName, "-smoke")
	testName = strings.TrimSuffix(testName, "-spike")

	generatedFile, knownExpectedFile, equalContents := readFileAndCompareWithTemplatedFile(
		path,
		fmt.Sprintf("./expected_generated_files/%s/%s/testrun.json.tmpl", testVersion, deployEnv),
		map[string]string{
			"DirName":                     dirName,
			"DeployEnv":                   deployEnv,
			"ManifestGenerationTimestamp": manifestGenerationTimestamp,
			"TestName":                    testName,
			"TestId":                      dirName,
			"UniqName":                    uniqName,
		},
	)
	if !equalContents {
		t.Errorf("generate %s: expected \n%s, actual \n%s", testVersion, knownExpectedFile, generatedFile)
	}
}

func extractUniqueIdFromGeneratedTestRun(filepath string) (string, error) {
	generatedFile, err := os.ReadFile(filepath)
	if err != nil {
		log.Fatal(err)
	}

	var generatedTestRun unstructured.Unstructured
	err = json.Unmarshal([]byte(generatedFile), &generatedTestRun)
	if err != nil {
		return "", err
	}
	return generatedTestRun.GetName(), nil
}

func validateConfigMap(path, testVersion, dirName string, t *testing.T) {
	generatedFile, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}

	var generatedConfigMap corev1.ConfigMap
	err = json.Unmarshal([]byte(generatedFile), &generatedConfigMap)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if !strings.Contains(generatedConfigMap.Name, dirName) {
		t.Errorf("generate %s: expected \n%s to contain \n%s", testVersion, generatedConfigMap.Name, dirName)
	}
	if !strings.Contains(generatedConfigMap.Annotations["k6-action-image/testid"], dirName) {
		t.Errorf("generate %s: expected \n%s, to contain \n%s", testVersion, generatedConfigMap.Annotations["k6-action-image/testid"], dirName)
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

var generateExamplesVersion = []string{"v1", "v2", "v3", "v4", "v5", "v6", "v7", "v8", "v9", "v10", "v11", "v12", "v13"}

func TestGenerate(t *testing.T) {
	for _, version := range generateExamplesVersion {
		preTest(version)
		fmt.Printf("Testing Generation for %s\n", version)
		testVersion := version
		distDir, confDir, buildDir := generateTempDirectories()
		var g Generator = initGenerator(confDir, distDir, buildDir)
		cf := g.Initialize(fmt.Sprintf("./example_configfiles/%s.yaml", testVersion))
		g.Generate(*cf)

		validateConfigFolder(confDir, testVersion, t)

		_ = filepath.Walk(distDir, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				fmt.Printf("Validating %s\n", path)
				dirName, deployEnv := getInfoFromFilePath(path)
				if info.Name() == "testrun.json" {
					validateTestRun(path, dirName, testVersion, deployEnv, t)
				} else if info.Name() == "configmap.json" {
					validateConfigMap(path, testVersion, dirName, t)
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

func getInfoFromFilePath(path string) (dirName, deployEnv string) {
	tempSplit := strings.Split(path, "/")
	dirName = tempSplit[3]
	deployEnv = strings.Split(dirName, "-")[0]
	return
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
