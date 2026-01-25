# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

k3s-envtest is a Go library for creating lightweight k3s-based test environments, similar to how envtest provides Kubernetes API server environments for testing.

This library was extracted from the opendatahub-operator project and made independent with:
- No kustomize dependency (replaced with directory-based YAML loading)
- Minimal dependencies (10 direct dependencies)
- Proper Kubernetes YAML parsing using gopkg.in/yaml.v3 + runtime.Decoder
- Clean separation from original codebase

## Recent Improvements (2024)

Recent architectural enhancements include:
- **Structured Configuration System** - Replaced flat Options with logical groupings (WebhookConfig, CRDConfig, K3sConfig, LoggingConfig, etc.)
- **Environment Variable Support** - Added Viper-based configuration with K3SENV_ prefix using mapstructure tags
- **Simplified K3sEnv Architecture** - Eliminated field duplication by using single Options struct directly
- **Centralized Default Options** - Created NewDefaultOptions() function in k3senv_opts.go for better organization
- **Enhanced Testing Framework** - Converted all tests to vanilla Gomega with dot imports for better readability
- **Custom K3s Arguments** - Added support for custom k3s server arguments via WithK3sArgs()
- **Network Configuration** - Support for custom Docker networks, network aliases, and network modes via WithK3sNetwork(), WithK3sNetworkAliases(), WithK3sNetworkMode()
- **Component-Specific Polling** - Separate PollInterval configuration for CRD (100ms) and Webhook (500ms) components
- **Container Log Redirection** - Forward k3s container logs to configurable Logger interface with [k3s] prefix
- **Testcontainers Lifecycle Logging** - Forward testcontainers framework logs with emoji filtering via [testcontainers] prefix
- **Structured Logging Interface** - Logger interface compatible with testing.T and debugging support
- **Generic JQ Functions** - Type-safe JQ queries (QueryTyped[T], QuerySlice[T], QueryMap[K,V]) reduce boilerplate by 70%
- **Pointer Booleans** - Boolean config fields use *bool to distinguish "not set" from "false" (breaking change)
- **Podman Compatibility** - Full support for Podman as container runtime using `host-gateway` mechanism

**Note:** Tests require Docker or Podman to be running as they spin up k3s containers using testcontainers-go.

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

### Testing with Podman

k3s-envtest supports Podman as an alternative to Docker:

```bash
# Set up Podman environment
export DOCKER_HOST=unix://$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')
export TESTCONTAINERS_RYUK_DISABLED=true

# Run tests
go test ./pkg/k3senv/...
```

**Requirements:**
- Podman 4.1+ (for `host-gateway` support)
- Podman machine running (macOS/Windows)
- **Rootful mode for CI**: k3s containers require privileged mode for cgroup access. GitHub Actions CI uses rootful Podman (`sudo systemctl enable --now podman.socket`) with socket at `/run/podman/podman.sock`

**Technical Details:**
- Uses `host.containers.internal:host-gateway` for container-to-host communication
- Uses `CustomizeRequest` with `ExtraHosts` instead of `WithHostConfigModifier` to avoid overwriting k3s module's privileged settings
- No special network configuration required
- **Privileged containers**: k3s module sets privileged mode automatically. Rootless Podman cannot grant true privileged access, causing cgroup permission errors

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
- Installs CRDs and webhook configurations with component-specific polling intervals
- Provides webhook server setup with auto-generated TLS certificates
- Redirects container logs to configurable Logger interface
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
// Environment variables (K3SENV_*) are automatically loaded
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

### Configuration System

The library now uses a structured configuration approach with logical groupings:

**Configuration Types:**
- `WebhookConfig` - Webhook port, auto-install, timeouts (ready, health check), polling interval (500ms default)
- `CRDConfig` - CRD establishment timeout and polling interval configuration (100ms default)
- `K3sConfig` - K3s container image and server arguments
- `CertificateConfig` - Certificate directory and validity settings
- `ManifestConfig` - Manifest paths and runtime objects

**Configuration Methods:**
1. **Functional Options Pattern** - `k3senv.WithWebhookPort(9443)`, `k3senv.WithK3sArgs("--disable=traefik")`
2. **Structured Options** - Direct `&k3senv.Options{Webhook: k3senv.WebhookConfig{Port: 9443}}`  
3. **Environment Variables** - `K3SENV_WEBHOOK_PORT=9443` using Viper + mapstructure (automatically loaded)

**Environment Variable Format:**
- Prefix: `K3SENV_`
- Nested config: `K3SENV_WEBHOOK_PORT`, `K3SENV_K3S_IMAGE`, `K3SENV_CERTIFICATE_PATH`
- Polling intervals: `K3SENV_WEBHOOK_POLL_INTERVAL`, `K3SENV_CRD_POLL_INTERVAL`
- Network config: `K3SENV_K3S_NETWORK_NAME`, `K3SENV_K3S_NETWORK_MODE`, `K3SENV_K3S_NETWORK_ALIASES`
- Automatically loaded by `k3senv.New()` - explicit options override environment variables

### Dependencies

**Direct Dependencies (11):**
- `github.com/testcontainers/testcontainers-go/modules/k3s` - K3s container management
- `sigs.k8s.io/controller-runtime` - Kubernetes client and webhook server
- `k8s.io/apimachinery` - Kubernetes type system
- `k8s.io/apiextensions-apiserver` - CRD types
- `k8s.io/client-go` - Kubernetes client libraries
- `github.com/itchyny/gojq` - JQ expression evaluation for webhook patching
- `github.com/mdelapenya/tlscert` - TLS certificate generation
- `gopkg.in/yaml.v3` - Multi-document YAML parsing
- `github.com/onsi/gomega` - Testing assertions
- `github.com/spf13/viper` - Configuration management
- `github.com/go-viper/mapstructure/v2` - Struct field mapping (added with Viper)

**Total Dependencies:** ~105 (expected for Kubernetes ecosystem due to controller-runtime)

### Common Patterns

**Modern Testing with Gomega (Recommended):**
All tests now use vanilla Gomega with dot imports for better readability:

```go
import . "github.com/onsi/gomega"

func TestMyController(t *testing.T) {
    g := NewWithT(t)
    ctx := context.Background()
    
    env, err := k3senv.New(
        k3senv.WithLogger(t), // Enable debug logging
        k3senv.WithManifests("testdata/crds"),
    )
    g.Expect(err).NotTo(HaveOccurred())

    g.Expect(env.Start(ctx)).To(Succeed())
    defer env.Stop(ctx)

    // CRDs are now installed and established
    client := env.Client()
    g.Expect(client).NotTo(BeNil())
}
```

**Testing with Webhooks:**
```go
func TestMyWebhook(t *testing.T) {
    g := NewWithT(t)
    ctx := context.Background()
    
    env, err := k3senv.New(
        k3senv.WithLogger(t),
        k3senv.WithManifests("testdata/webhooks"),
        k3senv.WithAutoInstallWebhooks(true),
    )
    g.Expect(err).NotTo(HaveOccurred())

    // Start webhook server
    srv := env.WebhookServer()
    srv.Register("/validate", myWebhookHandler)
    go func() {
        defer GinkgoRecover()
        _ = srv.Start(ctx)
    }()

    g.Expect(env.Start(ctx)).To(Succeed())
    defer env.Stop(ctx)

    // Webhooks are now active and configured
}
```

**Environment Configuration Testing:**
```go
func TestWithEnvConfig(t *testing.T) {
    g := NewWithT(t)
    
    // Test environment variable configuration
    opts, err := k3senv.LoadConfigFromEnv()
    g.Expect(err).NotTo(HaveOccurred())
    g.Expect(opts.Webhook.Port).To(Equal(k3senv.DefaultWebhookPort))
    
    // Environment variables are automatically loaded
    env, err := k3senv.New(
        k3senv.WithLogger(t),
        k3senv.WithManifests("testdata/extra"),
    )
    g.Expect(err).NotTo(HaveOccurred())
}
```

**Structured Configuration Testing:**
```go
func TestStructuredConfig(t *testing.T) {
    g := NewWithT(t)
    
    env, err := k3senv.New(&k3senv.Options{
        Webhook: k3senv.WebhookConfig{
            Port: 9443,
            AutoInstall: k3senv.Bool(true),  // Use Bool() helper for pointer
        },
        K3s: k3senv.K3sConfig{
            Image: "rancher/k3s:latest",
            Args: []string{"--disable=traefik"},
            LogRedirection: k3senv.Bool(false),  // Explicit false now possible
        },
        Certificate: k3senv.CertificateConfig{
            Path: t.TempDir(),
        },
    })
    g.Expect(err).NotTo(HaveOccurred())
    g.Expect(env).NotTo(BeNil())
}
```

## Architecture & Design

For detailed information about architectural decisions, design patterns, and implementation details, see:
- **[README.md](README.md)** - User documentation and examples
- **[docs/architecture.md](docs/architecture.md)** - Architectural decisions and design rationale

## Key Files

**Main Package (`pkg/k3senv/`):**
- `k3senv.go` - Core K3sEnv struct and lifecycle management
- `k3senv_opts.go` - Configuration system, options, and environment variable support
- `k3senv_support.go` - Certificate generation, manifest loading, and JQ transforms
- `k3senv_*_test.go` - Comprehensive test suite using Gomega

**Internal Packages:**
- `internal/gvk/` - GroupVersionKind constants for resource identification
- `internal/jq/` - JQ transformation and query utilities with generic type-safe functions
- `internal/resources/` - Resource conversion and manipulation utilities
- `internal/testutil/` - Test utility functions for project root detection

### JQ Transformation Utilities (`internal/jq`)

The `internal/jq` package provides utilities for transforming and querying Kubernetes unstructured objects using JQ expressions.

**Available Functions:**

1. **`Transform(obj, expression, args...)`** - Mutates object in place with JQ transformation
2. **`Query(obj, expression, args...)`** - Returns raw `interface{}` result (for dynamic types)
3. **`QueryTyped[T](obj, expression, args...)`** - Returns typed single value (generic)
4. **`QuerySlice[T](obj, expression, args...)`** - Returns typed slice (generic)
5. **`QueryMap[K, V](obj, expression, args...)`** - Returns typed map (generic)

**When to Use Each:**

- Use **`Transform`** when modifying objects (webhook configs, CRDs)
- Use **`Query`** when you need dynamic type handling or complex nested structures
- Use **`QueryTyped[T]`** when extracting a single value of known type
- Use **`QuerySlice[T]`** when extracting arrays of known element type
- Use **`QueryMap[K,V]`** when extracting maps with known key/value types

**Examples:**

```go
import "github.com/lburgazzoli/k3s-envtest/internal/jq"

// Extract a single string value
name, err := jq.QueryTyped[string](obj, `.metadata.name`)

// Extract a boolean
enabled, err := jq.QueryTyped[bool](obj, `.spec.enabled`)

// Extract a slice of strings (e.g., webhook URLs)
urls, err := jq.QuerySlice[string](obj, `[.webhooks[].clientConfig.url]`)

// Extract a slice of numbers (JSON numbers are float64)
ports, err := jq.QuerySlice[float64](obj, `[.spec.ports[].port]`)

// Extract a map
labels, err := jq.QueryMap[string, string](obj, `.metadata.labels`)

// Transform an object in place
err := jq.Transform(obj, `.spec.replicas = %d`, 3)

// Complex transformation with JQ expression
err := jq.Transform(webhookConfig, `
    .webhooks |= map(
        .clientConfig.url = "%s" + (.clientConfig.service.path // "/") |
        .clientConfig.caBundle = "%s" |
        del(.clientConfig.service)
    )
`, baseURL, caBundle)
```

**Benefits of Generic Functions:**

- **Type Safety**: Compile-time type checking for expected return types
- **Reduced Boilerplate**: Eliminates manual type assertions (14 lines → 3 lines for common cases)
- **Better Error Messages**: Clear type mismatch errors with expected vs actual types
- **IDE Support**: Better autocompletion and type inference

**Error Handling:**

All JQ functions return errors for:
- Invalid JQ expressions (parse errors)
- JQ execution errors (runtime errors)
- Type mismatches (when using generic functions)

Returning `nil` result with `nil` error is valid and indicates the JQ query found no matching data.