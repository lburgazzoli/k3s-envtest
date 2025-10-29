//nolint:dupl
package resources_test

import (
	"encoding/base64"
	"testing"

	"github.com/lburgazzoli/k3s-envtest/internal/resources"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/onsi/gomega"
)

const (
	testCABundleStr = "test-ca-bundle-data"
)

func TestExtractWebhookURLs_InvalidURL_Mutating(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "test-webhook"},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			{
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL: ptr.To("ht!tp://invalid"),
				},
			},
		},
	}

	_, err := resources.ExtractWebhookURLs(webhook)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid URL"))
	g.Expect(err.Error()).To(ContainSubstring("test-webhook"))
	g.Expect(err.Error()).To(ContainSubstring("mutating"))
}

func TestExtractWebhookURLs_InvalidURL_Validating(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "test-validating-webhook"},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL: ptr.To("ht!tp://invalid-validating"),
				},
			},
		},
	}

	_, err := resources.ExtractWebhookURLs(webhook)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid URL"))
	g.Expect(err.Error()).To(ContainSubstring("test-validating-webhook"))
	g.Expect(err.Error()).To(ContainSubstring("validating"))
}

func TestExtractWebhookURLs_ValidURL_Mutating(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "test-webhook"},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			{
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL: ptr.To("https://example.com/webhook"),
				},
			},
		},
	}

	urls, err := resources.ExtractWebhookURLs(webhook)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(urls).To(HaveLen(1))
	g.Expect(urls[0]).To(Equal("https://example.com/webhook"))
}

func TestExtractWebhookURLs_ValidURL_Validating(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "test-webhook"},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL: ptr.To("https://example.com/validate"),
				},
			},
		},
	}

	urls, err := resources.ExtractWebhookURLs(webhook)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(urls).To(HaveLen(1))
	g.Expect(urls[0]).To(Equal("https://example.com/validate"))
}

func TestExtractWebhookURLs_NilURL(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "test-webhook"},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			{
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL: nil,
				},
			},
		},
	}

	urls, err := resources.ExtractWebhookURLs(webhook)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(urls).To(BeEmpty())
}

func TestExtractWebhookURLs_MultipleURLs(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "test-webhook"},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			{
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL: ptr.To("https://example.com/webhook1"),
				},
			},
			{
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL: ptr.To("https://example.com/webhook2"),
				},
			},
		},
	}

	urls, err := resources.ExtractWebhookURLs(webhook)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(urls).To(HaveLen(2))
	g.Expect(urls).To(ContainElements("https://example.com/webhook1", "https://example.com/webhook2"))
}

func TestPatchWebhookConfiguration_Validating(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-validating-webhook",
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "validate.example.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-service",
						Path:      ptr.To("/validate"),
					},
				},
			},
		},
	}

	resources.PatchValidatingWebhookConfiguration(webhook, testBaseURL, testCABundleStr)

	g.Expect(webhook.Webhooks).To(HaveLen(1))
	g.Expect(webhook.Webhooks[0].ClientConfig.URL).To(Equal(ptr.To(testBaseURL + "/validate")))
	g.Expect(webhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte(testCABundleStr)))
	g.Expect(webhook.Webhooks[0].ClientConfig.Service).To(BeNil())
}

func TestPatchWebhookConfiguration_Mutating(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mutating-webhook",
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			{
				Name: "mutate.example.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-service",
						Path:      ptr.To("/mutate"),
					},
				},
			},
		},
	}

	resources.PatchMutatingWebhookConfiguration(webhook, testBaseURL, testCABundleStr)

	g.Expect(webhook.Webhooks).To(HaveLen(1))
	g.Expect(webhook.Webhooks[0].ClientConfig.URL).To(Equal(ptr.To(testBaseURL + "/mutate")))
	g.Expect(webhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte(testCABundleStr)))
	g.Expect(webhook.Webhooks[0].ClientConfig.Service).To(BeNil())
}

func TestPatchWebhookConfiguration_MultipleWebhooks(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-multiple-webhooks",
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "webhook1.example.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-service",
						Path:      ptr.To("/validate1"),
					},
				},
			},
			{
				Name: "webhook2.example.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-service",
						Path:      ptr.To("/validate2"),
					},
				},
			},
		},
	}

	resources.PatchValidatingWebhookConfiguration(webhook, testBaseURL, testCABundleStr)

	g.Expect(webhook.Webhooks).To(HaveLen(2))
	g.Expect(webhook.Webhooks[0].ClientConfig.URL).To(Equal(ptr.To(testBaseURL + "/validate1")))
	g.Expect(webhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte(testCABundleStr)))
	g.Expect(webhook.Webhooks[1].ClientConfig.URL).To(Equal(ptr.To(testBaseURL + "/validate2")))
	g.Expect(webhook.Webhooks[1].ClientConfig.CABundle).To(Equal([]byte(testCABundleStr)))
}

func TestPatchWebhookConfiguration_DefaultPath(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-default-path",
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "validate.example.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-service",
					},
				},
			},
		},
	}

	resources.PatchValidatingWebhookConfiguration(webhook, testBaseURL, testCABundleStr)

	g.Expect(webhook.Webhooks[0].ClientConfig.URL).To(Equal(ptr.To(testBaseURL + "/")))
}

func TestPatchWebhookConfiguration_RealWorldExample(t *testing.T) {
	g := NewWithT(t)

	caCert := base64.StdEncoding.EncodeToString([]byte("mock-ca-certificate"))
	baseURL := "https://host.testcontainers.internal:9443"

	sideEffects := admissionregistrationv1.SideEffectClassNone
	webhook := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-policy-webhook",
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "validate.pods.policy.io",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Namespace: "webhook-system",
						Name:      "webhook-server",
						Port:      ptr.To(int32(443)),
						Path:      ptr.To("/validate-v1-pod"),
					},
				},
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Update,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				},
				AdmissionReviewVersions: []string{"v1"},
				SideEffects:             &sideEffects,
			},
		},
	}

	resources.PatchValidatingWebhookConfiguration(webhook, baseURL, caCert)

	g.Expect(webhook.Webhooks).To(HaveLen(1))
	g.Expect(webhook.Webhooks[0].ClientConfig.URL).To(Equal(ptr.To(baseURL + "/validate-v1-pod")))
	g.Expect(webhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte(caCert)))
	g.Expect(webhook.Webhooks[0].ClientConfig.Service).To(BeNil())
	g.Expect(webhook.Webhooks[0].Rules).To(HaveLen(1))
	g.Expect(webhook.Webhooks[0].AdmissionReviewVersions).To(Equal([]string{"v1"}))
}
