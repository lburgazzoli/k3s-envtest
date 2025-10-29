package resources

import (
	"net/url"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
)

// PatchMutatingWebhookConfiguration patches a mutating webhook configuration
// to use the provided base URL and CA bundle. It modifies the webhook in-place.
//
// For each webhook in the configuration:
// - Sets clientConfig.url to baseURL + path (defaults to "/")
// - Sets clientConfig.caBundle to the provided CA bundle
// - Removes clientConfig.service field.
func PatchMutatingWebhookConfiguration(
	webhook *admissionregistrationv1.MutatingWebhookConfiguration,
	baseURL string,
	caBundle string,
) {
	for i := range webhook.Webhooks {
		path := "/"
		if webhook.Webhooks[i].ClientConfig.Service != nil && webhook.Webhooks[i].ClientConfig.Service.Path != nil {
			path = *webhook.Webhooks[i].ClientConfig.Service.Path
		} else if webhook.Webhooks[i].ClientConfig.URL != nil {
			if parsedURL, err := url.Parse(*webhook.Webhooks[i].ClientConfig.URL); err == nil {
				path = parsedURL.Path
			}
		}

		webhook.Webhooks[i].ClientConfig.URL = ptr.To(baseURL + path)
		webhook.Webhooks[i].ClientConfig.CABundle = []byte(caBundle)
		webhook.Webhooks[i].ClientConfig.Service = nil
	}
}

// PatchValidatingWebhookConfiguration patches a validating webhook configuration
// to use the provided base URL and CA bundle. It modifies the webhook in-place.
//
// For each webhook in the configuration:
// - Sets clientConfig.url to baseURL + path (defaults to "/")
// - Sets clientConfig.caBundle to the provided CA bundle
// - Removes clientConfig.service field.
func PatchValidatingWebhookConfiguration(
	webhook *admissionregistrationv1.ValidatingWebhookConfiguration,
	baseURL string,
	caBundle string,
) {
	for i := range webhook.Webhooks {
		path := "/"
		if webhook.Webhooks[i].ClientConfig.Service != nil && webhook.Webhooks[i].ClientConfig.Service.Path != nil {
			path = *webhook.Webhooks[i].ClientConfig.Service.Path
		} else if webhook.Webhooks[i].ClientConfig.URL != nil {
			if parsedURL, err := url.Parse(*webhook.Webhooks[i].ClientConfig.URL); err == nil {
				path = parsedURL.Path
			}
		}

		webhook.Webhooks[i].ClientConfig.URL = ptr.To(baseURL + path)
		webhook.Webhooks[i].ClientConfig.CABundle = []byte(caBundle)
		webhook.Webhooks[i].ClientConfig.Service = nil
	}
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
