package k3senv_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/lburgazzoli/k3s-envtest/internal/testdata/v1alpha1"
	"github.com/lburgazzoli/k3s-envtest/internal/testdata/v1beta1"
	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"
	"github.com/testcontainers/testcontainers-go"
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
	. "github.com/onsi/gomega/gstruct"
)

//nolint:gochecknoinits
func init() {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
}

// TestHostGatewayAccess verifies that host.containers.internal:host-gateway
// allows container-to-host communication. This is the mechanism used for
// Podman compatibility instead of testcontainers.WithHostPortAccess().
func TestHostGatewayAccess(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Start a simple HTTP server on the host
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Parse the port from the server URL
	serverURL, err := url.Parse(server.URL)
	g.Expect(err).NotTo(HaveOccurred())
	hostPort := serverURL.Port()

	// Start a container with host-gateway access using CustomizeRequest
	// (not WithHostConfigModifier, which overwrites existing modifiers)
	ctr, err := testcontainers.Run(ctx,
		"curlimages/curl:latest",
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				ExtraHosts: []string{k3senv.DefaultWebhookContainerHost + ":host-gateway"},
			},
		}),
		testcontainers.WithCmd("sleep", "infinity"),
	)
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(ctr)
	})

	// Execute curl from inside the container to reach the host
	hostAddr := net.JoinHostPort(k3senv.DefaultWebhookContainerHost, hostPort)
	code, reader, err := ctr.Exec(ctx, []string{
		"curl", "-sf", "http://" + hostAddr,
	})
	g.Expect(err).NotTo(HaveOccurred())

	body, _ := io.ReadAll(reader)
	g.Expect(code).To(Equal(0), "curl should succeed, got output: %s", string(body))
}

// Test constants.
const (
	testCRDGroup            = "example.k3senv.io"
	testCRDName             = "sampleresources.example.k3senv.io"
	testCRDKind             = "SampleResource"
	testWebhookValidatePath = "/validate"
	testWebhookMutatePath   = "/mutate"
)

// Test helpers.

func setupTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
	g.Expect(v1beta1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

// Test fixtures.

func newTestCRDWithConversion() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: testCRDName,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: testCRDGroup,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     testCRDKind,
				ListKind: testCRDKind + "List",
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
}

func newTestCRDNonConvertible() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
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
}

func newTestValidatingWebhook(name string, path string) *admissionv1.ValidatingWebhookConfiguration {
	failurePolicy := admissionv1.Fail
	sideEffects := admissionv1.SideEffectClassNone

	return &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name: "validate.example.com",
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-service",
						Path:      ptr.To(path),
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
}

func newTestMutatingWebhook(name string, path string) *admissionv1.MutatingWebhookConfiguration {
	failurePolicy := admissionv1.Fail
	sideEffects := admissionv1.SideEffectClassNone

	return &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Webhooks: []admissionv1.MutatingWebhook{
			{
				Name: "mutate.example.com",
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-service",
						Path:      ptr.To(path),
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
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	err = env.Start(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	err = env.InstallWebhooks(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	installedWebhook := webhook.DeepCopyObject().(client.Object)
	err = env.Client().Get(ctx, client.ObjectKey{Name: webhook.GetName()}, installedWebhook)
	g.Expect(err).NotTo(HaveOccurred())

	expectedURL := "https://host.containers.internal:9443" + expectedPath

	// Use type switch to handle both webhook types
	switch wh := installedWebhook.(type) {
	case *admissionv1.ValidatingWebhookConfiguration:
		g.Expect(wh.Webhooks).To(ContainElement(MatchFields(IgnoreExtras, Fields{
			"ClientConfig": MatchFields(IgnoreExtras, Fields{
				"URL":      PointTo(Equal(expectedURL)),
				"CABundle": Not(BeEmpty()),
				"Service":  BeNil(),
			}),
		})))
	case *admissionv1.MutatingWebhookConfiguration:
		g.Expect(wh.Webhooks).To(ContainElement(MatchFields(IgnoreExtras, Fields{
			"ClientConfig": MatchFields(IgnoreExtras, Fields{
				"URL":      PointTo(Equal(expectedURL)),
				"CABundle": Not(BeEmpty()),
				"Service":  BeNil(),
			}),
		})))
	}
}

func TestK3sEnv_GetKubeconfig_Success(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	env, err := k3senv.New(
		k3senv.WithCertPath(t.TempDir()),
	)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	err = env.Start(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	kubeconfigData, err := env.GetKubeconfig(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(kubeconfigData).NotTo(BeEmpty())

	config, err := clientcmd.Load(kubeconfigData)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(config).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Clusters":  Not(BeEmpty()),
		"AuthInfos": Not(BeEmpty()),
		"Contexts":  Not(BeEmpty()),
	})))
}

func TestK3sEnv_GetKubeconfig_BeforeStart(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	env, err := k3senv.New()
	g.Expect(err).NotTo(HaveOccurred())

	_, err = env.GetKubeconfig(ctx)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cluster not started"))
}

func TestK3sEnv_GetKubeconfig_MatchesConfig(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	env, err := k3senv.New(
		k3senv.WithCertPath(t.TempDir()),
	)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	err = env.Start(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	kubeconfigData, err := env.GetKubeconfig(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	config, err := clientcmd.Load(kubeconfigData)
	g.Expect(err).NotTo(HaveOccurred())

	restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	g.Expect(err).NotTo(HaveOccurred())

	envConfig := env.Config()
	g.Expect(restConfig.Host).To(Equal(envConfig.Host))
	g.Expect(restConfig.CAData).To(Equal(envConfig.CAData))
}

func TestInstallWebhooks_ConvertibleCRD_ConfiguresConversionEndpoint(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := setupTestScheme(t)
	crd := newTestCRDWithConversion()

	env, err := k3senv.New(
		k3senv.WithScheme(scheme),
		k3senv.WithObjects(crd),
		k3senv.WithCertPath(t.TempDir()),
	)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	err = env.Start(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	err = env.InstallWebhooks(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
	err = env.Client().Get(ctx, client.ObjectKey{Name: crd.Name}, updatedCRD)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(updatedCRD.Spec.Conversion).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Strategy": Equal(apiextensionsv1.WebhookConverter),
		"Webhook": PointTo(MatchFields(IgnoreExtras, Fields{
			"ClientConfig": PointTo(MatchFields(IgnoreExtras, Fields{
				"URL":      PointTo(ContainSubstring("https://host.containers.internal:9443/convert")),
				"CABundle": Not(BeEmpty()),
			})),
			"ConversionReviewVersions": ContainElement("v1"),
		})),
	})))
}

func TestInstallWebhooks_NonConvertibleCRD_SkipsConversion(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())

	crd := newTestCRDNonConvertible()

	env, err := k3senv.New(
		k3senv.WithScheme(scheme),
		k3senv.WithObjects(crd),
		k3senv.WithCertPath(t.TempDir()),
	)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	err = env.Start(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	err = env.InstallWebhooks(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
	err = env.Client().Get(ctx, client.ObjectKey{Name: crd.Name}, updatedCRD)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(updatedCRD.Spec.Conversion).To(Or(
		BeNil(),
		PointTo(MatchFields(IgnoreExtras, Fields{
			"Strategy": Equal(apiextensionsv1.NoneConverter),
		})),
	))
}

func TestInstallWebhooks_ValidatingWebhook_ConfiguresURLAndCA(t *testing.T) {
	webhook := newTestValidatingWebhook("test-validating-webhook", testWebhookValidatePath)
	testAdmissionWebhookConfiguration(t, webhook, testWebhookValidatePath)
}

func TestInstallWebhooks_MutatingWebhook_ConfiguresURLAndCA(t *testing.T) {
	webhook := newTestMutatingWebhook("test-mutating-webhook", testWebhookMutatePath)
	testAdmissionWebhookConfiguration(t, webhook, testWebhookMutatePath)
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
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	err = env.Start(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	err = env.InstallWebhooks(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	installedWebhook := &admissionv1.ValidatingWebhookConfiguration{}
	err = env.Client().Get(ctx, client.ObjectKey{Name: webhook.Name}, installedWebhook)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(installedWebhook.Webhooks).To(HaveLen(1))
	g.Expect(installedWebhook.Webhooks[0].ClientConfig.URL).To(PointTo(Equal("https://host.containers.internal:9443/")))
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
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	err = env.Start(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	err = env.InstallWebhooks(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	installedWebhook := &admissionv1.ValidatingWebhookConfiguration{}
	err = env.Client().Get(ctx, client.ObjectKey{Name: webhook.Name}, installedWebhook)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(installedWebhook.Webhooks).To(HaveLen(2))
	g.Expect(installedWebhook.Webhooks[0].ClientConfig.URL).To(PointTo(Equal("https://host.containers.internal:9443/validate1")))
	g.Expect(installedWebhook.Webhooks[1].ClientConfig.URL).To(PointTo(Equal("https://host.containers.internal:9443/validate2")))
	g.Expect(installedWebhook.Webhooks[0].ClientConfig.CABundle).NotTo(BeEmpty())
	g.Expect(installedWebhook.Webhooks[1].ClientConfig.CABundle).NotTo(BeEmpty())
	g.Expect(installedWebhook.Webhooks[0].ClientConfig.CABundle).To(Equal(installedWebhook.Webhooks[1].ClientConfig.CABundle))
}

// Validation Tests

func TestNew_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero port", 0},
		{"negative port", -1},
		{"port too high", 70000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := k3senv.New(k3senv.WithWebhookPort(tt.port))
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("webhook port must be 1-65535"))
			g.Expect(err.Error()).To(ContainSubstring("FindAvailablePort"))
		})
	}
}

func TestNew_EmptyImage(t *testing.T) {
	g := NewWithT(t)

	_, err := k3senv.New(k3senv.WithK3sImage(""))
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("k3s image cannot be empty"))
}

func TestNew_NegativeCertValidity(t *testing.T) {
	g := NewWithT(t)

	_, err := k3senv.New(k3senv.WithCertValidity(-1))
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("certificate validity must be positive"))
}

func TestNew_NegativeTimeout(t *testing.T) {
	tests := []struct {
		name   string
		opts   []k3senv.Option
		errMsg string
	}{
		{
			name: "negative webhook ready timeout",
			opts: []k3senv.Option{
				k3senv.WithWebhookReadyTimeout(-1),
			},
			errMsg: "webhook ready timeout must be positive",
		},
		{
			name: "negative webhook health check timeout",
			opts: []k3senv.Option{
				k3senv.WithWebhookHealthCheckTimeout(-1),
			},
			errMsg: "webhook health check timeout must be positive",
		},
		{
			name: "negative CRD ready timeout",
			opts: []k3senv.Option{
				k3senv.WithCRDReadyTimeout(-1),
			},
			errMsg: "CRD ready timeout must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := k3senv.New(tt.opts...)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tt.errMsg))
		})
	}
}

func TestNew_ZeroTimeout(t *testing.T) {
	tests := []struct {
		name   string
		opts   []k3senv.Option
		errMsg string
	}{
		{
			name: "zero webhook ready timeout",
			opts: []k3senv.Option{
				k3senv.WithWebhookReadyTimeout(0),
			},
			errMsg: "webhook ready timeout must be positive",
		},
		{
			name: "zero CRD ready timeout",
			opts: []k3senv.Option{
				k3senv.WithCRDReadyTimeout(0),
			},
			errMsg: "CRD ready timeout must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := k3senv.New(tt.opts...)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tt.errMsg))
		})
	}
}

func TestNew_TooSmallPollInterval(t *testing.T) {
	tests := []struct {
		name   string
		opts   []k3senv.Option
		errMsg string
	}{
		{
			name: "webhook poll interval too small",
			opts: []k3senv.Option{
				k3senv.WithWebhookPollInterval(1),
			},
			errMsg: "webhook poll interval too small",
		},
		{
			name: "CRD poll interval too small",
			opts: []k3senv.Option{
				k3senv.WithCRDPollInterval(1),
			},
			errMsg: "CRD poll interval too small",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := k3senv.New(tt.opts...)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tt.errMsg))
		})
	}
}
