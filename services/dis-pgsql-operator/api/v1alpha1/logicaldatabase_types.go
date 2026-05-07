package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// LogicalDatabaseDeletionPolicy controls what happens to the PostgreSQL
// database when the LogicalDatabase resource is deleted.
// +kubebuilder:validation:Enum=Retain
type LogicalDatabaseDeletionPolicy string

const (
	LogicalDatabaseDeletionPolicyRetain LogicalDatabaseDeletionPolicy = "Retain"
)

// LogicalDatabaseServerRef references the shared Database that hosts this
// logical database.
type LogicalDatabaseServerRef struct {
	// name is the same-namespace Database resource to use as the shared server.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// LogicalDatabaseTenantSpec identifies the tenant that owns this logical
// database.
type LogicalDatabaseTenantSpec struct {
	// id is the stable tenant identifier.
	// +kubebuilder:validation:MinLength=1
	ID string `json:"id"`

	// environment is the tenant environment.
	// +kubebuilder:validation:MinLength=1
	Environment string `json:"environment"`
}

// LogicalDatabasePrincipalSpec contains an Entra principal that should get
// access to the logical database.
type LogicalDatabasePrincipalSpec struct {
	// name is the Entra principal name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// principalId is the Entra principal object ID.
	// +kubebuilder:validation:MinLength=1
	PrincipalId string `json:"principalId"`
}

// LogicalDatabaseAccessSpec describes access requirements for the logical
// database.
type LogicalDatabaseAccessSpec struct {
	// app is the runtime application principal.
	App LogicalDatabasePrincipalSpec `json:"app"`

	// owner is the Entra group for the team that owns the logical database.
	Owner LogicalDatabasePrincipalSpec `json:"owner"`
}

// LogicalDatabaseSpec defines the desired state of LogicalDatabase.
type LogicalDatabaseSpec struct {
	// serverRef points to a same-namespace shared Database.
	ServerRef LogicalDatabaseServerRef `json:"serverRef"`

	// databaseKey is a short database purpose key.
	// +kubebuilder:validation:MinLength=1
	DatabaseKey string `json:"databaseKey"`

	// tenant identifies the tenant that owns this logical database.
	Tenant LogicalDatabaseTenantSpec `json:"tenant"`

	// access defines the identity that should get access to this logical database.
	Access LogicalDatabaseAccessSpec `json:"access"`

	// deletionPolicy controls database cleanup when this resource is deleted.
	// Only Retain is supported in this API slice.
	// +optional
	// +kubebuilder:default=Retain
	DeletionPolicy LogicalDatabaseDeletionPolicy `json:"deletionPolicy,omitempty"`
}

// LogicalDatabaseValidationError captures a validation failure observed by the
// controller.
type LogicalDatabaseValidationError struct {
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

// LogicalDatabaseStatus defines the observed state of LogicalDatabase.
type LogicalDatabaseStatus struct {
	// databaseName is the PostgreSQL database name derived by the operator.
	// It is populated in a later reconciliation slice.
	// +optional
	DatabaseName string `json:"databaseName,omitempty"`

	// host is the PostgreSQL server host for this logical database.
	// It is populated in a later reconciliation slice.
	// +optional
	Host string `json:"host,omitempty"`

	// port is the PostgreSQL server port for this logical database.
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
	ValidationErrors []LogicalDatabaseValidationError `json:"validationErrors,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// LogicalDatabase is the Schema for the logicaldatabases API.
type LogicalDatabase struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of LogicalDatabase.
	// +required
	Spec LogicalDatabaseSpec `json:"spec"`

	// status defines the observed state of LogicalDatabase.
	// +optional
	Status LogicalDatabaseStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// LogicalDatabaseList contains a list of LogicalDatabase.
type LogicalDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []LogicalDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogicalDatabase{}, &LogicalDatabaseList{})
}
