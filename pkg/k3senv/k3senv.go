package k3senv

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/lburgazzoli/k3s-envtest/internal/gvk"
	"github.com/lburgazzoli/k3s-envtest/internal/jq"
	"github.com/lburgazzoli/k3s-envtest/internal/resources"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
)

const (
	// DefaultWebhookServerHost is the default host address for the webhook server.
	// It binds to all network interfaces (0.0.0.0) to allow connections from containers.
	DefaultWebhookServerHost = "0.0.0.0"

	// CACertFileName is the filename for the CA certificate PEM file.
	CACertFileName = "cert-ca.pem"

	// CertFileName is the filename for the TLS certificate PEM file.
	CertFileName = "cert-tls.pem"

	// KeyFileName is the filename for the TLS private key PEM file.
	KeyFileName = "key-tls.pem"

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

// loggerConsumer forwards testcontainer logs to the k3senv Logger.
type loggerConsumer struct {
	logger Logger
}

func (lc *loggerConsumer) Accept(log testcontainers.Log) {
	if lc.logger != nil {
		message := strings.TrimSpace(string(log.Content))
		if message != "" {
			lc.logger.Logf("[k3s] %s", message)
		}
	}
}

type K3sEnv struct {
	container *k3s.K3sContainer
	cfg       *rest.Config
	cli       client.Client

	options Options

	certData      *CertData
	manifests     []unstructured.Unstructured
	teardownTasks []TeardownTask
}

func New(opts ...Option) (*K3sEnv, error) {
	options, err := LoadConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Apply explicit options (these override env vars)
	options.ApplyOptions(opts)

	if options.Webhook.Port < 1 || options.Webhook.Port > 65535 {
		return nil, fmt.Errorf("webhookPort must be in range 1-65535, got %d", options.Webhook.Port)
	}
	if options.K3s.Image == "" {
		return nil, errors.New("k3sImage cannot be empty")
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
	e.debugf("Loaded %d manifests", len(e.manifests))

	if err := e.installCRDsIfNeeded(ctx); err != nil {
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

func (e *K3sEnv) AddTeardownFn(fn func(context.Context) error) {
	e.AddTeardown(fn)
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

func (e *K3sEnv) CRDs() []unstructured.Unstructured {
	var result []unstructured.Unstructured
	for _, manifest := range e.manifests {
		if manifest.GroupVersionKind() == gvk.CustomResourceDefinition {
			result = append(result, *manifest.DeepCopy())
		}
	}
	return result
}

func (e *K3sEnv) WebhookConfigs() []unstructured.Unstructured {
	var result []unstructured.Unstructured
	for _, manifest := range e.manifests {
		gvkType := manifest.GroupVersionKind()
		if gvkType == gvk.MutatingWebhookConfiguration || gvkType == gvk.ValidatingWebhookConfiguration {
			result = append(result, *manifest.DeepCopy())
		}
	}
	return result
}

func (e *K3sEnv) GetWebhookHost(ctx context.Context) (string, error) {
	return net.JoinHostPort("host.testcontainers.internal", strconv.Itoa(e.options.Webhook.Port)), nil
}

func (e *K3sEnv) WebhookServer() ctrlwebhook.Server {
	return ctrlwebhook.NewServer(ctrlwebhook.Options{
		Port:     e.options.Webhook.Port,
		Host:     DefaultWebhookServerHost,
		CertDir:  e.options.Certificate.Path,
		CertName: CertFileName,
		KeyName:  KeyFileName,
		TLSOpts: []func(*tls.Config){
			func(config *tls.Config) {
				config.MinVersion = tls.VersionTLS12
			},
		},
	})
}

func (e *K3sEnv) InstallWebhooks(ctx context.Context) error {
	webhookHostPort, err := e.GetWebhookHost(ctx)
	if err != nil {
		return fmt.Errorf("failed to get webhook host: %w", err)
	}

	e.debugf("Installing webhooks with host: %s", webhookHostPort)

	webhookConfigs, err := e.patchWebhookConfigurations(webhookHostPort)
	if err != nil {
		return fmt.Errorf("failed to patch webhook configurations: %w", err)
	}

	for i := range webhookConfigs {
		wh := &webhookConfigs[i]
		if err := e.cli.Create(ctx, wh); err != nil {
			return fmt.Errorf("failed to create webhook config %s: %w", wh.GetName(), err)
		}

		if !ptr.Deref(e.options.Webhook.CheckReadiness, false) {
			continue
		}

		if err := e.waitForWebhookEndpointsReady(ctx, wh, e.options.Webhook.Port); err != nil {
			return fmt.Errorf("webhook config %s endpoints not ready: %w", wh.GetName(), err)
		}
	}

	crds := e.CRDs()
	convertibleCRDs, err := determineConvertibleCRDs(crds, e.options.Scheme)
	if err != nil {
		return fmt.Errorf("failed to determine convertible CRDs: %w", err)
	}

	if len(convertibleCRDs) > 0 {
		if err := e.patchAndUpdateCRDConversions(ctx, convertibleCRDs, webhookHostPort); err != nil {
			return fmt.Errorf("failed to patch and update CRD conversions: %w", err)
		}

		if err := e.waitForCRDsEstablished(ctx, extractNames(convertibleCRDs)); err != nil {
			return fmt.Errorf("failed waiting for converted CRDs to be re-established: %w", err)
		}
	}

	return nil
}

func (e *K3sEnv) CreateCRD(
	ctx context.Context,
	crd client.Object,
) error {
	if err := e.cli.Create(ctx, crd); err != nil && !k8serr.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create CRD %s: %w",
			resources.FormatObjectReference(crd),
			err,
		)
	}

	return e.waitForCRDsEstablished(ctx, []string{crd.GetName()})
}

func (e *K3sEnv) startK3sContainer(ctx context.Context) error {
	opts := []testcontainers.ContainerCustomizer{
		testcontainers.WithHostPortAccess(e.options.Webhook.Port),
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
	autoGeneratedCertDir := false
	if e.options.Certificate.Path == "" {
		e.options.Certificate.Path = fmt.Sprintf("%s%s", DefaultCertDirPrefix, e.container.GetContainerID())
		autoGeneratedCertDir = true
	}

	certData, err := generateCertificates(e.options.Certificate.Path, e.options.Certificate.Validity)
	if err != nil {
		return fmt.Errorf("failed to generate certificates in path %s: %w", e.options.Certificate.Path, err)
	}

	e.certData = certData

	if autoGeneratedCertDir {
		certDirToClean := e.options.Certificate.Path
		e.AddTeardownFn(func(ctx context.Context) error {
			return os.RemoveAll(certDirToClean)
		})
	}

	return nil
}

func (e *K3sEnv) prepareManifests() error {
	e.manifests = []unstructured.Unstructured{}

	if len(e.options.Manifest.Paths) > 0 {
		manifests, err := loadManifestsFromPaths(e.options.Manifest.Paths)
		if err != nil {
			return fmt.Errorf("failed to load manifests from paths %v: %w", e.options.Manifest.Paths, err)
		}
		e.manifests = append(e.manifests, manifests...)
	}

	if len(e.options.Manifest.Objects) > 0 {
		manifests, err := loadObjectsToManifests(e.options.Scheme, e.options.Manifest.Objects)
		if err != nil {
			return fmt.Errorf("failed to load %d runtime objects: %w", len(e.options.Manifest.Objects), err)
		}
		e.manifests = append(e.manifests, manifests...)
	}

	return nil
}

func (e *K3sEnv) installCRDsIfNeeded(ctx context.Context) error {
	crds := e.CRDs()
	if len(crds) == 0 {
		return nil
	}

	if err := e.installCRDs(ctx); err != nil {
		return fmt.Errorf("failed to install %d CRDs: %w", len(crds), err)
	}

	if err := e.waitForCRDsEstablished(ctx, extractNames(crds)); err != nil {
		return fmt.Errorf("failed waiting for CRDs to be established: %w", err)
	}

	return nil
}

func (e *K3sEnv) patchAndUpdateCRDConversions(
	ctx context.Context,
	convertibleCRDs []unstructured.Unstructured,
	hostPort string,
) error {
	baseURL := fmt.Sprintf("%s://%s", WebhookURLScheme, hostPort)
	caBundle := string(e.certData.CABundle())

	for i := range convertibleCRDs {
		crd := convertibleCRDs[i].DeepCopy()

		if err := e.cli.Get(ctx, client.ObjectKeyFromObject(crd), crd); err != nil {
			return fmt.Errorf("failed to get CRD %s: %w", crd.GetName(), err)
		}

		err := jq.Transform(
			crd, `
			.spec.conversion = {
				"strategy": "Webhook",
				"webhook": {
					"conversionReviewVersions": ["v1", "v1beta1"],
					"clientConfig": {
						"url": "%s",
						"caBundle": "%s"
					}
				}
			}
		`, baseURL+WebhookConvertPath, caBundle)

		if err != nil {
			return fmt.Errorf("failed to patch CRD %s: %w", crd.GetName(), err)
		}

		if err := e.cli.Update(ctx, crd); err != nil {
			return fmt.Errorf("failed to update CRD %s with conversion: %w", crd.GetName(), err)
		}
	}

	return nil
}

func (e *K3sEnv) installCRDs(ctx context.Context) error {
	// CRDs() already returns deep copies, no need to copy again
	crds := e.CRDs()
	for i := range crds {
		err := e.cli.Create(ctx, &crds[i])

		if err != nil && !k8serr.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create CRD %s: %w",
				resources.FormatObjectReference(&crds[i]),
				err,
			)
		}
	}

	return nil
}

func (e *K3sEnv) waitForCRDsEstablished(
	ctx context.Context,
	crdNames []string,
) error {
	e.debugf("Waiting for CRDs to be established...")

	for _, crdName := range crdNames {
		e.debugf("Checking CRD: %s", crdName)

		err := wait.PollUntilContextTimeout(ctx, e.options.CRD.PollInterval, e.options.CRD.ReadyTimeout, true, func(ctx context.Context) (bool, error) {
			obj := &apiextensionsv1.CustomResourceDefinition{}
			err := e.cli.Get(ctx, client.ObjectKey{Name: crdName}, obj)
			switch {
			case k8serr.IsNotFound(err):
				return false, nil
			case err != nil:
				return false, fmt.Errorf("failed to get CRD %s: %w", crdName, err)
			}

			for _, condition := range obj.Status.Conditions {
				if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
					e.debugf("CRD %s is now active", crdName)
					return true, nil
				}
			}

			return false, nil
		})

		if err != nil {
			return fmt.Errorf("CRD %s not established: %w", crdName, err)
		}
	}

	return nil
}

func (e *K3sEnv) patchWebhookConfigurations(
	hostPort string,
) ([]unstructured.Unstructured, error) {
	baseURL := fmt.Sprintf("%s://%s", WebhookURLScheme, hostPort)
	caBundle := string(e.certData.CABundle())

	webhookConfigs := e.WebhookConfigs()
	for i := range webhookConfigs {
		wh := &webhookConfigs[i]

		err := jq.Transform(wh, `
			.webhooks |= map(
				.clientConfig.url = "%s" + (.clientConfig.service.path // "/") |
				.clientConfig.caBundle = "%s" |
				del(.clientConfig.service)
			)
		`, baseURL, caBundle)

		if err != nil {
			return nil, fmt.Errorf("failed to patch webhook %s: %w", wh.GetName(), err)
		}
	}

	return webhookConfigs, nil
}

// debugf logs a debug message if a logger is configured.
func (e *K3sEnv) debugf(format string, args ...interface{}) {
	if e.options.Logger != nil {
		e.options.Logger.Logf("[k3senv] "+format, args...)
	}
}
