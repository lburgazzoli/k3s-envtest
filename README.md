# k3s-envtest

A Go library for creating lightweight k3s-based test environments, similar to how envtest provides Kubernetes API server environments for testing.

[![Go Reference](https://pkg.go.dev/badge/github.com/lburgazzoli/k3s-envtest.svg)](https://pkg.go.dev/github.com/lburgazzoli/k3s-envtest)

## Features

- 🚀 **Lightweight k3s containers** using testcontainers-go
- 📁 **Directory-based manifest loading** - no kustomize dependency  
- 🔧 **Automatic CRD installation** and establishment waiting
- 🔐 **Built-in webhook testing** with auto-generated TLS certificates
- ⚙️ **Structured configuration** with environment variable support
- 🧪 **Minimal dependencies** - only 10 direct dependencies
- 🐳 **Docker networking support** for webhook testing

## Quick Start

### Prerequisites

- Go 1.24.8 or later
- Docker running locally (for testcontainers)

### Installation

```bash
go get github.com/lburgazzoli/k3s-envtest
```

### Basic Usage

```go
package main

import (
    "context"
    "log"
    
    "github.com/lburgazzoli/k3s-envtest/pkg/k3senv"
)

func main() {
    ctx := context.Background()
    
    // Create a k3s test environment
    env, err := k3senv.New(
        k3senv.WithManifests("testdata/crds"),
        k3senv.WithAutoInstallWebhooks(true),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Start the environment
    if err := env.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer env.Stop(ctx)
    
    // Use the Kubernetes client
    client := env.Client()
    config := env.Config()
    
    // Your test code here...
}
```

## Configuration

k3s-envtest supports both functional options and structured configuration:

### Functional Options

```go
env, err := k3senv.New(
    k3senv.WithK3sImage("rancher/k3s:v1.32.9-k3s1"),
    k3senv.WithWebhookPort(9443),
    k3senv.WithCertPath("/tmp/certs"),
    k3senv.WithManifests("testdata/crds", "testdata/webhooks"),
    k3senv.WithAutoInstallWebhooks(true),
    k3senv.WithK3sArgs("--disable=traefik", "--disable=metrics-server"),
    k3senv.WithLogger(t), // Enable container log redirection
)
```

### Structured Configuration

```go
env, err := k3senv.New(&k3senv.Options{
    K3s: k3senv.K3sConfig{
        Image:          "rancher/k3s:v1.32.9-k3s1",
        Args:           []string{"--disable=traefik"},
        LogRedirection: k3senv.Bool(false), // Use Bool() for pointer booleans
    },
    Webhook: k3senv.WebhookConfig{
        Port:               9443,
        AutoInstall:        k3senv.Bool(true),  // Use Bool() for pointer booleans
        CheckReadiness:     k3senv.Bool(false), // Explicit false now possible
        ReadyTimeout:       30 * time.Second,
        HealthCheckTimeout: 5 * time.Second,
        PollInterval:       500 * time.Millisecond, // Webhook polling rate
    },
    CRD: k3senv.CRDConfig{
        ReadyTimeout: 60 * time.Second,
        PollInterval: 100 * time.Millisecond, // Faster CRD polling
    },
    Certificate: k3senv.CertificateConfig{
        Path:     "/tmp/certs",
        Validity: 24 * time.Hour,
    },
    Manifest: k3senv.ManifestConfig{
        Paths: []string{"testdata/crds", "testdata/webhooks"},
    },
    Logger: t, // Enable container log redirection
})
```

> **Note:** Boolean configuration fields (`AutoInstall`, `CheckReadiness`, `LogRedirection`) use pointer types (`*bool`) to distinguish between "not set" and "explicitly false". Use `k3senv.Bool(value)` or `ptr.To(value)` from `k8s.io/utils/ptr`. Functional options like `WithAutoInstallWebhooks()` handle this automatically.

### Environment Variables

Configuration is automatically loaded from environment variables with the `K3SENV_` prefix when calling `New()`:

```bash
export K3SENV_K3S_IMAGE="rancher/k3s:v1.32.9-k3s1"
export K3SENV_WEBHOOK_PORT=9443
export K3SENV_WEBHOOK_AUTO_INSTALL=true
export K3SENV_WEBHOOK_POLL_INTERVAL=500ms
export K3SENV_CRD_POLL_INTERVAL=100ms
export K3SENV_CERTIFICATE_PATH="/tmp/certs"
```

```go
// Environment variables are automatically loaded
env, err := k3senv.New(
    // Explicit options override environment variables
    k3senv.WithManifests("testdata/extra"),
)
```

## Performance Configuration

k3s-envtest provides component-specific polling intervals for optimal performance:

### Default Poll Intervals

- **CRD Polling**: 100ms (fast) - Quick establishment detection
- **Webhook Polling**: 500ms (moderate) - Balanced performance

### Custom Poll Intervals

Configure different polling rates for different components:

```go
// Fast webhook polling for responsive webhook testing
env, err := k3senv.New(&k3senv.Options{
    Webhook: k3senv.WebhookConfig{
        PollInterval: 100 * time.Millisecond,
    },
    CRD: k3senv.CRDConfig{
        PollInterval: 50 * time.Millisecond, // Very fast CRD polling
    },
})

// Or via environment variables
export K3SENV_WEBHOOK_POLL_INTERVAL=200ms
export K3SENV_CRD_POLL_INTERVAL=50ms
```

### Performance Tips

- **Faster CRD polling** (50-100ms) for quick test startup
- **Slower webhook polling** (500ms-1s) for resource-intensive webhook operations
- **Use Logger** to monitor timing and adjust intervals as needed

## Advanced Features

### Webhook Testing

k3s-envtest provides comprehensive webhook testing capabilities with controller-runtime integration:

```go
import (
    ctrl "sigs.k8s.io/controller-runtime"
)

env, err := k3senv.New(
    k3senv.WithManifests("testdata/webhooks"),
    k3senv.WithAutoInstallWebhooks(true),
)
if err != nil {
    return err
}

// Start k3s environment first
if err := env.Start(ctx); err != nil {
    return err
}
defer env.Stop(ctx)

// Create manager with pre-configured webhook server
mgr, err := ctrl.NewManager(env.Config(), ctrl.Options{
    Scheme:        scheme,
    WebhookServer: env.WebhookServer(),
})
if err != nil {
    return err
}

// Register your webhooks
mgr.GetWebhookServer().Register("/validate", &myValidator{})
mgr.GetWebhookServer().Register("/mutate", &myMutator{})

// Start manager
go func() {
    if err := mgr.Start(ctx); err != nil {
        log.Printf("Manager error: %v", err)
    }
}()

// Webhooks are now active and configured
```

### Custom Resource Definitions

CRDs are automatically installed and waited for establishment:

```go
env, err := k3senv.New(k3senv.WithManifests("testdata/crds"))
if err != nil {
    return err
}

if err := env.Start(ctx); err != nil {
    return err
}
defer env.Stop(ctx)

// CRDs are now established and ready for use
client := env.Client()
```

### Manifest Organization

Organize your test manifests in directories:

```
testdata/
├── crds/
│   ├── myresource.yaml
│   └── anotherresource.yaml
├── webhooks/
│   ├── validating-webhook.yaml
│   └── mutating-webhook.yaml
└── samples/
    └── sample-cr.yaml
```

k3s-envtest automatically processes:
- CustomResourceDefinitions (`apiextensions.k8s.io/v1`)
- ValidatingWebhookConfigurations (`admissionregistration.k8s.io/v1`)
- MutatingWebhookConfigurations (`admissionregistration.k8s.io/v1`)

### Logging and Debugging

Enable debug logging and container log redirection to see what k3s-envtest is doing:

```go
import "testing"

func TestWithLogging(t *testing.T) {
    env, err := k3senv.New(
        k3senv.WithLogger(t), // Use testing.T as logger
        k3senv.WithManifests("testdata/crds"),
    )
    // ...
}
```

When a Logger is configured, k3s-envtest provides:
- **Debug logging**: Operations, timing, and configuration details
- **Container log redirection**: k3s container logs are forwarded to your logger with `[k3s]` prefix
- **Performance insights**: Monitor polling intervals and timing to optimize configuration

Example log output:
```
[k3senv] Starting k3s environment with image: rancher/k3s:v1.32.9-k3s1
[k3s] 2024/01/15 10:30:15 [INFO]  Starting k3s server
[k3senv] Generated certificates in: /tmp/k3senv-certs-abc123
[k3senv] Loaded 5 manifests
[k3s] 2024/01/15 10:30:20 [INFO]  Kubernetes API server listening on port 6443
[k3senv] k3s environment started successfully
```

## Examples

### Testing a Controller

```go
func TestMyController(t *testing.T) {
    g := NewWithT(t)
    ctx := context.Background()
    
    env, err := k3senv.New(
        k3senv.WithLogger(t),
        k3senv.WithManifests("testdata/crds"),
    )
    g.Expect(err).NotTo(HaveOccurred())
    
    g.Expect(env.Start(ctx)).To(Succeed())
    defer env.Stop(ctx)
    
    // Start your controller
    mgr, err := manager.New(env.Config(), manager.Options{
        Scheme: env.Scheme(),
    })
    g.Expect(err).NotTo(HaveOccurred())
    
    err = (&MyController{}).SetupWithManager(mgr)
    g.Expect(err).NotTo(HaveOccurred())
    
    go func() {
        defer GinkgoRecover()
        err := mgr.Start(ctx)
        g.Expect(err).NotTo(HaveOccurred())
    }()
    
    // Test your controller logic...
}
```

### Testing Webhooks

```go
func TestWebhookValidation(t *testing.T) {
    g := NewWithT(t)
    ctx := context.Background()
    
    env, err := k3senv.New(
        k3senv.WithLogger(t),
        k3senv.WithManifests("testdata/crds", "testdata/webhooks"),
        k3senv.WithAutoInstallWebhooks(true),
    )
    g.Expect(err).NotTo(HaveOccurred())
    
    g.Expect(env.Start(ctx)).To(Succeed())
    defer env.Stop(ctx)
    
    // Create manager with pre-configured webhook server
    mgr, err := ctrl.NewManager(env.Config(), ctrl.Options{
        Scheme:        scheme,
        WebhookServer: env.WebhookServer(),
    })
    g.Expect(err).NotTo(HaveOccurred())
    
    // Register your webhooks
    mgr.GetWebhookServer().Register("/validate", &myValidator{})
    
    go func() {
        defer GinkgoRecover()
        err := mgr.Start(ctx)
        g.Expect(err).NotTo(HaveOccurred())
    }()
    
    // Test webhook validation...
    client := env.Client()
    obj := &MyCustomResource{...}
    
    err = client.Create(ctx, obj)
    // Assert webhook behavior...
}
```

### Parallel Testing

k3s-envtest supports running tests in parallel (`t.Parallel()`), but webhook tests require unique ports for each parallel test. The library provides port discovery utilities to handle this automatically.

**Important**: Port 0 (auto-assignment) is not supported because webhook URLs must be constructed before the webhook server starts. Use `FindAvailablePort()` instead.

#### Basic Parallel Testing

```go
func TestWebhook_Parallel(t *testing.T) {
    t.Parallel() // Enable parallel execution
    g := NewWithT(t)
    
    // Find an available port for this test
    port, err := k3senv.FindAvailablePort()
    g.Expect(err).NotTo(HaveOccurred())
    
    env, err := k3senv.New(
        k3senv.WithWebhookPort(port),
        k3senv.WithObjects(webhook),
    )
    g.Expect(err).NotTo(HaveOccurred())
    
    t.Cleanup(func() {
        _ = env.Stop(context.Background())
    })
    
    err = env.Start(context.Background())
    g.Expect(err).NotTo(HaveOccurred())
    
    // Test your webhook...
}
```

#### Port Range Constraints

If you need to constrain ports to a specific range (e.g., firewall rules):

```go
// Only use ports in allowed range
port, err := k3senv.FindAvailablePortInRange(9443, 9543)
if err != nil {
    t.Skip("No available port in allowed range")
}

env, err := k3senv.New(k3senv.WithWebhookPort(port))
```

#### Test Helper Pattern

For multiple parallel tests, create a helper function:

```go
func setupParallelEnv(t *testing.T, opts ...k3senv.Option) *k3senv.K3sEnv {
    t.Helper()
    g := NewWithT(t)
    
    // Find available port
    port, err := k3senv.FindAvailablePort()
    g.Expect(err).NotTo(HaveOccurred())
    
    // Prepend port option
    allOpts := append([]k3senv.Option{k3senv.WithWebhookPort(port)}, opts...)
    
    env, err := k3senv.New(allOpts...)
    g.Expect(err).NotTo(HaveOccurred())
    
    t.Cleanup(func() {
        _ = env.Stop(context.Background())
    })
    
    return env
}

func TestWebhookA(t *testing.T) {
    t.Parallel()
    env := setupParallelEnv(t, k3senv.WithObjects(webhookA))
    // Test...
}

func TestWebhookB(t *testing.T) {
    t.Parallel()
    env := setupParallelEnv(t, k3senv.WithObjects(webhookB))
    // Test...
}
```

**Note**: There is a small race condition between finding a port and using it where another process could grab the port. In practice, this is extremely rare and negligible for testing purposes.

## Troubleshooting

### Docker Issues

**Problem**: `Failed to start k3s container`
**Solution**: Ensure Docker is running and accessible:
```bash
docker ps  # Should work without errors
```

### Port Conflicts

**Problem**: `Webhook port already in use`
**Solution**: Use a different port or let k3s-envtest choose one:
```go
env, err := k3senv.New(
    k3senv.WithWebhookPort(0), // Use random available port
)
```

### Certificate Issues

**Problem**: `Webhook TLS certificate errors`
**Solution**: k3s-envtest auto-generates certificates with proper SANs for Docker networking. If you see certificate errors, ensure the webhook server is started before calling `env.Start()`.

### Manifest Loading

**Problem**: `No CRDs found in directory`
**Solution**: Ensure your YAML files:
- Have `.yaml` or `.yml` extensions
- Contain valid Kubernetes resources
- Include `apiVersion`, `kind`, and `metadata.name`

### Environment Variables

**Problem**: Environment variables not being loaded
**Solution**: Use the correct prefix and format:
```bash
# Correct
export K3SENV_WEBHOOK_PORT=9443
export K3SENV_K3S_IMAGE="rancher/k3s:latest"

# Incorrect
export WEBHOOK_PORT=9443  # Missing K3SENV_ prefix
```

## Architecture

For detailed information about design decisions and architecture, see [docs/architecture.md](docs/architecture.md).

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test` and `make lint`
6. Submit a pull request

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.