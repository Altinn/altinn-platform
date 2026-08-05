// k8s
package k8s

import (
	"reflect"

	_jsii_ "github.com/aws/jsii-runtime-go/runtime"
)

func init() {
	_jsii_.RegisterStruct(
		"k8s.Affinity",
		reflect.TypeOf((*Affinity)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.AwsElasticBlockStoreVolumeSource",
		reflect.TypeOf((*AwsElasticBlockStoreVolumeSource)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.AzureDiskVolumeSource",
		reflect.TypeOf((*AzureDiskVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.AzureFileVolumeSource",
		reflect.TypeOf((*AzureFileVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.Capabilities",
		reflect.TypeOf((*Capabilities)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.CephFsVolumeSource",
		reflect.TypeOf((*CephFsVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.CinderVolumeSource",
		reflect.TypeOf((*CinderVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.ConfigMapEnvSource",
		reflect.TypeOf((*ConfigMapEnvSource)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.ConfigMapKeySelector",
		reflect.TypeOf((*ConfigMapKeySelector)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.ConfigMapProjection",
		reflect.TypeOf((*ConfigMapProjection)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.ConfigMapVolumeSource",
		reflect.TypeOf((*ConfigMapVolumeSource)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.Container",
		reflect.TypeOf((*Container)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.ContainerPort",
		reflect.TypeOf((*ContainerPort)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.CsiVolumeSource",
		reflect.TypeOf((*CsiVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.DeploymentSpec",
		reflect.TypeOf((*DeploymentSpec)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.DeploymentStrategy",
		reflect.TypeOf((*DeploymentStrategy)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.DownwardApiProjection",
		reflect.TypeOf((*DownwardApiProjection)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.DownwardApiVolumeFile",
		reflect.TypeOf((*DownwardApiVolumeFile)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.DownwardApiVolumeSource",
		reflect.TypeOf((*DownwardApiVolumeSource)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.EmptyDirVolumeSource",
		reflect.TypeOf((*EmptyDirVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.EnvFromSource",
		reflect.TypeOf((*EnvFromSource)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.EnvVar",
		reflect.TypeOf((*EnvVar)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.EnvVarSource",
		reflect.TypeOf((*EnvVarSource)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.EphemeralContainer",
		reflect.TypeOf((*EphemeralContainer)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.EphemeralVolumeSource",
		reflect.TypeOf((*EphemeralVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.ExecAction",
		reflect.TypeOf((*ExecAction)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.FcVolumeSource",
		reflect.TypeOf((*FcVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.FlexVolumeSource",
		reflect.TypeOf((*FlexVolumeSource)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.FlockerVolumeSource",
		reflect.TypeOf((*FlockerVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.GcePersistentDiskVolumeSource",
		reflect.TypeOf((*GcePersistentDiskVolumeSource)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.GitRepoVolumeSource",
		reflect.TypeOf((*GitRepoVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.GlusterfsVolumeSource",
		reflect.TypeOf((*GlusterfsVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.GrpcAction",
		reflect.TypeOf((*GrpcAction)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.HostAlias",
		reflect.TypeOf((*HostAlias)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.HostPathVolumeSource",
		reflect.TypeOf((*HostPathVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.HttpGetAction",
		reflect.TypeOf((*HttpGetAction)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.HttpHeader",
		reflect.TypeOf((*HttpHeader)(nil)).Elem(),
	)

	_jsii_.RegisterClass(
		"k8s.IntOrString",
		reflect.TypeOf((*IntOrString)(nil)).Elem(),
		[]_jsii_.Member{
			_jsii_.MemberProperty{JsiiProperty: "value", GoGetter: "Value"},
		},
		func() interface{} {
			return &jsiiProxy_IntOrString{}
		},
	)

	_jsii_.RegisterStruct(
		"k8s.IscsiVolumeSource",
		reflect.TypeOf((*IscsiVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.KeyToPath",
		reflect.TypeOf((*KeyToPath)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.KubeDeploymentProps",
		reflect.TypeOf((*KubeDeploymentProps)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.KubeServiceAccountProps",
		reflect.TypeOf((*KubeServiceAccountProps)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.LabelSelector",
		reflect.TypeOf((*LabelSelector)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.LabelSelectorRequirement",
		reflect.TypeOf((*LabelSelectorRequirement)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.Lifecycle",
		reflect.TypeOf((*Lifecycle)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.LifecycleHandler",
		reflect.TypeOf((*LifecycleHandler)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.LocalObjectReference",
		reflect.TypeOf((*LocalObjectReference)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.ManagedFieldsEntry",
		reflect.TypeOf((*ManagedFieldsEntry)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.NfsVolumeSource",
		reflect.TypeOf((*NfsVolumeSource)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.NodeAffinity",
		reflect.TypeOf((*NodeAffinity)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.NodeSelector",
		reflect.TypeOf((*NodeSelector)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.NodeSelectorRequirement",
		reflect.TypeOf((*NodeSelectorRequirement)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.NodeSelectorTerm",
		reflect.TypeOf((*NodeSelectorTerm)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.ObjectFieldSelector",
		reflect.TypeOf((*ObjectFieldSelector)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.ObjectMeta",
		reflect.TypeOf((*ObjectMeta)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.ObjectReference",
		reflect.TypeOf((*ObjectReference)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.Overhead",
		reflect.TypeOf((*Overhead)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.OwnerReference",
		reflect.TypeOf((*OwnerReference)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.PersistentVolumeClaimSpec",
		reflect.TypeOf((*PersistentVolumeClaimSpec)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.PersistentVolumeClaimTemplate",
		reflect.TypeOf((*PersistentVolumeClaimTemplate)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.PersistentVolumeClaimVolumeSource",
		reflect.TypeOf((*PersistentVolumeClaimVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.PhotonPersistentDiskVolumeSource",
		reflect.TypeOf((*PhotonPersistentDiskVolumeSource)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.PodAffinity",
		reflect.TypeOf((*PodAffinity)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.PodAffinityTerm",
		reflect.TypeOf((*PodAffinityTerm)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.PodAntiAffinity",
		reflect.TypeOf((*PodAntiAffinity)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.PodDnsConfig",
		reflect.TypeOf((*PodDnsConfig)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.PodDnsConfigOption",
		reflect.TypeOf((*PodDnsConfigOption)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.PodOs",
		reflect.TypeOf((*PodOs)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.PodReadinessGate",
		reflect.TypeOf((*PodReadinessGate)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.PodSecurityContext",
		reflect.TypeOf((*PodSecurityContext)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.PodSpec",
		reflect.TypeOf((*PodSpec)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.PodTemplateSpec",
		reflect.TypeOf((*PodTemplateSpec)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.PortworxVolumeSource",
		reflect.TypeOf((*PortworxVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.PreferredSchedulingTerm",
		reflect.TypeOf((*PreferredSchedulingTerm)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.Probe",
		reflect.TypeOf((*Probe)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.ProjectedVolumeSource",
		reflect.TypeOf((*ProjectedVolumeSource)(nil)).Elem(),
	)
	_jsii_.RegisterClass(
		"k8s.Quantity",
		reflect.TypeOf((*Quantity)(nil)).Elem(),
		[]_jsii_.Member{
			_jsii_.MemberProperty{JsiiProperty: "value", GoGetter: "Value"},
		},
		func() interface{} {
			return &jsiiProxy_Quantity{}
		},
	)

	_jsii_.RegisterStruct(
		"k8s.QuobyteVolumeSource",
		reflect.TypeOf((*QuobyteVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.RbdVolumeSource",
		reflect.TypeOf((*RbdVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.ResourceFieldSelector",
		reflect.TypeOf((*ResourceFieldSelector)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.ResourceRequirements",
		reflect.TypeOf((*ResourceRequirements)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.RollingUpdateDeployment",
		reflect.TypeOf((*RollingUpdateDeployment)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.ScaleIoVolumeSource",
		reflect.TypeOf((*ScaleIoVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.SeLinuxOptions",
		reflect.TypeOf((*SeLinuxOptions)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.SeccompProfile",
		reflect.TypeOf((*SeccompProfile)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.SecretEnvSource",
		reflect.TypeOf((*SecretEnvSource)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.SecretKeySelector",
		reflect.TypeOf((*SecretKeySelector)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.SecretProjection",
		reflect.TypeOf((*SecretProjection)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.SecretVolumeSource",
		reflect.TypeOf((*SecretVolumeSource)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.SecurityContext",
		reflect.TypeOf((*SecurityContext)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.ServiceAccountTokenProjection",
		reflect.TypeOf((*ServiceAccountTokenProjection)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.StorageOsVolumeSource",
		reflect.TypeOf((*StorageOsVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.Sysctl",
		reflect.TypeOf((*Sysctl)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.TcpSocketAction",
		reflect.TypeOf((*TcpSocketAction)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.Toleration",
		reflect.TypeOf((*Toleration)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.TopologySpreadConstraint",
		reflect.TypeOf((*TopologySpreadConstraint)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.TypedLocalObjectReference",
		reflect.TypeOf((*TypedLocalObjectReference)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.Volume",
		reflect.TypeOf((*Volume)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.VolumeDevice",
		reflect.TypeOf((*VolumeDevice)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.VolumeMount",
		reflect.TypeOf((*VolumeMount)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.VolumeProjection",
		reflect.TypeOf((*VolumeProjection)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.VsphereVirtualDiskVolumeSource",
		reflect.TypeOf((*VsphereVirtualDiskVolumeSource)(nil)).Elem(),
	)

	_jsii_.RegisterStruct(
		"k8s.WeightedPodAffinityTerm",
		reflect.TypeOf((*WeightedPodAffinityTerm)(nil)).Elem(),
	)
	_jsii_.RegisterStruct(
		"k8s.WindowsSecurityContextOptions",
		reflect.TypeOf((*WindowsSecurityContextOptions)(nil)).Elem(),
	)
}
