package messaging

import (
	"strings"
	"testing"
)

func TestPrefix(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		namespace   string
		annotations map[string]string
		want        string
		wantErr     bool
	}{
		"product namespace":                  {namespace: "product-team-a", want: "team-a."},
		"annotation overrides the default":   {namespace: "product-team-a", annotations: map[string]string{PrefixAnnotation: "shared."}, want: "shared."},
		"annotation on another namespace":    {namespace: "platform-a", annotations: map[string]string{PrefixAnnotation: "platform/a_"}, want: "platform/a_"},
		"other annotations are ignored":      {namespace: "product-team-a", annotations: map[string]string{"team": "b."}, want: "team-a."},
		"namespace without a prefix":         {namespace: "platform-a", wantErr: true},
		"product namespace without a name":   {namespace: "product-", wantErr: true},
		"empty annotation":                   {namespace: "product-team-a", annotations: map[string]string{PrefixAnnotation: ""}, wantErr: true},
		"annotation without separator":       {namespace: "product-team-a", annotations: map[string]string{PrefixAnnotation: "team-a"}, wantErr: true},
		"annotation starts with a separator": {namespace: "product-team-a", annotations: map[string]string{PrefixAnnotation: ".team-a."}, wantErr: true},
		"annotation with a space":            {namespace: "product-team-a", annotations: map[string]string{PrefixAnnotation: "team a."}, wantErr: true},
		"annotation leaves no room":          {namespace: "product-team-a", annotations: map[string]string{PrefixAnnotation: strings.Repeat("a", 259) + "."}, wantErr: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := Prefix(tt.namespace, tt.annotations)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got prefix %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got prefix %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	t.Parallel()

	shared := map[string]string{PrefixAnnotation: "shared."}

	tests := map[string]struct {
		namespace   string
		annotations map[string]string
		name        string
		wantErr     bool
	}{
		"name with the product prefix":      {namespace: "product-team-a", name: "team-a.app-one"},
		"name with the annotation prefix":   {namespace: "product-team-a", annotations: shared, name: "shared.app-one/orders"},
		"name without a prefix":             {namespace: "product-team-a", name: "app-one", wantErr: true},
		"name of another product":           {namespace: "product-team-a", name: "team-b.app-one", wantErr: true},
		"product name without the dot":      {namespace: "product-team-a", name: "team-aapp-one", wantErr: true},
		"name is only the prefix":           {namespace: "product-team-a", name: "team-a.", wantErr: true},
		"prefix in another case":            {namespace: "product-team-a", name: "TEAM-A.app-one", wantErr: true},
		"default prefix with an annotation": {namespace: "product-team-a", annotations: shared, name: "team-a.app-one", wantErr: true},
		"namespace without a prefix":        {namespace: "platform-a", name: "platform-a.app-one", wantErr: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := ValidateName(tt.namespace, tt.annotations, tt.name)
			if tt.wantErr && err == nil {
				t.Fatalf("expected an error for queue name %q", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
