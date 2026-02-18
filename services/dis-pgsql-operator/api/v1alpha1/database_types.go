package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseAuth contains the identities that should get access to the database.
type DatabaseAuth struct {
	// admin defines the identity used for admin access.
	Admin AdminIdentitySpec `json:"admin"`

	// user defines the identity used for normal user access.
	User UserIdentitySpec `json:"user"`
}

// +kubebuilder:validation:XValidation:rule="has(self.identity.identityRef) || has(self.serviceAccountName)",message="serviceAccountName is required when identity.identityRef is not set."
// AdminIdentitySpec contains admin identity configuration and the workload identity ServiceAccount.
type AdminIdentitySpec struct {
	// identity defines the Entra identity source (direct values or ApplicationIdentity reference).
	Identity IdentitySource `json:"identity"`

	// serviceAccountName is the ServiceAccount name used for workload identity
	// when provisioning normal DB users for this database.
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
// DatabaseExtension is a curated PostgreSQL extension allowed by this operator.
type DatabaseExtension string

const (
	DatabaseExtensionHstore           DatabaseExtension = "hstore"
	DatabaseExtensionPgCron           DatabaseExtension = "pg_cron"
	DatabaseExtensionPgStatStatements DatabaseExtension = "pg_stat_statements"
	DatabaseExtensionPgAudit          DatabaseExtension = "pgaudit"
	DatabaseExtensionUUIDOSSP         DatabaseExtension = "uuid-ossp"
)

// DatabaseSpec defines the desired state of Database.
type DatabaseSpec struct {
	// version is the major version of PostgreSQL to run (e.g. 17).
	// +kubebuilder:validation:Minimum=9
	Version int `json:"version"`

	// serverType selects the size/profile of the database server (e.g. "dev", "prod").
	// +kubebuilder:validation:MinLength=1
	ServerType string `json:"serverType"`

	// auth defines which AppIdentities should have access to this database.
	Auth DatabaseAuth `json:"auth"`

	// enableExtensions is the curated list of PostgreSQL extensions that should be enabled.
	// Some extensions require shared_preload_libraries and are configured automatically.
	// +optional
	// +listType=set
	EnableExtensions []DatabaseExtension `json:"enableExtensions,omitempty"`

	// +optional
	Storage *DatabaseStorageSpec `json:"storage,omitempty"`
}

type DatabaseStorageSpec struct {
	// sizeGB is the initial storage size in GB.
	// If omitted, the operator will default it.
	// +optional
	SizeGB *int32 `json:"sizeGB,omitempty"`

	// tier is the storage performance tier (e.g. P10).
	// If omitted, the operator will default it.
	// +optional
	Tier *string `json:"tier,omitempty"`
}

// DatabaseStatus defines the observed state of Database.
type DatabaseStatus struct {
	// subnetCIDR is the /28 network block allocated for this database's subnet.
	// It is set by the controller once allocation succeeds.
	// +optional
	SubnetCIDR string `json:"subnetCIDR,omitempty"`

	// conditions represent the current state of the Database resource.
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
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Database is the Schema for the databases API.
type Database struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Database.
	// +required
	Spec DatabaseSpec `json:"spec"`

	// status defines the observed state of Database.
	// +optional
	Status DatabaseStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DatabaseList contains a list of Database.
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Database{}, &DatabaseList{})
}
