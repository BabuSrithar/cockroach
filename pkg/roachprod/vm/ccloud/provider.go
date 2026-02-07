// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package ccloud

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/cockroachdb/cockroach/pkg/roachprod/logger"
	"github.com/cockroachdb/cockroach/pkg/roachprod/vm"
	"github.com/cockroachdb/cockroach/pkg/roachprod/vm/flagstub"
	"github.com/cockroachdb/errors"
	"github.com/spf13/pflag"
)

const (
	// ProviderName is the name used to identify this provider.
	ProviderName = "ccloud"

	// Environment variable for Cockroach Cloud API key
	apiKeyEnvVar = "COCKROACH_CLOUD_API_KEY"

	// Default configuration values
	defaultMachineType = "n1-standard-4"
	defaultRemoteUser  = "ubuntu"
)

var (
	// ErrMissingAuth is returned when neither environment variable nor CLI is configured
	ErrMissingAuth = fmt.Errorf(
		"neither %s environment variable nor cockroach cloud CLI found in path",
		apiKeyEnvVar,
	)
)

// providerInstance is the global instance of the Cockroach Cloud provider used by the
// roachprod CLI.
var providerInstance *Provider

// Init initializes the Cockroach Cloud provider instance for the roachprod CLI.
func Init() (err error) {
	hasCliOrEnv := func() bool {
		// If the credentials environment variable is set, we can use the Cockroach Cloud API.
		if os.Getenv(apiKeyEnvVar) != "" {
			return true
		}

		// If the cockroach cloud CLI is installed, we assume the user wants
		// the Cockroach Cloud provider to be enabled.
		if _, err := exec.LookPath("ccloud"); err == nil {
			return true
		}

		// Nothing points to the Cockroach Cloud provider being used.
		return false
	}

	if !hasCliOrEnv() {
		vm.Providers[ProviderName] = flagstub.New(&Provider{}, ErrMissingAuth.Error())
		return nil
	}

	providerInstance, err = NewProvider()
	if err != nil {
		fmt.Printf("failed to create Cockroach Cloud provider: %v\n", err)
		vm.Providers[ProviderName] = flagstub.New(&Provider{}, err.Error())
		return nil
	}

	vm.Providers[ProviderName] = providerInstance
	return nil
}

// Provider implements the vm.Provider interface for Cockroach Cloud.
type Provider struct {
	opts ProviderOpts

	// API key for Cockroach Cloud
	apiKey string

	// DNS provider for the cluster
	dnsProvider vm.DNSProvider
}

// NewProvider creates a new Cockroach Cloud provider instance.
func NewProvider() (*Provider, error) {
	apiKey := os.Getenv(apiKeyEnvVar)
	if apiKey == "" {
		return nil, errors.Newf("Cockroach Cloud API key not found in environment variable %s", apiKeyEnvVar)
	}

	p := &Provider{
		apiKey: apiKey,
	}

	return p, nil
}

// ConfigureProviderFlags configures the flags for the Cockroach Cloud provider.
func (p *Provider) ConfigureProviderFlags(flags *pflag.FlagSet, _ vm.MultipleProjectsOption) {
	flags.StringVar(&p.opts.MachineType, "ccloud-machine-type", defaultMachineType,
		"Machine type for Cockroach Cloud VMs")
	flags.StringSliceVar(&p.opts.Zones, "ccloud-zones", nil,
		"Zones for Cockroach Cloud cluster (comma-separated)")
}

// ConfigureClusterCleanupFlags configures flags for cluster cleanup operations.
func (p *Provider) ConfigureClusterCleanupFlags(flags *pflag.FlagSet) {
	// No specific cleanup flags needed for initial implementation
}

// CreateProviderOpts returns the provider-specific options.
func (p *Provider) CreateProviderOpts() vm.ProviderOpts {
	return &p.opts
}

// CleanSSH cleans SSH configurations for the provider.
func (p *Provider) CleanSSH(l *logger.Logger) error {
	// No SSH cleanup needed for Cockroach Cloud as it manages SSH internally
	return nil
}

// IsCentralizedProvider returns true if the provider is centralized.
// Cockroach Cloud is a centralized provider.
func (p *Provider) IsCentralizedProvider() bool {
	return true
}

// ConfigSSH configures SSH for machines in the given zones.
func (p *Provider) ConfigSSH(l *logger.Logger, zones []string) error {
	// Cockroach Cloud manages SSH configuration internally
	l.Printf("SSH configuration is managed by Cockroach Cloud")
	return nil
}

// Create provisions new VMs in Cockroach Cloud.
func (p *Provider) Create(
	l *logger.Logger, names []string, opts vm.CreateOpts, providerOpts vm.ProviderOpts,
) (vm.List, error) {
	l.Printf("Creating %d VMs in Cockroach Cloud", len(names))
	
	// TODO: Implement actual VM creation using Cockroach Cloud API
	// This is a placeholder implementation
	vms := vm.List{}
	
	for _, name := range names {
		// Create VM representation
		vmInstance := vm.VM{
			Name:       name,
			CreatedAt:  time.Now(),
			Lifetime:   opts.Lifetime,
			RemoteUser: defaultRemoteUser,
			Provider:   ProviderName,
			ProviderID: fmt.Sprintf("ccloud-%s", name),
		}
		vms = append(vms, &vmInstance)
	}
	
	return vms, nil
}

// Grow adds new VMs to an existing cluster.
func (p *Provider) Grow(
	l *logger.Logger, vms vm.List, clusterName string, names []string,
) (vm.List, error) {
	l.Printf("Growing cluster %s with %d new VMs", clusterName, len(names))
	
	// TODO: Implement cluster growth
	return nil, errors.New("Grow not yet implemented for Cockroach Cloud provider")
}

// Shrink removes VMs from a cluster.
func (p *Provider) Shrink(l *logger.Logger, vmsToRemove vm.List, clusterName string) error {
	l.Printf("Shrinking cluster %s by %d VMs", clusterName, len(vmsToRemove))
	
	// TODO: Implement cluster shrinking
	return errors.New("Shrink not yet implemented for Cockroach Cloud provider")
}

// Reset resets the VMs to a clean state.
func (p *Provider) Reset(l *logger.Logger, vms vm.List) error {
	l.Printf("Resetting %d VMs", len(vms))
	
	// TODO: Implement VM reset
	return errors.New("Reset not yet implemented for Cockroach Cloud provider")
}

// Delete removes VMs from Cockroach Cloud.
func (p *Provider) Delete(l *logger.Logger, vms vm.List) error {
	l.Printf("Deleting %d VMs from Cockroach Cloud", len(vms))
	
	// TODO: Implement VM deletion using Cockroach Cloud API
	return nil
}

// Extend extends the lifetime of VMs.
func (p *Provider) Extend(l *logger.Logger, vms vm.List, lifetime time.Duration) error {
	l.Printf("Extending lifetime of %d VMs by %s", len(vms), lifetime)
	
	// TODO: Implement lifetime extension
	return errors.New("Extend not yet implemented for Cockroach Cloud provider")
}

// FindActiveAccount returns the active Cockroach Cloud account.
func (p *Provider) FindActiveAccount(l *logger.Logger) (string, error) {
	// TODO: Implement account discovery
	return "cockroach-cloud-account", nil
}

// List retrieves the list of VMs from Cockroach Cloud.
func (p *Provider) List(ctx context.Context, l *logger.Logger, opts vm.ListOptions) (vm.List, error) {
	l.Printf("Listing VMs from Cockroach Cloud")
	
	// TODO: Implement VM listing using Cockroach Cloud API
	return vm.List{}, nil
}

// AddLabels adds labels to VMs.
func (p *Provider) AddLabels(l *logger.Logger, vms vm.List, labels map[string]string) error {
	l.Printf("Adding labels to %d VMs", len(vms))
	
	// TODO: Implement label addition
	return nil
}

// RemoveLabels removes labels from VMs.
func (p *Provider) RemoveLabels(l *logger.Logger, vms vm.List, labels []string) error {
	l.Printf("Removing labels from %d VMs", len(vms))
	
	// TODO: Implement label removal
	return nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return ProviderName
}

// Active returns true if the provider is properly configured and active.
func (p *Provider) Active() bool {
	return p.apiKey != ""
}

// ProjectActive returns true if the given project is active.
func (p *Provider) ProjectActive(project string) bool {
	// Cockroach Cloud uses a single account model
	return true
}

// CreateVolume creates a new volume.
func (p *Provider) CreateVolume(l *logger.Logger, vco vm.VolumeCreateOpts) (vm.Volume, error) {
	return vm.Volume{}, errors.New("CreateVolume not yet implemented for Cockroach Cloud provider")
}

// ListVolumes lists volumes attached to a VM.
func (p *Provider) ListVolumes(l *logger.Logger, vmInstance *vm.VM) ([]vm.Volume, error) {
	return nil, errors.New("ListVolumes not yet implemented for Cockroach Cloud provider")
}

// DeleteVolume deletes a volume.
func (p *Provider) DeleteVolume(l *logger.Logger, volume vm.Volume, vmInstance *vm.VM) error {
	return errors.New("DeleteVolume not yet implemented for Cockroach Cloud provider")
}

// AttachVolume attaches a volume to a VM.
func (p *Provider) AttachVolume(l *logger.Logger, volume vm.Volume, vmInstance *vm.VM) (string, error) {
	return "", errors.New("AttachVolume not yet implemented for Cockroach Cloud provider")
}

// CreateVolumeSnapshot creates a snapshot of a volume.
func (p *Provider) CreateVolumeSnapshot(
	l *logger.Logger, volume vm.Volume, vsco vm.VolumeSnapshotCreateOpts,
) (vm.VolumeSnapshot, error) {
	return vm.VolumeSnapshot{}, errors.New("CreateVolumeSnapshot not yet implemented for Cockroach Cloud provider")
}

// ListVolumeSnapshots lists volume snapshots.
func (p *Provider) ListVolumeSnapshots(
	l *logger.Logger, vslo vm.VolumeSnapshotListOpts,
) ([]vm.VolumeSnapshot, error) {
	return nil, errors.New("ListVolumeSnapshots not yet implemented for Cockroach Cloud provider")
}

// DeleteVolumeSnapshots deletes volume snapshots.
func (p *Provider) DeleteVolumeSnapshots(l *logger.Logger, snapshots ...vm.VolumeSnapshot) error {
	return errors.New("DeleteVolumeSnapshots not yet implemented for Cockroach Cloud provider")
}

// SupportsSpotVMs returns whether the provider supports spot VMs.
func (p *Provider) SupportsSpotVMs() bool {
	return false
}

// GetPreemptedSpotVMs returns preempted spot VMs.
func (p *Provider) GetPreemptedSpotVMs(
	l *logger.Logger, vms vm.List, since time.Time,
) ([]vm.PreemptedVM, error) {
	return nil, nil
}

// GetHostErrorVMs returns VMs that had host errors.
func (p *Provider) GetHostErrorVMs(l *logger.Logger, vms vm.List, since time.Time) ([]string, error) {
	return nil, nil
}

// GetLiveMigrationVMs returns VMs that had live migrations.
func (p *Provider) GetLiveMigrationVMs(l *logger.Logger, vms vm.List, since time.Time) ([]string, error) {
	return nil, nil
}

// GetVMSpecs returns VM specifications.
func (p *Provider) GetVMSpecs(l *logger.Logger, vms vm.List) (map[string]map[string]interface{}, error) {
	return nil, errors.New("GetVMSpecs not yet implemented for Cockroach Cloud provider")
}

// CreateLoadBalancer creates a load balancer.
func (p *Provider) CreateLoadBalancer(l *logger.Logger, vms vm.List, port int) error {
	return errors.New("CreateLoadBalancer not yet implemented for Cockroach Cloud provider")
}

// DeleteLoadBalancer deletes a load balancer.
func (p *Provider) DeleteLoadBalancer(l *logger.Logger, vms vm.List, port int) error {
	return errors.New("DeleteLoadBalancer not yet implemented for Cockroach Cloud provider")
}

// ListLoadBalancers lists load balancers.
func (p *Provider) ListLoadBalancers(l *logger.Logger, vms vm.List) ([]vm.ServiceAddress, error) {
	return nil, errors.New("ListLoadBalancers not yet implemented for Cockroach Cloud provider")
}

// String returns a string representation of the provider.
func (p *Provider) String() string {
	return fmt.Sprintf("Cockroach Cloud Provider (active: %t)", p.Active())
}
