// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

//go:build !linux

package numa

import (
	"os"

	"github.com/cockroachdb/errors"
)

// SysfsNodePath is the standard sysfs location for NUMA node information. On
// non-Linux platforms, this path does not exist.
const SysfsNodePath = "/sys/devices/system/node"

// GetNUMATopology is a stub on non-Linux platforms. NUMA topology detection
// relies on the Linux sysfs interface and is not available on other operating
// systems.
func GetNUMATopology(basePath string) (*NUMATopology, error) {
	return nil, errors.Wrap(os.ErrNotExist, "NUMA topology detection is only supported on Linux")
}
