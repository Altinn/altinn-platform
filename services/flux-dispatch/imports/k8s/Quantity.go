package k8s

import (
	"encoding/json"
)

// Quantity represents a fixed-point number with a magnitude suffix used
// throughout the Kubernetes API for resource requests/limits (e.g. "50m",
// "64Mi").
//
// See the NOTE on IntOrString in this same package for why this is a
// hand-written, non-jsii replacement of the type cdk8s normally generates:
// it is a pure data value, so this is semantically identical to the
// generated proxy without depending on a locally-built jsii kernel tarball
// this environment cannot produce offline.
type Quantity interface {
	Value() interface{}
}

type quantity struct {
	value interface{}
}

func (q *quantity) Value() interface{} { return q.value }

// MarshalJSON makes quantity serialize as the bare wrapped scalar, matching
// the JSON/YAML shape Kubernetes expects for Quantity fields.
func (q *quantity) MarshalJSON() ([]byte, error) {
	return json.Marshal(q.value)
}

func Quantity_FromNumber(value *float64) Quantity {
	if err := validateQuantity_FromNumberParameters(value); err != nil {
		panic(err)
	}
	return &quantity{value: *value}
}

func Quantity_FromString(value *string) Quantity {
	if err := validateQuantity_FromStringParameters(value); err != nil {
		panic(err)
	}
	return &quantity{value: *value}
}
