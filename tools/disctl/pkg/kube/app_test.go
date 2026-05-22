package kube

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetAppInfos(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   string
		statusCode     int
		expectedResult []AppInfo
		expectError    bool
	}{
		{
			name: "Valid Response",
			mockResponse: `[
				{
					"version": "1.0",
					"release": "app1"
				},
				{
					"version": "2.0",
					"release": "app2"
				}
			]`,
			statusCode: http.StatusOK,
			expectedResult: []AppInfo{
				{Version: "1.0", Release: "app1"},
				{Version: "2.0", Release: "app2"},
			},
			expectError: false,
		},
		{
			name:           "Malformed JSON Response",
			mockResponse:   `[{"version": "1.0", "release": "app1"`,
			statusCode:     http.StatusOK,
			expectedResult: nil,
			expectError:    true,
		},
		{
			name:           "Empty Response",
			mockResponse:   `[]`,
			statusCode:     http.StatusOK,
			expectedResult: []AppInfo{},
			expectError:    false,
		},
		{
			name:           "Server Error",
			mockResponse:   `Internal Server Error`,
			statusCode:     http.StatusInternalServerError,
			expectedResult: nil,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server that returns the mock response
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				// check return value, otherwise lint will complain
				if _, err := w.Write([]byte(tt.mockResponse)); err != nil {
					t.Errorf("failed to write mock response: %v", err)
				}
			}))
			defer server.Close()

			// Call GetAppInfos with the test server URL
			apps, err := GetAppInfos(server.URL)

			if (err != nil) != tt.expectError {
				t.Fatalf("Expected error: %v, got: %v", tt.expectError, err)
			}

			if diff := cmp.Diff(apps, tt.expectedResult); diff != "" {
				t.Errorf("GetAppInfos() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
