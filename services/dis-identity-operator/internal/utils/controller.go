package utils

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// IsOwnedBy checks if the given object is owned by the specified owner.
func IsOwnedBy(object metav1.Object, owner metav1.Object) bool {
	for _, ref := range object.GetOwnerReferences() {
		if ref.UID == owner.GetUID() {
			return true
		}
	}
	return false
}
