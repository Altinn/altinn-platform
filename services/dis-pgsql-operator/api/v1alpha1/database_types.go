package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// DatabaseDeletionPolicy controls what happens to the PostgreSQL
// database when the Database resource is deleted.
// +kubebuilder:validation:Enum=Retain
type DatabaseDeletionPolicy string

const (
	DatabaseDeletionPolicyRetain DatabaseDeletionPolicy = "Retain"
)

// DatabaseServerReference identifies the DatabaseServer that hosts this
// database.
type DatabaseServerReference struct {
	// name is the same-namespace DatabaseServer resource to use as the server.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// +kubebuilder:validation:Enum=Reader;Writer;Owner
// DatabaseAccessRole is the database role granted to an access principal.
type DatabaseAccessRole string

const (
	// DatabaseAccessRoleReader grants read-only database access.
	DatabaseAccessRoleReader DatabaseAccessRole = "Reader"

	// DatabaseAccessRoleWriter grants read/write DML access without DDL.
	DatabaseAccessRoleWriter DatabaseAccessRole = "Writer"

	// DatabaseAccessRoleOwner grants read/write access plus schema ownership for DDL.
	DatabaseAccessRoleOwner DatabaseAccessRole = "Owner"
)

// DatabaseGroupPrincipalSpec contains an existing Entra group that should get
// access to the database.
type DatabaseGroupPrincipalSpec struct {
	// name is the Entra group display name used as the PostgreSQL principal name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// principalId is the Entra group object ID.
	// +kubebuilder:validation:MinLength=1
	PrincipalId string `json:"principalId"`
}

// +kubebuilder:validation:XValidation:rule="(has(self.identityRef) && !has(self.group)) || (!has(self.identityRef) && has(self.group))",message="Provide exactly one principal source: identityRef or group."
// DatabaseAccessPrincipalSpec describes one principal and the role it should get.
type DatabaseAccessPrincipalSpec struct {
	// role is the managed database access role granted to the principal.
	Role DatabaseAccessRole `json:"role"`

	// identityRef points to an ApplicationIdentity in the same namespace.
	// The operator resolves the managed identity name and principalId from status.
	// +optional
	IdentityRef *ApplicationIdentityRef `json:"identityRef,omitempty"`

	// group identifies an existing Entra group.
	// +optional
	Group *DatabaseGroupPrincipalSpec `json:"group,omitempty"`
}

// DatabaseAccessSpec describes role-based access requirements for the database.
type DatabaseAccessSpec struct {
	// principals is the list of Entra principals that should get database access.
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	Principals []DatabaseAccessPrincipalSpec `json:"principals"`
}

// DatabaseSpec defines the desired state of Database.
//
// The PostgreSQL database name is spec.name.
type DatabaseSpec struct {
	// name is the PostgreSQL database name to create inside the selected server.
	// It must be unique per server.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// server identifies the same-namespace DatabaseServer.
	Server DatabaseServerReference `json:"server"`

	// access defines the principals that should get access to this database.
	Access DatabaseAccessSpec `json:"access"`

	// deletionPolicy controls database cleanup when this resource is deleted.
	// Only Retain is supported in this API slice.
	// +optional
	// +kubebuilder:default=Retain
	DeletionPolicy DatabaseDeletionPolicy `json:"deletionPolicy,omitempty"`
}

// DatabaseValidationError captures a validation failure observed by the
// controller.
type DatabaseValidationError struct {
	// field is the JSON path of the invalid field.
	// +kubebuilder:validation:MinLength=1
	Field string `json:"field"`

	// reason is a machine-readable reason for the validation failure.
	// +kubebuilder:validation:MinLength=1
	Reason string `json:"reason"`

	// message is a human-readable description of the validation failure.
	// +kubebuilder:validation:MinLength=1
	Message string `json:"message"`
}

// DatabaseStatus defines the observed state of Database.
type DatabaseStatus struct {
	// databaseName is the PostgreSQL database name managed by the operator.
	// +optional
	DatabaseName string `json:"databaseName,omitempty"`

	// host is the PostgreSQL server host for this database.
	// It is populated in a later reconciliation slice.
	// +optional
	Host string `json:"host,omitempty"`

	// port is the PostgreSQL server port for this database.
	// It is populated in a later reconciliation slice.
	// +optional
	Port int32 `json:"port,omitempty"`

	// observedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions represent the current validation/provisioning state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// validationErrors contains field-level validation failures.
	// +listType=map
	// +listMapKey=field
	// +optional
	ValidationErrors []DatabaseValidationError `json:"validationErrors,omitempty"`
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
