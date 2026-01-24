package k3senv

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/lburgazzoli/k3s-envtest/internal/cert"
	"github.com/lburgazzoli/k3s-envtest/internal/gvk"
	"github.com/lburgazzoli/k3s-envtest/internal/resources"
	"github.com/lburgazzoli/k3s-envtest/internal/resources/filter"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	"github.com/testcontainers/testcontainers-go/network"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
)

const (
	// DefaultWebhookServerHost is the default host address for the webhook server.
	// It binds to all network interfaces (0.0.0.0) to allow connections from containers.
	DefaultWebhookServerHost = "0.0.0.0"

	// DefaultWebhookContainerHost is the hostname containers use to reach the host machine.
	// This is mapped to host-gateway in the container's /etc/hosts, enabling container-to-host
	// communication on both Docker and Podman (4.1+).
	DefaultWebhookContainerHost = "host.containers.internal"

	// WebhookURLScheme is the URL scheme used for webhook endpoints.
	// Always HTTPS since webhooks require TLS certificates.
	WebhookURLScheme = "https"

	// WebhookConvertPath is the default path for CRD conversion webhook endpoints.
	WebhookConvertPath = "/convert"
)

var (
	// CertificateSANs contains the Subject Alternative Names (SANs) used when
	// generating TLS certificates for webhook testing. This list includes common
	// Docker networking hostnames and IP addresses to ensure webhooks can connect
	// across different container networking configurations.
	CertificateSANs = []string{
		"host.containers.internal", // Primary: works on both Docker and Podman
		"host.docker.internal",
		"host.testcontainers.internal",
		"localhost",
		"*.*.svc",
		"*.*.svc.cluster.local",
		"127.0.0.1",
		"172.17.0.1",
		"172.18.0.1",
		"172.19.0.1",
		"172.20.0.1",
	}
)

type TeardownTask func(context.Context) error

// CertificatePaths contains the file paths for all TLS certificates used by k3s-envtest.
type CertificatePaths struct {
	Dir     string // Base certificate directory
	CAFile  string // Full path to CA certificate (cert-ca.pem)
	TLSCert string // Full path to TLS certificate (cert-tls.pem)
	TLSKey  string // Full path to TLS key (key-tls.pem)
}

// Manifests contains typed Kubernetes resources loaded from manifest files.
type Manifests struct {
	CustomResourceDefinitions       []apiextensionsv1.CustomResourceDefinition
	MutatingWebhookConfigurations   []admissionregistrationv1.MutatingWebhookConfiguration
	ValidatingWebhookConfigurations []admissionregistrationv1.ValidatingWebhookConfiguration
}

type K3sEnv struct {
	container *k3s.K3sContainer
	cfg       *rest.Config
	cli       client.Client

	options Options

	certData      *cert.Data
	manifests     Manifests
	teardownTasks []TeardownTask
}

func New(opts ...Option) (*K3sEnv, error) {
	options, err := LoadConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Apply explicit options (these override env vars)
	options.ApplyOptions(opts)

	// Validate all configuration
	if err := options.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	if options.Scheme == nil {
		options.Scheme = runtime.NewScheme()
	}

	env := &K3sEnv{
		options:       *options,
		teardownTasks: []TeardownTask{},
	}

	return env, nil
}

// Start initializes and starts the k3s environment. It performs the following operations:
// - Starts k3s container using testcontainers-go
// - Configures kubeconfig for cluster access
// - Creates Kubernetes clients
// - Generates TLS certificates for webhook testing
// - Loads and installs CRDs (waits for them to be established)
// - Optionally installs webhooks if AutoInstall is enabled
//
// IMPORTANT: Always register cleanup immediately after New() to ensure proper resource cleanup:
//
//	env, err := k3senv.New(...)
//	if err != nil {
//	    return err
//	}
//	t.Cleanup(func() {
//	    _ = env.Stop(ctx)
//	})
//
//	err = env.Start(ctx)
//	if err != nil {
//	    return err  // Stop() will clean up partial resources
//	}
//
// Or using defer:
//
//	env, err := k3senv.New(...)
//	if err != nil {
//	    return err
//	}
//	defer func() {
//	    _ = env.Stop(ctx)
//	}()
//
//	err = env.Start(ctx)
//	if err != nil {
//	    return err  // Stop() will clean up partial resources
//	}
//
// The Stop() method is safe to call even if Start() fails partway through,
// as it handles nil/uninitialized fields gracefully.
func (e *K3sEnv) Start(ctx context.Context) error {
	// Configure testcontainers global logger based on user preferences.
	// WARNING: This modifies global state and affects all testcontainers in this process.
	e.configureTestcontainersLogger()

	e.debugf("Starting k3s environment with image: %s", e.options.K3s.Image)
	if len(e.options.K3s.Args) > 0 {
		e.debugf("Using custom k3s arguments: %v", e.options.K3s.Args)
	}

	if err := e.startK3sContainer(ctx); err != nil {
		return err
	}

	if err := e.setupKubeConfig(ctx); err != nil {
		return err
	}
	e.debugf("Successfully configured k3s cluster")

	if err := e.createKubernetesClients(); err != nil {
		return err
	}

	if err := e.setupCertificates(); err != nil {
		return err
	}
	e.debugf("Generated certificates in: %s", e.options.Certificate.Path)

	if err := e.prepareManifests(); err != nil {
		return err
	}
	totalManifests := len(e.manifests.CustomResourceDefinitions) + len(e.manifests.MutatingWebhookConfigurations) + len(e.manifests.ValidatingWebhookConfigurations)
	e.debugf("Loaded %d manifests", totalManifests)

	if err := e.installCRDs(ctx); err != nil {
		return err
	}

	if ptr.Deref(e.options.Webhook.AutoInstall, false) {
		e.debugf("Installing webhooks automatically")
		if err := e.InstallWebhooks(ctx); err != nil {
			return fmt.Errorf("failed to auto-install webhooks: %w", err)
		}
	}

	e.debugf("k3s environment started successfully")
	return nil
}

func (e *K3sEnv) Stop(ctx context.Context) error {
	e.debugf("Stopping k3s environment")
	var errs []error

	for i := len(e.teardownTasks) - 1; i >= 0; i-- {
		if err := e.teardownTasks[i](ctx); err != nil {
			errs = append(errs, fmt.Errorf("teardown task %d failed: %w", i, err))
		}
	}

	if e.container != nil {
		if err := testcontainers.TerminateContainer(e.container); err != nil {
			errs = append(errs, fmt.Errorf("failed to terminate container: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (e *K3sEnv) AddTeardown(task TeardownTask) {
	e.teardownTasks = append(e.teardownTasks, task)
}

func (e *K3sEnv) Config() *rest.Config {
	return e.cfg
}

func (e *K3sEnv) Client() client.Client {
	return e.cli
}

func (e *K3sEnv) Scheme() *runtime.Scheme {
	return e.options.Scheme
}

func (e *K3sEnv) CertPath() string {
	return e.options.Certificate.Path
}

func (e *K3sEnv) CABundle() []byte {
	if e.certData == nil {
		return nil
	}

	return e.certData.CABundle()
}

func (e *K3sEnv) ContainerID() string {
	if e.container == nil {
		return ""
	}
	return e.container.GetContainerID()
}

func (e *K3sEnv) CertificatePaths() CertificatePaths {
	return CertificatePaths{
		Dir:     e.options.Certificate.Path,
		CAFile:  filepath.Join(e.options.Certificate.Path, cert.CACertFileName),
		TLSCert: filepath.Join(e.options.Certificate.Path, cert.CertFileName),
		TLSKey:  filepath.Join(e.options.Certificate.Path, cert.KeyFileName),
	}
}

func (e *K3sEnv) GetKubeconfig(ctx context.Context) ([]byte, error) {
	if e.container == nil {
		return nil, errors.New("cluster not started - call Start() first")
	}

	kc, err := e.container.GetKubeConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	return kc, nil
}

// CustomResourceDefinitions returns a deep copy of all CustomResourceDefinitions loaded from the provided manifests.
//
// Note: This method creates deep copies to prevent external modification of internal state.
// If calling this method multiple times (e.g., in a loop), consider caching the result
// to avoid repeated copying overhead.
func (e *K3sEnv) CustomResourceDefinitions() []apiextensionsv1.CustomResourceDefinition {
	result := make([]apiextensionsv1.CustomResourceDefinition, len(e.manifests.CustomResourceDefinitions))
	for i := range e.manifests.CustomResourceDefinitions {
		result[i] = *e.manifests.CustomResourceDefinitions[i].DeepCopy()
	}
	return result
}

// MutatingWebhookConfigurations returns a deep copy of all MutatingWebhookConfigurations loaded from the provided manifests.
//
// Note: This method creates deep copies to prevent external modification of internal state.
// If calling this method multiple times (e.g., in a loop), consider caching the result
// to avoid repeated copying overhead.
func (e *K3sEnv) MutatingWebhookConfigurations() []admissionregistrationv1.MutatingWebhookConfiguration {
	result := make([]admissionregistrationv1.MutatingWebhookConfiguration, len(e.manifests.MutatingWebhookConfigurations))
	for i := range e.manifests.MutatingWebhookConfigurations {
		result[i] = *e.manifests.MutatingWebhookConfigurations[i].DeepCopy()
	}
	return result
}

// ValidatingWebhookConfigurations returns a deep copy of all ValidatingWebhookConfigurations loaded from the provided manifests.
//
// Note: This method creates deep copies to prevent external modification of internal state.
// If calling this method multiple times (e.g., in a loop), consider caching the result
// to avoid repeated copying overhead.
func (e *K3sEnv) ValidatingWebhookConfigurations() []admissionregistrationv1.ValidatingWebhookConfiguration {
	result := make([]admissionregistrationv1.ValidatingWebhookConfiguration, len(e.manifests.ValidatingWebhookConfigurations))
	for i := range e.manifests.ValidatingWebhookConfigurations {
		result[i] = *e.manifests.ValidatingWebhookConfigurations[i].DeepCopy()
	}
	return result
}

func (e *K3sEnv) WebhookHost() string {
	return net.JoinHostPort(DefaultWebhookContainerHost, strconv.Itoa(e.options.Webhook.Port))
}

func (e *K3sEnv) WebhookServer() ctrlwebhook.Server {
	return ctrlwebhook.NewServer(ctrlwebhook.Options{
		Port:     e.options.Webhook.Port,
		Host:     DefaultWebhookServerHost,
		CertDir:  e.options.Certificate.Path,
		CertName: cert.CertFileName,
		KeyName:  cert.KeyFileName,
		TLSOpts: []func(*tls.Config){
			func(config *tls.Config) {
				config.MinVersion = tls.VersionTLS12
			},
		},
	})
}

func (e *K3sEnv) InstallWebhooks(ctx context.Context) error {
	webhookHostPort := e.WebhookHost()

	e.debugf("Installing webhooks with host: %s", webhookHostPort)

	if err := e.installWebhooks(ctx, webhookHostPort); err != nil {
		return fmt.Errorf("failed to install webhook configurations: %w", err)
	}

	crds, err := resources.FilterConvertibleCRDs(e.options.Scheme, e.CustomResourceDefinitions())
	if err != nil {
		return fmt.Errorf("failed to determine convertible CRDs: %w", err)
	}

	if len(crds) > 0 {
		if err := e.patchAndUpdateCRDConversions(ctx, crds, webhookHostPort); err != nil {
			return fmt.Errorf("failed to patch and update CRD conversions: %w", err)
		}
	}

	return nil
}

func (e *K3sEnv) InstallCRD(
	ctx context.Context,
	crd *apiextensionsv1.CustomResourceDefinition,
) error {
	e.debugf("Installing CRD %s", crd.GetName())

	if err := resources.EnsureGroupVersionKind(e.options.Scheme, crd); err != nil {
		return fmt.Errorf("failed to set GVK for CRD %s: %w", crd.GetName(), err)
	}

	// Convert to unstructured for apply configuration
	unstructuredCRD, err := resources.ToUnstructured(crd)
	if err != nil {
		return fmt.Errorf("failed to convert CRD %s to unstructured: %w", crd.GetName(), err)
	}

	applyConfig := client.ApplyConfigurationFromUnstructured(unstructuredCRD)
	err = e.cli.Apply(ctx, applyConfig, client.ForceOwnership, client.FieldOwner("k3s-envtest"))
	if err != nil {
		return fmt.Errorf("failed to apply CRD %s: %w", crd.GetName(), err)
	}

	e.debugf("Waiting for CRD %s to be established...", crd.GetName())

	err = resources.WaitForCRDEstablished(
		ctx,
		e.cli,
		crd.GetName(),
		e.options.CRD.PollInterval,
		e.options.CRD.ReadyTimeout,
	)
	if err != nil {
		return fmt.Errorf("failed to wait for CRD to be established: %w", err)
	}

	e.debugf("CRD %s is now active", crd.GetName())

	return nil
}

func (e *K3sEnv) startK3sContainer(ctx context.Context) error {
	opts := []testcontainers.ContainerCustomizer{
		withHostAccess(),
	}

	// Apply network configuration if specified
	if e.options.K3s.Network != nil {
		if e.options.K3s.Network.Name != "" {
			aliases := e.options.K3s.Network.Aliases
			if aliases == nil {
				aliases = []string{}
			}
			e.debugf("Using custom Docker network: %s with aliases: %v", e.options.K3s.Network.Name, aliases)
			opts = append(opts, network.WithNetworkName(aliases, e.options.K3s.Network.Name))
		} else if len(e.options.K3s.Network.Aliases) > 0 {
			e.debugf("Setting network aliases: %v", e.options.K3s.Network.Aliases)
		}

		if e.options.K3s.Network.Mode != "" {
			e.debugf("Using network mode: %s", e.options.K3s.Network.Mode)
			opts = append(opts, withNetworkMode(e.options.K3s.Network.Mode))
		}
	}

	// If custom k3s arguments are provided, modify the container command
	if len(e.options.K3s.Args) > 0 {
		cmd := make([]string, 0, 1+len(e.options.K3s.Args))
		cmd = append(cmd, "server")
		cmd = append(cmd, e.options.K3s.Args...)

		opts = append(opts, testcontainers.WithCmd(cmd...))
	}

	// Add log consumer to forward container logs to k3senv Logger
	if ptr.Deref(e.options.K3s.LogRedirection, false) && e.options.Logger != nil {
		opts = append(opts, testcontainers.WithLogConsumers(&loggerConsumer{
			logger: e.options.Logger,
		}))
	}

	container, err := k3s.Run(ctx, e.options.K3s.Image, opts...)
	if err != nil {
		return fmt.Errorf("failed to start k3s container with image %s: %w", e.options.K3s.Image, err)
	}
	e.container = container
	return nil
}

// withHostAccess enables container -> host communication by adding
// host.containers.internal to the container's /etc/hosts, mapped to host-gateway.
// This works on both Docker and Podman (4.1+).
//
// Note: We use CustomizeRequest instead of WithHostConfigModifier because
// WithHostConfigModifier replaces any existing modifier (like k3s's privileged setting).
// CustomizeRequest merges the ExtraHosts slice properly.
func withHostAccess() testcontainers.ContainerCustomizer {
	return testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			ExtraHosts: []string{DefaultWebhookContainerHost + ":host-gateway"},
		},
	})
}

// withNetworkMode creates a customizer that sets the container's network mode.
func withNetworkMode(mode string) testcontainers.ContainerCustomizer {
	return testcontainers.CustomizeRequestOption(func(req *testcontainers.GenericContainerRequest) error {
		req.NetworkMode = dockercontainer.NetworkMode(mode)
		return nil
	})
}

func (e *K3sEnv) setupKubeConfig(ctx context.Context) error {
	kubeconfig, err := e.container.GetKubeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig from container %s: %w", e.container.GetContainerID(), err)
	}

	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to create REST config from kubeconfig: %w", err)
	}
	e.cfg = cfg
	return nil
}

func (e *K3sEnv) createKubernetesClients() error {
	cli, err := client.New(e.cfg, client.Options{Scheme: e.options.Scheme})
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client with scheme: %w", err)
	}

	e.cli = cli

	return nil
}

func (e *K3sEnv) setupCertificates() error {
	if e.options.Certificate.Path == "" {
		cd := fmt.Sprintf("%s%s", DefaultCertDirPrefix, e.container.GetContainerID())

		e.AddTeardown(func(ctx context.Context) error {
			return os.RemoveAll(cd)
		})

		e.options.Certificate.Path = cd
	}

	certData, err := cert.New(e.options.Certificate.Path, e.options.Certificate.Validity, CertificateSANs)
	if err != nil {
		return fmt.Errorf("failed to generate certificates in path %s: %w", e.options.Certificate.Path, err)
	}

	e.certData = certData

	return nil
}

func (e *K3sEnv) prepareManifests() error {
	e.manifests = Manifests{}

	// Define the filter for CRDs and webhook configurations
	manifestFilter := filter.ByType(
		gvk.CustomResourceDefinition,
		gvk.MutatingWebhookConfiguration,
		gvk.ValidatingWebhookConfiguration,
	)

	var unstructuredObjs []runtime.Object

	if len(e.options.Manifest.Paths) > 0 {
		manifests, err := resources.LoadFromPaths(
			e.options.Manifest.Paths,
			manifestFilter,
		)
		if err != nil {
			return fmt.Errorf("failed to load manifests from paths %v: %w", e.options.Manifest.Paths, err)
		}
		for _, m := range manifests {
			unstructuredObjs = append(unstructuredObjs, &m)
		}
	}

	if len(e.options.Manifest.Objects) > 0 {
		manifests, err := resources.UnstructuredFromObjects(
			e.options.Scheme,
			e.options.Manifest.Objects,
			manifestFilter,
		)
		if err != nil {
			return fmt.Errorf("failed to load %d runtime objects: %w", len(e.options.Manifest.Objects), err)
		}
		for _, m := range manifests {
			unstructuredObjs = append(unstructuredObjs, &m)
		}
	}

	// Convert unstructured objects to typed objects
	for _, obj := range unstructuredObjs {
		uns, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		objGVK := uns.GroupVersionKind()

		switch objGVK {
		case gvk.CustomResourceDefinition:
			var crd apiextensionsv1.CustomResourceDefinition
			if err := resources.Convert(e.options.Scheme, uns, &crd); err != nil {
				return fmt.Errorf("failed to convert CRD %s: %w", uns.GetName(), err)
			}
			e.manifests.CustomResourceDefinitions = append(e.manifests.CustomResourceDefinitions, crd)

		case gvk.MutatingWebhookConfiguration:
			var webhook admissionregistrationv1.MutatingWebhookConfiguration
			if err := resources.Convert(e.options.Scheme, uns, &webhook); err != nil {
				return fmt.Errorf("failed to convert MutatingWebhookConfiguration %s: %w", uns.GetName(), err)
			}
			e.manifests.MutatingWebhookConfigurations = append(e.manifests.MutatingWebhookConfigurations, webhook)

		case gvk.ValidatingWebhookConfiguration:
			var webhook admissionregistrationv1.ValidatingWebhookConfiguration
			if err := resources.Convert(e.options.Scheme, uns, &webhook); err != nil {
				return fmt.Errorf("failed to convert ValidatingWebhookConfiguration %s: %w", uns.GetName(), err)
			}
			e.manifests.ValidatingWebhookConfigurations = append(e.manifests.ValidatingWebhookConfigurations, webhook)
		}
	}

	return nil
}

// debugf logs a debug message if a logger is configured.
func (e *K3sEnv) debugf(format string, args ...interface{}) {
	if e.options.Logger != nil {
		e.options.Logger.Logf("[k3senv] "+format, args...)
	}
}
