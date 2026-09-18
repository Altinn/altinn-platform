package main

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestRenderYAML(t *testing.T) {
	tests := []struct {
		name string
		obj  any
		// want maps a dotted path (numeric segments index lists) to the
		// value expected at it in the rendered document.
		want map[string]any
		// absent lists dotted paths that must not appear.
		absent []string
		// wantErr, when set, is a substring of the expected error.
		wantErr string
	}{
		{
			name: "top-level status is removed",
			obj: &corev1.Service{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{Name: "svc"},
				Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}},
				}},
			},
			want:   map[string]any{"metadata.name": "svc"},
			absent: []string{"status"},
		},
		{
			name: "empty Deployment strategy is removed",
			obj: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
			},
			absent: []string{"spec.strategy", "status"},
		},
		{
			name: "non-empty Deployment strategy is kept",
			obj: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
				Spec: appsv1.DeploymentSpec{
					Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
				},
			},
			want: map[string]any{"spec.strategy": map[string]any{"type": "Recreate"}},
		},
		{
			name: "empty strategy on a non-Deployment kind is kept",
			obj: map[string]any{
				"apiVersion": "example.com/v1",
				"kind":       "Example",
				"spec":       map[string]any{"strategy": map[string]any{}},
			},
			want: map[string]any{"spec.strategy": map[string]any{}},
		},
		{
			name: "NetworkPolicy podSelector {} survives",
			obj: &networkingv1.NetworkPolicy{
				TypeMeta: metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy"},
			},
			want: map[string]any{"spec.podSelector": map[string]any{}},
		},
		{
			name: "pod emptyDir {} volume survives",
			obj: &corev1.Pod{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name:         "scratch",
						VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
					}},
				},
			},
			want:   map[string]any{"spec.volumes.0.emptyDir": map[string]any{}},
			absent: []string{"status"},
		},
		{
			name:    "missing apiVersion and kind is an error",
			obj:     &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}},
			wantErr: "set its TypeMeta",
		},
		{
			name:    "missing kind is an error",
			obj:     &corev1.ConfigMap{TypeMeta: metav1.TypeMeta{APIVersion: "v1"}},
			wantErr: "set its TypeMeta",
		},
		{
			name:    "missing apiVersion is an error",
			obj:     &corev1.ConfigMap{TypeMeta: metav1.TypeMeta{Kind: "ConfigMap"}},
			wantErr: "set its TypeMeta",
		},
		{
			name:    "nil object is an error",
			obj:     nil,
			wantErr: "set its TypeMeta",
		},
		{
			name:    "nil typed pointer is an error",
			obj:     (*appsv1.Deployment)(nil),
			wantErr: "set its TypeMeta",
		},
		{
			// 2^53 + 1: the smallest integer a float64 cannot represent.
			name: "large integer round-trips exactly",
			obj: map[string]any{
				"apiVersion": "example.com/v1",
				"kind":       "Example",
				"spec":       map[string]any{"value": int64(9007199254740993)},
			},
			want: map[string]any{"spec.value": json.Number("9007199254740993")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := renderYAML(tt.obj)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("renderYAML() error = %v, want an error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("renderYAML() error = %v", err)
			}

			var doc map[string]any
			if err := yaml.Unmarshal(out, &doc, useNumber); err != nil {
				t.Fatalf("rendered YAML does not parse: %v\n%s", err, out)
			}
			for path, want := range tt.want {
				got, ok := lookup(doc, path)
				switch {
				case !ok:
					t.Errorf("%s is missing from\n%s", path, out)
				case !reflect.DeepEqual(got, want):
					t.Errorf("%s = %#v, want %#v", path, got, want)
				}
			}
			for _, path := range tt.absent {
				if got, ok := lookup(doc, path); ok {
					t.Errorf("%s = %#v, want it removed from\n%s", path, got, out)
				}
			}
		})
	}
}

// useNumber decodes numbers as json.Number, so the test sees them exactly.
func useNumber(d *json.Decoder) *json.Decoder {
	d.UseNumber()
	return d
}

// lookup walks a dotted path through a decoded document; numeric segments
// index lists.
func lookup(doc map[string]any, path string) (any, bool) {
	var cur any = doc
	for key := range strings.SplitSeq(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			next, ok := node[key]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(node) {
				return nil, false
			}
			cur = node[i]
		default:
			return nil, false
		}
	}

	return cur, true
}
