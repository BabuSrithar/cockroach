// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package numa

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetNUMATopology(t *testing.T) {
	for _, tc := range []struct {
		name     string
		dirs     map[string]string // path relative to base -> cpulist content
		wantErr  string
		wantStr  string
		numNodes int
	}{
		{
			name:    "missing sysfs directory",
			dirs:    nil, // don't create base dir
			wantErr: "reading NUMA sysfs directory",
		},
		{
			name:     "single NUMA node",
			dirs:     map[string]string{"node0/cpulist": "0-3\n"},
			numNodes: 1,
			wantStr:  "1 NUMA node(s); node 0 cpus: 0-3",
		},
		{
			name: "two NUMA nodes",
			dirs: map[string]string{
				"node0/cpulist": "0-7\n",
				"node1/cpulist": "8-15\n",
			},
			numNodes: 2,
			wantStr:  "2 NUMA node(s); node 0 cpus: 0-7; node 1 cpus: 8-15",
		},
		{
			name: "four NUMA nodes",
			dirs: map[string]string{
				"node0/cpulist": "0-13\n",
				"node1/cpulist": "14-27\n",
				"node2/cpulist": "28-41\n",
				"node3/cpulist": "42-55\n",
			},
			numNodes: 4,
			wantStr:  "4 NUMA node(s); node 0 cpus: 0-13; node 1 cpus: 14-27; node 2 cpus: 28-41; node 3 cpus: 42-55",
		},
		{
			name: "non-node directories are ignored",
			dirs: map[string]string{
				"node0/cpulist": "0-3\n",
				"power/x":       "ignored\n",
			},
			numNodes: 1,
			wantStr:  "1 NUMA node(s); node 0 cpus: 0-3",
		},
		{
			name:     "no node directories",
			dirs:     map[string]string{"online": "0\n"},
			numNodes: 0,
			wantStr:  "no NUMA nodes detected",
		},
		{
			name: "node directory without cpulist",
			dirs: map[string]string{
				"node0/meminfo": "something\n",
			},
			wantErr: "reading CPU list for NUMA node 0",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var basePath string
			if tc.dirs == nil {
				basePath = filepath.Join(t.TempDir(), "nonexistent")
			} else {
				basePath = t.TempDir()
				for relPath, content := range tc.dirs {
					fullPath := filepath.Join(basePath, relPath)
					require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
					require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
				}
			}

			topo, err := GetNUMATopology(basePath)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, topo.Nodes, tc.numNodes)
			require.Equal(t, tc.wantStr, topo.String())
		})
	}
}
