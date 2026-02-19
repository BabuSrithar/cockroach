// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package numa

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
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

// nodeRegexp matches directories named "nodeN" where N is a non-negative
// integer.
var nodeRegexp = regexp.MustCompile(`^node(\d+)$`)

const SysfsNodePath = "/sys/devices/system/node"

// GetNUMATopology reads the NUMA topology from the given sysfs node directory.
// Use SysfsNodePath as the basePath for the standard location.
func GetNUMATopology(basePath string) (*NUMATopology, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading NUMA sysfs directory %s", basePath)
	}

	var nodes []NUMANode
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m := nodeRegexp.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		var nodeID int
		// The regexp guarantees a valid non-negative integer.
		nodeID, _ = strconv.Atoi(m[1])

		cpulistPath := filepath.Join(basePath, e.Name(), "cpulist")
		data, err := os.ReadFile(cpulistPath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading CPU list for NUMA node %d", nodeID)
		}
		cpus := strings.TrimSpace(string(data))
		nodes = append(nodes, NUMANode{ID: nodeID, CPUs: cpus})
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	return &NUMATopology{Nodes: nodes}, nil
}
