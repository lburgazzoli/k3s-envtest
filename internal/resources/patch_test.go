//nolint:dupl
package resources_test

import (
	"encoding/base64"
	"testing"

	"github.com/lburgazzoli/k3s-envtest/internal/resources"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/onsi/gomega"
)

const (
	testBaseURL     = "https://example.com:9443"
	testCABundleStr = "test-ca-bundle-data"
)

var testCABundleBytes = []byte("test-ca-bundle-data")

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

func TestPatchCRDConversion_Success(t *testing.T) {
	g := NewWithT(t)

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "examples.test.example.com",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "test.example.com",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "Example",
				Plural: "examples",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
				},
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: false,
				},
			},
		},
	}

	resources.PatchCRDConversion(crd, testBaseURL, testCABundleBytes)

	g.Expect(crd.Spec.Conversion).NotTo(BeNil())
	g.Expect(crd.Spec.Conversion.Strategy).To(Equal(apiextensionsv1.WebhookConverter))
	g.Expect(crd.Spec.Conversion.Webhook).NotTo(BeNil())
	g.Expect(crd.Spec.Conversion.Webhook.ConversionReviewVersions).To(Equal([]string{"v1", "v1beta1"}))
	g.Expect(crd.Spec.Conversion.Webhook.ClientConfig).NotTo(BeNil())
	g.Expect(crd.Spec.Conversion.Webhook.ClientConfig.URL).NotTo(BeNil())
	g.Expect(*crd.Spec.Conversion.Webhook.ClientConfig.URL).To(Equal(testBaseURL + "/convert"))
	g.Expect(crd.Spec.Conversion.Webhook.ClientConfig.CABundle).To(Equal(testCABundleBytes))
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
