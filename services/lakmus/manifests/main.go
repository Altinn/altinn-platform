package main

import (
	"github.com/Altinn/altinn-platform/services/lakmus/imports/k8s"
	"github.com/Altinn/altinn-platform/services/lakmus/manifests/internal/k8scompat"
	"github.com/aws/constructs-go/constructs/v10"
	_jsii_ "github.com/aws/jsii-runtime-go"
	cdk8s "github.com/cdk8s-team/cdk8s-core-go/cdk8s/v2"
)

const (
	containerImage = "controller:latest"
	lakmusName     = "lakmus"
	namespace      = "monitoring"
)

func main() {
	app := cdk8s.NewApp(&cdk8s.AppProps{
		Outdir:              _jsii_.String("config"),
		OutputFileExtension: _jsii_.String(".yaml"),
		YamlOutputType:      cdk8s.YamlOutputType_FILE_PER_CHART,
	})

	newLakmusChart(app, "lakmus")
	newKustomizationChart(app, "kustomization")

	app.Synth()
}

func newLakmusChart(scope constructs.Construct, id string) cdk8s.Chart {
	chart := cdk8s.NewChart(scope, _jsii_.String(id), nil)

	labels := stringMap(map[string]string{
		"app":   lakmusName,
		"owner": "platform",
	})
	podLabels := stringMap(map[string]string{
		"app":                         lakmusName,
		"owner":                       "platform",
		"azure.workload.identity/use": "true",
	})

	baseObjectMeta := &k8s.ObjectMeta{
		Name:      _jsii_.String(lakmusName),
		Namespace: _jsii_.String(namespace),
		Labels:    labels,
	}

	sa := k8scompat.NewKubeServiceAccount(chart, _jsii_.String("sa"), &k8s.KubeServiceAccountProps{
		Metadata: &k8s.ObjectMeta{
			Name:      _jsii_.String(lakmusName),
			Namespace: _jsii_.String(namespace),
			Labels:    labels,
			Annotations: stringMap(map[string]string{
				"azure.workload.identity/client-id": "${LAKMUS_WORKLOAD_IDENTITY_CLIENT_ID}",
			}),
		},
		AutomountServiceAccountToken: _jsii_.Bool(false),
	})

	k8scompat.NewKubeDeployment(chart, _jsii_.String("deployment"), &k8s.KubeDeploymentProps{
		Metadata: baseObjectMeta,
		Spec: &k8s.DeploymentSpec{
			Replicas: _jsii_.Number(1),
			Selector: &k8s.LabelSelector{
				MatchLabels: labels,
			},
			Template: &k8s.PodTemplateSpec{
				Metadata: &k8s.ObjectMeta{
					Labels: podLabels,
					Annotations: stringMap(map[string]string{
						"cluster-autoscaler.kubernetes.io/safe-to-evict": "true",
					}),
				},
				Spec: &k8s.PodSpec{
					ServiceAccountName:           sa.Name(),
					AutomountServiceAccountToken: _jsii_.Bool(false),
					EnableServiceLinks:           _jsii_.Bool(false),
					SecurityContext: &k8s.PodSecurityContext{
						RunAsNonRoot: _jsii_.Bool(true),
						SeccompProfile: &k8s.SeccompProfile{
							Type: _jsii_.String("RuntimeDefault"),
						},
					},
					Containers: &[]*k8s.Container{
						{
							Name:  _jsii_.String(lakmusName),
							Image: _jsii_.String(containerImage),
							Args: &[]*string{
								_jsii_.String("--subscription-id=$(AZURE_SUBSCRIPTION_ID)"),
							},
							Env: &[]*k8s.EnvVar{
								{
									Name:  _jsii_.String("AZURE_SUBSCRIPTION_ID"),
									Value: _jsii_.String("${AZURE_SUBSCRIPTION_ID}"),
								},
							},
							SecurityContext: &k8s.SecurityContext{
								AllowPrivilegeEscalation: _jsii_.Bool(false),
								ReadOnlyRootFilesystem:   _jsii_.Bool(true),
								Capabilities: &k8s.Capabilities{
									Drop: &[]*string{_jsii_.String("ALL")},
								},
							},
							Ports: &[]*k8s.ContainerPort{
								{
									Name:          _jsii_.String("http"),
									ContainerPort: _jsii_.Number(8080),
								},
							},
						},
					},
				},
			},
		},
	})

	podMonitor := cdk8s.NewApiObject(chart, _jsii_.String("lakmus-podmonitor"), &cdk8s.ApiObjectProps{
		ApiVersion: _jsii_.String("azmonitoring.coreos.com/v1"),
		Kind:       _jsii_.String("PodMonitor"),
		Metadata: &cdk8s.ApiObjectMetadata{
			Name:      _jsii_.String(lakmusName),
			Namespace: _jsii_.String(namespace),
			Labels:    labels,
		},
	})
	podMonitor.AddJsonPatch(cdk8s.JsonPatch_Add(_jsii_.String("/spec"), map[string]any{
		"selector": map[string]any{
			"matchLabels": rawLabels(),
		},
		"namespaceSelector": map[string]any{
			"any": true,
		},
		"podMetricsEndpoints": []any{
			map[string]any{
				"port":     "http",
				"path":     "/metrics",
				"interval": "30s",
			},
		},
	}))

	return chart
}

func newKustomizationChart(scope constructs.Construct, id string) cdk8s.Chart {
	chart := cdk8s.NewChart(scope, _jsii_.String(id), nil)

	kustomization := cdk8s.NewApiObject(chart, _jsii_.String("kustomization"), &cdk8s.ApiObjectProps{
		ApiVersion: _jsii_.String("kustomize.config.k8s.io/v1beta1"),
		Kind:       _jsii_.String("Kustomization"),
		Metadata: &cdk8s.ApiObjectMetadata{
			Name: _jsii_.String(lakmusName),
		},
	})
	kustomization.AddJsonPatch(cdk8s.JsonPatch_Add(_jsii_.String("/resources"), []any{"lakmus.yaml"}))

	return chart
}

func rawLabels() map[string]any {
	return map[string]any{
		"app":   lakmusName,
		"owner": "platform",
	}
}

func stringMap(values map[string]string) *map[string]*string {
	out := make(map[string]*string, len(values))
	for key, value := range values {
		out[key] = _jsii_.String(value)
	}

	return &out
}
