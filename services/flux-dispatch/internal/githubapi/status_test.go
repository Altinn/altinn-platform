package githubapi

import (
	"net/http"
	"testing"
)

func TestRetryable(t *testing.T) {
	tests := []struct {
		name   string
		status int
		header http.Header
		body   string
		want   bool
	}{
		{name: "server error", status: http.StatusInternalServerError, want: true},
		{name: "bad gateway", status: http.StatusBadGateway, want: true},
		{name: "service unavailable", status: http.StatusServiceUnavailable, want: true},
		{name: "too many requests", status: http.StatusTooManyRequests, want: true},

		{
			name:   "forbidden with retry-after",
			status: http.StatusForbidden,
			header: http.Header{"Retry-After": []string{"60"}},
			want:   true,
		},
		{
			name:   "forbidden with exhausted budget",
			status: http.StatusForbidden,
			header: http.Header{"X-Ratelimit-Remaining": []string{"0"}},
			want:   true,
		},
		{
			name:   "forbidden with secondary rate limit message",
			status: http.StatusForbidden,
			body:   `{"message":"You have exceeded a secondary rate limit."}`,
			want:   true,
		},
		{
			name:   "forbidden with abuse message",
			status: http.StatusForbidden,
			body:   `{"message":"You have triggered an abuse detection mechanism."}`,
			want:   true,
		},
		{
			// A real refusal: the App is not permitted on this repo.
			name:   "forbidden without rate-limit signal",
			status: http.StatusForbidden,
			body:   `{"message":"Resource not accessible by integration"}`,
			want:   false,
		},
		{
			name:   "budget remaining",
			status: http.StatusForbidden,
			header: http.Header{"X-Ratelimit-Remaining": []string{"4999"}},
			want:   false,
		},

		{name: "unauthorized", status: http.StatusUnauthorized, want: false},
		{name: "not found", status: http.StatusNotFound, want: false},
		{name: "unprocessable", status: http.StatusUnprocessableEntity, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := tt.header
			if header == nil {
				header = http.Header{}
			}
			if got := Retryable(tt.status, header, []byte(tt.body)); got != tt.want {
				t.Errorf("Retryable(%d, %v, %q) = %v, want %v", tt.status, header, tt.body, got, tt.want)
			}
		})
	}
}
