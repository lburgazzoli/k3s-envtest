package webhook

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Client is a webhook testing client that simplifies making calls to
// webhook endpoints with AdmissionReview payloads.
type Client struct {
	host       string
	port       int
	httpClient *http.Client
	opts       ClientOptions
}

// NewClient creates a new webhook client for testing webhook endpoints.
// The client will make HTTPS requests to https://{host}:{port}{path}.
//
// Options can be provided in functional style, struct style, or mixed:
//
//	// Functional style
//	client, err := webhook.NewClient("localhost", 9443,
//	    webhook.WithClientCACert(caCert),
//	)
//
//	// Struct style
//	client, err := webhook.NewClient("localhost", 9443, &webhook.ClientOptions{
//	    CACert: caCert,
//	})
//
// If no CA certificate is provided, the client will skip TLS verification (insecure).
// Per-call timeouts can be configured using WithCallTimeout() when calling Call().
func NewClient(host string, port int, opts ...ClientOption) (*Client, error) {
	if host == "" {
		return nil, errors.New("host cannot be empty")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port: %d (must be 1-65535)", port)
	}

	options := &ClientOptions{}
	options.ApplyOptions(opts)

	tlsConfig, err := buildTLSConfig(options)
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return &Client{
		host:       host,
		port:       port,
		httpClient: httpClient,
		opts:       *options,
	}, nil
}

// Address returns the base address (host:port) that the client connects to.
func (c *Client) Address() string {
	return net.JoinHostPort(c.host, strconv.Itoa(c.port))
}

// newHealthCheckReview creates a minimal AdmissionReview for health checking webhook endpoints.
func newHealthCheckReview() admissionv1.AdmissionReview {
	return admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID:       types.UID("00000000-0000-0000-0000-000000000000"),
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: []byte("{}")},
		},
	}
}

// Call sends an AdmissionReview request to the specified webhook path and
// returns the AdmissionReview response.
//
// Options can be provided to override the default settings:
//
//	response, err := client.Call(ctx, "/validate", review,
//	    webhook.WithCallTimeout(5*time.Second),
//	)
//
// The method POSTs the review as JSON to https://{host}:{port}{path} and
// parses the response. It returns an error for 5xx status codes but accepts
// 2xx, 3xx, and 4xx responses (since webhooks may reject payloads with 4xx
// but are still functioning correctly).
func (c *Client) Call(
	ctx context.Context,
	path string,
	review admissionv1.AdmissionReview,
	opts ...CallOption,
) (*admissionv1.AdmissionReview, error) {
	callOpts := &CallOptions{
		Timeout: DefaultCallTimeout,
	}
	for _, opt := range opts {
		opt.ApplyToCallOptions(callOpts)
	}

	// Apply timeout to context if specified
	if callOpts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOpts.Timeout)
		defer cancel()
	}

	if path == "" {
		path = "/"
	}

	hostPort := net.JoinHostPort(c.host, strconv.Itoa(c.port))
	url := fmt.Sprintf("https://%s%s", hostPort, path)

	body, err := json.Marshal(review)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal AdmissionReview: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("webhook returned server error: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var reviewResp admissionv1.AdmissionReview
	if err := json.Unmarshal(respBody, &reviewResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AdmissionReview response: %w", err)
	}

	return &reviewResp, nil
}

// WaitForEndpoints polls the given webhook URLs until they respond successfully
// or the context times out. It extracts the path from each URL and calls the
// webhook endpoint with a health check AdmissionReview.
//
// Options can be provided to configure polling behavior:
//
//	err := client.WaitForEndpoints(ctx, webhookURLs,
//	    webhook.WithPollInterval(200*time.Millisecond),
//	    webhook.WithReadyTimeout(60*time.Second),
//	    webhook.WithWaitCallTimeout(5*time.Second),
//	)
//
// The method will wait for each endpoint sequentially in the order provided.
func (c *Client) WaitForEndpoints(
	ctx context.Context,
	webhookURLs []string,
	opts ...WaitOption,
) error {
	waitOpts := &WaitOptions{
		PollInterval: DefaultPollInterval,
		ReadyTimeout: DefaultReadyTimeout,
		CallTimeout:  DefaultCallTimeout,
	}
	waitOpts.ApplyOptions(opts)

	healthCheckReview := newHealthCheckReview()

	for _, webhookURL := range webhookURLs {
		parsedURL, err := url.Parse(webhookURL)
		if err != nil {
			return fmt.Errorf("invalid webhook URL %s: %w", webhookURL, err)
		}

		path := parsedURL.Path
		if path == "" {
			path = "/"
		}

		err = wait.PollUntilContextTimeout(
			ctx,
			waitOpts.PollInterval,
			waitOpts.ReadyTimeout,
			true,
			func(ctx context.Context) (bool, error) {
				_, err := c.Call(ctx, path, healthCheckReview, WithCallTimeout(waitOpts.CallTimeout))
				return err == nil, nil
			},
		)

		if err != nil {
			return fmt.Errorf("webhook endpoint %s not ready: %w", path, err)
		}
	}

	return nil
}

func buildTLSConfig(opts *ClientOptions) (*tls.Config, error) {
	cfg := tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if len(opts.CACert) > 0 {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(opts.CACert) {
			return nil, errors.New("failed to parse CA certificate")
		}

		cfg.RootCAs = caCertPool
	} else {
		// InsecureSkipVerify is intentional for testing without CA certs
		//
		//nolint:gosec
		cfg.InsecureSkipVerify = true
	}

	return &cfg, nil
}
