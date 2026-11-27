package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	const endpoint, queue = "sb-team-a.servicebus.windows.net", "team-a.orders"
	tests := []struct {
		name    string
		env     map[string]string // replaces values of the valid environment
		want    Config
		wantErr string // the variable that the error must name first
	}{
		{name: "defaults", want: Config{endpoint, queue, 30 * time.Second, 20 * time.Second, ":8080"}},
		{
			name: "all set",
			env:  map[string]string{"CANARY_INTERVAL": "1m", "CANARY_TIMEOUT": "45s", "CANARY_METRICS_ADDR": ":9090"},
			want: Config{endpoint, queue, time.Minute, 45 * time.Second, ":9090"},
		},
		{name: "no endpoint", env: map[string]string{"ENDPOINT": ""}, wantErr: "ENDPOINT"},
		{name: "endpoint with scheme", env: map[string]string{"ENDPOINT": "sb://" + endpoint}, wantErr: "ENDPOINT"},
		{name: "endpoint with port", env: map[string]string{"ENDPOINT": endpoint + ":5671"}, wantErr: "ENDPOINT"},
		{name: "endpoint with path", env: map[string]string{"ENDPOINT": endpoint + "/"}, wantErr: "ENDPOINT"},
		{name: "endpoint with a line break", env: map[string]string{"ENDPOINT": endpoint + "\n"}, wantErr: "ENDPOINT"},
		{name: "no queue", env: map[string]string{"QUEUE": ""}, wantErr: "QUEUE"},
		{name: "queue with a space", env: map[string]string{"QUEUE": "team-a orders"}, wantErr: "QUEUE"},
		{name: "interval is not a duration", env: map[string]string{"CANARY_INTERVAL": "30"}, wantErr: "CANARY_INTERVAL"},
		{name: "interval is zero", env: map[string]string{"CANARY_INTERVAL": "0s"}, wantErr: "CANARY_INTERVAL"},
		{name: "interval below one second", env: map[string]string{"CANARY_INTERVAL": "30ms"}, wantErr: "CANARY_INTERVAL"},
		{name: "timeout is negative", env: map[string]string{"CANARY_TIMEOUT": "-1s"}, wantErr: "CANARY_TIMEOUT"},
		{name: "timeout equals the interval", env: map[string]string{"CANARY_TIMEOUT": "30s"}, wantErr: "CANARY_TIMEOUT"},
		{name: "interval below the default timeout", env: map[string]string{"CANARY_INTERVAL": "10s"}, wantErr: "CANARY_TIMEOUT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(func(name string) string {
				if value, ok := tt.env[name]; ok {
					return value
				}
				return map[string]string{"ENDPOINT": endpoint, "QUEUE": queue}[name]
			})
			if tt.wantErr != "" {
				if err == nil || !strings.HasPrefix(err.Error(), tt.wantErr+" ") {
					t.Fatalf("error: got %v, want an error that starts with %s", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}
