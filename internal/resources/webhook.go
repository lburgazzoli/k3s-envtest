package resources

import (
	"fmt"
	"net/url"

	"sigs.k8s.io/controller-runtime/pkg/client"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/ptr"
)

// urlFromClientConfig extracts and parses the URL from a WebhookClientConfig.
// Returns the URL string if valid, empty string if nil, or an error if the URL is malformed.
func urlFromClientConfig(config admissionregistrationv1.WebhookClientConfig) (string, error) {
	urlStr := ptr.Deref(config.URL, "")
	if urlStr == "" {
		return "", nil
	}

	if _, err := url.Parse(urlStr); err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	return urlStr, nil
}

// ExtractWebhookURLs extracts all ClientConfig URLs from a webhook configuration.
// Returns URLs that are non-nil. Supports both MutatingWebhookConfiguration and ValidatingWebhookConfiguration.
func ExtractWebhookURLs(obj client.Object) ([]string, error) {
	var urls []string

	switch webhook := obj.(type) {
	case *admissionregistrationv1.MutatingWebhookConfiguration:
		for _, wh := range webhook.Webhooks {
			urlStr, err := urlFromClientConfig(wh.ClientConfig)
			if err != nil {
				return nil, fmt.Errorf("invalid URL in mutating webhook %s: %w", webhook.GetName(), err)
			}
			if urlStr != "" {
				urls = append(urls, urlStr)
			}
		}
	case *admissionregistrationv1.ValidatingWebhookConfiguration:
		for _, wh := range webhook.Webhooks {
			urlStr, err := urlFromClientConfig(wh.ClientConfig)
			if err != nil {
				return nil, fmt.Errorf("invalid URL in validating webhook %s: %w", webhook.GetName(), err)
			}
			if urlStr != "" {
				urls = append(urls, urlStr)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported webhook configuration type: %T", obj)
	}

	return urls, nil
}

// patchClientConfig updates a WebhookClientConfig to use a direct URL instead of a service reference.
func patchClientConfig(
	config *admissionregistrationv1.WebhookClientConfig,
	baseURL string,
	caBundle string,
) {
	path := "/"
	if config.Service != nil && config.Service.Path != nil {
		path = *config.Service.Path
	} else if config.URL != nil {
		if parsedURL, err := url.Parse(*config.URL); err == nil {
			path = parsedURL.Path
		}
	}

	config.URL = ptr.To(baseURL + path)
	config.CABundle = []byte(caBundle)
	config.Service = nil
}

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
		patchClientConfig(&webhook.Webhooks[i].ClientConfig, baseURL, caBundle)
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
		patchClientConfig(&webhook.Webhooks[i].ClientConfig, baseURL, caBundle)
	}
}
