package altinn

import (
	"github.com/altinn/altinn-platform/daisctl/pkg/kube"
	"github.com/google/go-cmp/cmp"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInitAppsData(t *testing.T) {
	tests := []struct {
		name               string
		statusCode         int
		initialDeployments *Deployments
		expectedResult     map[string]*kube.AppVersions
		expectError        bool
	}{
		{
			name:               "Valid Response from kubewrapper",
			initialDeployments: newDeployments(),
			statusCode:         http.StatusOK,
			expectError:        false,
			expectedResult: map[string]*kube.AppVersions{
				"altinn-access-management": {
					AppName: "altinn-access-management",
					Versions: map[string]string{
						"dev": "111",
					},
				},
				"altinn-authentication": {
					AppName: "altinn-authentication",
					Versions: map[string]string{
						"dev": "222",
					},
				},
				"altinn-authorization": {
					AppName: "altinn-authorization",
					Versions: map[string]string{
						"dev": "333",
					},
				},
			},
		},
		{
			name: "Merges response with existing",
			initialDeployments: &Deployments{
				Apps: map[string]*kube.AppVersions{
					"altinn-access-management": {
						AppName: "altinn-access-management",
						Versions: map[string]string{
							"dev":  "111",
							"prod": "111",
						},
					},
				},
			},
			statusCode:  http.StatusOK,
			expectError: false,
			expectedResult: map[string]*kube.AppVersions{
				"altinn-access-management": {
					AppName: "altinn-access-management",
					Versions: map[string]string{
						"dev":  "112",
						"prod": "111",
					},
				},
				"altinn-authentication": {
					AppName: "altinn-authentication",
					Versions: map[string]string{
						"dev": "222",
					},
				},
				"altinn-authorization": {
					AppName: "altinn-authorization",
					Versions: map[string]string{
						"dev": "333",
					},
				},
			},
		},
		{
			name:               "No deployments returned from kubewrapper",
			initialDeployments: newDeployments(),
			statusCode:         http.StatusOK,
			expectError:        false,
			expectedResult:     make(map[string]*kube.AppVersions),
		},
		{
			name:               "Invalid Response from kubewrapper result in error",
			initialDeployments: newDeployments(),
			statusCode:         http.StatusOK,
			expectError:        true,
			expectedResult:     make(map[string]*kube.AppVersions),
		},
		{
			name:               "Server Error from kubewrapper result in error",
			initialDeployments: newDeployments(),
			statusCode:         http.StatusInternalServerError,
			expectError:        true,
			expectedResult:     make(map[string]*kube.AppVersions),
		},
	}
	const testGroupName = "TestInitAppsData"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server that returns the mock response
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				// check return value, otherwise lint will complain
				mockResponse, err := readMockResponseFromFile(testGroupName, tt.name)
				if err != nil {
					t.Errorf("failed to read mock response file: %v", err)
				}
				if _, err := w.Write(mockResponse); err != nil {
					t.Errorf("failed to write mock response: %v", err)
				}
			}))
			defer server.Close()

			// Call GetEnvironments with the test server URL
			err := tt.initialDeployments.initAppsData(server.URL, Environment{
				Name: "dev",
			})

			if (err != nil) != tt.expectError {
				t.Fatalf("Expected error: %v, got: %v", tt.expectError, err)
			}
			if diff := cmp.Diff(tt.initialDeployments.GetAllApps(), tt.expectedResult); diff != "" {
				t.Errorf("initAppsData() mismatch (-want +got):\n%s\nInput located here: %s", diff, getCompleteFilepath(testGroupName, tt.name))
			}
		})
	}
}

func TestGetAppVersions(t *testing.T) {
	deployments := &Deployments{
		Apps: map[string]*kube.AppVersions{
			"altinn-access-management": {
				AppName: "altinn-access-management",
				Versions: map[string]string{
					"dev": "111",
				},
			},
			"altinn-authentication": {
				AppName: "altinn-authentication",
				Versions: map[string]string{
					"dev": "222",
				},
			},
		},
	}
	tests := []struct {
		name     string
		appName  string
		expected map[string]*kube.AppVersions
	}{
		{
			name:    "Get app versions for altinn-access-management",
			appName: "altinn-access-management",
			expected: map[string]*kube.AppVersions{
				"altinn-access-management": {
					AppName: "altinn-access-management",
					Versions: map[string]string{
						"dev": "111",
					},
				},
			},
		},
		{
			name:     "Get non-existing app versions returns nil",
			appName:  "non-existing-app",
			expected: nil,
		},
	}
	for _, tt := range tests {
		if diff := cmp.Diff(deployments.GetAppVersions(tt.appName), tt.expected); diff != "" {
			t.Errorf("GetAppVersions() mismatch (-want +got):\n%s", diff)
		}
	}
}
