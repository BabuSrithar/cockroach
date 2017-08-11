// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package jobs

import (
	"math"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/cockroachdb/cockroach/pkg/gossip"
	"github.com/cockroachdb/cockroach/pkg/internal/client"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlutil"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
)

func TestRegistryCancelation(t *testing.T) {
	defer leaktest.AfterTest(t)()

	ctx, stopper := context.Background(), stop.NewStopper()
	defer stopper.Stop(ctx)

	var db *client.DB
	var ex sqlutil.InternalExecutor
	var gossip *gossip.Gossip
	clock := hlc.NewClock(hlc.UnixNano, time.Nanosecond)
	registry := MakeRegistry(clock, db, ex, gossip, FakeNodeID, FakeClusterID)

	const nodeCount = 1
	nodeLiveness := NewFakeNodeLiveness(clock, nodeCount)

	const cancelInterval = time.Nanosecond
	const adoptInterval = time.Duration(math.MaxInt64)
	if err := registry.Start(ctx, stopper, nodeLiveness, cancelInterval, adoptInterval); err != nil {
		t.Fatal(err)
	}

	wait := func() {
		// Every turn of the registry's liveness poll loop will generate exactly one
		// call to nodeLiveness.Self. Only after we've witnessed two calls can we be
		// sure that the first turn of the registry's loop has completed.
		//
		// Waiting for only the first call to nodeLiveness.Self is racy, as we'd
		// perform our assertions concurrently with the registry loop's observation
		// of our injected liveness failure, if any.
		<-nodeLiveness.SelfCalledCh
		<-nodeLiveness.SelfCalledCh
	}

	cancelCount := 0

	register := func(id int64) {
		job := Job{cancelFn: func() { cancelCount++ }}
		if err := registry.register(id, &job); err != nil {
			t.Fatal(err)
		}
	}

	const nodeID = roachpb.NodeID(1)

	// Jobs that complete while the node is live should not be canceled.
	register(1)
	wait()
	registry.unregister(1)
	wait()
	if e, a := 0, cancelCount; e != a {
		t.Fatalf("expected cancelCount of %d, but got %d", e, a)
	}

	// Jobs that are in-progress when the liveness epoch is incremented should be
	// canceled.
	register(2)
	nodeLiveness.FakeIncrementEpoch(nodeID)
	wait()
	if e, a := 1, cancelCount; e != a {
		t.Fatalf("expected cancelCount of %d, but got %d", e, a)
	}

	// Jobs started in the new epoch that complete while the new epoch is live
	// should not be canceled.
	register(3)
	wait()
	registry.unregister(3)
	wait()
	if e, a := 1, cancelCount; e != a {
		t.Fatalf("expected cancelCount of %d, but got %d", e, a)
	}

	// Jobs that are in-progress when the liveness lease expires should be
	// canceled.
	register(4)
	nodeLiveness.FakeSetExpiration(nodeID, hlc.MinTimestamp)
	wait()
	if e, a := 2, cancelCount; e != a {
		t.Fatalf("expected cancelCount of %d, but got %d", e, a)
	}

	// Jobs that are started while the liveness lease is expired should be
	// canceled.
	register(5)
	wait()
	if e, a := 3, cancelCount; e != a {
		t.Fatalf("expected cancelCount of %d, but got %d", e, a)
	}
}

func TestRegistryRegister(t *testing.T) {
	defer leaktest.AfterTest(t)()

	var db *client.DB
	var ex sqlutil.InternalExecutor
	var gossip *gossip.Gossip
	clock := hlc.NewClock(hlc.UnixNano, time.Nanosecond)
	registry := MakeRegistry(clock, db, ex, gossip, FakeNodeID, FakeClusterID)

	if err := registry.register(42, &Job{}); err != nil {
		t.Fatal(err)
	}

	if err := registry.register(42, &Job{}); !testutils.IsError(err, "already tracking job ID") {
		t.Fatalf("expected 'already tracking job ID', but got '%s'", err)
	}

	// Unregistering the same ID multiple times is not an error.
	registry.unregister(42)
	registry.unregister(42)
}
