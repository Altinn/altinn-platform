package k8s

import (
	"encoding/json"
)

// IntOrString represents a value that can be either an int or a string (used
// e.g. for container ports referenced by name).
//
// NOTE: cdk8s normally generates this type as a proxy that round-trips
// through the jsii kernel using a locally-built "k8s" jsii assembly tarball
// (imports/k8s/jsii/k8s-0.0.0.tgz). That tarball is produced by `cdk8s import
// k8s --language go --output imports` (see lakmus's manifests-imports Make
// target) and is gitignored — neither lakmus nor this package commits it, and
// this environment cannot run the Node-based cdk8s CLI to regenerate it
// offline. IntOrString is a pure data value with no behavior beyond
// "serialize as the wrapped scalar", so this hand-written replacement is
// semantically identical to the generated proxy and avoids the kernel
// dependency entirely. If imports/k8s is ever refreshed via the real `cdk8s
// import` tool, this file will be regenerated back to the jsii-proxy form,
// which is also fine.
type IntOrString interface {
	Value() interface{}
}

type intOrString struct {
	value interface{}
}

func (i *intOrString) Value() interface{} { return i.value }

// MarshalJSON makes intOrString serialize as the bare wrapped scalar, matching
// the JSON/YAML shape Kubernetes expects for IntOrString fields.
func (i *intOrString) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}

func IntOrString_FromNumber(value *float64) IntOrString {
	if err := validateIntOrString_FromNumberParameters(value); err != nil {
		panic(err)
	}
	return &intOrString{value: *value}
}

func IntOrString_FromString(value *string) IntOrString {
	if err := validateIntOrString_FromStringParameters(value); err != nil {
		panic(err)
	}
	return &intOrString{value: *value}
}
