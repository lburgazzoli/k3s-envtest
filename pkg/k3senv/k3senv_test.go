package k3senv_test

import (
	"context"
	"testing"

	"github.com/lburgazzoli/k3s-envtest/internal/jq"
	"github.com/lburgazzoli/k3s-envtest/internal/resources"
	"github.com/lburgazzoli/k3s-envtest/internal/testdata/v1alpha1"
	"github.com/lburgazzoli/k3s-envtest/internal/testdata/v1beta1"
	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"

	. "github.com/onsi/gomega"
)

//nolint:gochecknoinits
func init() {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
}

func testAdmissionWebhookConfiguration(
	t *testing.T,
	webhook client.Object,
	expectedPath string,
) {
	t.Helper()
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	err := admissionv1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	env, err := k3senv.New(
		k3senv.WithScheme(scheme),
		k3senv.WithObjects(webhook),
		k3senv.WithCertPath(t.TempDir()),
		k3senv.WithWebhookCheckReadiness(false),
	)
	g.Expect(err).NotTo(HaveOccurred())

	err = env.Start(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	err = env.InstallWebhooks(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	installedWebhook := webhook.DeepCopyObject().(client.Object)
	err = env.Client().Get(ctx, client.ObjectKey{Name: webhook.GetName()}, installedWebhook)
	g.Expect(err).NotTo(HaveOccurred())

	unstructuredWebhook, err := resources.ToUnstructured(installedWebhook)
	g.Expect(err).NotTo(HaveOccurred())

	url, err := jq.QueryTyped[string](
		unstructuredWebhook,
		`.webhooks[0].clientConfig.url`,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(url).To(Equal("https://host.testcontainers.internal:9443" + expectedPath))

	caBundle, err := jq.QueryTyped[string](
		unstructuredWebhook,
		`.webhooks[0].clientConfig.caBundle`,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(caBundle).NotTo(BeEmpty())

	service, err := jq.Query(
		unstructuredWebhook,
		`.webhooks[0].clientConfig.service`,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(service).To(BeNil())
}

func TestK3sEnv_GetKubeconfig_Success(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	env, err := k3senv.New(k3senv.WithCertPath(t.TempDir()))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = env.Start(ctx)
	g.Expect(err).ShouldNot(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	kubeconfigData, err := env.GetKubeconfig(ctx)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(kubeconfigData).ToNot(BeEmpty())

	config, err := clientcmd.Load(kubeconfigData)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(config).ToNot(BeNil())
	g.Expect(config.Clusters).ToNot(BeEmpty())
	g.Expect(config.AuthInfos).ToNot(BeEmpty())
	g.Expect(config.Contexts).ToNot(BeEmpty())
}

func TestK3sEnv_GetKubeconfig_BeforeStart(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	env, err := k3senv.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	_, err = env.GetKubeconfig(ctx)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cluster not started"))
}

func TestK3sEnv_GetKubeconfig_MatchesConfig(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	env, err := k3senv.New(k3senv.WithCertPath(t.TempDir()))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = env.Start(ctx)
	g.Expect(err).ShouldNot(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	kubeconfigData, err := env.GetKubeconfig(ctx)
	g.Expect(err).ShouldNot(HaveOccurred())

	config, err := clientcmd.Load(kubeconfigData)
	g.Expect(err).ShouldNot(HaveOccurred())

	restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	g.Expect(err).ShouldNot(HaveOccurred())

	envConfig := env.Config()
	g.Expect(restConfig.Host).To(Equal(envConfig.Host))
	g.Expect(restConfig.CAData).To(Equal(envConfig.CAData))
}

func TestInstallWebhooks_ConvertibleCRD_ConfiguresConversionEndpoint(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()

	g.Expect(apiextensionsv1.AddToScheme(scheme)).NotTo(HaveOccurred())
	g.Expect(v1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sampleresources.example.k3senv.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "example.k3senv.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     "SampleResource",
				ListKind: "SampleResourceList",
				Plural:   "sampleresources",
				Singular: "sampleresource",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: false,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"spec": {
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"fieldAlpha": {Type: "string"},
									},
								},
								"status": {
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"conditions": {
											Type: "array",
											Items: &apiextensionsv1.JSONSchemaPropsOrArray{
												Schema: &apiextensionsv1.JSONSchemaProps{Type: "object"},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					Name:    "v1beta1",
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"spec": {
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"fieldBeta": {Type: "string"},
									},
								},
								"status": {
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"conditions": {
											Type: "array",
											Items: &apiextensionsv1.JSONSchemaPropsOrArray{
												Schema: &apiextensionsv1.JSONSchemaProps{Type: "object"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	env, err := k3senv.New(
		k3senv.WithScheme(scheme),
		k3senv.WithObjects(crd),
		k3senv.WithCertPath(t.TempDir()),
	)
	g.Expect(err).NotTo(HaveOccurred())

	err = env.Start(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	err = env.InstallWebhooks(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
	err = env.Client().Get(ctx, client.ObjectKey{Name: crd.Name}, updatedCRD)
	g.Expect(err).NotTo(HaveOccurred())

	unstructuredCRD, err := resources.ToUnstructured(updatedCRD)
	g.Expect(err).NotTo(HaveOccurred())

	strategy, err := jq.QueryTyped[string](
		unstructuredCRD,
		`.spec.conversion.strategy`,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(strategy).To(Equal("Webhook"))

	url, err := jq.QueryTyped[string](
		unstructuredCRD,
		`.spec.conversion.webhook.clientConfig.url`,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(url).To(ContainSubstring("https://host.testcontainers.internal:9443/convert"))

	caBundle, err := jq.QueryTyped[string](
		unstructuredCRD,
		`.spec.conversion.webhook.clientConfig.caBundle`,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(caBundle).NotTo(BeEmpty())

	versions, err := jq.QuerySlice[string](
		unstructuredCRD,
		`.spec.conversion.webhook.conversionReviewVersions`,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(versions).To(ContainElement("v1"))
}

func TestInstallWebhooks_NonConvertibleCRD_SkipsConversion(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	err := apiextensionsv1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nonconvertibles.example.com",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "example.com",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     "NonConvertible",
				ListKind: "NonConvertibleList",
				Plural:   "nonconvertibles",
				Singular: "nonconvertible",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
						},
					},
				},
			},
		},
	}

	env, err := k3senv.New(
		k3senv.WithScheme(scheme),
		k3senv.WithObjects(crd),
		k3senv.WithCertPath(t.TempDir()),
	)
	g.Expect(err).NotTo(HaveOccurred())

	err = env.Start(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	err = env.InstallWebhooks(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
	err = env.Client().Get(ctx, client.ObjectKey{Name: crd.Name}, updatedCRD)
	g.Expect(err).NotTo(HaveOccurred())

	unstructuredCRD, err := resources.ToUnstructured(updatedCRD)
	g.Expect(err).NotTo(HaveOccurred())

	strategy, err := jq.QueryTyped[string](
		unstructuredCRD,
		`.spec.conversion.strategy`,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(strategy).To(Or(BeEmpty(), Equal("None")))
}

func TestInstallWebhooks_ValidatingWebhook_ConfiguresURLAndCA(t *testing.T) {
	failurePolicy := admissionv1.Fail
	sideEffects := admissionv1.SideEffectClassNone

	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-validating-webhook",
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name: "validate.example.com",
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-service",
						Path:      ptr.To("/validate"),
					},
				},
				Rules: []admissionv1.RuleWithOperations{
					{
						Operations: []admissionv1.OperationType{admissionv1.Create},
						Rule: admissionv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				},
				FailurePolicy:           &failurePolicy,
				SideEffects:             &sideEffects,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	testAdmissionWebhookConfiguration(t, webhook, "/validate")
}

func TestInstallWebhooks_MutatingWebhook_ConfiguresURLAndCA(t *testing.T) {
	failurePolicy := admissionv1.Fail
	sideEffects := admissionv1.SideEffectClassNone

	webhook := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mutating-webhook",
		},
		Webhooks: []admissionv1.MutatingWebhook{
			{
				Name: "mutate.example.com",
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-service",
						Path:      ptr.To("/mutate"),
					},
				},
				Rules: []admissionv1.RuleWithOperations{
					{
						Operations: []admissionv1.OperationType{admissionv1.Create},
						Rule: admissionv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				},
				FailurePolicy:           &failurePolicy,
				SideEffects:             &sideEffects,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	testAdmissionWebhookConfiguration(t, webhook, "/mutate")
}

func TestInstallWebhooks_WebhookWithDefaultPath_UsesSlash(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	err := admissionv1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	failurePolicy := admissionv1.Fail
	sideEffects := admissionv1.SideEffectClassNone

	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-default-path-webhook",
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name: "validate-default.example.com",
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-service",
					},
				},
				Rules: []admissionv1.RuleWithOperations{
					{
						Operations: []admissionv1.OperationType{admissionv1.Create},
						Rule: admissionv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				},
				FailurePolicy:           &failurePolicy,
				SideEffects:             &sideEffects,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	env, err := k3senv.New(
		k3senv.WithScheme(scheme),
		k3senv.WithObjects(webhook),
		k3senv.WithCertPath(t.TempDir()),
		k3senv.WithWebhookCheckReadiness(false),
	)
	g.Expect(err).NotTo(HaveOccurred())

	err = env.Start(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	err = env.InstallWebhooks(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	installedWebhook := &admissionv1.ValidatingWebhookConfiguration{}
	err = env.Client().Get(ctx, client.ObjectKey{Name: webhook.Name}, installedWebhook)
	g.Expect(err).NotTo(HaveOccurred())

	unstructuredWebhook, err := resources.ToUnstructured(installedWebhook)
	g.Expect(err).NotTo(HaveOccurred())

	url, err := jq.QueryTyped[string](
		unstructuredWebhook,
		`.webhooks[0].clientConfig.url`,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(url).To(Equal("https://host.testcontainers.internal:9443/"))
}

func TestInstallWebhooks_MultipleWebhooks_ConfiguresAll(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	err := admissionv1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	failurePolicy := admissionv1.Fail
	sideEffects := admissionv1.SideEffectClassNone

	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-multiple-webhooks",
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name: "validate-1.example.com",
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-service-1",
						Path:      ptr.To("/validate1"),
					},
				},
				Rules: []admissionv1.RuleWithOperations{
					{
						Operations: []admissionv1.OperationType{admissionv1.Create},
						Rule: admissionv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				},
				FailurePolicy:           &failurePolicy,
				SideEffects:             &sideEffects,
				AdmissionReviewVersions: []string{"v1"},
			},
			{
				Name: "validate-2.example.com",
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-service-2",
						Path:      ptr.To("/validate2"),
					},
				},
				Rules: []admissionv1.RuleWithOperations{
					{
						Operations: []admissionv1.OperationType{admissionv1.Update},
						Rule: admissionv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps"},
						},
					},
				},
				FailurePolicy:           &failurePolicy,
				SideEffects:             &sideEffects,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	env, err := k3senv.New(
		k3senv.WithScheme(scheme),
		k3senv.WithObjects(webhook),
		k3senv.WithCertPath(t.TempDir()),
		k3senv.WithWebhookCheckReadiness(false),
	)
	g.Expect(err).NotTo(HaveOccurred())

	err = env.Start(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	err = env.InstallWebhooks(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	installedWebhook := &admissionv1.ValidatingWebhookConfiguration{}
	err = env.Client().Get(ctx, client.ObjectKey{Name: webhook.Name}, installedWebhook)
	g.Expect(err).NotTo(HaveOccurred())

	unstructuredWebhook, err := resources.ToUnstructured(installedWebhook)
	g.Expect(err).NotTo(HaveOccurred())

	urls, err := jq.QuerySlice[string](
		unstructuredWebhook,
		`[.webhooks[].clientConfig.url]`,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(urls).To(HaveLen(2))
	g.Expect(urls[0]).To(Equal("https://host.testcontainers.internal:9443/validate1"))
	g.Expect(urls[1]).To(Equal("https://host.testcontainers.internal:9443/validate2"))

	caBundles, err := jq.QuerySlice[string](
		unstructuredWebhook,
		`[.webhooks[].clientConfig.caBundle]`,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(caBundles).To(HaveLen(2))
	g.Expect(caBundles[0]).NotTo(BeEmpty())
	g.Expect(caBundles[1]).NotTo(BeEmpty())
	g.Expect(caBundles[0]).To(Equal(caBundles[1]))
}
