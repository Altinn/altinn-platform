package k8scompat

import (
	"encoding/json"
	"strings"

	"github.com/Altinn/altinn-platform/services/lakmus/imports/k8s"
	"github.com/aws/constructs-go/constructs/v10"
	_jsii_ "github.com/aws/jsii-runtime-go"
	cdk8s "github.com/cdk8s-team/cdk8s-core-go/cdk8s/v2"
)

func NewKubeDeployment(scope constructs.Construct, id *string, props *k8s.KubeDeploymentProps) cdk8s.ApiObject {
	return newPatchedApiObject(scope, id, "apps/v1", "Deployment", props.Metadata, props)
}

func NewKubeServiceAccount(scope constructs.Construct, id *string, props *k8s.KubeServiceAccountProps) cdk8s.ApiObject {
	return newPatchedApiObject(scope, id, "v1", "ServiceAccount", props.Metadata, props)
}

func newPatchedApiObject(scope constructs.Construct, id *string, apiVersion string, kind string, metadata *k8s.ObjectMeta, props any) cdk8s.ApiObject {
	obj := cdk8s.NewApiObject(scope, id, &cdk8s.ApiObjectProps{
		ApiVersion: _jsii_.String(apiVersion),
		Kind:       _jsii_.String(kind),
		Metadata:   toApiObjectMetadata(metadata),
	})

	topLevel := toMap(props)
	delete(topLevel, "metadata")
	for key, value := range topLevel {
		if value == nil {
			continue
		}
		obj.AddJsonPatch(cdk8s.JsonPatch_Add(_jsii_.String("/"+escapeJSONPointer(key)), value))
	}

	return obj
}

func toApiObjectMetadata(metadata *k8s.ObjectMeta) *cdk8s.ApiObjectMetadata {
	if metadata == nil {
		return nil
	}

	return &cdk8s.ApiObjectMetadata{
		Name:        metadata.Name,
		Namespace:   metadata.Namespace,
		Labels:      metadata.Labels,
		Annotations: metadata.Annotations,
	}
}

func toMap(value any) map[string]any {
	bytes, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}

	out := map[string]any{}
	if err := json.Unmarshal(bytes, &out); err != nil {
		panic(err)
	}

	return out
}

func escapeJSONPointer(segment string) string {
	segment = strings.ReplaceAll(segment, "~", "~0")
	return strings.ReplaceAll(segment, "/", "~1")
}
