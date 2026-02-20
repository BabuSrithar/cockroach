// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package numa

import (
	"fmt"
	"strings"
)

// NUMANode represents a single NUMA node and the CPUs assigned to it.
type NUMANode struct {
	// ID is the node number (e.g. 0, 1, 2).
	ID int
	// CPUs is the CPU list string (e.g. "0-3,8-11").
	CPUs string
}

// NUMATopology describes the NUMA topology of the machine.
type NUMATopology struct {
	// Nodes contains the detected NUMA nodes.
	Nodes []NUMANode
}

// String returns a human-readable summary of the NUMA topology.
func (t *NUMATopology) String() string {
	if len(t.Nodes) == 0 {
		return "no NUMA nodes detected"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d NUMA node(s)", len(t.Nodes))
	for _, n := range t.Nodes {
		fmt.Fprintf(&b, "; node %d cpus: %s", n.ID, n.CPUs)
	}
	return b.String()
}
