// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package sysutil

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

func TestExitStatus(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 42")
	err := cmd.Run()
	if err == nil {
		t.Fatalf("%s did not return error", cmd.Args)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("%s returned error of type %T, but expected *exec.ExitError", cmd.Args, err)
	}
	if status := ExitStatus(exitErr); status != 42 {
		t.Fatalf("expected exit status 42, but got %d", status)
	}
}

func TestIsAddrInUse(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, err = net.Listen("tcp", ln.Addr().String())
	require.True(t, IsAddrInUse(err))
}

func TestParseNodeList(t *testing.T) {
	tests := []struct {
		list     string
		want     []int
		hasError bool
	}{{
		list: "0",
		want: []int{0},
	}, {
		list: "0-2",
		want: []int{0, 1, 2},
	}, {
		list: "0,2-3",
		want: []int{0, 2, 3},
	}, {
		list: "",
		want: nil,
	}, {
		list:     "2-1",
		hasError: true,
	}}

	for _, tt := range tests {
		t.Run(tt.list, func(t *testing.T) {
			got, err := parseNodeList(tt.list)
			if tt.hasError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGetNUMATopology(t *testing.T) {
	dir := t.TempDir()
	nodeDir := filepath.Join(dir, "nodes")
	require.NoError(t, os.MkdirAll(filepath.Join(nodeDir, "node0"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(nodeDir, "node1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nodeDir, "node0", "cpulist"), []byte("0-3\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(nodeDir, "node1", "cpulist"), []byte("4-7\n"), 0o644))

	statusPath := filepath.Join(dir, "status")
	statusContents := "Name:\tproc\nMems_allowed_list:\t0-1\n"
	require.NoError(t, os.WriteFile(statusPath, []byte(statusContents), 0o644))

	topology, available, err := getNUMATopology(nodeDir, statusPath)
	require.NoError(t, err)
	require.True(t, available)
	require.Equal(t, []NUMANodeInfo{
		{ID: 0, CPUList: "0-3"},
		{ID: 1, CPUList: "4-7"},
	}, topology.Nodes)
	require.Equal(t, []int{0, 1}, topology.AllowedNodes)
	require.Equal(t, "0-1", topology.AllowedNodesList)

	missingDir := filepath.Join(dir, "missing")
	topology, available, err = getNUMATopology(missingDir, statusPath)
	require.NoError(t, err)
	require.False(t, available)
	require.Empty(t, topology.Nodes)
}
