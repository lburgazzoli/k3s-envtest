package k3senv_test

import (
	"fmt"
	"testing"

	sigsyaml "sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"

	. "github.com/onsi/gomega"
)

const (
	testBaseURL  = "https://localhost:9443"
	testCABundle = "Y2FCdW5kbGU="
)

const webhookConfigInput = `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: test-webhook-config
webhooks:
  - name: test-webhook.example.com
    clientConfig:
      service:
        name: webhook-service
        namespace: default
        path: /validate
`

const webhookConfigExpected = `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: test-webhook-config
webhooks:
  - name: test-webhook.example.com
    clientConfig:
      url: https://localhost:9443/validate
      caBundle: Y2FCdW5kbGU=
`

const webhookConfigDefaultPathInput = `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: test-webhook-config
webhooks:
  - name: test-webhook.example.com
    clientConfig:
      service:
        name: webhook-service
        namespace: default
`

const webhookConfigDefaultPathExpected = `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: test-webhook-config
webhooks:
  - name: test-webhook.example.com
    clientConfig:
      url: https://localhost:9443/
      caBundle: Y2FCdW5kbGU=
`

const multipleWebhooksInput = `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: test-webhook-config
webhooks:
  - name: webhook-1.example.com
    clientConfig:
      service:
        name: service-1
        path: /validate1
  - name: webhook-2.example.com
    clientConfig:
      service:
        name: service-2
        path: /validate2
`

const multipleWebhooksExpected = `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: test-webhook-config
webhooks:
  - name: webhook-1.example.com
    clientConfig:
      url: https://localhost:9443/validate1
      caBundle: Y2FCdW5kbGU=
  - name: webhook-2.example.com
    clientConfig:
      url: https://localhost:9443/validate2
      caBundle: Y2FCdW5kbGU=
`

const simpleFieldUpdateInput = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  replicas: 1
`

const simpleFieldUpdateExpected = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  replicas: 3
`

func TestApplyJQTransform_WebhookConfiguration(t *testing.T) {
	g := NewWithT(t)

	obj, err := yamlToUnstructured(webhookConfigInput)
	g.Expect(err).ToNot(HaveOccurred())

	err = k3senv.ApplyJQTransform(obj, `
		.webhooks |= map(
			.clientConfig.url = "%s" + (.clientConfig.service.path // "/") |
			.clientConfig.caBundle = "%s" |
			del(.clientConfig.service)
		)
	`, testBaseURL, testCABundle)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(obj).To(WithTransform(toYAML, MatchYAML(webhookConfigExpected)))
}

func TestApplyJQTransform_WebhookWithDefaultPath(t *testing.T) {
	g := NewWithT(t)

	obj, err := yamlToUnstructured(webhookConfigDefaultPathInput)
	g.Expect(err).ToNot(HaveOccurred())

	err = k3senv.ApplyJQTransform(obj, `
		.webhooks |= map(
			.clientConfig.url = "%s" + (.clientConfig.service.path // "/") |
			.clientConfig.caBundle = "%s" |
			del(.clientConfig.service)
		)
	`, testBaseURL, testCABundle)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(obj).To(WithTransform(toYAML, MatchYAML(webhookConfigDefaultPathExpected)))
}

func TestApplyJQTransform_MultipleWebhooks(t *testing.T) {
	g := NewWithT(t)

	obj, err := yamlToUnstructured(multipleWebhooksInput)
	g.Expect(err).ToNot(HaveOccurred())

	err = k3senv.ApplyJQTransform(obj, `
		.webhooks |= map(
			.clientConfig.url = "%s" + (.clientConfig.service.path // "/") |
			.clientConfig.caBundle = "%s" |
			del(.clientConfig.service)
		)
	`, testBaseURL, testCABundle)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(obj).To(WithTransform(toYAML, MatchYAML(multipleWebhooksExpected)))
}

func TestApplyJQTransform_InvalidExpression(t *testing.T) {
	g := NewWithT(t)

	obj, err := yamlToUnstructured("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\ndata:\n  test: value")
	g.Expect(err).ToNot(HaveOccurred())

	err = k3senv.ApplyJQTransform(obj, `invalid jq syntax {{{`)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to parse jq expression"))
}

func TestApplyJQTransform_SimpleFieldUpdate(t *testing.T) {
	g := NewWithT(t)

	obj, err := yamlToUnstructured(simpleFieldUpdateInput)
	g.Expect(err).ToNot(HaveOccurred())

	err = k3senv.ApplyJQTransform(obj, `.spec.replicas = %d`, 3)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(obj).To(WithTransform(toYAML, MatchYAML(simpleFieldUpdateExpected)))
}

func yamlToUnstructured(yamlStr string) (*unstructured.Unstructured, error) {
	var data map[string]interface{}
	if err := sigsyaml.Unmarshal([]byte(yamlStr), &data); err != nil {
		return nil, fmt.Errorf("failure to unmarshal yaml: %w", err)
	}

	return &unstructured.Unstructured{Object: data}, nil
}

func toYAML(obj *unstructured.Unstructured) string {
	yamlBytes, err := sigsyaml.Marshal(obj.Object)
	if err != nil {
		return ""
	}
	return string(yamlBytes)
}
