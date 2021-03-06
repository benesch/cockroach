// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License included
// in the file licenses/BSL.txt and at www.mariadb.com/bsl11.
//
// Change Date: 2022-10-01
//
// On the date above, in accordance with the Business Source License, use
// of this software will be governed by the Apache License, Version 2.0,
// included in the file licenses/APL.txt and at
// https://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"context"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

func runClockJump(ctx context.Context, t *test, c *cluster, tc clockJumpTestCase) {
	// Test with a single node so that the node does not crash due to MaxOffset
	// violation when injecting offset
	if c.nodes != 1 {
		t.Fatalf("Expected num nodes to be 1, got: %d", c.nodes)
	}

	t.Status("deploying offset injector")
	offsetInjector := newOffsetInjector(c)
	offsetInjector.deploy(ctx)

	if err := c.RunE(ctx, c.Node(1), "test -x ./cockroach"); err != nil {
		c.Put(ctx, cockroach, "./cockroach", c.All())
	}
	c.Wipe(ctx)
	c.Start(ctx, t)

	db := c.Conn(ctx, c.nodes)
	defer db.Close()
	if _, err := db.Exec(
		fmt.Sprintf(
			`SET CLUSTER SETTING server.clock.forward_jump_check_enabled= %v`,
			tc.jumpCheckEnabled)); err != nil {
		t.Fatal(err)
	}

	// Wait for Cockroach to process the above cluster setting
	time.Sleep(10 * time.Second)

	if !isAlive(db) {
		t.Fatal("Node unexpectedly crashed")
	}

	t.Status("injecting offset")
	// If we expect the node to crash, make sure it's restarted after the
	// test is done to pacify the dead node detector. Do this after the
	// clock offset is reset or the node will crash again.
	var aliveAfterOffset bool
	defer func() {
		if !aliveAfterOffset {
			c.Start(ctx, t, c.Node(1))
		}
	}()
	defer offsetInjector.recover(ctx, c.nodes)
	offsetInjector.offset(ctx, c.nodes, tc.offset)

	t.Status("validating health")
	aliveAfterOffset = isAlive(db)
	if aliveAfterOffset != tc.aliveAfterOffset {
		t.Fatalf("Expected node health %v, got %v", tc.aliveAfterOffset, aliveAfterOffset)
	}
}

type clockJumpTestCase struct {
	name             string
	jumpCheckEnabled bool
	offset           time.Duration
	aliveAfterOffset bool
}

func makeClockJumpTests() testSpec {
	testCases := []clockJumpTestCase{
		{
			name:             "large_forward_enabled",
			offset:           500 * time.Millisecond,
			jumpCheckEnabled: true,
			aliveAfterOffset: false,
		},
		{
			name: "small_forward_enabled",
			// NB: The offset here needs to be small enough such that this jump plus
			// the forward jump check interval (125ms) is less than the tolerated
			// forward clock jump (250ms).
			offset:           100 * time.Millisecond,
			jumpCheckEnabled: true,
			aliveAfterOffset: true,
		},
		{
			name:             "large_backward_enabled",
			offset:           -500 * time.Millisecond,
			jumpCheckEnabled: true,
			aliveAfterOffset: true,
		},
		{
			name:             "large_forward_disabled",
			offset:           500 * time.Millisecond,
			jumpCheckEnabled: false,
			aliveAfterOffset: true,
		},
		{
			name:             "large_backward_disabled",
			offset:           -500 * time.Millisecond,
			jumpCheckEnabled: false,
			aliveAfterOffset: true,
		},
	}

	spec := testSpec{
		Name: "jump",
	}

	for i := range testCases {
		tc := testCases[i]
		spec.SubTests = append(spec.SubTests, testSpec{
			Name: tc.name,
			Run: func(ctx context.Context, t *test, c *cluster) {
				runClockJump(ctx, t, c, tc)
			},
		})
	}

	return spec
}

func registerClock(r *registry) {
	r.Add(testSpec{
		Name:    "clock",
		Cluster: makeClusterSpec(1),
		SubTests: []testSpec{
			makeClockJumpTests(),
			makeClockMonotonicTests(),
		},
	})
}
