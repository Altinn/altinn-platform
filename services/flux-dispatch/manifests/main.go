// Command manifests synthesizes the Kubernetes manifests for flux-dispatch
// into config/. See RFC 0010 (rfcs/0010-flux-reconcile-webhooks.md) §"Kubernetes
// deployment" and §NetworkPolicy for the normative shape of these resources.
package main

import (
	"fmt"

	"github.com/Altinn/altinn-platform/services/flux-dispatch/imports/k8s"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/manifests/internal/k8scompat"
	"github.com/aws/constructs-go/constructs/v10"
	_jsii_ "github.com/aws/jsii-runtime-go"
	cdk8s "github.com/cdk8s-team/cdk8s-core-go/cdk8s/v2"
)

const (
	containerImage = "flux-dispatch:latest"
	appName        = "flux-dispatch"
	namespace      = "dis-platform"

	webhookPort = 8080
	metricsPort = 9090

	fluxSystemNamespace = "flux-system"
	monitoringNamespace = "monitoring"
	kubeSystemNamespace = "kube-system"

	// Secret name matches the brief's Task 4a naming (flux-dispatch-github-app-key).
	githubAppKeySecretName = "flux-dispatch-github-app-key"

	githubAppKeyMountPath = "/etc/flux-dispatch/secrets/github-app"

	githubAppKeyDataKey = "private-key.pem"

	// scalingNoteAnnotation documents why replicas is pinned to 1. cdk8s
	// synthesizes YAML from JSON patches, which has no comment support, so this
	// rides along as a Deployment annotation instead of a YAML comment — visible
	// via `kubectl get/describe`, not just in this source file.
	scalingNoteAnnotation = "dis.altinn.cloud/scaling-note"
	scalingNoteText       = "replicas is pinned to 1: the dedup tracker (RFC 0010 Deduplication) lives in " +
		"this pod's own memory. A second replica has a disjoint dedup map, so the same Flux reconcile " +
		"event (or a notification-controller retry) delivered to both pods dispatches twice — duplicate " +
		"GitHub Actions runs for one deploy, the exact failure mode this service exists to prevent. Do not " +
		"scale horizontally without first moving dedup to shared storage."

	// secretStoreName is the namespaced external-secrets SecretStore that both
	// ExternalSecrets below reference. See the assumption documented on
	// newExternalSecrets.
	secretStoreName = "flux-dispatch-kv-store"
)

func main() {
	app := cdk8s.NewApp(&cdk8s.AppProps{
		Outdir:              _jsii_.String("config"),
		OutputFileExtension: _jsii_.String(".yaml"),
		YamlOutputType:      cdk8s.YamlOutputType_FILE_PER_CHART,
	})

	newFluxDispatchChart(app, "flux-dispatch")
	newKustomizationChart(app, "kustomization")

	app.Synth()
}

func newFluxDispatchChart(scope constructs.Construct, id string) cdk8s.Chart {
	chart := cdk8s.NewChart(scope, _jsii_.String(id), nil)

	labels := stringMap(map[string]string{
		"app":   appName,
		"owner": "platform",
	})
	// podLabels intentionally omits azure.workload.identity/use: flux-dispatch
	// never calls Azure directly — external-secrets performs the token
	// exchange itself via the SecretStore's serviceAccountRef (see
	// newExternalSecrets), which impersonates the ServiceAccount below through
	// the Kubernetes TokenRequest API, not through the workload-identity
	// mutating webhook. That webhook only triggers off this pod label, so
	// setting it here would just inject an unused projected token volume and
	// env vars into this pod.
	podLabels := stringMap(map[string]string{
		"app":   appName,
		"owner": "platform",
	})

	// The ServiceAccount doubles as the workload identity for the ExternalSecrets
	// SecretStore below (see newExternalSecrets) — flux-dispatch itself never
	// calls Azure directly, but external-secrets impersonates this identity to
	// pull the two Key Vault secrets it mounts.
	sa := k8scompat.NewKubeServiceAccount(chart, _jsii_.String("sa"), &k8s.KubeServiceAccountProps{
		Metadata: &k8s.ObjectMeta{
			Name:      _jsii_.String(appName),
			Namespace: _jsii_.String(namespace),
			Labels:    labels,
			Annotations: stringMap(map[string]string{
				"azure.workload.identity/client-id": "${FLUX_DISPATCH_WORKLOAD_IDENTITY_CLIENT_ID}",
			}),
		},
		AutomountServiceAccountToken: _jsii_.Bool(false),
	})

	newDeployment(chart, sa, labels, podLabels)
	newService(chart, labels)
	newNetworkPolicies(chart)
	newPodMonitor(chart, labels)
	newExternalSecrets(chart, labels)

	return chart
}

// newDeployment defines the single-replica flux-dispatch Deployment. Env vars
// mirror internal/config/config.go exactly: the three GitHub App variables
// and DRY_RUN get values here via ${...} postBuild placeholders (see
// README.md "DRY_RUN mode" for why DRY_RUN is always set explicitly rather
// than relying on its code default); optional variables whose defaults
// already match RFC 0010 are left unset.
func newDeployment(chart cdk8s.Chart, sa cdk8s.ApiObject, labels, podLabels *map[string]*string) {
	k8scompat.NewKubeDeployment(chart, _jsii_.String("deployment"), &k8s.KubeDeploymentProps{
		Metadata: &k8s.ObjectMeta{
			Name:      _jsii_.String(appName),
			Namespace: _jsii_.String(namespace),
			Labels:    labels,
			Annotations: stringMap(map[string]string{
				scalingNoteAnnotation: scalingNoteText,
			}),
		},
		Spec: &k8s.DeploymentSpec{
			// See scalingNoteAnnotation above for why this is 1 and must stay 1.
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
					Volumes: &[]*k8s.Volume{
						{
							Name: _jsii_.String("github-app-key"),
							Secret: &k8s.SecretVolumeSource{
								SecretName: _jsii_.String(githubAppKeySecretName),
								// Optional: lets the pod start before the
								// ExternalSecret can materialize this
								// Secret. See README.md "DRY_RUN mode" —
								// paired with the config.Load startup check.
								Optional: _jsii_.Bool(true),
							},
						},
					},
					Containers: &[]*k8s.Container{
						{
							Name:  _jsii_.String(appName),
							Image: _jsii_.String(containerImage),
							Ports: &[]*k8s.ContainerPort{
								{
									Name:          _jsii_.String("webhook"),
									ContainerPort: _jsii_.Number(webhookPort),
								},
								{
									Name:          _jsii_.String("metrics"),
									ContainerPort: _jsii_.Number(metricsPort),
								},
							},
							Env: &[]*k8s.EnvVar{
								{
									Name:  _jsii_.String("LISTEN_ADDR"),
									Value: _jsii_.String(fmt.Sprintf(":%d", webhookPort)),
								},
								{
									Name:  _jsii_.String("METRICS_ADDR"),
									Value: _jsii_.String(fmt.Sprintf(":%d", metricsPort)),
								},
								{
									Name:  _jsii_.String("DRY_RUN"),
									Value: _jsii_.String("${DRY_RUN}"),
								},
								{
									Name:  _jsii_.String("GITHUB_APP_ID"),
									Value: _jsii_.String("${GITHUB_APP_ID}"),
								},
								{
									Name:  _jsii_.String("GITHUB_INSTALLATION_ID"),
									Value: _jsii_.String("${GITHUB_INSTALLATION_ID}"),
								},
								{
									Name:  _jsii_.String("GITHUB_PRIVATE_KEY_PATH"),
									Value: _jsii_.String(githubAppKeyMountPath + "/" + githubAppKeyDataKey),
								},
							},
							VolumeMounts: &[]*k8s.VolumeMount{
								{
									Name:      _jsii_.String("github-app-key"),
									MountPath: _jsii_.String(githubAppKeyMountPath),
									ReadOnly:  _jsii_.Bool(true),
								},
							},
							LivenessProbe: &k8s.Probe{
								HttpGet: &k8s.HttpGetAction{
									Path: _jsii_.String("/healthz"),
									Port: k8s.IntOrString_FromString(_jsii_.String("webhook")),
								},
							},
							ReadinessProbe: &k8s.Probe{
								HttpGet: &k8s.HttpGetAction{
									Path: _jsii_.String("/readyz"),
									Port: k8s.IntOrString_FromString(_jsii_.String("webhook")),
								},
							},
							Resources: &k8s.ResourceRequirements{
								Requests: &map[string]k8s.Quantity{
									"cpu":    k8s.Quantity_FromString(_jsii_.String("50m")),
									"memory": k8s.Quantity_FromString(_jsii_.String("64Mi")),
								},
							},
							SecurityContext: &k8s.SecurityContext{
								AllowPrivilegeEscalation: _jsii_.Bool(false),
								ReadOnlyRootFilesystem:   _jsii_.Bool(true),
								Capabilities: &k8s.Capabilities{
									Drop: &[]*string{_jsii_.String("ALL")},
								},
							},
						},
					},
				},
			},
		},
	})
}

// newService defines the ClusterIP Service fronting the webhook port. The
// metrics port is intentionally not exposed here — the PodMonitor below
// scrapes pods directly, matching lakmus's pattern.
func newService(chart cdk8s.Chart, labels *map[string]*string) {
	svc := cdk8s.NewApiObject(chart, _jsii_.String("service"), &cdk8s.ApiObjectProps{
		ApiVersion: _jsii_.String("v1"),
		Kind:       _jsii_.String("Service"),
		Metadata: &cdk8s.ApiObjectMetadata{
			Name:      _jsii_.String(appName),
			Namespace: _jsii_.String(namespace),
			Labels:    labels,
		},
	})
	svc.AddJsonPatch(cdk8s.JsonPatch_Add(_jsii_.String("/spec"), map[string]any{
		"type":     "ClusterIP",
		"selector": rawLabels(),
		"ports": []any{
			map[string]any{
				"name":       "webhook",
				"port":       webhookPort,
				"targetPort": "webhook",
				"protocol":   "TCP",
			},
		},
	}))
}

// newNetworkPolicies defines the three NetworkPolicies copied verbatim from
// RFC 0010 §NetworkPolicy (field-for-field, including the podSelector using
// only `app: flux-dispatch`, not the broader two-key `labels` map used
// elsewhere in this file — Kubernetes label selectors are a subset match, so
// this still selects the Deployment's pods).
//
// The brief also asked to check the other dis-* operators (dis-apim-operator,
// dis-identity-operator, dis-pgsql-operator) for the local egress convention
// before writing the egress rule. None of their config/network-policy
// manifests define an egress policy at all — they are unmodified kubebuilder
// scaffolding (generic `webhook: enabled` / `metrics: enabled` namespace-label
// placeholders, ingress-only). There is no established egress convention to
// reconcile with, so the RFC's verbatim block is authoritative as-is.
func newNetworkPolicies(chart cdk8s.Chart) {
	newIngressNetworkPolicy(chart, "allow-webhook-traffic", "flux-dispatch-allow-webhook-traffic", fluxSystemNamespace, webhookPort)
	newIngressNetworkPolicy(chart, "allow-metrics-traffic", "flux-dispatch-allow-metrics-traffic", monitoringNamespace, metricsPort)
	newEgressNetworkPolicy(chart)
}

func newIngressNetworkPolicy(chart cdk8s.Chart, id, name, fromNamespace string, port int) {
	np := cdk8s.NewApiObject(chart, _jsii_.String(id), &cdk8s.ApiObjectProps{
		ApiVersion: _jsii_.String("networking.k8s.io/v1"),
		Kind:       _jsii_.String("NetworkPolicy"),
		Metadata: &cdk8s.ApiObjectMetadata{
			Name:      _jsii_.String(name),
			Namespace: _jsii_.String(namespace),
		},
	})
	np.AddJsonPatch(cdk8s.JsonPatch_Add(_jsii_.String("/spec"), map[string]any{
		"podSelector": map[string]any{
			"matchLabels": map[string]any{"app": appName},
		},
		"policyTypes": []any{"Ingress"},
		"ingress": []any{
			map[string]any{
				"from": []any{
					map[string]any{
						"namespaceSelector": map[string]any{
							"matchLabels": map[string]any{"kubernetes.io/metadata.name": fromNamespace},
						},
					},
				},
				"ports": []any{
					map[string]any{"protocol": "TCP", "port": port},
				},
			},
		},
	}))
}

func newEgressNetworkPolicy(chart cdk8s.Chart) {
	np := cdk8s.NewApiObject(chart, _jsii_.String("allow-egress"), &cdk8s.ApiObjectProps{
		ApiVersion: _jsii_.String("networking.k8s.io/v1"),
		Kind:       _jsii_.String("NetworkPolicy"),
		Metadata: &cdk8s.ApiObjectMetadata{
			Name:      _jsii_.String("flux-dispatch-allow-egress"),
			Namespace: _jsii_.String(namespace),
		},
	})
	np.AddJsonPatch(cdk8s.JsonPatch_Add(_jsii_.String("/spec"), map[string]any{
		"podSelector": map[string]any{
			"matchLabels": map[string]any{"app": appName},
		},
		"policyTypes": []any{"Egress"},
		"egress": []any{
			map[string]any{
				"to": []any{
					map[string]any{
						"namespaceSelector": map[string]any{
							"matchLabels": map[string]any{"kubernetes.io/metadata.name": kubeSystemNamespace},
						},
					},
				},
				"ports": []any{
					map[string]any{"protocol": "UDP", "port": 53},
					map[string]any{"protocol": "TCP", "port": 53},
				},
			},
			map[string]any{
				"ports": []any{
					map[string]any{"protocol": "TCP", "port": 443},
				},
			},
		},
	}))
}

// newPodMonitor scrapes the metrics port. lakmus uses PodMonitor (not
// ServiceMonitor) on the azmonitoring.coreos.com/v1 group — Azure Managed
// Prometheus's own CRD group — so this mirrors that exactly.
func newPodMonitor(chart cdk8s.Chart, labels *map[string]*string) {
	pm := cdk8s.NewApiObject(chart, _jsii_.String("flux-dispatch-podmonitor"), &cdk8s.ApiObjectProps{
		ApiVersion: _jsii_.String("azmonitoring.coreos.com/v1"),
		Kind:       _jsii_.String("PodMonitor"),
		Metadata: &cdk8s.ApiObjectMetadata{
			Name:      _jsii_.String(appName),
			Namespace: _jsii_.String(namespace),
			Labels:    labels,
		},
	})
	pm.AddJsonPatch(cdk8s.JsonPatch_Add(_jsii_.String("/spec"), map[string]any{
		"selector": map[string]any{
			"matchLabels": rawLabels(),
		},
		"namespaceSelector": map[string]any{
			"any": true,
		},
		"podMetricsEndpoints": []any{
			map[string]any{
				"port":     "metrics",
				"path":     "/metrics",
				"interval": "30s",
			},
		},
	}))
}

// newExternalSecrets defines the namespaced SecretStore and the
// ExternalSecret that materializes the GitHub App private key as a
// Kubernetes Secret. See README.md "Secret management" for why this shape
// (rather than dis-vault-operator's Vault CRD) and for vaultUrl/the secret
// name, which are placeholders pending Key Vault provisioning.
func newExternalSecrets(chart cdk8s.Chart, labels *map[string]*string) {
	store := cdk8s.NewApiObject(chart, _jsii_.String("kv-store"), &cdk8s.ApiObjectProps{
		ApiVersion: _jsii_.String("external-secrets.io/v1"),
		Kind:       _jsii_.String("SecretStore"),
		Metadata: &cdk8s.ApiObjectMetadata{
			Name:      _jsii_.String(secretStoreName),
			Namespace: _jsii_.String(namespace),
			Labels:    labels,
		},
	})
	store.AddJsonPatch(cdk8s.JsonPatch_Add(_jsii_.String("/spec"), map[string]any{
		"provider": map[string]any{
			"azurekv": map[string]any{
				"authType": "WorkloadIdentity",
				"vaultUrl": "${KV_URI}",
				"serviceAccountRef": map[string]any{
					"name":      appName,
					"namespace": namespace,
				},
			},
		},
	}))

	newExternalSecret(chart, githubAppKeySecretName, labels,
		githubAppKeyDataKey, "${KV_SECRET_NAME_GITHUB_APP_KEY}")
}

// newExternalSecret derives both the cdk8s construct id and the object's
// metadata.name from targetSecretName using the single "<secret>-external-secret"
// convention, rather than taking an independent id argument that could drift
// from the name it labels.
func newExternalSecret(chart cdk8s.Chart, targetSecretName string, labels *map[string]*string, secretKey, remoteKey string) {
	id := targetSecretName + "-external-secret"
	es := cdk8s.NewApiObject(chart, _jsii_.String(id), &cdk8s.ApiObjectProps{
		ApiVersion: _jsii_.String("external-secrets.io/v1"),
		Kind:       _jsii_.String("ExternalSecret"),
		Metadata: &cdk8s.ApiObjectMetadata{
			Name:      _jsii_.String(id),
			Namespace: _jsii_.String(namespace),
			Labels:    labels,
		},
	})
	es.AddJsonPatch(cdk8s.JsonPatch_Add(_jsii_.String("/spec"), map[string]any{
		"refreshInterval": "1h",
		"secretStoreRef": map[string]any{
			"kind": "SecretStore",
			"name": secretStoreName,
		},
		"target": map[string]any{
			"name":           targetSecretName,
			"creationPolicy": "Owner",
		},
		"data": []any{
			map[string]any{
				"secretKey": secretKey,
				"remoteRef": map[string]any{
					"key": remoteKey,
				},
			},
		},
	}))
}

func newKustomizationChart(scope constructs.Construct, id string) cdk8s.Chart {
	chart := cdk8s.NewChart(scope, _jsii_.String(id), nil)

	kustomization := cdk8s.NewApiObject(chart, _jsii_.String("kustomization"), &cdk8s.ApiObjectProps{
		ApiVersion: _jsii_.String("kustomize.config.k8s.io/v1beta1"),
		Kind:       _jsii_.String("Kustomization"),
		Metadata: &cdk8s.ApiObjectMetadata{
			Name: _jsii_.String(appName),
		},
	})
	kustomization.AddJsonPatch(cdk8s.JsonPatch_Add(_jsii_.String("/resources"), []any{"flux-dispatch.yaml"}))

	return chart
}

func rawLabels() map[string]any {
	return map[string]any{
		"app":   appName,
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
