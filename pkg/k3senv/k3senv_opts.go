package k3senv

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime"
)

type Option interface {
	ApplyToOptions(opts *Options)
}

type Options struct {
	Scheme              *runtime.Scheme
	ManifestDirs        []string
	Objects             []client.Object
	CertDir             string
	AutoInstallWebhooks bool
	WebhookPort         int
	K3sImage            string
	CertValidity        time.Duration
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
	if len(o.ManifestDirs) > 0 {
		target.ManifestDirs = append(target.ManifestDirs, o.ManifestDirs...)
	}
	if len(o.Objects) > 0 {
		target.Objects = append(target.Objects, o.Objects...)
	}
	if o.CertDir != "" {
		target.CertDir = o.CertDir
	}
	if o.AutoInstallWebhooks {
		target.AutoInstallWebhooks = o.AutoInstallWebhooks
	}
	if o.WebhookPort != 0 {
		target.WebhookPort = o.WebhookPort
	}
	if o.K3sImage != "" {
		target.K3sImage = o.K3sImage
	}
	if o.CertValidity != 0 {
		target.CertValidity = o.CertValidity
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

type ManifestDir struct {
	path string
}

func WithManifestDir(p string) Option {
	return &ManifestDir{path: p}
}

func (m *ManifestDir) ApplyToOptions(o *Options) {
	o.ManifestDirs = append(o.ManifestDirs, m.path)
}

type ManifestDirs struct {
	paths []string
}

func WithManifestDirs(paths ...string) Option {
	return &ManifestDirs{paths: paths}
}

func (m *ManifestDirs) ApplyToOptions(o *Options) {
	o.ManifestDirs = append(o.ManifestDirs, m.paths...)
}

type CertDir struct {
	dir string
}

func WithCertDir(dir string) Option {
	return &CertDir{dir: dir}
}

func (c *CertDir) ApplyToOptions(o *Options) {
	o.CertDir = c.dir
}

type Objects struct {
	objects []client.Object
}

func WithObjects(objects ...client.Object) Option {
	return &Objects{objects: objects}
}

func (obj *Objects) ApplyToOptions(o *Options) {
	o.Objects = append(o.Objects, obj.objects...)
}

type AutoInstallWebhooks struct {
	enable bool
}

func WithAutoInstallWebhooks(enable bool) Option {
	return &AutoInstallWebhooks{enable: enable}
}

func (a *AutoInstallWebhooks) ApplyToOptions(o *Options) {
	o.AutoInstallWebhooks = a.enable
}

type WebhookPort struct {
	port int
}

func WithWebhookPort(port int) Option {
	return &WebhookPort{port: port}
}

func (w *WebhookPort) ApplyToOptions(o *Options) {
	o.WebhookPort = w.port
}

type K3sImage struct {
	image string
}

func WithK3sImage(image string) Option {
	return &K3sImage{image: image}
}

func (k *K3sImage) ApplyToOptions(o *Options) {
	o.K3sImage = k.image
}

type CertValidity struct {
	duration time.Duration
}

func WithCertValidity(duration time.Duration) Option {
	return &CertValidity{duration: duration}
}

func (c *CertValidity) ApplyToOptions(o *Options) {
	o.CertValidity = c.duration
}
