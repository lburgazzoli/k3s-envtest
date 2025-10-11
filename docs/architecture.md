# Architecture & Design Decisions

This document outlines the key architectural decisions and design patterns used in k3s-envtest.

## Overview

k3s-envtest provides a lightweight, containerized Kubernetes testing environment using k3s. The architecture emphasizes simplicity, testability, and minimal dependencies while providing comprehensive webhook and CRD testing capabilities.

## Core Design Principles

### 1. Container-First Approach

**Decision**: Use testcontainers-go with k3s containers instead of in-process Kubernetes components.

**Rationale**:
- **Isolation**: Each test gets a completely isolated Kubernetes cluster
- **Realistic Environment**: Tests run against real k3s, not mocked components
- **Docker Compatibility**: Leverages existing Docker infrastructure in CI/CD
- **Networking**: Proper container networking enables webhook testing

**Trade-offs**:
- ✅ High fidelity testing environment
- ✅ Complete isolation between tests
- ✅ Realistic networking behavior
- ❌ Slower startup compared to in-process solutions
- ❌ Requires Docker runtime

### 2. No Kustomize Dependency

**Decision**: Load pre-rendered YAML manifests from directories instead of using kustomize.

**Rationale**:
- **Simplicity**: Reduces complexity and dependency chain
- **Performance**: No templating overhead during test execution
- **Flexibility**: Works with any YAML generation tool
- **Debugging**: Easy to inspect actual manifests being loaded

**Implementation**:
- Uses `gopkg.in/yaml.v3` for multi-document YAML parsing
- Leverages `runtime.Decoder` for proper Kubernetes type handling
- Supports both files and directories with recursive scanning

### 3. Structured Configuration System

**Decision**: Replace flat Options struct with logical groupings and support multiple configuration methods.

**Evolution**:
```go
// Before: Flat structure
type Options struct {
    WebhookPort      int
    K3sImage         string
    CertDir          string
    AutoInstallWebhooks bool
    // ... many more fields
}

// After: Structured groupings
type Options struct {
    Webhook     WebhookConfig
    K3s         K3sConfig  
    Certificate CertificateConfig
    Manifest    ManifestConfig
    CRD         CRDConfig
}
```

**Benefits**:
- **Organization**: Related options are grouped logically
- **Maintainability**: Easier to extend and modify configuration
- **Environment Variables**: Natural mapping to nested env vars (`K3SENV_WEBHOOK_PORT`)
- **Multiple Configuration Methods**: Functional options, struct literals, environment variables

**Implementation Details**:
- Uses `mapstructure` tags for environment variable mapping
- Maintains backward compatibility through functional options
- Single source of truth via `NewDefaultOptions()`

### 4. Functional Options Pattern

**Decision**: Use functional options for flexible, extensible configuration.

**Pattern**:
```go
type Option interface {
    ApplyToOptions(opts *Options)
}

func WithWebhookPort(port int) Option { ... }
func WithK3sImage(image string) Option { ... }
```

**Benefits**:
- **Extensibility**: Easy to add new options without breaking changes
- **Composability**: Options can be combined and reused
- **Optional Parameters**: Clean handling of optional configuration
- **Type Safety**: Compile-time validation of option types

**Trade-offs**:
- ✅ Excellent API usability
- ✅ Backward compatible extensions
- ✅ Self-documenting option names
- ❌ Slightly more verbose than simple struct initialization

### 5. Automatic Certificate Management

**Decision**: Auto-generate TLS certificates with proper Subject Alternative Names (SANs) for Docker networking.

**Implementation**:
- Uses `github.com/mdelapenya/tlscert` for certificate generation
- Includes comprehensive SANs for Docker networking scenarios:
  - `host.docker.internal` - Docker Desktop
  - `host.testcontainers.internal` - Testcontainers
  - Container network gateway IPs (`172.17.0.1`, `172.18.0.1`, etc.)
  - Standard localhost variants

**Rationale**:
- **Zero Configuration**: Webhooks work out of the box
- **Docker Networking**: Handles complex container-to-container communication
- **Security**: Each test environment gets unique certificates
- **Debugging**: Certificates can be inspected for troubleshooting

### 6. JQ-Based Configuration Patching

**Decision**: Use JQ expressions for webhook configuration patching instead of manual field manipulation.

**Implementation**:
```go
err := ApplyJQTransform(webhookConfig, `
    .webhooks |= map(
        .clientConfig.url = "%s" + (.clientConfig.service.path // "/") |
        .clientConfig.caBundle = "%s" |
        del(.clientConfig.service)
    )
`, baseURL, caBundle)
```

**Benefits**:
- **Elegance**: Declarative transformation of complex nested structures
- **Flexibility**: Handles arbitrary webhook configurations
- **Maintainability**: Changes to transformation logic are localized
- **Correctness**: JQ is well-tested for JSON/YAML manipulation

### 7. Minimal Internal Packages

**Decision**: Extract only necessary functionality into internal packages.

**Package Structure**:
```
internal/
├── gvk/          # GroupVersionKind constants
├── resources/    # Resource conversion utilities  
└── testutil/     # Test utility functions
```

**Rationale**:
- **Focused Scope**: Each package has a single, clear responsibility
- **Minimal Surface Area**: Reduces maintenance burden
- **Clear Dependencies**: Obvious what functionality is being used

### 8. Lifecycle Management

**Decision**: Implement clear Start/Stop lifecycle with automatic cleanup.

**Pattern**:
```go
env, err := k3senv.New(options...)
if err := env.Start(ctx); err != nil { ... }
defer env.Stop(ctx)  // Automatic cleanup
```

**Features**:
- **Teardown Tasks**: Registered cleanup functions executed in reverse order
- **Resource Management**: Automatic cleanup of containers, certificates
- **Error Handling**: Graceful degradation when cleanup fails
- **Context Support**: Cancellation and timeout support

## Testing Philosophy

### Vanilla Gomega with Dot Imports

**Decision**: Standardize on vanilla Gomega assertions with dot imports.

**Pattern**:
```go
import . "github.com/onsi/gomega"

func TestMyFeature(t *testing.T) {
    g := NewWithT(t)
    g.Expect(result).To(Equal(expected))
    g.Expect(err).NotTo(HaveOccurred())
}
```

**Benefits**:
- **Readability**: Natural language assertions
- **Consistency**: Uniform testing patterns across codebase
- **Rich Matchers**: Comprehensive assertion library
- **Better Error Messages**: Descriptive failure output

### Logger Interface for Debugging

**Decision**: Provide a simple Logger interface compatible with `testing.T` with container log redirection.

**Implementation**:
```go
type Logger interface {
    Printf(format string, args ...interface{})
}

// Usage
env, err := k3senv.New(k3senv.WithLogger(t))
```

**Features**:
- **Debug Logging**: k3s-envtest operations with `[k3senv]` prefix
- **Container Log Redirection**: k3s container logs forwarded with `[k3s]` prefix
- **Performance Monitoring**: Poll interval timing and configuration details

**Benefits**:
- **Debugging**: Visibility into k3s-envtest operations and container behavior
- **Testing Integration**: Works seamlessly with Go testing
- **Flexibility**: Compatible with any logging framework
- **Troubleshooting**: Easy identification of timing and configuration issues

## Configuration Architecture

### Three-Layer Configuration Model

1. **Defaults** - Sensible defaults via `NewDefaultOptions()`
2. **Environment Variables** - External configuration via `K3SENV_*` variables  
3. **Explicit Options** - Runtime configuration via functional options

**Precedence**: Explicit Options > Environment Variables > Defaults

### Environment Variable Mapping

**Strategy**: Use Viper with mapstructure tags for automatic mapping.

**Example**:
```go
type WebhookConfig struct {
    Port        int           `mapstructure:"port"`
    AutoInstall bool          `mapstructure:"auto_install"`  
    Ready       time.Duration `mapstructure:"ready_timeout"`
}
```

**Environment Variables**:
```bash
K3SENV_WEBHOOK_PORT=9443
K3SENV_WEBHOOK_AUTO_INSTALL=true
K3SENV_WEBHOOK_READY_TIMEOUT=30s
```

## Performance Considerations

### Component-Specific Polling Intervals

**Decision**: Separate polling intervals for different component types based on their characteristics.

**Implementation**:
```go
type WebhookConfig struct {
    PollInterval time.Duration `mapstructure:"poll_interval"` // 500ms default
}

type CRDConfig struct {
    PollInterval time.Duration `mapstructure:"poll_interval"` // 100ms default
}
```

**Rationale**:
- **CRD Establishment**: Faster polling (100ms) for quick test startup since CRDs establish quickly
- **Webhook Health Checks**: Slower polling (500ms) to reduce load on webhook endpoints
- **Configurable Performance**: Users can tune intervals based on their specific requirements

**Benefits**:
- **Optimized Startup**: Faster CRD polling reduces overall test environment startup time
- **Resource Efficiency**: Slower webhook polling reduces unnecessary network traffic
- **Customizable**: Environment variables allow per-deployment tuning

### Container Startup Optimization

**Strategies**:
- Use specific k3s image tags to avoid pulls
- Leverage testcontainers reuse capabilities where possible
- Minimize k3s startup time with `--disable` flags for unused components
- Component-specific polling intervals reduce overall wait times

### Manifest Loading Efficiency

**Optimizations**:
- Single-pass directory scanning
- Efficient YAML parsing with `gopkg.in/yaml.v3`
- Resource filtering at load time (only CRDs and webhooks)
- Lazy loading of object conversion

### Memory Management

**Approach**:
- Cleanup certificates and temporary directories
- Properly terminate containers in teardown
- Avoid memory leaks in long-running test suites
- Container log redirection uses minimal buffering

## Extension Points

### Adding New Configuration Options

1. Add field to appropriate config struct with `mapstructure` tag
2. Create functional option function
3. Update `LoadConfigFromEnv()` viper defaults if needed
4. Update `ApplyToOptions()` method to handle the new field
5. Add environment variable documentation

### Supporting New Resource Types

1. Add GroupVersionKind constants to `internal/gvk/`
2. Update manifest loading logic in `k3senv_support.go`
3. Add processing logic in `Start()` method

### Custom Webhook Transformations

1. Extend `ApplyJQTransform` function
2. Add new JQ expressions for specific webhook types
3. Update webhook patching logic

## Future Considerations

### Potential Improvements

1. **Configuration Validation** - JSON Schema validation for configuration
2. **Plugin System** - Extensible hooks for custom initialization
3. **Parallel Testing** - Better support for concurrent test execution
4. **Enhanced Observability** - Metrics collection and structured logging events
5. **Advanced Performance Tuning** - Container reuse, caching, and adaptive polling intervals
6. **Log Filtering** - Configurable log levels and filtering for container logs

### Backward Compatibility

The architecture prioritizes backward compatibility:
- Functional options remain stable
- Default behavior is preserved
- New features are opt-in
- Deprecation warnings before breaking changes

## Conclusion

The k3s-envtest architecture balances simplicity with functionality, providing a robust foundation for Kubernetes testing while maintaining extensibility and ease of use. The structured configuration system, container-first approach, and comprehensive testing capabilities make it suitable for a wide range of Kubernetes development scenarios.