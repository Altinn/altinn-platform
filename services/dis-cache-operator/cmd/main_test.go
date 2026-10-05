package main

import (
	"slices"
	"testing"
)

func TestParseNetworks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value string
		want  []string
		fails bool
	}{
		{
			"two networks", "10.240.0.0/16,fd10:59f0:8c79:240::/64",
			[]string{"10.240.0.0/16", "fd10:59f0:8c79:240::/64"}, false,
		},
		{"spaces and an empty entry", " 10.240.0.0/16 , ,10.241.0.0/16", []string{"10.240.0.0/16", "10.241.0.0/16"}, false},
		{"empty", "", nil, true},
		{"not a cidr", "10.240.0.0", nil, true},
		{"bad prefix", "10.240.0.0/33", nil, true},
	}
	for _, tc := range cases {
		got, err := parseNetworks(tc.value)
		if tc.fails {
			if err == nil {
				t.Errorf("%s: want an error, got %v", tc.name, got)
			}
			continue
		}
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("%s: want %v, got %v (%v)", tc.name, tc.want, got, err)
		}
	}
}
