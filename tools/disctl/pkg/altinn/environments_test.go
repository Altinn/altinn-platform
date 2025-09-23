package altinn

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetEnvironments(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		expectedResult []Environment
		expectError    bool
	}{
		{
			name:       "Valid Response",
			statusCode: http.StatusOK,
			expectedResult: []Environment{
				{
					PlatformUrl:    "https://a.com",
					Hostname:       "a.com",
					AppPrefix:      "apps",
					PlatformPrefix: "platform",
					Name:           "dev",
					Type:           "test",
				},
				{
					PlatformUrl:    "https://b.com",
					Hostname:       "b.com",
					AppPrefix:      "apps",
					PlatformPrefix: "platform",
					Name:           "prod",
					Type:           "production",
				},
			},
			expectError: false,
		},
		{
			name:           "Malformed JSON Response",
			statusCode:     http.StatusOK,
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "Server Error",
			statusCode:     http.StatusInternalServerError,
			expectedResult: nil,
			expectError:    true,
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
				mockResponse, err := readMockResponseFromFile("TestGetEnvironments", tt.name)
				if err != nil {
					t.Errorf("failed to read mock response file: %v", err)
				}
				if _, err := w.Write(mockResponse); err != nil {
					t.Errorf("failed to write mock response: %v", err)
				}
			}))
			defer server.Close()
			// Call GetEnvironments with the test server URL
			envs, err := GetEnvironments(server.URL)

			if (err != nil) != tt.expectError {
				t.Fatalf("Expected error: %v, got: %v", tt.expectError, err)
			}

			if diff := cmp.Diff(envs, tt.expectedResult); diff != "" {
				t.Errorf("GetEnvironments() mismatch (-want +got):\n%s\nInput located here: %s", diff, getCompleteFilepath(testGroupName, tt.name))
			}
		})
	}
}

func readMockResponseFromFile(testGroupName, testName string) ([]byte, error) {
	mockResponseFilePath := getCompleteFilepath(testGroupName, testName)
	mockBytes, err := os.ReadFile(mockResponseFilePath)
	if err != nil {
		return nil, err
	}
	return mockBytes, nil
}

func getCompleteFilepath(testGroupName, testName string) string {
	mockResponseFilename := fmt.Sprintf("%s.json", strings.ReplaceAll(strings.ToLower(testName), " ", "_"))
	mockResponseFilePath := filepath.Join("testdata", strings.ReplaceAll(strings.ToLower(testGroupName), " ", "_"), mockResponseFilename)
	return mockResponseFilePath
}
