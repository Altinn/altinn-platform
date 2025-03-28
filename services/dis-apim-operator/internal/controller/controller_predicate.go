package controller

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
)

func getCommonPredicateFuncs(suffix string) predicate.Funcs {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return strings.HasSuffix(object.GetNamespace(), suffix)
	})
}
