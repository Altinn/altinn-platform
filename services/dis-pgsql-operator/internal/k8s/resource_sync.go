package k8s

import "k8s.io/apimachinery/pkg/api/equality"

// SyncSpecAndLabels mutates an existing resource in-memory to match desired state.
//
// It compares existing vs desired spec, and ensures every desired label key/value is
// present. Missing or different desired labels are upserted. Existing labels that are
// not in desiredLabels are preserved.
//
// Example:
//   - existing labels: {"a":"1","keep":"x"}
//   - desired labels:  {"a":"2","b":"3"}
//   - result labels:   {"a":"2","b":"3","keep":"x"}
//
// The returned bool is true when an API Update call is required to persist changes.
func SyncSpecAndLabels[S any](
	existingSpec *S,
	desiredSpec S,
	existingLabels map[string]string,
	desiredLabels map[string]string,
) (map[string]string, bool) {
	updated := false

	if !equality.Semantic.DeepEqual(*existingSpec, desiredSpec) {
		*existingSpec = desiredSpec
		updated = true
	}

	if existingLabels == nil {
		existingLabels = map[string]string{}
	}

	for key, value := range desiredLabels {
		if existingLabels[key] != value {
			existingLabels[key] = value
			updated = true
		}
	}

	return existingLabels, updated
}
