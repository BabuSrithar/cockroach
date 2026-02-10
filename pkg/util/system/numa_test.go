// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCPUList(t *testing.T) {
	testCases := []struct {
		input    string
		expected []int
		hasError bool
	}{
		{"0", []int{0}, false},
		{"0-3", []int{0, 1, 2, 3}, false},
		{"0,2,4", []int{0, 2, 4}, false},
		{"0-1,4-5", []int{0, 1, 4, 5}, false},
		{"", nil, false},
		{"0-3,8-11", []int{0, 1, 2, 3, 8, 9, 10, 11}, false},
		{"invalid", nil, true},
		{"0-", nil, true},
		{"-3", nil, true},
	}
	
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := parseCPUList(tc.input)
			if tc.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestFormatCPUList(t *testing.T) {
	testCases := []struct {
		input    []int
		expected string
	}{
		{[]int{0}, "0"},
		{[]int{0, 1, 2, 3}, "0-3"},
		{[]int{0, 2, 4}, "0,2,4"},
		{[]int{}, ""},
	}
	
	for _, tc := range testCases {
		result := formatCPUList(tc.input)
		require.Equal(t, tc.expected, result)
	}
}

func TestProcessSpansNUMANodes(t *testing.T) {
	testCases := []struct {
		name        string
		allowedCPUs []int
		nodeCPUs    map[int][]int
		expected    bool
	}{
		{
			name:        "single node, all CPUs allowed",
			allowedCPUs: []int{0, 1, 2, 3},
			nodeCPUs:    map[int][]int{0: {0, 1, 2, 3}},
			expected:    false,
		},
		{
			name:        "single node, subset of CPUs allowed",
			allowedCPUs: []int{0, 1},
			nodeCPUs:    map[int][]int{0: {0, 1, 2, 3}},
			expected:    false,
		},
		{
			name:        "two nodes, CPUs from both nodes allowed",
			allowedCPUs: []int{0, 1, 4, 5},
			nodeCPUs:    map[int][]int{0: {0, 1, 2, 3}, 1: {4, 5, 6, 7}},
			expected:    true,
		},
		{
			name:        "two nodes, CPUs from only one node allowed",
			allowedCPUs: []int{0, 1},
			nodeCPUs:    map[int][]int{0: {0, 1, 2, 3}, 1: {4, 5, 6, 7}},
			expected:    false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := processSpansNUMANodes(tc.allowedCPUs, tc.nodeCPUs)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestGetNUMATopology(t *testing.T) {
	// Create a temporary directory structure mimicking /sys/devices/system/node
	tmpDir := t.TempDir()
	nodeDir := filepath.Join(tmpDir, "node")
	require.NoError(t, os.Mkdir(nodeDir, 0755))
	
	// Write the online nodes file
	require.NoError(t, os.WriteFile(filepath.Join(nodeDir, "online"), []byte("0-1\n"), 0644))
	
	// Create node0 directory and cpulist
	node0Dir := filepath.Join(nodeDir, "node0")
	require.NoError(t, os.Mkdir(node0Dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(node0Dir, "cpulist"), []byte("0-3\n"), 0644))
	
	// Create node1 directory and cpulist
	node1Dir := filepath.Join(nodeDir, "node1")
	require.NoError(t, os.Mkdir(node1Dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(node1Dir, "cpulist"), []byte("4-7\n"), 0644))
	
	// Create a mock /proc/self/status file
	statusFile := filepath.Join(tmpDir, "status")
	statusContent := `Name:	test
Cpus_allowed:	ff
Cpus_allowed_list:	0-7
`
	require.NoError(t, os.WriteFile(statusFile, []byte(statusContent), 0644))
	
	// Test topology detection
	topology, err := getNUMATopology(nodeDir, statusFile)
	require.NoError(t, err)
	require.NotNil(t, topology)
	require.Equal(t, []int{0, 1}, topology.Nodes)
	require.Equal(t, []int{0, 1, 2, 3}, topology.NodeCPUs[0])
	require.Equal(t, []int{4, 5, 6, 7}, topology.NodeCPUs[1])
	require.True(t, topology.ProcessSpansNodes)
	
	// Test with process pinned to single node
	statusContentPinned := `Name:	test
Cpus_allowed:	0f
Cpus_allowed_list:	0-3
`
	require.NoError(t, os.WriteFile(statusFile, []byte(statusContentPinned), 0644))
	
	topology, err = getNUMATopology(nodeDir, statusFile)
	require.NoError(t, err)
	require.NotNil(t, topology)
	require.False(t, topology.ProcessSpansNodes)
}

func TestNUMATopologyString(t *testing.T) {
	topology := &NUMATopology{
		Nodes: []int{0, 1},
		NodeCPUs: map[int][]int{
			0: {0, 1, 2, 3},
			1: {4, 5, 6, 7},
		},
		ProcessSpansNodes: true,
	}
	
	str := topology.String()
	require.Contains(t, str, "2 NUMA node(s)")
	require.Contains(t, str, "node 0: CPUs 0-3")
	require.Contains(t, str, "node 1: CPUs 4-7")
}
