// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

//go:build !linux

package system

import "github.com/cockroachdb/errors"

// GetNUMATopology returns an error on non-Linux platforms as NUMA detection
// is only supported on Linux via /sys/devices/system/node.
func GetNUMATopology() (*NUMATopology, error) {
	return nil, errors.New("NUMA detection is only supported on Linux")
}
