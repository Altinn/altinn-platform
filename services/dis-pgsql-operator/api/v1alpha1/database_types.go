package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseAuth contains the identities that should get access to the database.
type DatabaseAuth struct {
	// adminAppIdentity is the name of the AppIdentity that should have full admin access to the database.
	// +kubebuilder:validation:MinLength=1
	AdminAppIdentity string `json:"adminAppIdentity"`

	// userAppIdentity is the name of the AppIdentity that should have non-admin access to the database.
	// +kubebuilder:validation:MinLength=1
	UserAppIdentity string `json:"userAppIdentity"`
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
