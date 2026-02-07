// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package ccloud

import (
	"github.com/cockroachdb/cockroach/pkg/roachprod/vm"
)

// ProviderOpts provides Cockroach Cloud-specific configuration options.
type ProviderOpts struct {
	// MachineType specifies the machine type for VMs.
	MachineType string

	// Zones specifies the availability zones for the cluster.
	Zones []string

	// Region specifies the cloud region.
	Region string

	// NetworkConfig specifies network configuration.
	NetworkConfig string
}

// ConfigureCreateFlags is part of the ProviderOpts interface.
func (o *ProviderOpts) ConfigureCreateFlags(flags vm.ProviderFlags, opts vm.CreateOpts) error {
	// Configure flags for VM creation
	return nil
}

// ConfigureClusterFlags is part of the ProviderOpts interface.
func (o *ProviderOpts) ConfigureClusterFlags(flags vm.ProviderFlags, opts vm.ClusterSettingOpts) {
	// Configure flags for cluster operations
}
