# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

k3s-envtest is a Go library for creating lightweight k3s-based test environments, similar to how envtest provides Kubernetes API server environments for testing.

This library was extracted from the opendatahub-operator project and made independent with:
- No kustomize dependency (replaced with directory-based YAML loading)
- Minimal dependencies (10 direct dependencies)
- Proper Kubernetes YAML parsing using gopkg.in/yaml.v3 + runtime.Decoder
- Clean separation from original codebase

**Note:** Tests require Docker to be running as they spin up k3s containers using testcontainers-go.

## Development Commands

### Building
```bash
# Build all packages
go build ./...

# Clean build cache and test cache
make clean
```

### Testing
```bash
# Run all tests (requires Docker)
make test

# Run tests with verbose output
go test -v ./...

# Run a single test
go test -v -run TestName ./path/to/package

# Run tests with race detection
make test/race

# Run tests with coverage report
make test/cover
# Opens coverage.html in browser
```

### Linting
```bash
# Run linter (uses golangci-lint v2.5.0)
make lint

# Auto-fix issues where possible
make lint/fix

# Format code
make fmt
```

### Module Management
```bash
# Add dependencies
go get <package>

# Tidy dependencies
make deps
# or: go mod tidy

# Vendor dependencies (if needed)
go mod vendor
```

## Architecture

### Package Structure

```
pkg/k3senv/         # Main k3senv package
internal/
  gvk/              # GroupVersionKind constants for CRDs and webhooks
  resources/        # Resource conversion utilities
  testutil/         # Test utility functions
```

### Core Components

**K3sEnv** - Main test environment that:
- Spins up a k3s container using testcontainers-go
- Loads Kubernetes manifests from directories (YAML files)
- Installs CRDs and webhook configurations
- Provides webhook server setup with auto-generated TLS certificates
- Manages environment lifecycle (Start/Stop)

**Manifest Loading** - Loads pre-rendered YAML manifests from directories:
- Recursively scans directories for `.yaml` and `.yml` files
- Uses gopkg.in/yaml.v3 decoder for multi-document YAML iteration
- Leverages runtime.Decoder for proper Kubernetes type decoding
- Automatically categorizes resources by GVK (CRDs, webhook configs)
- No kustomize dependency - expects pre-rendered manifests
- Skips resources with missing Kind or empty documents

**Webhook Support** - Full webhook testing capabilities:
- Auto-generates TLS certificates with proper SANs for Docker networking
- Patches webhook configurations to use correct URLs and CA bundles
- Supports both admission webhooks and CRD conversion webhooks
- Health checks to ensure webhooks are ready before tests proceed

### Usage Patterns

#### Basic Setup
```go
env, err := k3senv.New(
    k3senv.WithManifestDir("testdata/manifests"),
    k3senv.WithAutoInstallWebhooks(true),
)
if err != nil {
    return err
}

if err := env.Start(ctx); err != nil {
    return err
}
defer env.Stop(ctx)

// Use env.Client() for Kubernetes operations
// Use env.Config() for REST config
```

#### Loading Manifests
Place CRD and webhook YAML files in directories:
```
testdata/
  manifests/
    crds/
      myresource.yaml
    webhooks/
      validating.yaml
```

Then load them:
```go
env, err := k3senv.New(
    k3senv.WithManifestDir("testdata/manifests/crds"),
    k3senv.WithManifestDir("testdata/manifests/webhooks"),
)
```

Manifests are automatically categorized by GVK. Only CRDs and webhook configurations are processed.

#### Webhook Testing
```go
env, err := k3senv.New(
    k3senv.WithManifestDir("testdata/webhooks"),
    k3senv.WithAutoInstallWebhooks(true),
)
if err != nil {
    return err
}

if err := env.Start(ctx); err != nil {
    return err
}
defer env.Stop(ctx)

// Webhook server runs on env.WebhookServer()
// Start your webhook handlers before or during Start()
webhookServer := env.WebhookServer()
// Register webhook handlers with webhookServer
go func() {
    _ = webhookServer.Start(ctx)
}()
```

### Key Design Decisions

1. **No Kustomize Dependency** - Expects pre-rendered YAML manifests in directories, reducing complexity and dependencies
2. **Minimal Internal Packages** - Only extracts what's needed from opendatahub-operator (GVKs, resource utils)
3. **Container-Based** - Uses testcontainers-go for k3s, ensuring isolation and Docker compatibility
4. **Webhook-First** - Built-in support for webhook testing with automatic certificate generation and configuration
5. **Lifecycle Management** - Clear Start/Stop lifecycle with teardown tasks for cleanup
6. **Proper YAML Parsing** - Uses gopkg.in/yaml.v3 + runtime.Decoder instead of fragile string splitting on `---`
7. **JQ Transforms** - Uses github.com/itchyny/gojq for elegant webhook configuration patching

### Dependencies

**Direct Dependencies (10):**
- `github.com/testcontainers/testcontainers-go/modules/k3s` - K3s container management
- `sigs.k8s.io/controller-runtime` - Kubernetes client and webhook server
- `k8s.io/apimachinery` - Kubernetes type system
- `k8s.io/apiextensions-apiserver` - CRD types
- `k8s.io/client-go` - Kubernetes client libraries
- `github.com/itchyny/gojq` - JQ expression evaluation for webhook patching
- `github.com/mdelapenya/tlscert` - TLS certificate generation
- `gopkg.in/yaml.v3` - Multi-document YAML parsing
- `github.com/onsi/gomega` - Testing assertions

**Total Dependencies:** ~105 (expected for Kubernetes ecosystem due to controller-runtime)

### Common Patterns

**Testing with CRDs:**
```go
func TestMyController(t *testing.T) {
    env, err := k3senv.New(k3senv.WithManifestDir("testdata/crds"))
    require.NoError(t, err)

    require.NoError(t, env.Start(context.Background()))
    defer env.Stop(context.Background())

    // CRDs are now installed and established
    // Use env.Client() to create custom resources
}
```

**Testing with Webhooks:**
```go
func TestMyWebhook(t *testing.T) {
    env, err := k3senv.New(
        k3senv.WithManifestDir("testdata/webhooks"),
        k3senv.WithAutoInstallWebhooks(true),
    )
    require.NoError(t, err)

    // Start webhook server
    srv := env.WebhookServer()
    srv.Register("/validate", myWebhookHandler)
    go srv.Start(context.Background())

    require.NoError(t, env.Start(context.Background()))
    defer env.Stop(context.Background())

    // Webhooks are now active and configured
}
```