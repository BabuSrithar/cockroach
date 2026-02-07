# Adding Cockroach Cloud as a New Provider to Roachprod

## Executive Summary

This document provides a comprehensive guide on how to add Cockroach Cloud as a new cloud vendor to the roachprod framework, similar to existing providers like AWS, GCP, Azure, and IBM Cloud. This implementation demonstrates the extensible architecture of roachprod and provides a blueprint for adding any new cloud provider.

## Architecture Overview

### What is Roachprod?

Roachprod is CockroachDB's cluster provisioning and management tool that provides:
- VM lifecycle management across multiple cloud providers
- Cluster creation, configuration, and destruction
- Multi-cloud support with a unified interface
- Testing and development environment setup

### Provider Architecture

The roachprod framework follows a **Provider Pattern** where each cloud vendor implements a standard `vm.Provider` interface. This design enables:

1. **Abstraction**: Cloud-specific details are encapsulated within each provider
2. **Uniformity**: All providers expose the same operations (Create, Delete, List, etc.)
3. **Extensibility**: New providers can be added without modifying core roachprod code
4. **Flexibility**: Each provider can have unique configuration options

### Directory Structure

```
pkg/roachprod/
├── roachprod.go              # Core orchestration and provider initialization
├── vm/
│   ├── vm.go                 # Provider interface definition
│   ├── aws/                  # AWS provider implementation
│   ├── gce/                  # Google Cloud provider implementation
│   ├── azure/                # Azure provider implementation
│   ├── ibm/                  # IBM Cloud provider implementation
│   ├── ccloud/               # Cockroach Cloud provider (NEW)
│   │   ├── provider.go       # Main provider implementation
│   │   ├── provider_opts.go  # Provider-specific options
│   │   ├── provider_test.go  # Unit tests
│   │   ├── BUILD.bazel       # Bazel build configuration
│   │   └── README.md         # Provider documentation
│   └── local/                # Local provider (for testing)
```

## The Provider Interface

Every cloud provider must implement the `vm.Provider` interface defined in `pkg/roachprod/vm/vm.go`. The interface includes:

### Core Lifecycle Methods

```go
type Provider interface {
    // Configuration
    ConfigureProviderFlags(*pflag.FlagSet, MultipleProjectsOption)
    ConfigureClusterCleanupFlags(*pflag.FlagSet)
    CreateProviderOpts() ProviderOpts
    
    // VM Lifecycle
    Create(l *logger.Logger, names []string, opts CreateOpts, providerOpts ProviderOpts) (List, error)
    Delete(l *logger.Logger, vms List) error
    List(ctx context.Context, l *logger.Logger, opts ListOptions) (List, error)
    Reset(l *logger.Logger, vms List) error
    
    // Cluster Operations
    Grow(l *logger.Logger, vms List, clusterName string, names []string) (List, error)
    Shrink(l *logger.Logger, vmsToRemove List, clusterName string) error
    Extend(l *logger.Logger, vms List, lifetime time.Duration) error
    
    // Provider Info
    Name() string
    Active() bool
    ProjectActive(project string) bool
    FindActiveAccount(l *logger.Logger) (string, error)
    
    // Optional Features
    CreateVolume(...) (Volume, error)
    AttachVolume(...) (string, error)
    CreateLoadBalancer(...) error
    SupportsSpotVMs() bool
    // ... and more
}
```

### Key Concepts

1. **Centralized vs Distributed Providers**:
   - Centralized providers (like Cockroach Cloud) manage state on their servers
   - Distributed providers (like AWS, GCP) require local state management

2. **Provider Options**: Each provider can define custom flags and configuration

3. **VM Abstraction**: Providers return generic `vm.VM` structs, hiding cloud-specific details

## Implementation Steps

### Step 1: Create Provider Package

Create a new directory `pkg/roachprod/vm/ccloud/` for the Cockroach Cloud provider.

### Step 2: Implement the Provider Interface

**File: `provider.go`**

```go
package ccloud

import (
    "context"
    "time"
    "github.com/cockroachdb/cockroach/pkg/roachprod/logger"
    "github.com/cockroachdb/cockroach/pkg/roachprod/vm"
)

const (
    ProviderName = "ccloud"
    apiKeyEnvVar = "COCKROACH_CLOUD_API_KEY"
)

type Provider struct {
    opts ProviderOpts
    apiKey string
    dnsProvider vm.DNSProvider
}

// Implement all interface methods...
func (p *Provider) Name() string {
    return ProviderName
}

func (p *Provider) Active() bool {
    return p.apiKey != ""
}

// ... (see full implementation in provider.go)
```

**Key Design Decisions for Cockroach Cloud**:

1. **Centralized Provider**: Set `IsCentralizedProvider()` to return `true`
   - Cockroach Cloud manages all state centrally
   - Simplifies local state management
   - Aligns with Cockroach Cloud's architecture

2. **API-First Design**: Use Cockroach Cloud's REST API for all operations
   - Authentication via API key (`COCKROACH_CLOUD_API_KEY`)
   - No need for CLI tool dependencies

3. **Managed SSH**: Cockroach Cloud handles SSH configuration
   - `ConfigSSH()` and `CleanSSH()` are no-ops
   - Simplifies security model

### Step 3: Provider Initialization

**Init Function Pattern**:

```go
func Init() (err error) {
    // Check if provider should be enabled
    hasCliOrEnv := func() bool {
        if os.Getenv(apiKeyEnvVar) != "" {
            return true
        }
        if _, err := exec.LookPath("ccloud"); err == nil {
            return true
        }
        return false
    }

    if !hasCliOrEnv() {
        // Register as inactive stub
        vm.Providers[ProviderName] = flagstub.New(&Provider{}, ErrMissingAuth.Error())
        return nil
    }

    // Create and register active provider
    providerInstance, err = NewProvider()
    if err != nil {
        vm.Providers[ProviderName] = flagstub.New(&Provider{}, err.Error())
        return nil
    }

    vm.Providers[ProviderName] = providerInstance
    return nil
}
```

**Why This Pattern?**:
- Graceful degradation: Provider appears in `roachprod status-providers` even if disabled
- Clear error messages guide users to set up authentication
- Doesn't break roachprod if Cockroach Cloud isn't configured

### Step 4: Register Provider in Roachprod

**File: `pkg/roachprod/roachprod.go`**

Add import:
```go
import (
    // ... other imports
    "github.com/cockroachdb/cockroach/pkg/roachprod/vm/ccloud"
)
```

Register in `InitProviders()`:
```go
func InitProviders() map[string]string {
    providersState := make(map[string]string)

    for _, prov := range []struct {
        name  string
        init  func() error
        empty vm.Provider
    }{
        // ... existing providers
        {
            name:  ccloud.ProviderName,
            init:  ccloud.Init,
            empty: &ccloud.Provider{},
        },
        // ...
    } {
        // Initialization logic
    }
    
    return providersState
}
```

### Step 5: Add Bazel Build Configuration

**File: `BUILD.bazel`**

```bazel
load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "ccloud",
    srcs = [
        "provider.go",
        "provider_opts.go",
    ],
    importpath = "github.com/cockroachdb/cockroach/pkg/roachprod/vm/ccloud",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/roachprod/logger",
        "//pkg/roachprod/vm",
        "//pkg/roachprod/vm/flagstub",
        "@com_github_cockroachdb_errors//:errors",
        "@com_github_spf13_pflag//:pflag",
    ],
)

go_test(
    name = "ccloud_test",
    srcs = ["provider_test.go"],
    embed = [":ccloud"],
    deps = [
        "//pkg/roachprod/logger",
        "//pkg/roachprod/vm",
        "@com_github_stretchr_testify//assert",
        "@com_github_stretchr_testify//require",
    ],
)
```

### Step 6: Create Tests

**File: `provider_test.go`**

```go
package ccloud

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestProviderInterface(t *testing.T) {
    // Verify interface implementation
    var _ vm.Provider = &Provider{}
}

func TestProviderActive(t *testing.T) {
    tests := []struct {
        name     string
        apiKey   string
        expected bool
    }{
        {"active with API key", "test-key", true},
        {"inactive without API key", "", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p := &Provider{apiKey: tt.apiKey}
            assert.Equal(t, tt.expected, p.Active())
        })
    }
}
```

## Implementation Details

### Authentication

The Cockroach Cloud provider supports two authentication methods:

1. **Environment Variable** (Recommended):
   ```bash
   export COCKROACH_CLOUD_API_KEY="your-api-key"
   ```

2. **CLI Tool** (Optional):
   ```bash
   ccloud auth login
   ```

### Provider Flags

Custom flags allow users to configure Cockroach Cloud-specific options:

```go
func (p *Provider) ConfigureProviderFlags(flags *pflag.FlagSet, _ vm.MultipleProjectsOption) {
    flags.StringVar(&p.opts.MachineType, "ccloud-machine-type", defaultMachineType,
        "Machine type for Cockroach Cloud VMs")
    flags.StringSliceVar(&p.opts.Zones, "ccloud-zones", nil,
        "Zones for Cockroach Cloud cluster")
}
```

Usage:
```bash
roachprod create my-cluster --clouds ccloud --ccloud-machine-type n1-standard-8
```

### VM Creation Flow

1. User runs: `roachprod create my-cluster --clouds ccloud -n 3`
2. Roachprod calls `ccloud.Provider.Create()`
3. Provider:
   - Validates configuration
   - Calls Cockroach Cloud API to create cluster
   - Maps API response to `vm.VM` structs
   - Returns list of VMs
4. Roachprod caches VM metadata locally

### State Management

**Centralized Model** (Cockroach Cloud):
- All cluster state lives on Cockroach Cloud servers
- `List()` always queries the API for current state
- Local cache only for performance optimization

**Distributed Model** (AWS, GCP):
- State tracked locally in `~/.roachprod/`
- `List()` may combine local cache + cloud queries
- More complex state reconciliation

## Extending the Implementation

The current implementation provides a **foundation** with placeholder methods. To complete it:

### 1. Add Cockroach Cloud API Client

```go
type APIClient struct {
    baseURL string
    apiKey  string
    httpClient *http.Client
}

func (c *APIClient) CreateCluster(opts CreateClusterOpts) (*Cluster, error) {
    // Make API call to Cockroach Cloud
}
```

### 2. Implement Create() Method

```go
func (p *Provider) Create(
    l *logger.Logger, names []string, opts vm.CreateOpts, providerOpts vm.ProviderOpts,
) (vm.List, error) {
    ccloudOpts := providerOpts.(*ProviderOpts)
    
    // Create cluster via API
    cluster, err := p.apiClient.CreateCluster(CreateClusterOpts{
        Name: names[0],
        MachineType: ccloudOpts.MachineType,
        Zones: ccloudOpts.Zones,
        NumNodes: len(names),
    })
    if err != nil {
        return nil, err
    }
    
    // Map to vm.VM structs
    vms := make(vm.List, len(cluster.Nodes))
    for i, node := range cluster.Nodes {
        vms[i] = &vm.VM{
            Name: node.Name,
            Provider: ProviderName,
            ProviderID: node.ID,
            PublicIP: node.PublicIP,
            PrivateIP: node.PrivateIP,
            CreatedAt: node.CreatedAt,
        }
    }
    
    return vms, nil
}
```

### 3. Implement Other Lifecycle Methods

Follow similar patterns for:
- `Delete()`: Call API to destroy cluster
- `List()`: Query API for all clusters
- `Extend()`: Update cluster lifetime
- `AddLabels()`: Tag clusters via API

## Testing Strategy

### Unit Tests
- Test each method with mocked API responses
- Verify interface compliance
- Test error handling

### Integration Tests
- Require actual Cockroach Cloud account
- Test full lifecycle (Create → Use → Delete)
- Verify cleanup on test failures

### Example Test
```go
func TestCreateAndDelete(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    provider, err := NewProvider()
    require.NoError(t, err)
    
    vms, err := provider.Create(logger.New(), []string{"test-vm-1"}, vm.CreateOpts{}, &ProviderOpts{})
    require.NoError(t, err)
    defer provider.Delete(logger.New(), vms)
    
    assert.Len(t, vms, 1)
    assert.NotEmpty(t, vms[0].PublicIP)
}
```

## Comparison with Other Providers

### AWS Provider
- **Complexity**: High (Terraform, VPCs, security groups)
- **API**: boto3/AWS SDK
- **State**: Hybrid (local + cloud)
- **SSH**: Manual setup required

### GCE Provider
- **Complexity**: Medium (gcloud CLI)
- **API**: gcloud + REST API
- **State**: Hybrid
- **SSH**: Managed by GCE

### Cockroach Cloud Provider
- **Complexity**: Low (fully managed)
- **API**: REST API only
- **State**: Centralized
- **SSH**: Managed by Cockroach Cloud

## Best Practices

1. **Error Handling**:
   - Return descriptive errors
   - Use `errors.Wrap()` for context
   - Log important operations

2. **Idempotency**:
   - Make operations safe to retry
   - Check if resources exist before creating

3. **Resource Cleanup**:
   - Always implement proper Delete()
   - Use labels for garbage collection
   - Set reasonable default lifetimes

4. **Documentation**:
   - Maintain comprehensive README
   - Document all configuration options
   - Provide usage examples

## Future Enhancements

1. **Volume Management**: Implement persistent volume operations
2. **Load Balancers**: Add load balancer support for clusters
3. **Multi-Region**: Support cross-region cluster deployment
4. **Monitoring Integration**: Export metrics to Prometheus
5. **Cost Optimization**: Support for spot instances (if available)

## Troubleshooting

### Provider Shows as Inactive

**Check**:
1. Is `COCKROACH_CLOUD_API_KEY` set?
2. Is the API key valid?
3. Run `roachprod status-providers` to see error details

### VM Creation Fails

**Common Causes**:
1. Insufficient quota
2. Invalid machine type
3. Region/zone not available
4. API rate limiting

### Tests Fail

**Debugging**:
```bash
# Run with verbose output
./dev test pkg/roachprod/vm/ccloud -v

# Run specific test
./dev test pkg/roachprod/vm/ccloud -f TestProviderActive
```

## Conclusion

Adding Cockroach Cloud as a provider demonstrates roachprod's extensible architecture. The implementation:

1. ✅ Follows established patterns from AWS, GCE, Azure, IBM
2. ✅ Provides a clean abstraction over Cockroach Cloud
3. ✅ Integrates seamlessly with roachprod commands
4. ✅ Serves as a blueprint for future providers

The framework is designed to make adding new cloud vendors straightforward - just implement the `Provider` interface, register in `InitProviders()`, and you're done!

## References

- [Roachprod Documentation](../README.md)
- [Provider Interface](../vm.go)
- [AWS Provider Implementation](../aws/aws.go)
- [IBM Provider Implementation](../ibm/provider.go)
- [Cockroach Cloud Documentation](https://www.cockroachlabs.com/docs/cockroachcloud/)
