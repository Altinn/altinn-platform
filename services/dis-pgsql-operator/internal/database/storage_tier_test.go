package database

import "testing"

func TestResolveStorageTier(t *testing.T) {
	ptr := func(value string) *string {
		return &value
	}

	cases := []struct {
		name      string
		sizeGB    int32
		requested *string
		want      string
	}{
		{
			name:      "defaults when unset",
			sizeGB:    32,
			requested: nil,
			want:      "P10",
		},
		{
			name:      "invalid tier falls back to default",
			sizeGB:    32,
			requested: ptr("P999"),
			want:      "P10",
		},
		{
			name:      "baseline floor applies",
			sizeGB:    256,
			requested: ptr("P4"),
			want:      "P15",
		},
		{
			name:      "size based max applies",
			sizeGB:    32,
			requested: ptr("P80"),
			want:      "P50",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveStorageTier(tc.sizeGB, tc.requested); got != tc.want {
				t.Fatalf("ResolveStorageTier(%d, %v) = %q, want %q", tc.sizeGB, tc.requested, got, tc.want)
			}
		})
	}
}
