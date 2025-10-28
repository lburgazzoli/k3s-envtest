package filter

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectFilter is a predicate for filtering Kubernetes objects.
// Works with any client.Object (typed or unstructured).
type ObjectFilter func(client.Object) bool

// Any returns a filter that accepts objects matching ANY of the provided filters (OR logic).
// If no filters provided, accepts all objects.
//
// Usage:
//
//	filter := Any(
//	    ByType(gvk.CustomResourceDefinition),
//	    ByType(gvk.MutatingWebhookConfiguration),
//	)
func Any(filters ...ObjectFilter) ObjectFilter {
	return func(obj client.Object) bool {
		if len(filters) == 0 {
			return true
		}
		for _, filter := range filters {
			if filter(obj) {
				return true
			}
		}
		return false
	}
}

// All returns a filter that accepts objects matching ALL of the provided filters (AND logic).
// If no filters provided, accepts all objects.
//
// Usage:
//
//	filter := All(
//	    ByType(gvk.Pod),
//	    ByNamespace("default"),
//	)
func All(filters ...ObjectFilter) ObjectFilter {
	return func(obj client.Object) bool {
		if len(filters) == 0 {
			return true
		}
		for _, filter := range filters {
			if !filter(obj) {
				return false
			}
		}
		return true
	}
}

// Negate returns a filter that inverts the provided filter.
//
// Usage:
//
//	filter := Negate(ByType(gvk.Secret))  // Everything except Secrets
func Negate(filter ObjectFilter) ObjectFilter {
	return func(obj client.Object) bool {
		return !filter(obj)
	}
}

// ByType creates a filter that accepts only objects matching the given GVKs.
//
// Usage:
//
//	// Single type
//	crds := ByType(gvk.CustomResourceDefinition)
//
//	// Multiple types
//	webhooks := ByType(gvk.MutatingWebhookConfiguration, gvk.ValidatingWebhookConfiguration)
//
//	// Combined with Any/All
//	filter := ByType(
//	    gvk.CustomResourceDefinition,
//	    gvk.MutatingWebhookConfiguration,
//	    gvk.ValidatingWebhookConfiguration,
//	)
func ByType(gvks ...schema.GroupVersionKind) ObjectFilter {
	gvkSet := sets.New(gvks...)
	return func(obj client.Object) bool {
		return gvkSet.Has(obj.GetObjectKind().GroupVersionKind())
	}
}
