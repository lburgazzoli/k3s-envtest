package k3senv

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

const (
	DefaultK3sImage          = "rancher/k3s:v1.32.9-k3s1"
	DefaultK3sLogRedirection = false
	DefaultWebhookPort       = 9443
	DefaultCertDirPrefix     = "/tmp/k3senv-certs-"
	DefaultCertValidity      = 24 * time.Hour

	DefaultWebhookPollInterval = 500 * time.Millisecond
	DefaultCRDPollInterval     = 100 * time.Millisecond

	// WebhookReadyTimeout is the internal default maximum time to wait for each
	// individual webhook endpoint to become ready. The system polls each endpoint
	// until it responds successfully or this timeout expires.
	// Applied per endpoint, not cumulative across all endpoints.
	WebhookReadyTimeout = 30 * time.Second

	// WebhookHealthCheckTimeout is the internal default HTTP client timeout for each
	// individual health check request attempt to a webhook endpoint.
	// Multiple attempts may be made within WebhookReadyTimeout.
	WebhookHealthCheckTimeout = 5 * time.Second

	// CRDReadyTimeout is the internal default maximum time to wait for all CRDs
	// to reach the Established condition after installation.
	CRDReadyTimeout = 30 * time.Second
)

// Bool returns a pointer to the boolean value passed in.
// This is a convenience alias to ptr.To from k8s.io/utils/ptr.
// Use this for creating pointer boolean values for configuration.
func Bool(b bool) *bool {
	return ptr.To(b)
}

// Logger is a simple interface for structured logging, designed to be compatible
// with testing.T's Logf method. This allows tests to easily capture k3senv debug
// output without additional configuration.
//
// Usage with testing.T (most common):
//
//	env, err := k3senv.New(k3senv.WithLogger(t))
//
// Usage with standard library log:
//
//	logger := log.New(os.Stderr, "[k3senv] ", log.LstdFlags)
//	env, err := k3senv.New(k3senv.WithLogger(k3senv.LoggerFunc(logger.Printf)))
//
// Usage with zap or other loggers:
//
//	zapLogger, _ := zap.NewDevelopment()
//	env, err := k3senv.New(k3senv.WithLogger(k3senv.LoggerFunc(zapLogger.Sugar().Infof)))
type Logger interface {
	Logf(format string, args ...any)
}

// LoggerFunc is an adapter that allows a printf-style function to be used as a Logger.
// This makes it easy to integrate with any logging framework that provides a Printf-like method.
//
// Example:
//
//	logger := log.New(os.Stderr, "[k3senv] ", log.LstdFlags)
//	env, err := k3senv.New(k3senv.WithLogger(k3senv.LoggerFunc(logger.Printf)))
type LoggerFunc func(format string, args ...any)

// Logf implements the Logger interface by calling the underlying function.
func (f LoggerFunc) Logf(format string, args ...any) {
	f(format, args...)
}

type Option interface {
	ApplyToOptions(opts *Options)
}

// optionFunc is an adapter that allows a simple function to be used as an Option.
// This enables the functional options pattern with minimal boilerplate.
type optionFunc func(*Options)

func (f optionFunc) ApplyToOptions(o *Options) {
	f(o)
}

// WebhookConfig groups all webhook-related configuration.
type WebhookConfig struct {
	Port               int           `mapstructure:"port"`
	AutoInstall        *bool         `mapstructure:"auto_install"`
	CheckReadiness     *bool         `mapstructure:"check_readiness"`
	ReadyTimeout       time.Duration `mapstructure:"ready_timeout"`
	HealthCheckTimeout time.Duration `mapstructure:"health_check_timeout"`
	PollInterval       time.Duration `mapstructure:"poll_interval"`
}

// CRDConfig groups all CRD-related configuration.
type CRDConfig struct {
	ReadyTimeout time.Duration `mapstructure:"ready_timeout"`
	PollInterval time.Duration `mapstructure:"poll_interval"`
}

// NetworkConfig groups all Docker network-related configuration for the k3s container.
type NetworkConfig struct {
	// Name specifies the Docker network to connect the container to.
	// If empty, uses the default bridge network.
	Name string `mapstructure:"name"`

	// Aliases are DNS aliases for the container within the Docker network.
	// Useful for service discovery within custom networks.
	Aliases []string `mapstructure:"aliases"`

	// Mode specifies the Docker network mode (bridge, host, none, container:<name>).
	// If empty, uses Docker default (bridge).
	Mode string `mapstructure:"mode"`
}

// K3sConfig groups all k3s-related configuration.
type K3sConfig struct {
	Image          string         `mapstructure:"image"`
	Args           []string       `mapstructure:"args"`
	LogRedirection *bool          `mapstructure:"log_redirection"`
	Network        *NetworkConfig `mapstructure:"network"`
}

// CertificateConfig groups all certificate-related configuration.
type CertificateConfig struct {
	Path     string        `mapstructure:"path"`
	Validity time.Duration `mapstructure:"validity"`
}

// ManifestConfig groups all manifest-related configuration.
type ManifestConfig struct {
	Paths   []string        `mapstructure:"paths"`
	Objects []client.Object `mapstructure:"-"`
}

// LoggingConfig groups all logging-related configuration.
type LoggingConfig struct {
	// Enabled controls whether testcontainers lifecycle logging is enabled.
	// When disabled, testcontainers framework messages are completely suppressed.
	// Defaults to true (enabled with emoji filtering).
	Enabled *bool `mapstructure:"enabled"`
}

type Options struct {
	Scheme      *runtime.Scheme   `mapstructure:"-"`
	Webhook     WebhookConfig     `mapstructure:"webhook"`
	CRD         CRDConfig         `mapstructure:"crd"`
	K3s         K3sConfig         `mapstructure:"k3s"`
	Certificate CertificateConfig `mapstructure:"certificate"`
	Manifest    ManifestConfig    `mapstructure:"manifest"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	Logger      Logger            `mapstructure:"-"`
}

func (o *Options) ApplyOptions(opts []Option) *Options {
	for _, opt := range opts {
		opt.ApplyToOptions(o)
	}
	return o
}

func (o *Options) ApplyToOptions(target *Options) {
	if o.Scheme != nil {
		target.Scheme = o.Scheme
	}

	// Webhook config
	if o.Webhook.Port != 0 {
		target.Webhook.Port = o.Webhook.Port
	}
	if o.Webhook.AutoInstall != nil {
		target.Webhook.AutoInstall = o.Webhook.AutoInstall
	}
	if o.Webhook.CheckReadiness != nil {
		target.Webhook.CheckReadiness = o.Webhook.CheckReadiness
	}
	if o.Webhook.ReadyTimeout != 0 {
		target.Webhook.ReadyTimeout = o.Webhook.ReadyTimeout
	}
	if o.Webhook.HealthCheckTimeout != 0 {
		target.Webhook.HealthCheckTimeout = o.Webhook.HealthCheckTimeout
	}
	if o.Webhook.PollInterval != 0 {
		target.Webhook.PollInterval = o.Webhook.PollInterval
	}

	// CRD config
	if o.CRD.ReadyTimeout != 0 {
		target.CRD.ReadyTimeout = o.CRD.ReadyTimeout
	}
	if o.CRD.PollInterval != 0 {
		target.CRD.PollInterval = o.CRD.PollInterval
	}

	// K3s config
	if o.K3s.Image != "" {
		target.K3s.Image = o.K3s.Image
	}
	if len(o.K3s.Args) > 0 {
		target.K3s.Args = append(target.K3s.Args, o.K3s.Args...)
	}
	if o.K3s.LogRedirection != nil {
		target.K3s.LogRedirection = o.K3s.LogRedirection
	}
	if o.K3s.Network != nil {
		if target.K3s.Network == nil {
			target.K3s.Network = &NetworkConfig{}
		}
		if o.K3s.Network.Name != "" {
			target.K3s.Network.Name = o.K3s.Network.Name
		}
		if len(o.K3s.Network.Aliases) > 0 {
			target.K3s.Network.Aliases = append(target.K3s.Network.Aliases, o.K3s.Network.Aliases...)
		}
		if o.K3s.Network.Mode != "" {
			target.K3s.Network.Mode = o.K3s.Network.Mode
		}
	}

	// Certificate config
	if o.Certificate.Path != "" {
		target.Certificate.Path = o.Certificate.Path
	}
	if o.Certificate.Validity != 0 {
		target.Certificate.Validity = o.Certificate.Validity
	}

	// Manifest config
	if len(o.Manifest.Paths) > 0 {
		target.Manifest.Paths = append(target.Manifest.Paths, o.Manifest.Paths...)
	}
	if len(o.Manifest.Objects) > 0 {
		target.Manifest.Objects = append(target.Manifest.Objects, o.Manifest.Objects...)
	}

	// Logging config
	if o.Logging.Enabled != nil {
		target.Logging.Enabled = o.Logging.Enabled
	}

	// Logger
	if o.Logger != nil {
		target.Logger = o.Logger
	}
}

var _ Option = &Options{}

// Scheme options

func WithScheme(s *runtime.Scheme) Option {
	return optionFunc(func(o *Options) { o.Scheme = s })
}

// Manifest options

func WithManifests(paths ...string) Option {
	return optionFunc(func(o *Options) { o.Manifest.Paths = append(o.Manifest.Paths, paths...) })
}

func WithObjects(objects ...client.Object) Option {
	return optionFunc(func(o *Options) { o.Manifest.Objects = append(o.Manifest.Objects, objects...) })
}

// Certificate options

func WithCertPath(path string) Option {
	return optionFunc(func(o *Options) { o.Certificate.Path = path })
}

func WithCertValidity(duration time.Duration) Option {
	return optionFunc(func(o *Options) { o.Certificate.Validity = duration })
}

// Webhook options

func WithWebhookPort(port int) Option {
	return optionFunc(func(o *Options) { o.Webhook.Port = port })
}

func WithAutoInstallWebhooks(enable bool) Option {
	return optionFunc(func(o *Options) { o.Webhook.AutoInstall = &enable })
}

func WithWebhookCheckReadiness(enable bool) Option {
	return optionFunc(func(o *Options) { o.Webhook.CheckReadiness = &enable })
}

func WithWebhookReadyTimeout(duration time.Duration) Option {
	return optionFunc(func(o *Options) { o.Webhook.ReadyTimeout = duration })
}

func WithWebhookHealthCheckTimeout(duration time.Duration) Option {
	return optionFunc(func(o *Options) { o.Webhook.HealthCheckTimeout = duration })
}

func WithWebhookPollInterval(duration time.Duration) Option {
	return optionFunc(func(o *Options) { o.Webhook.PollInterval = duration })
}

// CRD options

func WithCRDReadyTimeout(duration time.Duration) Option {
	return optionFunc(func(o *Options) { o.CRD.ReadyTimeout = duration })
}

func WithCRDPollInterval(duration time.Duration) Option {
	return optionFunc(func(o *Options) { o.CRD.PollInterval = duration })
}

// K3s options

func WithK3sImage(image string) Option {
	return optionFunc(func(o *Options) { o.K3s.Image = image })
}

func WithK3sArgs(args ...string) Option {
	return optionFunc(func(o *Options) { o.K3s.Args = append(o.K3s.Args, args...) })
}

func WithK3sLogRedirection(enable bool) Option {
	return optionFunc(func(o *Options) { o.K3s.LogRedirection = &enable })
}

func WithK3sNetwork(name string) Option {
	return optionFunc(func(o *Options) {
		if o.K3s.Network == nil {
			o.K3s.Network = &NetworkConfig{}
		}
		o.K3s.Network.Name = name
	})
}

func WithK3sNetworkAliases(aliases ...string) Option {
	return optionFunc(func(o *Options) {
		if o.K3s.Network == nil {
			o.K3s.Network = &NetworkConfig{}
		}
		o.K3s.Network.Aliases = append(o.K3s.Network.Aliases, aliases...)
	})
}

func WithK3sNetworkMode(mode string) Option {
	return optionFunc(func(o *Options) {
		if o.K3s.Network == nil {
			o.K3s.Network = &NetworkConfig{}
		}
		o.K3s.Network.Mode = mode
	})
}

// Logger options

func WithLogger(logger Logger) Option {
	return optionFunc(func(o *Options) { o.Logger = logger })
}

// Logging options

// WithTestcontainersLogging controls whether testcontainers lifecycle logging is enabled.
// When enabled with a logger, testcontainers framework messages are forwarded to the logger
// without emojis. When disabled, all testcontainers lifecycle logging is suppressed.
// Default is true (enabled with emoji filtering).
func WithTestcontainersLogging(enable bool) Option {
	return optionFunc(func(o *Options) { o.Logging.Enabled = &enable })
}

// SuppressTestcontainersLogging is a convenience function that returns an Option
// to completely suppress testcontainers lifecycle logging.
// This is equivalent to WithTestcontainersLogging(false).
//
// Usage:
//
//	env, err := k3senv.New(
//	    k3senv.SuppressTestcontainersLogging(),
//	)
func SuppressTestcontainersLogging() Option {
	return WithTestcontainersLogging(false)
}

// LoadConfigFromEnv loads configuration from environment variables with K3SENV_ prefix
// and returns an Options struct that can be used with New().
func LoadConfigFromEnv() (*Options, error) {
	v := viper.New()

	// Set environment variable prefix
	v.SetEnvPrefix("K3SENV")
	v.AutomaticEnv()

	// Replace dots with underscores for nested config
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults that match the current defaults in New()
	v.SetDefault("webhook.port", DefaultWebhookPort)
	v.SetDefault("webhook.auto_install", false)
	v.SetDefault("webhook.check_readiness", false)
	v.SetDefault("webhook.ready_timeout", WebhookReadyTimeout)
	v.SetDefault("webhook.health_check_timeout", WebhookHealthCheckTimeout)
	v.SetDefault("webhook.poll_interval", DefaultWebhookPollInterval)
	v.SetDefault("crd.ready_timeout", CRDReadyTimeout)
	v.SetDefault("crd.poll_interval", DefaultCRDPollInterval)
	v.SetDefault("k3s.image", DefaultK3sImage)
	v.SetDefault("k3s.args", []string{})
	v.SetDefault("k3s.log_redirection", DefaultK3sLogRedirection)
	v.SetDefault("k3s.network.name", "")
	v.SetDefault("k3s.network.aliases", []string{})
	v.SetDefault("k3s.network.mode", "")
	v.SetDefault("certificate.path", "")
	v.SetDefault("certificate.validity", DefaultCertValidity)
	v.SetDefault("manifest.paths", []string{})
	v.SetDefault("logging.enabled", true)

	var opts Options

	if err := v.Unmarshal(&opts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config from environment: %w", err)
	}

	// Set pointer defaults if not set by environment variables
	if opts.Webhook.AutoInstall == nil {
		opts.Webhook.AutoInstall = ptr.To(false)
	}
	if opts.Webhook.CheckReadiness == nil {
		opts.Webhook.CheckReadiness = ptr.To(false)
	}
	if opts.K3s.LogRedirection == nil {
		opts.K3s.LogRedirection = ptr.To(DefaultK3sLogRedirection)
	}
	if opts.Logging.Enabled == nil {
		opts.Logging.Enabled = ptr.To(true)
	}

	return &opts, nil
}

// validate checks that all configuration values are valid.
// Returns an error if any configuration is invalid or out of acceptable range.
func (opts *Options) validate() error {
	// Webhook port must be in valid range
	if opts.Webhook.Port < 1 || opts.Webhook.Port > 65535 {
		return fmt.Errorf(
			"webhook port must be 1-65535, got %d (use FindAvailablePort() for parallel tests)",
			opts.Webhook.Port,
		)
	}

	// K3s image cannot be empty
	if opts.K3s.Image == "" {
		return errors.New("k3s image cannot be empty")
	}

	// Webhook timeouts must be positive
	if opts.Webhook.ReadyTimeout <= 0 {
		return fmt.Errorf("webhook ready timeout must be positive, got %v", opts.Webhook.ReadyTimeout)
	}
	if opts.Webhook.HealthCheckTimeout <= 0 {
		return fmt.Errorf("webhook health check timeout must be positive, got %v", opts.Webhook.HealthCheckTimeout)
	}

	// CRD timeout must be positive
	if opts.CRD.ReadyTimeout <= 0 {
		return fmt.Errorf("CRD ready timeout must be positive, got %v", opts.CRD.ReadyTimeout)
	}

	// Poll intervals must be positive and reasonable (>= 10ms to prevent tight loops)
	if opts.Webhook.PollInterval <= 0 {
		return fmt.Errorf("webhook poll interval must be positive, got %v", opts.Webhook.PollInterval)
	}
	if opts.Webhook.PollInterval < 10*time.Millisecond {
		return fmt.Errorf("webhook poll interval too small: %v (minimum: 10ms)", opts.Webhook.PollInterval)
	}

	if opts.CRD.PollInterval <= 0 {
		return fmt.Errorf("CRD poll interval must be positive, got %v", opts.CRD.PollInterval)
	}
	if opts.CRD.PollInterval < 10*time.Millisecond {
		return fmt.Errorf("CRD poll interval too small: %v (minimum: 10ms)", opts.CRD.PollInterval)
	}

	// Certificate validity must be positive
	if opts.Certificate.Validity <= 0 {
		return fmt.Errorf("certificate validity must be positive, got %v", opts.Certificate.Validity)
	}

	// Validate network configuration
	if opts.K3s.Network != nil {
		// Network mode validation (must be one of: bridge, host, none, or container:<name>)
		if opts.K3s.Network.Mode != "" {
			validModes := []string{"bridge", "host", "none"}
			isValid := slices.Contains(validModes, opts.K3s.Network.Mode)
			// Also allow "container:name" format
			if !isValid && !strings.HasPrefix(opts.K3s.Network.Mode, "container:") {
				return fmt.Errorf(
					"network mode must be one of: bridge, host, none, container:<name>, got %s",
					opts.K3s.Network.Mode,
				)
			}
		}
	}

	return nil
}
