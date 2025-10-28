package resources

import (
	"fmt"

	"github.com/lburgazzoli/k3s-envtest/internal/jq"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PatchWebhookConfiguration patches a webhook configuration (validating or mutating)
// to use the provided base URL and CA bundle. It modifies the webhook in-place.
//
// For each webhook in the configuration:
// - Sets clientConfig.url to baseURL + path (from clientConfig.service.path, defaults to "/")
// - Sets clientConfig.caBundle to the provided CA bundle
// - Removes clientConfig.service field.
func PatchWebhookConfiguration(
	webhook *unstructured.Unstructured,
	baseURL string,
	caBundle string,
) error {
	err := jq.Transform(webhook, `
		.webhooks |= map(
			.clientConfig.url = "%s" + (.clientConfig.service.path // "/") |
			.clientConfig.caBundle = "%s" |
			del(.clientConfig.service)
		)
	`, baseURL, caBundle)

	if err != nil {
		return fmt.Errorf("failed to patch webhook configuration: %w", err)
	}

	return nil
}

// PatchCRDConversion patches a CustomResourceDefinition to use webhook-based conversion
// with the provided base URL and CA bundle. It modifies the CRD in-place.
//
// Sets:
// - .spec.conversion.strategy = "Webhook"
// - .spec.conversion.webhook.conversionReviewVersions = ["v1", "v1beta1"]
// - .spec.conversion.webhook.clientConfig.url = baseURL + "/convert"
// - .spec.conversion.webhook.clientConfig.caBundle = caBundle.
func PatchCRDConversion(
	crd *unstructured.Unstructured,
	baseURL string,
	caBundle string,
) error {
	err := jq.Transform(
		crd, `
		.spec.conversion = {
			"strategy": "Webhook",
			"webhook": {
				"conversionReviewVersions": ["v1", "v1beta1"],
				"clientConfig": {
					"url": "%s",
					"caBundle": "%s"
				}
			}
		}
	`, baseURL+"/convert", caBundle)

	if err != nil {
		return fmt.Errorf("failed to patch CRD conversion: %w", err)
	}

	return nil
}
