package webhook

import "time"

const (
	DefaultTimeout = 5 * time.Second
)

// ClientOption is an interface for applying configuration to ClientOptions.
type ClientOption interface {
	ApplyToClientOptions(opts *ClientOptions)
}

// clientOptionFunc is an adapter that allows a simple function to be used as a ClientOption.
type clientOptionFunc func(*ClientOptions)

func (f clientOptionFunc) ApplyToClientOptions(o *ClientOptions) {
	f(o)
}

// ClientOptions contains configuration for the webhook client.
type ClientOptions struct {
	CACert  []byte
	Timeout time.Duration
}

// ApplyOptions applies a list of ClientOptions to the ClientOptions.
func (o *ClientOptions) ApplyOptions(opts []ClientOption) *ClientOptions {
	for _, opt := range opts {
		opt.ApplyToClientOptions(o)
	}
	return o
}

// ApplyToClientOptions implements the ClientOption interface, allowing ClientOptions
// to be used directly as an option (struct style initialization).
func (o *ClientOptions) ApplyToClientOptions(target *ClientOptions) {
	if len(o.CACert) > 0 {
		target.CACert = o.CACert
	}
	if o.Timeout != 0 {
		target.Timeout = o.Timeout
	}
}

// WithClientCACert configures the CA certificate for TLS verification.
// If not provided, the client will use InsecureSkipVerify.
func WithClientCACert(cert []byte) ClientOption {
	return clientOptionFunc(func(o *ClientOptions) {
		o.CACert = cert
	})
}

// WithClientTimeout configures the HTTP request timeout.
// Default is 5 seconds if not specified.
func WithClientTimeout(timeout time.Duration) ClientOption {
	return clientOptionFunc(func(o *ClientOptions) {
		o.Timeout = timeout
	})
}
