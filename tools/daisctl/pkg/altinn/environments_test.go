package altinn

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetEnvironments(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   string
		statusCode     int
		expectedResult []Environment
		expectError    bool
	}{
		{
			name: "Valid Response",
			mockResponse: `{
				"environments": [
					{
						"platformUrl": "https://a.com",
						"hostname": "a.com",
						"appPrefix": "apps",
						"platformPrefix": "platform",
						"name": "dev",
						"type": "test"
					},
					{
						"platformUrl": "https://b.com",
						"hostname": "b.com",
						"appPrefix": "apps",
						"platformPrefix": "platform",
						"name": "prod",
						"type": "production"
					}
				]
			}`,
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
			mockResponse:   `{"environments": [ { "platformUrl": "https://a.com", "name": "dev" `,
			statusCode:     http.StatusOK,
			expectedResult: nil,
			expectError:    true,
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
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			// Call GetEnvironments with the test server URL
			envs, err := GetEnvironments(server.URL)

			if (err != nil) != tt.expectError {
				t.Fatalf("Expected error: %v, got: %v", tt.expectError, err)
			}

			if diff := cmp.Diff(envs, tt.expectedResult); diff != "" {
				t.Errorf("GetEnvironments() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
