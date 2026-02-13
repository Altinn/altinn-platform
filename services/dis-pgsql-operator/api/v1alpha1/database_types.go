package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseAuth contains the identities that should get access to the database.
type DatabaseAuth struct {
	// adminAppIdentity is the Entra principal name that should have full admin access to the database.
	// +kubebuilder:validation:MinLength=1
	AdminAppIdentity string `json:"adminAppIdentity"`

	// adminAppPrincipalId is the Entra principal object ID (GUID) for adminAppIdentity.
	// +kubebuilder:validation:MinLength=1
	AdminAppPrincipalId string `json:"adminAppPrincipalId"`

	// adminServiceAccountName is the ServiceAccount name used for workload identity
	// when provisioning normal DB users for this database.
	// +kubebuilder:validation:MinLength=1
	AdminServiceAccountName string `json:"adminServiceAccountName"`

	// userAppIdentity is the Entra principal name that should have non-admin access to the database.
	// +kubebuilder:validation:MinLength=1
	UserAppIdentity string `json:"userAppIdentity"`

	// userAppPrincipalId is the Entra principal object ID (GUID) for userAppIdentity.
	// +kubebuilder:validation:MinLength=1
	UserAppPrincipalId string `json:"userAppPrincipalId"`
}

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
