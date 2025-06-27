package controller

import (
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// defaultPredicate returns a predicate that filters events based on the namespace suffix.
func defaultPredicate(namespaceSuffix string) predicate.Predicate {
	var namespacePredicate = predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return strings.HasSuffix(e.Object.GetNamespace(), namespaceSuffix)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return strings.HasSuffix(e.Object.GetNamespace(), namespaceSuffix)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return strings.HasSuffix(e.ObjectNew.GetNamespace(), namespaceSuffix)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return strings.HasSuffix(e.Object.GetNamespace(), namespaceSuffix)
		},
	}
	return namespacePredicate
}
