package utils

import (
	"testing"
)

func TestPromDurationToISO8601(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2w", "P2W"},
		{"1ms", "PT0.001S"},
		{"1000ms", "PT1S"},
		{"1y2w3d4h5m6s", "P1Y2W3DT4H5M6S"},
		{"1y2w3d4h5m6s7ms", "P1Y2W3DT4H5M6.007S"},
	}

	for _, test := range tests {
		output, err := PromDurationToISO8601(test.input)
		if err != nil {
			t.Errorf("Error converting %s: %v", test.input, err)
			continue
		}
		if output != test.expected {
			t.Errorf("Expected %s, got %s for input %s", test.expected, output, test.input)
		}
	}
}

func TestPromDurationToISO8601_InvalidFormat(t *testing.T) {
	tests := []string{
		"bad format",
		"1ms2w",
		"PT1M",
	}

	for _, input := range tests {
		_, err := PromDurationToISO8601(input)
		if err == nil {
			t.Errorf("Expected error for input %s, but got none", input)
		}
	}
}
