package resources

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
)

// FilterConvertibleCRDs filters a list of CRDs to only include those that support
// conversion between versions according to the provided scheme.
func FilterConvertibleCRDs(
	scheme *runtime.Scheme,
	crds []apiextensionsv1.CustomResourceDefinition,
) ([]apiextensionsv1.CustomResourceDefinition, error) {
	convertibles, err := AllConvertibleTypes(scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to determine convertible types: %w", err)
	}

	var convertibleCRDs []apiextensionsv1.CustomResourceDefinition
	for _, crd := range crds {
		group := crd.Spec.Group
		if group == "" {
			return nil, fmt.Errorf("CRD %s missing required field: spec.group", crd.GetName())
		}

		kind := crd.Spec.Names.Kind
		if kind == "" {
			return nil, fmt.Errorf("CRD %s missing required field: spec.names.kind", crd.GetName())
		}

		if convertibles.Has(schema.GroupKind{Group: group, Kind: kind}) {
			convertibleCRDs = append(convertibleCRDs, crd)
		}
	}

	return convertibleCRDs, nil
}

// IsCRDEstablished checks if a CRD has the Established condition set to true.
func IsCRDEstablished(crd *apiextensionsv1.CustomResourceDefinition) bool {
	for _, condition := range crd.Status.Conditions {
		if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
			return true
		}
	}
	return false
}

// WaitForCRDEstablished polls until a CRD becomes established or the timeout is reached.
func WaitForCRDEstablished(
	ctx context.Context,
	cli client.Client,
	crdName string,
	pollInterval time.Duration,
	timeout time.Duration,
) error {
	err := wait.PollUntilContextTimeout(ctx, pollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		crd := apiextensionsv1.CustomResourceDefinition{}

		err := cli.Get(ctx, types.NamespacedName{Name: crdName}, &crd)
		switch {
		case k8serr.IsNotFound(err):
			return false, nil
		case err != nil:
			return false, fmt.Errorf("failed to get CRD: %w", err)
		default:
			return IsCRDEstablished(&crd), nil
		}
	})

	if err != nil {
		return fmt.Errorf("CRD %s not established: %w", crdName, err)
	}

	return nil
}

// PatchCRDConversion patches a CustomResourceDefinition to use webhook-based conversion.
// It modifies the CRD in-place.
func PatchCRDConversion(
	crd *apiextensionsv1.CustomResourceDefinition,
	baseURL string,
	caBundle []byte,
) {
	crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ConversionReviewVersions: []string{"v1", "v1beta1"},
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				URL:      ptr.To(baseURL + "/convert"),
				CABundle: caBundle,
			},
		},
	}
}
