// Command manifests synthesizes the Kubernetes manifests for flux-dispatch
// into config/. See RFC 0010 (rfcs/0010-flux-reconcile-webhooks.md) §"Kubernetes
// deployment" and §NetworkPolicy for the normative shape of these resources.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

const (
	containerImage = "flux-dispatch:latest"
	appName        = "flux-dispatch"
	namespace      = "dis-platform"

	webhookPort = 8080
	metricsPort = 9090

	// The probes, the Service targetPort and the PodMonitor endpoint refer to
	// the container ports by these names.
	webhookPortName = "webhook"
	metricsPortName = "metrics"

	fluxSystemNamespace = "flux-system"
	monitoringNamespace = "monitoring"
	kubeSystemNamespace = "kube-system"

	// githubAppKeySecretName is the Secret that the ExternalSecret below
	// materializes and the Deployment mounts, holding the GitHub App private key.
	githubAppKeySecretName = "flux-dispatch-github-app-key"

	// githubAppKeyVolumeName links the Deployment's Secret volume to its
	// volumeMount.
	githubAppKeyVolumeName = "github-app-key"

	githubAppKeyMountPath = "/etc/flux-dispatch/secrets/github-app"

	githubAppKeyDataKey = "private-key.pem"

	// scalingNoteAnnotation documents why replicas is pinned to 1. The YAML in
	// config/ is generated from this file and carries no comments, so this
	// rides along as a Deployment annotation instead of a YAML comment — visible
	// via `kubectl get/describe`, not just in this source file.
	scalingNoteAnnotation = "dis.altinn.cloud/scaling-note"
	scalingNoteText       = "replicas is pinned to 1: the dedup tracker (RFC 0010 Deduplication) lives in " +
		"this pod's own memory. A second replica has a disjoint dedup map, so the same Flux reconcile " +
		"event (or a notification-controller retry) delivered to both pods dispatches twice — duplicate " +
		"GitHub Actions runs for one deploy, the exact failure mode this service exists to prevent. Do not " +
		"scale horizontally without first moving dedup to shared storage."

	// secretStoreName is the namespaced external-secrets SecretStore that the
	// ExternalSecret below references. See the assumption documented on
	// newExternalSecrets.
	secretStoreName = "flux-dispatch-kv-store"

	// configDir is relative to the working directory: `go run ./manifests` is
	// run from the service root.
	configDir         = "config"
	resourcesFile     = "flux-dispatch.yaml"
	kustomizationFile = "kustomization.yaml"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "manifests: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	if err := writeManifests(filepath.Join(configDir, resourcesFile), newFluxDispatchObjects()); err != nil {
		return err
	}

	return writeManifests(filepath.Join(configDir, kustomizationFile), []any{newKustomization()})
}

// writeManifests renders objects, in order, as the documents of one
// multi-document YAML file at path.
func writeManifests(path string, objects []any) error {
	var out bytes.Buffer
	for i, obj := range objects {
		doc, err := renderYAML(obj)
		if err != nil {
			return fmt.Errorf("render document %d of %s: %w", i, path, err)
		}
		if i > 0 {
			out.WriteString("---\n")
		}
		out.Write(doc)
	}

	return os.WriteFile(path, out.Bytes(), 0o644)
}

// renderYAML renders obj as one YAML document.
//
// The typed k8s.io/api structs hold some fields as plain structs tagged
// omitempty, which encoding/json renders even when zero: as-is, the
// Deployment would carry `status: {}` and `spec.strategy: {}`, and the
// Service `status: {loadBalancer: {}}`. status is owned by the API server and
// never belongs in a manifest, and an empty strategy means the same as none
// (the API server defaults both to RollingUpdate), so exactly those two paths
// are removed. Nothing else is pruned: an empty map such as `podSelector: {}`
// or `emptyDir: {}` means something to Kubernetes and must render as written.
func renderYAML(obj any) ([]byte, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	// UseNumber keeps integers exact instead of rounding them through float64.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}

	// A nil object decodes to an empty doc, so this rejects it too.
	apiVersion, _ := doc["apiVersion"].(string)
	kind, _ := doc["kind"].(string)
	if apiVersion == "" || kind == "" {
		return nil, fmt.Errorf("object of type %T has no apiVersion or kind: "+
			"set its TypeMeta, which typed k8s.io/api structs leave empty", obj)
	}

	delete(doc, "status")
	if spec, ok := doc["spec"].(map[string]any); ok && kind == "Deployment" {
		if strategy, ok := spec["strategy"].(map[string]any); ok && len(strategy) == 0 {
			delete(spec, "strategy")
		}
	}

	return yaml.Marshal(doc)
}

// newFluxDispatchObjects returns the documents of config/flux-dispatch.yaml,
// in the order they are written.
func newFluxDispatchObjects() []any {
	// podLabels intentionally omits azure.workload.identity/use: flux-dispatch
	// never calls Azure directly — external-secrets performs the token
	// exchange itself via the SecretStore's serviceAccountRef (see
	// newExternalSecrets), which impersonates the ServiceAccount below through
	// the Kubernetes TokenRequest API, not through the workload-identity
	// mutating webhook. That webhook only triggers off this pod label, so
	// setting it here would just inject an unused projected token volume and
	// env vars into this pod.
	podLabels := appLabels()

	// The ServiceAccount doubles as the workload identity for the external-secrets
	// SecretStore below (see newExternalSecrets) — flux-dispatch itself never
	// calls Azure directly, but external-secrets impersonates this identity to
	// pull the one Key Vault secret the pod mounts: the GitHub App private key.
	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
			Labels:    appLabels(),
			Annotations: map[string]string{
				"azure.workload.identity/client-id": "${FLUX_DISPATCH_WORKLOAD_IDENTITY_CLIENT_ID}",
			},
		},
		AutomountServiceAccountToken: ptr.To(false),
	}

	objects := []any{sa, newDeployment(sa, podLabels), newService()}
	objects = append(objects, newNetworkPolicies()...)
	objects = append(objects, newPodMonitor())

	return append(objects, newExternalSecrets()...)
}

// appLabels returns a new map of the labels shared by the objects' metadata
// and the selectors that match them. Every use gets its own map, so mutating
// one object's labels in place cannot silently change a selector (and the
// Deployment's selector is immutable once created).
func appLabels() map[string]string {
	return map[string]string{
		"app":   appName,
		"owner": "platform",
	}
}

// newDeployment defines the single-replica flux-dispatch Deployment. Its env
// vars cover internal/config/config.go as follows:
//   - DRY_RUN, GITHUB_APP_ID and GITHUB_INSTALLATION_ID come from ${...}
//     postBuild placeholders (see README.md "DRY_RUN mode" for why DRY_RUN is
//     always set explicitly rather than relying on its code default).
//   - GITHUB_PRIVATE_KEY_PATH is the literal path of the private key in the
//     mounted Secret volume.
//   - LISTEN_ADDR and METRICS_ADDR are derived from webhookPort and
//     metricsPort, so the listeners follow the port constants that the
//     container ports, the Service and the NetworkPolicies use.
//   - GITHUB_API_URL, DEDUP_TTL, DEDUP_MAX_ENTRIES and DEFAULT_DISPATCH_EVENT
//     are left unset: their code defaults already match RFC 0010.
func newDeployment(sa *corev1.ServiceAccount, podLabels map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
			Labels:    appLabels(),
			Annotations: map[string]string{
				scalingNoteAnnotation: scalingNoteText,
			},
		},
		Spec: appsv1.DeploymentSpec{
			// See scalingNoteAnnotation above for why this is 1 and must stay 1.
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: appLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
					Annotations: map[string]string{
						"cluster-autoscaler.kubernetes.io/safe-to-evict": "true",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:           sa.Name,
					AutomountServiceAccountToken: ptr.To(false),
					EnableServiceLinks:           ptr.To(false),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: githubAppKeyVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: githubAppKeySecretName,
									// Optional: lets the pod start before the
									// ExternalSecret can materialize this
									// Secret. See README.md "DRY_RUN mode" —
									// paired with the config.Load startup check.
									Optional: ptr.To(true),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  appName,
							Image: containerImage,
							Ports: []corev1.ContainerPort{
								{
									Name:          webhookPortName,
									ContainerPort: webhookPort,
								},
								{
									Name:          metricsPortName,
									ContainerPort: metricsPort,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "LISTEN_ADDR",
									Value: fmt.Sprintf(":%d", webhookPort),
								},
								{
									Name:  "METRICS_ADDR",
									Value: fmt.Sprintf(":%d", metricsPort),
								},
								{
									Name:  "DRY_RUN",
									Value: "${DRY_RUN}",
								},
								{
									Name:  "GITHUB_APP_ID",
									Value: "${GITHUB_APP_ID}",
								},
								{
									Name:  "GITHUB_INSTALLATION_ID",
									Value: "${GITHUB_INSTALLATION_ID}",
								},
								{
									Name:  "GITHUB_PRIVATE_KEY_PATH",
									Value: githubAppKeyMountPath + "/" + githubAppKeyDataKey,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      githubAppKeyVolumeName,
									MountPath: githubAppKeyMountPath,
									ReadOnly:  true,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromString(webhookPortName),
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.FromString(webhookPortName),
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								ReadOnlyRootFilesystem:   ptr.To(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
					},
				},
			},
		},
	}
}

// newService defines the ClusterIP Service fronting the webhook port. The
// metrics port is intentionally not exposed here — the PodMonitor below
// scrapes pods directly, matching lakmus's pattern.
func newService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
			Labels:    appLabels(),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: appLabels(),
			Ports: []corev1.ServicePort{
				{
					Name:       webhookPortName,
					Port:       webhookPort,
					TargetPort: intstr.FromString(webhookPortName),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// newNetworkPolicies defines the three NetworkPolicies copied verbatim from
// RFC 0010 §NetworkPolicy (field-for-field, including the podSelector using
// only `app: flux-dispatch`, not the broader two-key appLabels() set used
// elsewhere in this file — Kubernetes label selectors are a subset match, so
// this still selects the Deployment's pods).
//
// The other dis-* operators (dis-apim-operator, dis-identity-operator,
// dis-pgsql-operator) were checked for a local egress convention before
// writing the egress rule. None of their config/network-policy manifests
// define an egress policy at all — they are unmodified kubebuilder
// scaffolding (generic `webhook: enabled` / `metrics: enabled` namespace-label
// placeholders, ingress-only). There is no established egress convention to
// reconcile with, so the RFC's verbatim block is authoritative as-is.
func newNetworkPolicies() []any {
	return []any{
		newIngressNetworkPolicy("flux-dispatch-allow-webhook-traffic", fluxSystemNamespace, webhookPort),
		newIngressNetworkPolicy("flux-dispatch-allow-metrics-traffic", monitoringNamespace, metricsPort),
		newEgressNetworkPolicy(),
	}
}

func newIngressNetworkPolicy(name, fromNamespace string, port int32) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": appName},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"kubernetes.io/metadata.name": fromNamespace},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(port))},
					},
				},
			},
		},
	}
}

func newEgressNetworkPolicy() *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-dispatch-allow-egress",
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": appName},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"kubernetes.io/metadata.name": kubeSystemNamespace},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{Protocol: ptr.To(corev1.ProtocolUDP), Port: ptr.To(intstr.FromInt32(53))},
						{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(53))},
					},
				},
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(443))},
					},
				},
			},
		},
	}
}

// podMonitor holds the fields flux-dispatch sets on an
// azmonitoring.coreos.com/v1 PodMonitor. This type and the external-secrets
// and kustomize types below are declared here rather than imported: the
// azmonitoring.coreos.com group has no Go API package of its own (upstream
// prometheus-operator types would add that module and track a newer schema
// than Azure's CRD), and four small objects (PodMonitor, SecretStore, ExternalSecret, Kustomization)
// do not justify pulling the external-secrets and kustomize API modules into
// go.mod. Optional fields are omitempty, so leaving one unset omits it rather
// than rendering an empty value the API server may reject.
type podMonitor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              podMonitorSpec `json:"spec"`
}

type podMonitorSpec struct {
	Selector            metav1.LabelSelector        `json:"selector"`
	NamespaceSelector   podMonitorNamespaceSelector `json:"namespaceSelector"`
	PodMetricsEndpoints []podMetricsEndpoint        `json:"podMetricsEndpoints"`
}

type podMonitorNamespaceSelector struct {
	Any bool `json:"any,omitempty"`
}

type podMetricsEndpoint struct {
	Port     string `json:"port,omitempty"`
	Path     string `json:"path,omitempty"`
	Interval string `json:"interval,omitempty"`
}

// newPodMonitor scrapes the metrics port. lakmus uses PodMonitor (not
// ServiceMonitor) on the azmonitoring.coreos.com/v1 group — Azure Managed
// Prometheus's own CRD group — so this mirrors that exactly.
func newPodMonitor() *podMonitor {
	return &podMonitor{
		TypeMeta: metav1.TypeMeta{APIVersion: "azmonitoring.coreos.com/v1", Kind: "PodMonitor"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
			Labels:    appLabels(),
		},
		Spec: podMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: appLabels(),
			},
			NamespaceSelector: podMonitorNamespaceSelector{
				Any: true,
			},
			PodMetricsEndpoints: []podMetricsEndpoint{
				{
					Port:     metricsPortName,
					Path:     "/metrics",
					Interval: "30s",
				},
			},
		},
	}
}

// secretStore holds the fields flux-dispatch sets on an external-secrets.io/v1
// SecretStore.
type secretStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              secretStoreSpec `json:"spec"`
}

type secretStoreSpec struct {
	Provider secretStoreProvider `json:"provider"`
}

type secretStoreProvider struct {
	AzureKV azureKVProvider `json:"azurekv"`
}

type azureKVProvider struct {
	AuthType          string             `json:"authType,omitempty"`
	VaultURL          string             `json:"vaultUrl"`
	ServiceAccountRef *serviceAccountRef `json:"serviceAccountRef,omitempty"`
}

type serviceAccountRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// externalSecret holds the fields flux-dispatch sets on an
// external-secrets.io/v1 ExternalSecret.
type externalSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              externalSecretSpec `json:"spec"`
}

type externalSecretSpec struct {
	RefreshInterval string               `json:"refreshInterval,omitempty"`
	SecretStoreRef  secretStoreRef       `json:"secretStoreRef"`
	Target          externalSecretTarget `json:"target"`
	Data            []externalSecretData `json:"data,omitempty"`
}

type secretStoreRef struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name"`
}

type externalSecretTarget struct {
	Name           string `json:"name,omitempty"`
	CreationPolicy string `json:"creationPolicy,omitempty"`
}

type externalSecretData struct {
	SecretKey string                  `json:"secretKey"`
	RemoteRef externalSecretRemoteRef `json:"remoteRef"`
}

type externalSecretRemoteRef struct {
	Key string `json:"key"`
}

// newExternalSecrets defines the namespaced SecretStore and the
// ExternalSecret that materializes the GitHub App private key as a
// Kubernetes Secret. See README.md "Secret management" for why this shape
// (rather than dis-vault-operator's Vault CRD) and for vaultUrl/the secret
// name, which are placeholders pending Key Vault provisioning.
func newExternalSecrets() []any {
	store := &secretStore{
		TypeMeta: metav1.TypeMeta{APIVersion: "external-secrets.io/v1", Kind: "SecretStore"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretStoreName,
			Namespace: namespace,
			Labels:    appLabels(),
		},
		Spec: secretStoreSpec{
			Provider: secretStoreProvider{
				AzureKV: azureKVProvider{
					AuthType: "WorkloadIdentity",
					VaultURL: "${KV_URI}",
					ServiceAccountRef: &serviceAccountRef{
						Name:      appName,
						Namespace: namespace,
					},
				},
			},
		},
	}

	return []any{
		store,
		newExternalSecret(githubAppKeySecretName, githubAppKeyDataKey, "${KV_SECRET_NAME_GITHUB_APP_KEY}"),
	}
}

// newExternalSecret derives the object's metadata.name from targetSecretName
// using the single "<secret>-external-secret" convention, rather than taking
// an independent name argument that could drift from the Secret it
// materializes.
func newExternalSecret(targetSecretName, secretKey, remoteKey string) *externalSecret {
	return &externalSecret{
		TypeMeta: metav1.TypeMeta{APIVersion: "external-secrets.io/v1", Kind: "ExternalSecret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetSecretName + "-external-secret",
			Namespace: namespace,
			Labels:    appLabels(),
		},
		Spec: externalSecretSpec{
			RefreshInterval: "1h",
			SecretStoreRef: secretStoreRef{
				Kind: "SecretStore",
				Name: secretStoreName,
			},
			Target: externalSecretTarget{
				Name:           targetSecretName,
				CreationPolicy: "Owner",
			},
			Data: []externalSecretData{
				{
					SecretKey: secretKey,
					RemoteRef: externalSecretRemoteRef{
						Key: remoteKey,
					},
				},
			},
		},
	}
}

// kustomization holds the fields flux-dispatch sets on the
// kustomize.config.k8s.io/v1beta1 Kustomization in config/.
type kustomization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Resources         []string `json:"resources,omitempty"`
}

func newKustomization() *kustomization {
	return &kustomization{
		TypeMeta: metav1.TypeMeta{APIVersion: "kustomize.config.k8s.io/v1beta1", Kind: "Kustomization"},
		ObjectMeta: metav1.ObjectMeta{
			Name: appName,
		},
		Resources: []string{resourcesFile},
	}
}
