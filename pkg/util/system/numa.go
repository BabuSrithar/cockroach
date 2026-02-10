// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

//go:build linux

package system

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

// NUMATopology represents the NUMA topology of the system.
type NUMATopology struct {
	// Nodes is a list of NUMA node IDs present on the system.
	Nodes []int
	// NodeCPUs maps each NUMA node ID to the list of CPU IDs on that node.
	NodeCPUs map[int][]int
	// ProcessSpansNodes indicates whether the process is running across
	// multiple NUMA nodes.
	ProcessSpansNodes bool
}

// String returns a human-readable representation of the NUMA topology.
func (n *NUMATopology) String() string {
	if len(n.Nodes) == 0 {
		return "no NUMA nodes detected"
	}
	
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d NUMA node(s)", len(n.Nodes)))
	for _, nodeID := range n.Nodes {
		cpus := n.NodeCPUs[nodeID]
		sb.WriteString(fmt.Sprintf("; node %d: CPUs %v", nodeID, formatCPUList(cpus)))
	}
	return sb.String()
}

// formatCPUList formats a list of CPU IDs into a compact string representation.
// For example, [0, 1, 2, 3] becomes "0-3", and [0, 2, 4] becomes "0,2,4".
func formatCPUList(cpus []int) string {
	if len(cpus) == 0 {
		return ""
	}
	
	// Check if CPUs are consecutive
	consecutive := true
	for i := 1; i < len(cpus); i++ {
		if cpus[i] != cpus[i-1]+1 {
			consecutive = false
			break
		}
	}
	
	if consecutive {
		if len(cpus) == 1 {
			return strconv.Itoa(cpus[0])
		}
		return fmt.Sprintf("%d-%d", cpus[0], cpus[len(cpus)-1])
	}
	
	// Not consecutive, list them all
	parts := make([]string, len(cpus))
	for i, cpu := range cpus {
		parts[i] = strconv.Itoa(cpu)
	}
	return strings.Join(parts, ",")
}

// GetNUMATopology detects the NUMA topology of the system and whether the
// current process is running across multiple NUMA nodes.
func GetNUMATopology() (*NUMATopology, error) {
	return getNUMATopology("/sys/devices/system/node", "/proc/self/status")
}

// getNUMATopology is the internal implementation that accepts paths for testing.
func getNUMATopology(nodeDir, statusFile string) (*NUMATopology, error) {
	topology := &NUMATopology{
		NodeCPUs: make(map[int][]int),
	}
	
	// Check if NUMA is supported by checking if the node directory exists
	if _, err := os.Stat(nodeDir); os.IsNotExist(err) {
		return nil, errors.New("NUMA not supported: /sys/devices/system/node does not exist")
	}
	
	// Read the online nodes file to get the list of NUMA nodes
	onlineFile := filepath.Join(nodeDir, "online")
	onlineData, err := os.ReadFile(onlineFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read NUMA online nodes from %s", onlineFile)
	}
	
	// Parse the node list (e.g., "0" or "0-1" or "0,2,4")
	nodeList := strings.TrimSpace(string(onlineData))
	nodes, err := parseCPUList(nodeList)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse NUMA node list: %s", nodeList)
	}
	topology.Nodes = nodes
	
	// For each node, read the cpulist file to get the CPUs on that node
	for _, nodeID := range nodes {
		cpulistFile := filepath.Join(nodeDir, fmt.Sprintf("node%d", nodeID), "cpulist")
		cpulistData, err := os.ReadFile(cpulistFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read cpulist for NUMA node %d from %s", nodeID, cpulistFile)
		}
		
		cpuList := strings.TrimSpace(string(cpulistData))
		cpus, err := parseCPUList(cpuList)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse CPU list for NUMA node %d: %s", nodeID, cpuList)
		}
		topology.NodeCPUs[nodeID] = cpus
	}
	
	// Determine if the process spans multiple NUMA nodes
	allowedCPUs, err := getAllowedCPUs(statusFile)
	if err != nil {
		// If we can't determine allowed CPUs, assume spanning (conservative)
		topology.ProcessSpansNodes = len(nodes) > 1
	} else {
		topology.ProcessSpansNodes = processSpansNUMANodes(allowedCPUs, topology.NodeCPUs)
	}
	
	return topology, nil
}

// parseCPUList parses a CPU list string like "0-3" or "0,2,4" into a slice of ints.
func parseCPUList(list string) ([]int, error) {
	if list == "" {
		return nil, nil
	}
	
	var result []int
	parts := strings.Split(list, ",")
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			// Range like "0-3"
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, errors.Newf("invalid CPU range: %s", part)
			}
			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, errors.Wrapf(err, "invalid CPU range start: %s", rangeParts[0])
			}
			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, errors.Wrapf(err, "invalid CPU range end: %s", rangeParts[1])
			}
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
		} else {
			// Single CPU
			cpu, err := strconv.Atoi(part)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid CPU number: %s", part)
			}
			result = append(result, cpu)
		}
	}
	
	return result, nil
}

// getAllowedCPUs reads the Cpus_allowed_list from /proc/self/status
func getAllowedCPUs(statusFile string) ([]int, error) {
	file, err := os.Open(statusFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s", statusFile)
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Cpus_allowed_list:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			cpuList := strings.TrimSpace(parts[1])
			return parseCPUList(cpuList)
		}
	}
	
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrapf(err, "error reading %s", statusFile)
	}
	
	return nil, errors.New("Cpus_allowed_list not found in /proc/self/status")
}

// processSpansNUMANodes determines if the given set of allowed CPUs spans
// multiple NUMA nodes.
func processSpansNUMANodes(allowedCPUs []int, nodeCPUs map[int][]int) bool {
	// Build a set of allowed CPUs for quick lookup
	allowedSet := make(map[int]bool)
	for _, cpu := range allowedCPUs {
		allowedSet[cpu] = true
	}
	
	// Count how many NUMA nodes have at least one allowed CPU
	nodesWithAllowedCPUs := 0
	for _, cpus := range nodeCPUs {
		hasAllowedCPU := false
		for _, cpu := range cpus {
			if allowedSet[cpu] {
				hasAllowedCPU = true
				break
			}
		}
		if hasAllowedCPU {
			nodesWithAllowedCPUs++
		}
	}
	
	return nodesWithAllowedCPUs > 1
}
