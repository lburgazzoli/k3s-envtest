package resources

import (
	"fmt"

	"github.com/lburgazzoli/k3s-envtest/internal/jq"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// FilterConvertibleCRDs filters a list of CRDs to only include those that support
// conversion between versions according to the provided scheme.
func FilterConvertibleCRDs(
	scheme *runtime.Scheme,
	crds []unstructured.Unstructured,
) ([]unstructured.Unstructured, error) {
	convertibles, err := AllConvertibleTypes(scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to determine convertible types: %w", err)
	}

	var convertibleCRDs []unstructured.Unstructured
	for _, crd := range crds {
		group, err := jq.Query[string](&crd, `.spec.group`)
		if err != nil {
			return nil, fmt.Errorf("failed to extract group from CRD %s: %w", crd.GetName(), err)
		}
		if group == "" {
			return nil, fmt.Errorf("CRD %s missing required field: spec.group", crd.GetName())
		}

		kind, err := jq.Query[string](&crd, `.spec.names.kind`)
		if err != nil {
			return nil, fmt.Errorf("failed to extract kind from CRD %s: %w", crd.GetName(), err)
		}
		if kind == "" {
			return nil, fmt.Errorf("CRD %s missing required field: spec.names.kind", crd.GetName())
		}

		if convertibles.Has(schema.GroupKind{Group: group, Kind: kind}) {
			convertibleCRDs = append(convertibleCRDs, crd)
		}
	}

	return convertibleCRDs, nil
}
