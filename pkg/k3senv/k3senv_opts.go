package k3senv

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime"
)

const (
	DefaultK3sImage      = "rancher/k3s:v1.32.9-k3s1"
	DefaultWebhookPort   = 9443
	DefaultCertDirPrefix = "/tmp/k3senv-certs-"
	DefaultCertValidity  = 24 * time.Hour

	DefaultWebhookPollInterval = 500 * time.Millisecond
	DefaultCRDPollInterval     = 100 * time.Millisecond
	WebhookReadyTimeout        = 30 * time.Second
	WebhookHealthCheckTimeout  = 5 * time.Second
	CRDReadyTimeout            = 30 * time.Second
)

// Logger is a simple interface compatible with testing.T and most logging frameworks.
type Logger interface {
	Printf(format string, args ...interface{})
}

type Option interface {
	ApplyToOptions(opts *Options)
}

// WebhookConfig groups all webhook-related configuration.
type WebhookConfig struct {
	Port               int           `mapstructure:"port"`
	AutoInstall        bool          `mapstructure:"auto_install"`
	ReadyTimeout       time.Duration `mapstructure:"ready_timeout"`
	HealthCheckTimeout time.Duration `mapstructure:"health_check_timeout"`
	PollInterval       time.Duration `mapstructure:"poll_interval"`
}

// CRDConfig groups all CRD-related configuration.
type CRDConfig struct {
	ReadyTimeout time.Duration `mapstructure:"ready_timeout"`
	PollInterval time.Duration `mapstructure:"poll_interval"`
}

// K3sConfig groups all k3s-related configuration.
type K3sConfig struct {
	Image string   `mapstructure:"image"`
	Args  []string `mapstructure:"args"`
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

type Options struct {
	Scheme      *runtime.Scheme   `mapstructure:"-"`
	Webhook     WebhookConfig     `mapstructure:"webhook"`
	CRD         CRDConfig         `mapstructure:"crd"`
	K3s         K3sConfig         `mapstructure:"k3s"`
	Certificate CertificateConfig `mapstructure:"certificate"`
	Manifest    ManifestConfig    `mapstructure:"manifest"`
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
	if o.Webhook.AutoInstall {
		target.Webhook.AutoInstall = o.Webhook.AutoInstall
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

	// Logger
	if o.Logger != nil {
		target.Logger = o.Logger
	}
}

var _ Option = &Options{}

type Scheme struct {
	scheme *runtime.Scheme
}

func WithScheme(s *runtime.Scheme) Option {
	return &Scheme{scheme: s}
}

func (s *Scheme) ApplyToOptions(o *Options) {
	o.Scheme = s.scheme
}

type Manifests struct {
	paths []string
}

func WithManifests(paths ...string) Option {
	return &Manifests{paths: paths}
}

func (m *Manifests) ApplyToOptions(o *Options) {
	o.Manifest.Paths = append(o.Manifest.Paths, m.paths...)
}

type CertPath struct {
	path string
}

func WithCertPath(path string) Option {
	return &CertPath{path: path}
}

func (c *CertPath) ApplyToOptions(o *Options) {
	o.Certificate.Path = c.path
}

type Objects struct {
	objects []client.Object
}

func WithObjects(objects ...client.Object) Option {
	return &Objects{objects: objects}
}

func (obj *Objects) ApplyToOptions(o *Options) {
	o.Manifest.Objects = append(o.Manifest.Objects, obj.objects...)
}

type AutoInstallWebhooks struct {
	enable bool
}

func WithAutoInstallWebhooks(enable bool) Option {
	return &AutoInstallWebhooks{enable: enable}
}

func (a *AutoInstallWebhooks) ApplyToOptions(o *Options) {
	o.Webhook.AutoInstall = a.enable
}

type WebhookPort struct {
	port int
}

func WithWebhookPort(port int) Option {
	return &WebhookPort{port: port}
}

func (w *WebhookPort) ApplyToOptions(o *Options) {
	o.Webhook.Port = w.port
}

type K3sImage struct {
	image string
}

func WithK3sImage(image string) Option {
	return &K3sImage{image: image}
}

func (k *K3sImage) ApplyToOptions(o *Options) {
	o.K3s.Image = k.image
}

type K3sArgs struct {
	args []string
}

func WithK3sArgs(args ...string) Option {
	return &K3sArgs{args: args}
}

func (k *K3sArgs) ApplyToOptions(o *Options) {
	o.K3s.Args = append(o.K3s.Args, k.args...)
}

type CertValidity struct {
	duration time.Duration
}

func WithCertValidity(duration time.Duration) Option {
	return &CertValidity{duration: duration}
}

func (c *CertValidity) ApplyToOptions(o *Options) {
	o.Certificate.Validity = c.duration
}

type LoggerOption struct {
	logger Logger
}

func WithLogger(logger Logger) Option {
	return &LoggerOption{logger: logger}
}

func (l *LoggerOption) ApplyToOptions(o *Options) {
	o.Logger = l.logger
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
	v.SetDefault("webhook.ready_timeout", WebhookReadyTimeout)
	v.SetDefault("webhook.health_check_timeout", WebhookHealthCheckTimeout)
	v.SetDefault("webhook.poll_interval", DefaultWebhookPollInterval)
	v.SetDefault("crd.ready_timeout", CRDReadyTimeout)
	v.SetDefault("crd.poll_interval", DefaultCRDPollInterval)
	v.SetDefault("k3s.image", DefaultK3sImage)
	v.SetDefault("k3s.args", []string{})
	v.SetDefault("certificate.path", "")
	v.SetDefault("certificate.validity", DefaultCertValidity)
	v.SetDefault("manifest.paths", []string{})

	var opts Options

	if err := v.Unmarshal(&opts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config from environment: %w", err)
	}

	return &opts, nil
}
