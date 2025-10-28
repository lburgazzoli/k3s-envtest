//nolint:dupl
package resources_test

import (
	"encoding/base64"
	"testing"

	"github.com/lburgazzoli/k3s-envtest/internal/resources"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/onsi/gomega"
)

const (
	testBaseURL  = "https://example.com:9443"
	testCABundle = "test-ca-bundle-data"
)

func TestPatchWebhookConfiguration_Validating(t *testing.T) {
	g := NewWithT(t)

	webhook := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "ValidatingWebhookConfiguration",
			"metadata": map[string]interface{}{
				"name": "test-validating-webhook",
			},
			"webhooks": []interface{}{
				map[string]interface{}{
					"name": "validate.example.com",
					"clientConfig": map[string]interface{}{
						"service": map[string]interface{}{
							"namespace": "default",
							"name":      "webhook-service",
							"path":      "/validate",
						},
					},
				},
			},
		},
	}

	err := resources.PatchWebhookConfiguration(webhook, testBaseURL, testCABundle)
	g.Expect(err).NotTo(HaveOccurred())

	webhooks, found, err := unstructured.NestedSlice(webhook.Object, "webhooks")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(webhooks).To(HaveLen(1))

	firstWebhook := webhooks[0].(map[string]interface{})
	clientConfig := firstWebhook["clientConfig"].(map[string]interface{})

	g.Expect(clientConfig["url"]).To(Equal(testBaseURL + "/validate"))
	g.Expect(clientConfig["caBundle"]).To(Equal(testCABundle))
	g.Expect(clientConfig).NotTo(HaveKey("service"))
}

func TestPatchWebhookConfiguration_Mutating(t *testing.T) {
	g := NewWithT(t)

	webhook := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "MutatingWebhookConfiguration",
			"metadata": map[string]interface{}{
				"name": "test-mutating-webhook",
			},
			"webhooks": []interface{}{
				map[string]interface{}{
					"name": "mutate.example.com",
					"clientConfig": map[string]interface{}{
						"service": map[string]interface{}{
							"namespace": "default",
							"name":      "webhook-service",
							"path":      "/mutate",
						},
					},
				},
			},
		},
	}

	err := resources.PatchWebhookConfiguration(webhook, testBaseURL, testCABundle)
	g.Expect(err).NotTo(HaveOccurred())

	webhooks, found, err := unstructured.NestedSlice(webhook.Object, "webhooks")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(webhooks).To(HaveLen(1))

	firstWebhook := webhooks[0].(map[string]interface{})
	clientConfig := firstWebhook["clientConfig"].(map[string]interface{})

	g.Expect(clientConfig["url"]).To(Equal(testBaseURL + "/mutate"))
	g.Expect(clientConfig["caBundle"]).To(Equal(testCABundle))
	g.Expect(clientConfig).NotTo(HaveKey("service"))
}

func TestPatchWebhookConfiguration_MultipleWebhooks(t *testing.T) {
	g := NewWithT(t)

	webhook := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "ValidatingWebhookConfiguration",
			"metadata": map[string]interface{}{
				"name": "test-multiple-webhooks",
			},
			"webhooks": []interface{}{
				map[string]interface{}{
					"name": "webhook1.example.com",
					"clientConfig": map[string]interface{}{
						"service": map[string]interface{}{
							"namespace": "default",
							"name":      "webhook-service",
							"path":      "/validate1",
						},
					},
				},
				map[string]interface{}{
					"name": "webhook2.example.com",
					"clientConfig": map[string]interface{}{
						"service": map[string]interface{}{
							"namespace": "default",
							"name":      "webhook-service",
							"path":      "/validate2",
						},
					},
				},
			},
		},
	}

	err := resources.PatchWebhookConfiguration(webhook, testBaseURL, testCABundle)
	g.Expect(err).NotTo(HaveOccurred())

	webhooks, found, err := unstructured.NestedSlice(webhook.Object, "webhooks")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(webhooks).To(HaveLen(2))

	webhook1 := webhooks[0].(map[string]interface{})
	clientConfig1 := webhook1["clientConfig"].(map[string]interface{})
	g.Expect(clientConfig1["url"]).To(Equal(testBaseURL + "/validate1"))
	g.Expect(clientConfig1["caBundle"]).To(Equal(testCABundle))

	webhook2 := webhooks[1].(map[string]interface{})
	clientConfig2 := webhook2["clientConfig"].(map[string]interface{})
	g.Expect(clientConfig2["url"]).To(Equal(testBaseURL + "/validate2"))
	g.Expect(clientConfig2["caBundle"]).To(Equal(testCABundle))
}

func TestPatchWebhookConfiguration_DefaultPath(t *testing.T) {
	g := NewWithT(t)

	webhook := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "ValidatingWebhookConfiguration",
			"metadata": map[string]interface{}{
				"name": "test-default-path",
			},
			"webhooks": []interface{}{
				map[string]interface{}{
					"name": "validate.example.com",
					"clientConfig": map[string]interface{}{
						"service": map[string]interface{}{
							"namespace": "default",
							"name":      "webhook-service",
						},
					},
				},
			},
		},
	}

	err := resources.PatchWebhookConfiguration(webhook, testBaseURL, testCABundle)
	g.Expect(err).NotTo(HaveOccurred())

	webhooks, found, err := unstructured.NestedSlice(webhook.Object, "webhooks")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())

	firstWebhook := webhooks[0].(map[string]interface{})
	clientConfig := firstWebhook["clientConfig"].(map[string]interface{})

	g.Expect(clientConfig["url"]).To(Equal(testBaseURL + "/"))
}

func TestPatchWebhookConfiguration_InvalidObject(t *testing.T) {
	g := NewWithT(t)

	webhook := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "ValidatingWebhookConfiguration",
			"metadata": map[string]interface{}{
				"name": "test-invalid",
			},
		},
	}

	err := resources.PatchWebhookConfiguration(webhook, testBaseURL, testCABundle)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to patch webhook configuration"))
}

func TestPatchCRDConversion_Success(t *testing.T) {
	g := NewWithT(t)

	crd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "examples.test.example.com",
			},
			"spec": map[string]interface{}{
				"group": "test.example.com",
				"names": map[string]interface{}{
					"kind":   "Example",
					"plural": "examples",
				},
				"scope": "Namespaced",
				"versions": []interface{}{
					map[string]interface{}{
						"name":    "v1",
						"served":  true,
						"storage": true,
					},
					map[string]interface{}{
						"name":    "v1beta1",
						"served":  true,
						"storage": false,
					},
				},
			},
		},
	}

	err := resources.PatchCRDConversion(crd, testBaseURL, testCABundle)
	g.Expect(err).NotTo(HaveOccurred())

	conversion, found, err := unstructured.NestedMap(crd.Object, "spec", "conversion")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())

	g.Expect(conversion["strategy"]).To(Equal("Webhook"))

	webhook, found, err := unstructured.NestedMap(crd.Object, "spec", "conversion", "webhook")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())

	reviewVersions, found, err := unstructured.NestedStringSlice(crd.Object, "spec", "conversion", "webhook", "conversionReviewVersions")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(reviewVersions).To(Equal([]string{"v1", "v1beta1"}))

	clientConfig := webhook["clientConfig"].(map[string]interface{})
	g.Expect(clientConfig["url"]).To(Equal(testBaseURL + "/convert"))
	g.Expect(clientConfig["caBundle"]).To(Equal(testCABundle))
}

func TestPatchCRDConversion_InvalidObject(t *testing.T) {
	g := NewWithT(t)

	// Create an object with spec as a non-map type to cause jq transformation to fail
	crd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "invalid",
			},
			"spec": "invalid-not-a-map",
		},
	}

	err := resources.PatchCRDConversion(crd, testBaseURL, testCABundle)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to patch CRD conversion"))
}

func TestPatchWebhookConfiguration_RealWorldExample(t *testing.T) {
	g := NewWithT(t)

	caCert := base64.StdEncoding.EncodeToString([]byte("mock-ca-certificate"))
	baseURL := "https://host.testcontainers.internal:9443"

	webhook := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "ValidatingWebhookConfiguration",
			"metadata": map[string]interface{}{
				"name": "pod-policy-webhook",
			},
			"webhooks": []interface{}{
				map[string]interface{}{
					"name": "validate.pods.policy.io",
					"clientConfig": map[string]interface{}{
						"service": map[string]interface{}{
							"namespace": "webhook-system",
							"name":      "webhook-server",
							"port":      443,
							"path":      "/validate-v1-pod",
						},
					},
					"rules": []interface{}{
						map[string]interface{}{
							"apiGroups":   []interface{}{""},
							"apiVersions": []interface{}{"v1"},
							"operations":  []interface{}{"CREATE", "UPDATE"},
							"resources":   []interface{}{"pods"},
						},
					},
					"admissionReviewVersions": []interface{}{"v1"},
					"sideEffects":             "None",
				},
			},
		},
	}

	err := resources.PatchWebhookConfiguration(webhook, baseURL, caCert)
	g.Expect(err).NotTo(HaveOccurred())

	webhooks, found, err := unstructured.NestedSlice(webhook.Object, "webhooks")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())

	firstWebhook := webhooks[0].(map[string]interface{})
	clientConfig := firstWebhook["clientConfig"].(map[string]interface{})

	g.Expect(clientConfig["url"]).To(Equal(baseURL + "/validate-v1-pod"))
	g.Expect(clientConfig["caBundle"]).To(Equal(caCert))
	g.Expect(clientConfig).NotTo(HaveKey("service"))

	g.Expect(firstWebhook["rules"]).To(HaveLen(1))
	g.Expect(firstWebhook["admissionReviewVersions"]).To(Equal([]interface{}{"v1"}))
}
