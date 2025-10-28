package k3senv

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/lburgazzoli/k3s-envtest/internal/jq"
	"github.com/lburgazzoli/k3s-envtest/internal/resources"
	"github.com/lburgazzoli/k3s-envtest/internal/webhook"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

func determineConvertibleCRDs(
	crds []unstructured.Unstructured,
	scheme *runtime.Scheme,
) ([]unstructured.Unstructured, error) {
	convertibles, err := resources.AllConvertibleTypes(scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to determine convertible types: %w", err)
	}

	var convertibleCRDs []unstructured.Unstructured
	for _, crd := range crds {
		group, err := jq.Query[string](&crd, `.spec.group`)
		if err != nil {
			return nil, fmt.Errorf("failed to extract group from CRD %s: %w", crd.GetName(), err)
		}
		if group == "" {
			return nil, fmt.Errorf("CRD %s missing required field: spec.group", crd.GetName())
		}

		kind, err := jq.Query[string](&crd, `.spec.names.kind`)
		if err != nil {
			return nil, fmt.Errorf("failed to extract kind from CRD %s: %w", crd.GetName(), err)
		}
		if kind == "" {
			return nil, fmt.Errorf("CRD %s missing required field: spec.names.kind", crd.GetName())
		}

		if convertibles.Has(schema.GroupKind{Group: group, Kind: kind}) {
			convertibleCRDs = append(convertibleCRDs, crd)
		}
	}

	return convertibleCRDs, nil
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

	// Create webhook client once for all endpoint checks
	webhookClient, err := webhook.NewClient(
		"127.0.0.1",
		port,
		webhook.WithClientCACert(e.certData.CACert),
		webhook.WithClientTimeout(e.options.Webhook.HealthCheckTimeout),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook client: %w", err)
	}

	// Minimal AdmissionReview for health check
	healthCheckReview := admissionv1.AdmissionReview{
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

		e.debugf("Checking webhook endpoint: %s (local path: %s)", webhookURL, endpointPath)

		err = wait.PollUntilContextTimeout(ctx, e.options.Webhook.PollInterval, e.options.Webhook.ReadyTimeout, true, func(ctx context.Context) (bool, error) {
			_, err := webhookClient.Call(ctx, endpointPath, healthCheckReview)
			return err == nil, nil
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
