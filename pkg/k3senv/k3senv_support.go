package k3senv

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lburgazzoli/k3s-envtest/internal/gvk"
	"github.com/lburgazzoli/k3s-envtest/internal/jq"
	"github.com/lburgazzoli/k3s-envtest/internal/resources"
	"github.com/lburgazzoli/k3s-envtest/internal/testutil"
	"github.com/mdelapenya/tlscert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

type CertData struct {
	CACert     []byte
	ServerCert []byte
	ServerKey  []byte
}

func (cd *CertData) CABundle() []byte {
	return []byte(base64.StdEncoding.EncodeToString(cd.CACert))
}

func readFile(path string, elements ...string) ([]byte, error) {
	pathElements := []string{path}
	pathElements = append(pathElements, elements...)
	fullPath := filepath.Join(pathElements...)

	// filepath.Join cleans the path
	//
	//nolint:gosec
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", fullPath, err)
	}
	return data, nil
}

func (e *K3sEnv) createWebhookHTTPClient() (*http.Client, error) {
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(e.certData.CACert) {
		return nil, errors.New("failed to parse CA certificate")
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    caCertPool,
				MinVersion: tls.VersionTLS12,
			},
		},
	}, nil
}

func generateCertificates(
	certPath string,
	validity time.Duration,
) (*CertData, error) {
	if err := os.MkdirAll(certPath, DefaultCertDirPermission); err != nil {
		return nil, fmt.Errorf("failed to create cert directory: %w", err)
	}

	caCert := tlscert.SelfSignedFromRequest(tlscert.Request{
		Name:      "ca",
		Host:      "k3senv-ca",
		ValidFor:  validity,
		IsCA:      true,
		ParentDir: certPath,
	})

	serverCert := tlscert.SelfSignedFromRequest(tlscert.Request{
		Name:      "tls",
		Host:      strings.Join(CertificateSANs, ","),
		ValidFor:  validity,
		Parent:    caCert,
		ParentDir: certPath,
	})

	if caCert == nil || serverCert == nil {
		return nil, errors.New("failed to generate certificates")
	}

	caCertPEM, err := readFile(certPath, CACertFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}

	serverCertPEM, err := readFile(certPath, CertFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read server cert: %w", err)
	}

	serverKeyPEM, err := readFile(certPath, KeyFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read server key: %w", err)
	}

	return &CertData{
		CACert:     caCertPEM,
		ServerCert: serverCertPEM,
		ServerKey:  serverKeyPEM,
	}, nil
}

func determineConvertibleCRDs(
	crds []unstructured.Unstructured,
	scheme *runtime.Scheme,
) ([]unstructured.Unstructured, error) {
	convertibles := map[schema.GroupKind]struct{}{}
	for gvk := range scheme.AllKnownTypes() {
		obj, err := scheme.New(gvk)
		if err != nil {
			return nil, fmt.Errorf("failed to create a new API object for %s, %w", gvk, err)
		}
		if ok, err := conversion.IsConvertible(scheme, obj); ok && err == nil {
			convertibles[gvk.GroupKind()] = struct{}{}
		}
	}

	var convertibleCRDs []unstructured.Unstructured
	for _, crd := range crds {
		group, found, err := unstructured.NestedString(crd.Object, "spec", "group")
		if err != nil || !found {
			return nil, fmt.Errorf("failed to extract group from CRD %s: %w", crd.GetName(), err)
		}

		kind, found, err := unstructured.NestedString(crd.Object, "spec", "names", "kind")
		if err != nil || !found {
			return nil, fmt.Errorf("failed to extract kind from CRD %s: %w", crd.GetName(), err)
		}

		if _, ok := convertibles[schema.GroupKind{Group: group, Kind: kind}]; ok {
			convertibleCRDs = append(convertibleCRDs, crd)
		}
	}

	return convertibleCRDs, nil
}

// checkWebhookEndpoint performs a POST request to an HTTPS endpoint with a minimal AdmissionReview body.
// Returns an error if the endpoint is unreachable or returns a 5xx status code.
// Accepts 4xx responses since webhooks may reject the test payload but are still healthy.
func checkWebhookEndpoint(
	ctx context.Context,
	client *http.Client,
	url string,
	timeout time.Duration,
) error {
	// Create child context with timeout for this specific request
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Early bailout if context already cancelled
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	// Minimal AdmissionReview request for health check.
	// Webhooks expect POST with AdmissionReview body.
	body := []byte(`{
		"apiVersion": "admission.k8s.io/v1",
		"kind": "AdmissionReview",
		"request": {
			"uid": "00000000-0000-0000-0000-000000000000",
			"operation": "CREATE",
			"object": {}
		}
	}`)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request to %s: %w", url, err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to webhook endpoint %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Accept 2xx, 3xx, and 4xx status codes.
	// 4xx means webhook is responding but rejected our test payload (still healthy).
	// Only fail on 5xx (server errors).
	if resp.StatusCode >= 500 {
		return fmt.Errorf("returned error status: %d", resp.StatusCode)
	}

	return nil
}

func (e *K3sEnv) waitForWebhookEndpointsReady(
	ctx context.Context,
	webhookConfig *unstructured.Unstructured,
	port int,
) error {
	// Extract webhook endpoint URLs from the configuration using JQ
	webhookURLs, err := jq.QuerySlice[string](webhookConfig, `[.webhooks[].clientConfig.url]`)
	if err != nil {
		return fmt.Errorf("failed to extract webhook URLs: %w", err)
	}

	if len(webhookURLs) == 0 {
		e.debugf("No webhook endpoints found in config %s, skipping health check", webhookConfig.GetName())
		return nil
	}

	e.debugf("Checking %d webhook endpoints for %s...", len(webhookURLs), webhookConfig.GetName())

	// Create HTTP client once for all endpoint checks
	client, err := e.createWebhookHTTPClient()
	if err != nil {
		return fmt.Errorf("failed to create webhook HTTP client: %w", err)
	}

	// Check each webhook endpoint
	for _, webhookURL := range webhookURLs {
		parsedURL, err := url.Parse(webhookURL)
		if err != nil {
			return fmt.Errorf("invalid webhook URL %s: %w", webhookURL, err)
		}

		endpointPath := parsedURL.Path
		if endpointPath == "" {
			endpointPath = "/"
		}

		localURL := fmt.Sprintf("https://127.0.0.1:%d%s", port, endpointPath)
		e.debugf("Checking webhook endpoint: %s (local: %s)", webhookURL, localURL)

		err = wait.PollUntilContextTimeout(ctx, e.options.Webhook.PollInterval, e.options.Webhook.ReadyTimeout, true, func(ctx context.Context) (bool, error) {
			return checkWebhookEndpoint(ctx, client, localURL, e.options.Webhook.HealthCheckTimeout) == nil, nil
		})

		if err != nil {
			return fmt.Errorf("webhook endpoint %s not ready: %w", endpointPath, err)
		}

		e.debugf("Webhook endpoint %s is ready", endpointPath)
	}

	return nil
}

func extractNames(objs []unstructured.Unstructured) []string {
	names := make([]string, len(objs))
	for i := range objs {
		names[i] = objs[i].GetName()
	}
	return names
}

func loadManifestFromFile(
	filePath string,
) ([]unstructured.Unstructured, error) {
	data, err := readFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	manifests, err := resources.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode YAML from %s: %w", filePath, err)
	}

	var result []unstructured.Unstructured
	for i := range manifests {
		gvkType := manifests[i].GroupVersionKind()
		if gvkType == gvk.CustomResourceDefinition ||
			gvkType == gvk.MutatingWebhookConfiguration ||
			gvkType == gvk.ValidatingWebhookConfiguration {
			result = append(result, manifests[i])
		}
	}

	return result, nil
}

func loadManifestsFromDirFlat(
	dir string,
) ([]unstructured.Unstructured, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var result []unstructured.Unstructured
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		ext := strings.ToLower(filepath.Ext(fileName))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		filePath := filepath.Join(dir, fileName)
		manifests, err := loadManifestFromFile(filePath)
		if err != nil {
			return nil, err
		}
		result = append(result, manifests...)
	}

	return result, nil
}

func loadManifestPath(
	path string,
) ([]unstructured.Unstructured, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("manifest path does not exist: %s", path)
		}
		return nil, fmt.Errorf("failed to access manifest path %s: %w", path, err)
	}

	if info.IsDir() {
		return loadManifestsFromDirFlat(path)
	}

	return loadManifestFromFile(path)
}

func loadObjectsToManifests(
	scheme *runtime.Scheme,
	objects []client.Object,
) ([]unstructured.Unstructured, error) {
	result := make([]unstructured.Unstructured, 0, len(objects))
	for _, obj := range objects {
		if err := resources.EnsureGroupVersionKind(scheme, obj); err != nil {
			return nil, fmt.Errorf("failed to ensure GVK for object %T: %w", obj, err)
		}

		u, err := resources.ToUnstructured(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to convert object to unstructured: %w", err)
		}

		result = append(result, *u.DeepCopy())
	}

	return result, nil
}

func loadManifestsFromPaths(
	paths []string,
) ([]unstructured.Unstructured, error) {
	var result []unstructured.Unstructured

	for _, path := range paths {
		resolvedPath := path
		if !filepath.IsAbs(path) {
			projectRoot, err := testutil.FindProjectRoot()
			if err != nil {
				return nil, fmt.Errorf("failed to find project root for relative path %s: %w", path, err)
			}
			resolvedPath = filepath.Join(projectRoot, path)
		}

		if strings.ContainsAny(resolvedPath, "*?[]") {
			matches, err := filepath.Glob(resolvedPath)
			if err != nil {
				return nil, fmt.Errorf("invalid glob pattern %s: %w", resolvedPath, err)
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("glob pattern matched no files: %s", resolvedPath)
			}

			for _, match := range matches {
				manifests, err := loadManifestPath(match)
				if err != nil {
					return nil, err
				}
				result = append(result, manifests...)
			}
		} else {
			manifests, err := loadManifestPath(resolvedPath)
			if err != nil {
				return nil, err
			}
			result = append(result, manifests...)
		}
	}

	return result, nil
}

// FindAvailablePort finds an available TCP port on the local machine.
//
// The function binds to port 0, which causes the OS to assign any available port,
// then immediately closes the listener and returns the port number.
//
// Note: Go's net.Listen automatically sets SO_REUSEADDR on Unix-like systems,
// which allows the port to be reused even if it's in TIME_WAIT state. However,
// there is a small race condition window between closing the listener and actually
// using the port where another process could grab it. In practice, this is rare.
//
// This is useful for parallel testing where you need unique webhook ports:
//
//	port, err := k3senv.FindAvailablePort()
//	if err != nil {
//	    t.Fatal(err)
//	}
//	env, err := k3senv.New(k3senv.WithWebhookPort(port))
func FindAvailablePort() (int, error) {
	//nolint:noctx // Simple port discovery doesn't require context
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// FindAvailablePortInRange finds an available TCP port within the specified range.
//
// This is useful when you need to constrain ports to a specific range, for example
// when firewall rules only allow certain ports.
//
// The function tries each port in the range sequentially until it finds one that's
// available. Returns an error if no port is available in the range.
//
// Example usage:
//
//	// Only use ports allowed by firewall
//	port, err := k3senv.FindAvailablePortInRange(9443, 9543)
//	if err != nil {
//	    t.Skip("No available port in allowed range")
//	}
//	env, err := k3senv.New(k3senv.WithWebhookPort(port))
func FindAvailablePortInRange(minPort int, maxPort int) (int, error) {
	if minPort < 1 || maxPort > 65535 || minPort > maxPort {
		return 0, fmt.Errorf("invalid port range: %d-%d (must be 1-65535 and min <= max)", minPort, maxPort)
	}

	for port := minPort; port <= maxPort; port++ {
		//nolint:noctx // Simple port discovery doesn't require context
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue // Port not available, try next
		}
		_ = listener.Close()
		return port, nil
	}

	return 0, fmt.Errorf("no available port found in range %d-%d", minPort, maxPort)
}
