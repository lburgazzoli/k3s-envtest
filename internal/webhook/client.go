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
	"strconv"

	admissionv1 "k8s.io/api/admission/v1"
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
//	    webhook.WithClientTimeout(10*time.Second),
//	)
//
//	// Struct style
//	client, err := webhook.NewClient("localhost", 9443, &webhook.ClientOptions{
//	    CACert:  caCert,
//	    Timeout: 10 * time.Second,
//	})
//
// If no CA certificate is provided, the client will skip TLS verification (insecure).
func NewClient(host string, port int, opts ...ClientOption) (*Client, error) {
	if host == "" {
		return nil, errors.New("host cannot be empty")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port: %d (must be 1-65535)", port)
	}

	options := &ClientOptions{
		Timeout: DefaultTimeout,
	}
	options.ApplyOptions(opts)

	tlsConfig, err := buildTLSConfig(options)
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}

	httpClient := &http.Client{
		Timeout: options.Timeout,
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

// Call sends an AdmissionReview request to the specified webhook path and
// returns the AdmissionReview response.
//
// The method POSTs the review as JSON to https://{host}:{port}{path} and
// parses the response. It returns an error for 5xx status codes but accepts
// 2xx, 3xx, and 4xx responses (since webhooks may reject payloads with 4xx
// but are still functioning correctly).
func (c *Client) Call(
	ctx context.Context,
	path string,
	review admissionv1.AdmissionReview,
) (*admissionv1.AdmissionReview, error) {
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

func buildTLSConfig(opts *ClientOptions) (*tls.Config, error) {
	if len(opts.CACert) > 0 {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(opts.CACert) {
			return nil, errors.New("failed to parse CA certificate")
		}
		return &tls.Config{
			RootCAs:    caCertPool,
			MinVersion: tls.VersionTLS12,
		}, nil
	}

	//nolint:gosec // InsecureSkipVerify is intentional for testing without CA certs
	return &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}, nil
}
