// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

// Package sysutil is a cross-platform compatibility layer on top of package
// syscall. It exposes APIs for common operations that require package syscall
// and re-exports several symbols from package syscall that are known to be
// safe. Using package syscall directly from other packages is forbidden.
package sysutil

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/cockroachdb/errors"
)

// Signal is syscall.Signal.
type Signal = syscall.Signal

// Errno is syscall.Errno.
type Errno = syscall.Errno

// Exported syscall.Errno constants.
const (
	ECONNRESET   = syscall.ECONNRESET
	ECONNREFUSED = syscall.ECONNREFUSED
)

// ExitStatus returns the exit status contained within an exec.ExitError.
func ExitStatus(err *exec.ExitError) int {
	// err.Sys() is of type syscall.WaitStatus on all supported platforms.
	// syscall.WaitStatus has a different type on Windows, but that type has an
	// ExitStatus method with an identical signature, so no need for conditional
	// compilation.
	return err.Sys().(syscall.WaitStatus).ExitStatus()
}

const refreshSignal = syscall.SIGHUP

// RefreshSignaledChan returns a channel that will receive an os.Signal whenever
// the process receives a "refresh" signal (currently SIGHUP). A refresh signal
// indicates that the user wants to apply nondisruptive updates, like reloading
// certificates and flushing log files.
//
// On Windows, the returned channel will never receive any values, as Windows
// does not support signals. Consider exposing a refresh trigger through other
// means if Windows support is important.
func RefreshSignaledChan() <-chan os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, refreshSignal)
	return ch
}

// IsErrConnectionReset returns true if an
// error is a "connection reset by peer" error.
func IsErrConnectionReset(err error) bool {
	return errors.Is(err, syscall.ECONNRESET)
}

// IsErrConnectionRefused returns true if an error is a "connection refused" error.
func IsErrConnectionRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}

// IsErrTimedOut returns true if an error is an ETIMEDOUT error.
func IsErrTimedOut(err error) bool {
	return errors.Is(err, syscall.ETIMEDOUT)
}

// IsAddrInUse returns true if an error is an EADDRINUSE error.
func IsAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}

// InterruptSelf sends Interrupt to the process itself.
func InterruptSelf() error {
	pr, err := os.FindProcess(os.Getpid())
	if err != nil {
		// No-op.
		return nil //nolint:returnerrcheck
	}
	return pr.Signal(os.Interrupt)
}

// NUMANodeInfo describes a NUMA node and the CPUs associated with it.
type NUMANodeInfo struct {
	ID      int
	CPUList string
}

// NUMATopology captures the NUMA topology and the nodes available to the process.
type NUMATopology struct {
	Nodes            []NUMANodeInfo
	AllowedNodes     []int
	AllowedNodesList string
}

// String formats the topology for logging.
func (t NUMATopology) String() string {
	if len(t.Nodes) == 0 {
		return "unknown"
	}
	parts := make([]string, 0, len(t.Nodes))
	for _, node := range t.Nodes {
		cpuList := node.CPUList
		if cpuList == "" {
			cpuList = "unknown"
		}
		parts = append(parts, fmt.Sprintf("node%d CPUs=%s", node.ID, cpuList))
	}
	return strings.Join(parts, "; ")
}

// SpansMultipleNodes returns true if the process is allowed to run on multiple NUMA nodes.
func (t NUMATopology) SpansMultipleNodes() bool {
	return len(t.AllowedNodes) > 1
}

// AllowedNodesSummary returns a string describing the allowed NUMA nodes.
func (t NUMATopology) AllowedNodesSummary() string {
	if t.AllowedNodesList != "" {
		return t.AllowedNodesList
	}
	if len(t.AllowedNodes) == 0 {
		return ""
	}
	parts := make([]string, len(t.AllowedNodes))
	for i, node := range t.AllowedNodes {
		parts[i] = strconv.Itoa(node)
	}
	return strings.Join(parts, ",")
}

// GetNUMATopology returns the NUMA topology, whether it was available, and any error.
func GetNUMATopology() (NUMATopology, bool, error) {
	if runtime.GOOS != "linux" {
		return NUMATopology{}, false, nil
	}
	return getNUMATopology("/sys/devices/system/node", "/proc/self/status")
}

func getNUMATopology(nodeDir, statusPath string) (NUMATopology, bool, error) {
	nodes, err := readNUMANodes(nodeDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NUMATopology{}, false, nil
		}
		return NUMATopology{}, false, err
	}
	if len(nodes) == 0 {
		return NUMATopology{}, false, nil
	}
	topology := NUMATopology{Nodes: nodes}
	allowedList, allowedNodes, err := readAllowedNodes(statusPath)
	if err != nil {
		return topology, true, err
	}
	topology.AllowedNodes = allowedNodes
	topology.AllowedNodesList = allowedList
	return topology, true, nil
}

func readNUMANodes(nodeDir string) ([]NUMANodeInfo, error) {
	entries, err := os.ReadDir(nodeDir)
	if err != nil {
		return nil, err
	}
	var nodes []NUMANodeInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "node") {
			continue
		}
		idStr := strings.TrimPrefix(name, "node")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return nil, errors.Newf("unexpected NUMA node directory %q", name)
		}
		cpuListPath := filepath.Join(nodeDir, name, "cpulist")
		data, err := os.ReadFile(cpuListPath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading %s", cpuListPath)
		}
		nodes = append(nodes, NUMANodeInfo{
			ID:      id,
			CPUList: strings.TrimSpace(string(data)),
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
	return nodes, nil
}

func readAllowedNodes(statusPath string) (string, []int, error) {
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return "", nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "Mems_allowed_list:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return "", nil, errors.Newf("unexpected Mems_allowed_list line %q", line)
		}
		list := strings.TrimSpace(fields[1])
		nodes, err := parseNodeList(list)
		if err != nil {
			return "", nil, err
		}
		return list, nodes, nil
	}
	return "", nil, errors.New("Mems_allowed_list not found")
}

func parseNodeList(list string) ([]int, error) {
	list = strings.TrimSpace(list)
	if list == "" {
		return nil, nil
	}
	seen := make(map[int]struct{})
	for _, part := range strings.Split(list, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			bounds := strings.Split(part, "-")
			if len(bounds) != 2 {
				return nil, errors.Newf("invalid node range %q", part)
			}
			start, err := strconv.Atoi(bounds[0])
			if err != nil {
				return nil, errors.Wrapf(err, "invalid node range %q", part)
			}
			end, err := strconv.Atoi(bounds[1])
			if err != nil {
				return nil, errors.Wrapf(err, "invalid node range %q", part)
			}
			if start > end {
				return nil, errors.Newf("invalid node range %q", part)
			}
			for node := start; node <= end; node++ {
				seen[node] = struct{}{}
			}
			continue
		}
		node, err := strconv.Atoi(part)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid node entry %q", part)
		}
		seen[node] = struct{}{}
	}
	nodes := make([]int, 0, len(seen))
	for node := range seen {
		nodes = append(nodes, node)
	}
	sort.Ints(nodes)
	return nodes, nil
}
