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

// DatabasePrincipalSpec contains an Entra principal that should get
// access to the database.
type DatabasePrincipalSpec struct {
	// name is the Entra principal name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// principalId is the Entra principal object ID.
	// +kubebuilder:validation:MinLength=1
	PrincipalId string `json:"principalId"`
}

// DatabaseAccessSpec describes access requirements for the
// database.
type DatabaseAccessSpec struct {
	// app is the runtime application principal.
	App DatabasePrincipalSpec `json:"app"`

	// owner is the Entra group for the team that owns the database.
	Owner DatabasePrincipalSpec `json:"owner"`
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

	// access defines the identity that should get access to this database.
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
