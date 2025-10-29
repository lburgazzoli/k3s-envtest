package webhook

import "time"

// Default values for webhook operations.
const (
	DefaultCallTimeout  = 10 * time.Second
	DefaultPollInterval = 100 * time.Millisecond
	DefaultReadyTimeout = 30 * time.Second
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
	// CACert is the CA certificate for verifying the webhook server's TLS certificate.
	// If empty, TLS verification will be skipped (insecure).
	CACert []byte
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
}

// WithClientCACert configures the CA certificate for TLS verification.
// If not provided, the client will use InsecureSkipVerify.
func WithClientCACert(cert []byte) ClientOption {
	return clientOptionFunc(func(o *ClientOptions) {
		o.CACert = cert
	})
}

// CallOption configures individual Call method invocations.
type CallOption interface {
	ApplyToCallOptions(opts *CallOptions)
}

type callOptionFunc func(*CallOptions)

func (f callOptionFunc) ApplyToCallOptions(opts *CallOptions) {
	f(opts)
}

// CallOptions contains per-call configuration.
type CallOptions struct {
	// Timeout for the HTTP request.
	// Default: 10s
	Timeout time.Duration
}

// WithCallTimeout sets a custom timeout for a single Call invocation.
func WithCallTimeout(timeout time.Duration) CallOption {
	return callOptionFunc(func(opts *CallOptions) {
		opts.Timeout = timeout
	})
}

// WaitOption configures the WaitForEndpoints method.
type WaitOption interface {
	ApplyToWaitOptions(opts *WaitOptions)
}

type waitOptionFunc func(*WaitOptions)

func (f waitOptionFunc) ApplyToWaitOptions(opts *WaitOptions) {
	f(opts)
}

// WaitOptions contains configuration for endpoint readiness polling.
type WaitOptions struct {
	// PollInterval is how often to retry failed endpoints.
	// Default: 100ms
	PollInterval time.Duration

	// ReadyTimeout is the maximum time to wait per endpoint.
	// Default: 30s
	ReadyTimeout time.Duration

	// CallTimeout is the timeout for each individual health check call.
	// Default: 10s
	CallTimeout time.Duration
}

// WithPollInterval sets the interval between readiness check retries.
func WithPollInterval(interval time.Duration) WaitOption {
	return waitOptionFunc(func(opts *WaitOptions) {
		opts.PollInterval = interval
	})
}

// WithReadyTimeout sets the maximum time to wait for each endpoint.
func WithReadyTimeout(timeout time.Duration) WaitOption {
	return waitOptionFunc(func(opts *WaitOptions) {
		opts.ReadyTimeout = timeout
	})
}

// WithWaitCallTimeout sets the timeout for individual health check calls during waiting.
func WithWaitCallTimeout(timeout time.Duration) WaitOption {
	return waitOptionFunc(func(opts *WaitOptions) {
		opts.CallTimeout = timeout
	})
}

func (opts *WaitOptions) ApplyOptions(options []WaitOption) {
	for _, opt := range options {
		opt.ApplyToWaitOptions(opts)
	}
}
