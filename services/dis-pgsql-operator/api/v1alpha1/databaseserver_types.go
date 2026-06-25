package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// DatabaseServerAuth contains identities used for server administration.
type DatabaseServerAuth struct {
	// admin defines the identity used for admin access.
	Admin AdminIdentitySpec `json:"admin"`

	// user is retained for compatibility with early DatabaseServer manifests.
	// Server reconciliation ignores this field; use Database resources
	// to provision app and owner access.
	// +optional
	User *UserIdentitySpec `json:"user,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="has(self.identity.identityRef) || has(self.serviceAccountName)",message="serviceAccountName is required when identity.identityRef is not set."
// AdminIdentitySpec contains admin identity configuration and the workload identity ServiceAccount.
type AdminIdentitySpec struct {
	// identity defines the Entra identity source (direct values or ApplicationIdentity reference).
	Identity IdentitySource `json:"identity"`

	// serviceAccountName is the ServiceAccount name used for workload identity
	// when provisioning database access for child Database resources.
	// Optional when identityRef is set; defaults to identityRef.name.
	// +optional
	// +kubebuilder:validation:MinLength=1
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// UserIdentitySpec contains identity configuration for normal user access.
type UserIdentitySpec struct {
	// identity defines the Entra identity source (direct values or ApplicationIdentity reference).
	Identity IdentitySource `json:"identity"`
}

// +kubebuilder:validation:XValidation:rule="(has(self.identityRef) && !has(self.name) && !has(self.principalId)) || (!has(self.identityRef) && has(self.name) && has(self.principalId))",message="Provide either identityRef or both name and principalId."
// IdentitySource specifies either a reference to an ApplicationIdentity or direct identity values.
type IdentitySource struct {
	// identityRef points to an ApplicationIdentity in the same namespace.
	// +optional
	IdentityRef *ApplicationIdentityRef `json:"identityRef,omitempty"`

	// name is the Entra principal name (managed identity name).
	// +optional
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name,omitempty"`

	// principalId is the Entra principal object ID (GUID).
	// +optional
	// +kubebuilder:validation:MinLength=1
	PrincipalId string `json:"principalId,omitempty"`
}

// ApplicationIdentityRef references an ApplicationIdentity in the same namespace.
type ApplicationIdentityRef struct {
	// name is the ApplicationIdentity name in the same namespace.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// +kubebuilder:validation:Enum=hstore;pg_cron;pg_stat_statements;pgaudit;uuid-ossp
// DatabaseServerExtension is a curated PostgreSQL extension allowed by this operator.
type DatabaseServerExtension string

const (
	DatabaseServerExtensionHstore           DatabaseServerExtension = "hstore"
	DatabaseServerExtensionPgCron           DatabaseServerExtension = "pg_cron"
	DatabaseServerExtensionPgStatStatements DatabaseServerExtension = "pg_stat_statements"
	DatabaseServerExtensionPgAudit          DatabaseServerExtension = "pgaudit"
	DatabaseServerExtensionUUIDOSSP         DatabaseServerExtension = "uuid-ossp"
)

// +kubebuilder:validation:Enum=Dedicated;Shared
// DatabaseServerMode defines whether a DatabaseServer owns dedicated infrastructure or hosts databases.
type DatabaseServerMode string

const (
	DatabaseServerModeDedicated DatabaseServerMode = "Dedicated"
	DatabaseServerModeShared    DatabaseServerMode = "Shared"
)

// +kubebuilder:validation:XValidation:rule="self.name != 'azure.extensions' && self.name != 'shared_preload_libraries' && self.name != 'pgbouncer.enabled' && self.name != 'pgbouncer.max_prepared_statements' && self.name != 'pgbouncer.pool_mode' && self.name != 'max_connections'",message="azure.extensions/shared_preload_libraries are managed via enableExtensions, and pgbouncer/max_connections are managed by the operator."
// DatabaseServerParameter is a PostgreSQL server parameter with a scalar value.
type DatabaseServerParameter struct {
	// name is the PostgreSQL server parameter name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// value is the desired parameter value.
	Value intstr.IntOrString `json:"value"`
}

// DatabaseServerNetworkSpec references pre-existing network resources for shared databases.
type DatabaseServerNetworkSpec struct {
	// delegatedSubnetResourceId is the Azure ARM ID of an existing delegated subnet.
	// +kubebuilder:validation:MinLength=1
	DelegatedSubnetResourceID string `json:"delegatedSubnetResourceId"`

	// privateDnsZoneResourceId is the Azure ARM ID of an existing private DNS zone.
	// +kubebuilder:validation:MinLength=1
	PrivateDNSZoneResourceID string `json:"privateDnsZoneResourceId"`
}

// DatabaseServerSpec defines the desired state of DatabaseServer.
// +kubebuilder:validation:XValidation:rule="(has(self.mode) && self.mode == 'Shared') ? has(self.network) : !has(self.network)",message="spec.network is required when mode is Shared and must be omitted when mode is Dedicated."
type DatabaseServerSpec struct {
	// mode controls whether this DatabaseServer provisions a dedicated server or a shared server.
	// Defaults to Dedicated.
	// +optional
	// +kubebuilder:default=Dedicated
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="mode is immutable"
	Mode DatabaseServerMode `json:"mode,omitempty"`

	// version is the major version of PostgreSQL to run (e.g. 17).
	// +kubebuilder:validation:Minimum=9
	Version int `json:"version"`

	// serverType selects the size/profile of the database server (e.g. "dev", "prod").
	// +kubebuilder:validation:MinLength=1
	ServerType string `json:"serverType"`

	// auth defines the identities used for server administration.
	Auth DatabaseServerAuth `json:"auth"`

	// network references existing private access resources for shared databases.
	// It must be omitted for dedicated databases.
	// +optional
	Network *DatabaseServerNetworkSpec `json:"network,omitempty"`

	// enableExtensions is the curated list of PostgreSQL extensions that should be enabled.
	// Some extensions require shared_preload_libraries and are configured automatically.
	// +optional
	// +listType=set
	EnableExtensions []DatabaseServerExtension `json:"enableExtensions,omitempty"`

	// serverParams configures allowed PostgreSQL server parameters.
	// azure.extensions and shared_preload_libraries are managed via enableExtensions.
	// pgbouncer settings and max_connections are managed by the operator.
	// +optional
	// +listType=map
	// +listMapKey=name
	ServerParams []DatabaseServerParameter `json:"serverParams,omitempty"`

	// +optional
	Storage *DatabaseServerStorageSpec `json:"storage,omitempty"`

	// highAvailabilityEnabled controls whether PostgreSQL high availability is enabled.
	// If omitted, it defaults to true for prod/production server types and false otherwise.
	// +optional
	HighAvailabilityEnabled *bool `json:"highAvailabilityEnabled,omitempty"`

	// backupRetentionDays controls backup retention for the server.
	// If omitted, it defaults to 14 for non-prod server types and 30 for prod/production.
	// +optional
	// +kubebuilder:validation:Minimum=7
	// +kubebuilder:validation:Maximum=35
	BackupRetentionDays *int `json:"backupRetentionDays,omitempty"`
}

type DatabaseServerStorageSpec struct {
	// sizeGB is the initial storage size in GB.
	// If omitted, the operator will default it.
	// +optional
	SizeGB *int32 `json:"sizeGB,omitempty"`

	// tier is the storage performance tier (e.g. P10).
	// If omitted, the operator will default it.
	// +optional
	Tier *string `json:"tier,omitempty"`
}

// DatabaseServerParameterError captures a failed server parameter reconciliation.
type DatabaseServerParameterError struct {
	// name is the PostgreSQL server parameter name that failed.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// reason is the ASO/Azure reason for the failure, when available.
	// +optional
	Reason string `json:"reason,omitempty"`

	// message is a human-readable error from ASO/Azure.
	// +optional
	Message string `json:"message,omitempty"`
}

// DatabaseServerStatus defines the observed state of DatabaseServer.
type DatabaseServerStatus struct {
	// subnetCIDR is the /28 network block allocated for this database's subnet.
	// It is set by the controller once allocation succeeds.
	// +optional
	SubnetCIDR string `json:"subnetCIDR,omitempty"`

	// serverName is the Azure PostgreSQL Flexible Server name (the AzureName
	// of the owned FlexibleServer). It is the authoritative server identity
	// for consumers and may differ from metadata.name because the operator
	// appends the cluster-id ("<metadata.name>-<cluster-id>") to keep the
	// Azure name globally unique across clusters. Existing servers keep
	// their original AzureName so this can be rolled out without
	// recreating Azure resources.
	// +optional
	ServerName string `json:"serverName,omitempty"`

	// host is the fully qualified DNS name of the PostgreSQL Flexible Server
	// (server.Status.FullyQualifiedDomainName). It is populated once Azure
	// has provisioned the server. Consumers should read this instead of
	// deriving "<metadata.name>.postgres.database.azure.com".
	// +optional
	Host string `json:"host,omitempty"`

	// conditions represent the current state of the DatabaseServer resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types might include:
	// - "Ready": the database and its networking are fully provisioned
	// - "Provisioning": resources are being created
	// - "Error": the controller failed to reconcile the resource
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// serverParameterErrors contains per-parameter reconciliation failures reported
	// from owned FlexibleServersConfiguration resources.
	// +listType=map
	// +listMapKey=name
	// +optional
	ServerParameterErrors []DatabaseServerParameterError `json:"serverParameterErrors,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DatabaseServer is the Schema for the databases API.
type DatabaseServer struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DatabaseServer.
	// +required
	Spec DatabaseServerSpec `json:"spec"`

	// status defines the observed state of DatabaseServer.
	// +optional
	Status DatabaseServerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DatabaseServerList contains a list of DatabaseServer.
type DatabaseServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DatabaseServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DatabaseServer{}, &DatabaseServerList{})
}
