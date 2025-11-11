package k3senv_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"

	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/gomega"
)

const testCertPath = "/tmp/certs"

func TestOptions_FunctionalStyle(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()

	env, err := k3senv.New(
		k3senv.WithScheme(scheme),
		k3senv.WithCertPath(testCertPath),
		k3senv.WithManifests("/path/to/manifests1"),
		k3senv.WithManifests("/path/to/manifests2", "/path/to/manifests3"),
	)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env.Scheme()).To(Equal(scheme))
	g.Expect(env.CertPath()).To(Equal(testCertPath))
}

func TestOptions_StructStyle(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()

	env, err := k3senv.New(&k3senv.Options{
		Scheme: scheme,
		Certificate: k3senv.CertificateConfig{
			Path: testCertPath,
		},
		Manifest: k3senv.ManifestConfig{
			Paths: []string{"/path/to/manifests1", "/path/to/manifests2"},
		},
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env.Scheme()).To(Equal(scheme))
	g.Expect(env.CertPath()).To(Equal(testCertPath))
}

func TestOptions_MixedStyle(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()

	env, err := k3senv.New(
		&k3senv.Options{
			Scheme: scheme,
			Manifest: k3senv.ManifestConfig{
				Paths: []string{"/path/to/manifests1"},
			},
		},
		k3senv.WithCertPath(testCertPath),
		k3senv.WithManifests("/path/to/manifests2"),
	)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env.Scheme()).To(Equal(scheme))
	g.Expect(env.CertPath()).To(Equal(testCertPath))
}

func TestOptions_ApplyToOptions(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()

	opt1 := &k3senv.Options{
		Scheme: scheme,
		Certificate: k3senv.CertificateConfig{
			Path: testCertPath,
		},
	}

	opt2 := &k3senv.Options{
		Manifest: k3senv.ManifestConfig{
			Paths: []string{"/path/to/manifests"},
		},
	}

	target := &k3senv.Options{}
	opt1.ApplyToOptions(target)
	opt2.ApplyToOptions(target)

	g.Expect(target.Scheme).To(Equal(scheme))
	g.Expect(target.Certificate.Path).To(Equal(testCertPath))
	g.Expect(target.Manifest.Paths).To(HaveLen(1))
}

func TestTimeoutConfig_Defaults(t *testing.T) {
	g := NewWithT(t)

	env, err := k3senv.New(k3senv.WithCertPath(testCertPath))

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env).NotTo(BeNil())
}

func TestK3sArgs_WithK3sArgs(t *testing.T) {
	g := NewWithT(t)
	args := []string{"--disable=traefik", "--disable=metrics-server"}

	env, err := k3senv.New(
		k3senv.WithK3sArgs(args...),
		k3senv.WithCertPath(testCertPath),
	)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env).NotTo(BeNil())
}

func TestLogger_WithLogger(t *testing.T) {
	g := NewWithT(t)
	var logMessages []string
	mockLogger := &mockLogger{messages: &logMessages}

	env, err := k3senv.New(
		k3senv.WithLogger(mockLogger),
		k3senv.WithCertPath(testCertPath),
	)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env).NotTo(BeNil())
}

func TestLoadConfigFromEnv_EmptyEnvironment(t *testing.T) {
	g := NewWithT(t)

	opts, err := k3senv.LoadConfigFromEnv()
	g.Expect(err).NotTo(HaveOccurred())

	// Should have defaults
	g.Expect(opts.Webhook.Port).To(Equal(k3senv.DefaultWebhookPort))
	g.Expect(opts.K3s.Image).To(Equal(k3senv.DefaultK3sImage))
}

func TestNew_EnvironmentVariablePrecedence(t *testing.T) {
	g := NewWithT(t)

	// Set environment variables
	t.Setenv("K3SENV_WEBHOOK_PORT", "8080")
	t.Setenv("K3SENV_K3S_IMAGE", "rancher/k3s:env-test")

	// Test that env vars are loaded automatically and explicit options override them
	env, err := k3senv.New(
		k3senv.WithWebhookPort(9999), // Should override env var
		k3senv.WithCertPath(testCertPath),
	)
	g.Expect(err).NotTo(HaveOccurred())

	// Explicit option should override environment variable
	g.Expect(env).NotTo(BeNil())
	// We can't easily access internal options without exposing them,
	// but we can verify the environment was created successfully with mixed config
}

func TestNew_EnvironmentVariablesOnly(t *testing.T) {
	g := NewWithT(t)

	// Set environment variables
	t.Setenv("K3SENV_WEBHOOK_PORT", "8080")
	t.Setenv("K3SENV_K3S_IMAGE", "rancher/k3s:env-only-test")
	t.Setenv("K3SENV_CERTIFICATE_PATH", testCertPath)

	// First test that LoadConfigFromEnv works
	opts, err := k3senv.LoadConfigFromEnv()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts.Certificate.Path).To(Equal(testCertPath))

	// Test that env vars are loaded automatically
	env, err := k3senv.New()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env).NotTo(BeNil())
	g.Expect(env.CertPath()).To(Equal(testCertPath))
}

func TestPollIntervals_ComponentSpecific(t *testing.T) {
	g := NewWithT(t)

	// Test environment variables for poll intervals
	t.Setenv("K3SENV_WEBHOOK_POLL_INTERVAL", "1s")
	t.Setenv("K3SENV_CRD_POLL_INTERVAL", "200ms")

	opts, err := k3senv.LoadConfigFromEnv()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts.Webhook.PollInterval).To(Equal(1 * time.Second))
	g.Expect(opts.CRD.PollInterval).To(Equal(200 * time.Millisecond))

	// Remove env vars to test defaults
	g.Expect(os.Unsetenv("K3SENV_WEBHOOK_POLL_INTERVAL")).NotTo(HaveOccurred())
	g.Expect(os.Unsetenv("K3SENV_CRD_POLL_INTERVAL")).NotTo(HaveOccurred())

	opts3, err := k3senv.LoadConfigFromEnv()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts3.Webhook.PollInterval).To(Equal(k3senv.DefaultWebhookPollInterval))
	g.Expect(opts3.CRD.PollInterval).To(Equal(k3senv.DefaultCRDPollInterval))
}

func TestLogger_ContainerLogRedirection(t *testing.T) {
	g := NewWithT(t)
	var logMessages []string
	mockLogger := &mockLogger{messages: &logMessages}

	// Create environment with logger
	env, err := k3senv.New(
		k3senv.WithLogger(mockLogger),
		k3senv.WithCertPath(testCertPath),
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env).NotTo(BeNil())

	// The logger should be properly attached
	// (actual k3s container logs would be tested in integration tests)
}

func TestLogger_TestingTCompatibility(t *testing.T) {
	t.Run("WithLogger option", func(t *testing.T) {
		g := NewWithT(t)

		// Verify that testing.T can be passed directly to WithLogger
		env, err := k3senv.New(
			k3senv.WithLogger(t),
		)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(env).NotTo(BeNil())
	})

	t.Run("Struct initialization", func(t *testing.T) {
		g := NewWithT(t)

		// Verify that testing.T can be used in struct initialization
		env, err := k3senv.New(&k3senv.Options{
			Logger: t,
		})

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(env).NotTo(BeNil())

		// Both subtests pass if compilation succeeds,
		// proving that testing.T implements the Logger interface
	})
}

func TestLoggerFunc(t *testing.T) {
	g := NewWithT(t)

	// Test that LoggerFunc can wrap a Printf-style function
	var logMessages []string
	printfFunc := func(format string, args ...interface{}) {
		logMessages = append(logMessages, fmt.Sprintf(format, args...))
	}

	// Create LoggerFunc from the printf function
	logger := k3senv.LoggerFunc(printfFunc)

	// Verify it implements Logger interface
	var _ k3senv.Logger = logger

	// Test logging
	logger.Logf("test message: %s %d", "hello", 42)

	g.Expect(logMessages).To(HaveLen(1))
	g.Expect(logMessages[0]).To(Equal("test message: hello 42"))

	// Test with k3senv.New
	env, err := k3senv.New(k3senv.WithLogger(logger))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env).NotTo(BeNil())
}

func TestTestcontainersLogging_WithTestcontainersLogging(t *testing.T) {
	g := NewWithT(t)
	var logMessages []string
	mockLogger := &mockLogger{messages: &logMessages}

	// Create environment with testcontainers logging enabled (default)
	env, err := k3senv.New(
		k3senv.WithLogger(mockLogger),
		k3senv.WithTestcontainersLogging(true),
		k3senv.WithCertPath(testCertPath),
	)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env).NotTo(BeNil())
}

func TestTestcontainersLogging_SuppressTestcontainersLogging(t *testing.T) {
	g := NewWithT(t)

	// Create environment with testcontainers logging suppressed
	env, err := k3senv.New(
		k3senv.SuppressTestcontainersLogging(),
		k3senv.WithCertPath(testCertPath),
	)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env).NotTo(BeNil())
}

func TestTestcontainersLogging_EnvironmentVariable(t *testing.T) {
	g := NewWithT(t)

	// Test with environment variable set to false
	t.Setenv("K3SENV_LOGGING_ENABLED", "false")

	opts, err := k3senv.LoadConfigFromEnv()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts.Logging.Enabled).NotTo(BeNil())
	g.Expect(*opts.Logging.Enabled).To(BeFalse())

	// Remove env var
	g.Expect(os.Unsetenv("K3SENV_LOGGING_ENABLED")).NotTo(HaveOccurred())

	// Test default value (should be true)
	opts2, err := k3senv.LoadConfigFromEnv()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts2.Logging.Enabled).NotTo(BeNil())
	g.Expect(*opts2.Logging.Enabled).To(BeTrue())
}

func TestTestcontainersLogging_ExplicitOptionOverridesEnv(t *testing.T) {
	g := NewWithT(t)

	// Set env var to disable
	t.Setenv("K3SENV_LOGGING_ENABLED", "false")

	// Explicit option should override env var
	env, err := k3senv.New(
		k3senv.WithTestcontainersLogging(true),
		k3senv.WithCertPath(testCertPath),
	)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env).NotTo(BeNil())
}

func TestTestcontainersLogging_StructStyle(t *testing.T) {
	g := NewWithT(t)

	// Test struct-style configuration
	env, err := k3senv.New(&k3senv.Options{
		Logging: k3senv.LoggingConfig{
			Enabled: k3senv.Bool(false),
		},
		Certificate: k3senv.CertificateConfig{
			Path: testCertPath,
		},
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env).NotTo(BeNil())
}

// mockLogger implements the Logger interface for testing.
type mockLogger struct {
	messages *[]string
}

func (m *mockLogger) Logf(format string, args ...interface{}) {
	*m.messages = append(*m.messages, fmt.Sprintf(format, args...))
}
