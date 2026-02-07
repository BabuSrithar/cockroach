# Cockroach Cloud Provider for Roachprod

This package implements the Cockroach Cloud provider for the roachprod cluster management tool.

## Overview

The Cockroach Cloud provider allows roachprod to provision and manage CockroachDB clusters on Cockroach Cloud infrastructure. It implements the standard `vm.Provider` interface, enabling seamless integration with roachprod's cluster lifecycle management.

## Setup

### Prerequisites

1. **Cockroach Cloud API Key**: Obtain an API key from your Cockroach Cloud account.

2. **Environment Variable**: Set the `COCKROACH_CLOUD_API_KEY` environment variable:
   ```bash
   export COCKROACH_CLOUD_API_KEY="your-api-key-here"
   ```

3. **Cockroach Cloud CLI (Optional)**: Install the `ccloud` CLI tool for enhanced functionality:
   ```bash
   # Installation instructions for ccloud CLI
   # (To be added based on actual CLI availability)
   ```

### Verification

To verify that the Cockroach Cloud provider is properly configured:

```bash
roachprod status-providers
```

You should see the Cockroach Cloud provider listed as "Active".

## Usage

### Creating a Cluster

```bash
roachprod create my-cluster --clouds ccloud --ccloud-zones us-east-1a,us-east-1b -n 3
```

### Listing Clusters

```bash
roachprod list --provider ccloud
```

### Deleting a Cluster

```bash
roachprod destroy my-cluster
```

## Configuration Options

The Cockroach Cloud provider supports the following configuration flags:

- `--ccloud-machine-type`: Specifies the machine type for VMs (default: `n1-standard-4`)
- `--ccloud-zones`: Comma-separated list of availability zones

## Architecture

The Cockroach Cloud provider follows the standard roachprod provider architecture:

1. **Provider Interface**: Implements all required methods from `vm.Provider`
2. **Centralized Model**: Uses Cockroach Cloud's centralized API for cluster management
3. **Authentication**: Uses API key-based authentication
4. **State Management**: Leverages Cockroach Cloud's native state tracking

## Key Features

- **Centralized Management**: All cluster state is managed by Cockroach Cloud
- **API-Driven**: Uses Cockroach Cloud's REST API for all operations
- **SSH Management**: SSH configuration is handled automatically by Cockroach Cloud
- **Multi-Region Support**: Supports deployment across multiple cloud regions

## Implementation Status

The Cockroach Cloud provider is currently in development. The following features are implemented:

- [x] Provider registration and initialization
- [x] Basic provider interface implementation
- [ ] VM creation using Cockroach Cloud API
- [ ] VM deletion and lifecycle management
- [ ] Cluster listing and discovery
- [ ] Label management
- [ ] Volume operations
- [ ] Load balancer support

## Development

### Adding New Features

When extending the Cockroach Cloud provider:

1. Update the relevant method in `provider.go`
2. Add any necessary API client code
3. Update this README with new functionality
4. Add tests in `provider_test.go`

### Testing

```bash
# Run provider tests
./dev test pkg/roachprod/vm/ccloud
```

## Troubleshooting

### Provider Shows as Inactive

If the Cockroach Cloud provider appears as inactive:

1. Verify that `COCKROACH_CLOUD_API_KEY` is set correctly
2. Check that the API key has the necessary permissions
3. Ensure the `ccloud` CLI is installed (if using CLI-based auth)

### API Errors

If you encounter API errors:

1. Verify your API key is valid
2. Check that your Cockroach Cloud account has sufficient quota
3. Review Cockroach Cloud API status page for service issues

## References

- [Roachprod Documentation](../../README.md)
- [Cockroach Cloud Documentation](https://www.cockroachlabs.com/docs/cockroachcloud/)
- [Provider Interface](../vm.go)

## Contributing

Contributions to the Cockroach Cloud provider are welcome! Please follow the standard CockroachDB contribution guidelines.
