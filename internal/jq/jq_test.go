package jq_test

import (
	"testing"

	"github.com/lburgazzoli/k3s-envtest/internal/jq"
	"github.com/lburgazzoli/k3s-envtest/internal/resources"
	sigsyaml "sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/onsi/gomega"
)

const (
	testBaseURL  = "https://localhost:9443"
	testCABundle = "Y2FCdW5kbGU="
)

// Transform test constants.
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

const invalidConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  test: value
`

// QueryTyped test constants.
//
//nolint:gosec
const configMapWithName = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`

//nolint:gosec
const configMapWithEnabled = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  enabled: true
`

const specWithReplicas = `
spec:
  replicas: 3
`

const configMapTypeMismatch = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
`

const configMapSimple = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`

// QuerySlice test constants.
const webhookConfigWithURLs = `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: test-webhook
webhooks:
  - name: webhook1
    clientConfig:
      url: https://example.com/validate1
  - name: webhook2
    clientConfig:
      url: https://example.com/validate2
`

const specWithPorts = `
spec:
  ports:
    - 8080
    - 9090
    - 443
`

const dataWithNumbers = `
data:
  items:
    - 1
    - 2
    - 3
`

const dataWithEmptyStrings = `
data:
  items:
    - "value1"
    - ""
    - "value2"
`

// QueryMap test constants.
//
//nolint:gosec
const configMapWithLabels = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  labels:
    app: myapp
    env: prod
`

//nolint:gosec
const configMapWithEmptyLabels = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  labels: {}
`

const dataWithPorts = `
data:
  ports:
    http: 8080
    https: 443
`

// Transform tests.
func TestTransform_WebhookConfiguration(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(webhookConfigInput)
	g.Expect(err).ToNot(HaveOccurred())

	err = jq.Transform(obj, `
		.webhooks |= map(
			.clientConfig.url = "%s" + (.clientConfig.service.path // "/") |
			.clientConfig.caBundle = "%s" |
			del(.clientConfig.service)
		)
	`, testBaseURL, testCABundle)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(obj).To(WithTransform(toYAML, MatchYAML(webhookConfigExpected)))
}

func TestTransform_WebhookWithDefaultPath(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(webhookConfigDefaultPathInput)
	g.Expect(err).ToNot(HaveOccurred())

	err = jq.Transform(obj, `
		.webhooks |= map(
			.clientConfig.url = "%s" + (.clientConfig.service.path // "/") |
			.clientConfig.caBundle = "%s" |
			del(.clientConfig.service)
		)
	`, testBaseURL, testCABundle)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(obj).To(WithTransform(toYAML, MatchYAML(webhookConfigDefaultPathExpected)))
}

func TestTransform_MultipleWebhooks(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(multipleWebhooksInput)
	g.Expect(err).ToNot(HaveOccurred())

	err = jq.Transform(obj, `
		.webhooks |= map(
			.clientConfig.url = "%s" + (.clientConfig.service.path // "/") |
			.clientConfig.caBundle = "%s" |
			del(.clientConfig.service)
		)
	`, testBaseURL, testCABundle)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(obj).To(WithTransform(toYAML, MatchYAML(multipleWebhooksExpected)))
}

func TestTransform_InvalidExpression(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(invalidConfigMap)
	g.Expect(err).ToNot(HaveOccurred())

	err = jq.Transform(obj, `invalid jq syntax {{{`)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to parse jq expression"))
}

func TestTransform_SimpleFieldUpdate(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(simpleFieldUpdateInput)
	g.Expect(err).ToNot(HaveOccurred())

	err = jq.Transform(obj, `.spec.replicas = %d`, 3)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(obj).To(WithTransform(toYAML, MatchYAML(simpleFieldUpdateExpected)))
}

// QueryTyped tests.
func TestQueryTyped_String(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(configMapWithName)
	g.Expect(err).NotTo(HaveOccurred())

	result, err := jq.QueryTyped[string](obj, `.metadata.name`)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(Equal("test-config"))
}

func TestQueryTyped_Bool(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(configMapWithEnabled)
	g.Expect(err).NotTo(HaveOccurred())

	result, err := jq.QueryTyped[bool](obj, `.data.enabled`)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(BeTrue())
}

func TestQueryTyped_Number(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(specWithReplicas)
	g.Expect(err).NotTo(HaveOccurred())

	result, err := jq.QueryTyped[int](obj, `.spec.replicas`)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(Equal(3))
}

func TestQueryTyped_TypeMismatch(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(configMapTypeMismatch)
	g.Expect(err).NotTo(HaveOccurred())

	_, err = jq.QueryTyped[int](obj, `.metadata.name`)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("expected type int"))
}

func TestQueryTyped_NilResult(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(configMapSimple)
	g.Expect(err).NotTo(HaveOccurred())

	result, err := jq.QueryTyped[string](obj, `.nonexistent`)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(BeZero())
}

func TestQueryTyped_InvalidExpression(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(configMapSimple)
	g.Expect(err).NotTo(HaveOccurred())

	_, err = jq.QueryTyped[string](obj, `invalid {{{`)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to parse jq expression"))
}

// QuerySlice tests.
func TestQuerySlice_Strings(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(webhookConfigWithURLs)
	g.Expect(err).NotTo(HaveOccurred())

	urls, err := jq.QuerySlice[string](obj, `[.webhooks[].clientConfig.url]`)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(urls).To(Equal([]string{
		"https://example.com/validate1",
		"https://example.com/validate2",
	}))
}

func TestQuerySlice_Numbers(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(specWithPorts)
	g.Expect(err).NotTo(HaveOccurred())

	ports, err := jq.QuerySlice[int](obj, `.spec.ports`)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ports).To(Equal([]int{8080, 9090, 443}))
}

func TestQuerySlice_EmptyResult(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(configMapSimple)
	g.Expect(err).NotTo(HaveOccurred())

	result, err := jq.QuerySlice[string](obj, `[.webhooks[]?]`)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(BeEmpty())
}

func TestQuerySlice_NilResult(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(configMapSimple)
	g.Expect(err).NotTo(HaveOccurred())

	result, err := jq.QuerySlice[string](obj, `.nonexistent`)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(BeNil())
}

func TestQuerySlice_TypeMismatch(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(dataWithNumbers)
	g.Expect(err).NotTo(HaveOccurred())

	_, err = jq.QuerySlice[string](obj, `.data.items`)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("expected type string"))
}

func TestQuerySlice_NotAnArray(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(configMapSimple)
	g.Expect(err).NotTo(HaveOccurred())

	_, err = jq.QuerySlice[string](obj, `.metadata`)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("expected array result"))
}

func TestQuerySlice_WithEmptyStrings(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(dataWithEmptyStrings)
	g.Expect(err).NotTo(HaveOccurred())

	items, err := jq.QuerySlice[string](obj, `.data.items`)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(items).To(Equal([]string{"value1", "", "value2"}))
}

// QueryMap tests.
func TestQueryMap_StringString(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(configMapWithLabels)
	g.Expect(err).NotTo(HaveOccurred())

	labels, err := jq.QueryMap[string, string](obj, `.metadata.labels`)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(labels).To(Equal(map[string]string{
		"app": "myapp",
		"env": "prod",
	}))
}

func TestQueryMap_EmptyMap(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(configMapWithEmptyLabels)
	g.Expect(err).NotTo(HaveOccurred())

	labels, err := jq.QueryMap[string, string](obj, `.metadata.labels`)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(labels).To(BeEmpty())
}

func TestQueryMap_NilResult(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(configMapSimple)
	g.Expect(err).NotTo(HaveOccurred())

	result, err := jq.QueryMap[string, string](obj, `.nonexistent`)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(BeNil())
}

func TestQueryMap_NotAMap(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(configMapSimple)
	g.Expect(err).NotTo(HaveOccurred())

	_, err = jq.QueryMap[string, string](obj, `.metadata.name`)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("expected map result"))
}

func TestQueryMap_ValueTypeMismatch(t *testing.T) {
	g := NewWithT(t)

	obj, err := resources.YAMLToUnstructured(dataWithPorts)
	g.Expect(err).NotTo(HaveOccurred())

	_, err = jq.QueryMap[string, string](obj, `.data.ports`)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("expected value type string"))
}

// Helper function for test assertions.
func toYAML(obj *unstructured.Unstructured) string {
	yamlBytes, err := sigsyaml.Marshal(obj.Object)
	if err != nil {
		return ""
	}
	return string(yamlBytes)
}
