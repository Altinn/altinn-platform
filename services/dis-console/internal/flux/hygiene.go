package flux

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MaxRawBytes caps the stored raw payload of a single resource. Larger objects
// are stored as a compact stub so one pathological object cannot bloat its row
// (and its TOAST chunks) without bound. The status fields the API serves are
// projected into columns from the full object before this cap is applied, so
// only the opaque raw blob is affected.
const MaxRawBytes = 256 * 1024

// volatileMetadataFields are object-metadata fields the apiserver rewrites on
// every store without any semantic change: managedFields churns whenever any
// field manager re-applies, and resourceVersion bumps on every write at all.
// Stripping them keeps an unchanged object byte-stable across sweeps, which is
// what lets the store skip rewriting (re-TOASTing) the raw payload of rows that
// did not actually change.
var volatileMetadataFields = []string{"managedFields", "resourceVersion"}

// stripVolatileMetadata removes the churn-only metadata fields in place.
func stripVolatileMetadata(obj map[string]any) {
	for _, f := range volatileMetadataFields {
		unstructured.RemoveNestedField(obj, "metadata", f)
	}
}

// rawAndHash strips the volatile metadata, then returns the content hash (over
// the full stripped object) together with the payload to store: the full object
// when it fits, or a compact identifying stub when it exceeds MaxRawBytes. The
// hash is always taken over the full object so change detection stays accurate
// even for truncated rows.
func rawAndHash(obj map[string]any) (raw []byte, hash string, err error) {
	stripVolatileMetadata(obj)

	full, err := json.Marshal(obj)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(full)
	hash = hex.EncodeToString(sum[:])

	if len(full) <= MaxRawBytes {
		return full, hash, nil
	}
	stub, err := truncatedStub(obj, len(full))
	if err != nil {
		return nil, "", err
	}
	return stub, hash, nil
}

// truncatedStub builds the placeholder stored in place of an oversized raw
// payload: enough identity to recognize the object, plus a marker and the
// original byte size.
func truncatedStub(obj map[string]any, size int) ([]byte, error) {
	var namespace, name string
	if md, ok := obj["metadata"].(map[string]any); ok {
		namespace, _ = md["namespace"].(string)
		name, _ = md["name"].(string)
	}
	return json.Marshal(map[string]any{
		"apiVersion":           obj["apiVersion"],
		"kind":                 obj["kind"],
		"metadata":             map[string]any{"namespace": namespace, "name": name},
		"_disConsoleTruncated": true,
		"_disConsoleRawBytes":  size,
	})
}
