/*
 * Copyright (C) 2023  Intergral GmbH
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package tracepoint

import (
	"context"
	"testing"
	"time"

	gkLog "github.com/go-kit/log"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/dskit/kv/consul"
	"github.com/grafana/dskit/ring"
	"github.com/grafana/dskit/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForgetStaleInstances(t *testing.T) {
	now := time.Now()
	forgetPeriod := 10 * time.Minute

	desc := ring.NewDesc()
	desc.AddIngester("self", "addr-self", "", []uint32{1}, ring.ACTIVE, now.Add(-time.Hour), false, time.Time{})
	desc.AddIngester("fresh", "addr-fresh", "", []uint32{2}, ring.ACTIVE, now, false, time.Time{})
	desc.AddIngester("stale", "addr-stale", "", []uint32{3}, ring.ACTIVE, now, false, time.Time{})
	desc.AddIngester("just-inside", "addr-inside", "", []uint32{4}, ring.ACTIVE, now, false, time.Time{})

	setHeartbeat(desc, "self", now.Add(-time.Hour))
	setHeartbeat(desc, "fresh", now.Add(-time.Second))
	setHeartbeat(desc, "stale", now.Add(-forgetPeriod-time.Second))
	setHeartbeat(desc, "just-inside", now.Add(-forgetPeriod+time.Second))

	forgotten := forgetStaleInstances(desc, "self", forgetPeriod, now, gkLog.NewNopLogger())

	assert.Equal(t, []string{"stale"}, forgotten)
	assert.Contains(t, desc.Ingesters, "self", "an instance never forgets itself")
	assert.Contains(t, desc.Ingesters, "fresh")
	assert.Contains(t, desc.Ingesters, "just-inside")
	assert.NotContains(t, desc.Ingesters, "stale")
}

func TestForgetStaleInstances_NothingStale(t *testing.T) {
	now := time.Now()
	desc := ring.NewDesc()
	desc.AddIngester("a", "addr-a", "", []uint32{1}, ring.ACTIVE, now, false, time.Time{})
	setHeartbeat(desc, "a", now)

	assert.Empty(t, forgetStaleInstances(desc, "self", time.Minute, now, gkLog.NewNopLogger()))
	assert.Len(t, desc.Ingesters, 1)
}

// TestRunning_AutoForgetsDeadInstance runs the service loop against a real
// lifecycler and checks that an instance that stopped heartbeating without
// leaving the ring is removed, while the live instance stays.
func TestRunning_AutoForgetsDeadInstance(t *testing.T) {
	ringStore, closer := consul.NewInMemoryClient(ring.GetCodec(), gkLog.NewNopLogger(), nil)
	t.Cleanup(func() { _ = closer.Close() })

	var cfg ring.LifecyclerConfig
	flagext.DefaultValues(&cfg)
	cfg.RingConfig.KVStore.Mock = ringStore
	cfg.RingConfig.ReplicationFactor = 1
	cfg.RingConfig.HeartbeatTimeout = 200 * time.Millisecond
	cfg.HeartbeatPeriod = 50 * time.Millisecond
	cfg.Addr = "127.0.0.1"
	cfg.Port = 1
	cfg.ListenPort = 0
	cfg.NumTokens = 1
	cfg.ID = "live"
	cfg.FinalSleep = 0

	ts := &TPService{cfg: Config{LifecyclerConfig: cfg}, log: gkLog.NewNopLogger()}
	lc, err := ring.NewLifecycler(cfg, ts, "tracepoint", tracepointRingKey, false, gkLog.NewNopLogger(), nil)
	require.NoError(t, err)
	ts.lifecycler = lc

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	require.NoError(t, services.StartAndAwaitRunning(ctx, lc))
	t.Cleanup(func() { _ = services.StopAndAwaitTerminated(context.Background(), lc) })

	// A pod lost with its node: ACTIVE, owns tokens, last heartbeat long ago.
	require.NoError(t, ringStore.CAS(ctx, tracepointRingKey, func(in interface{}) (interface{}, bool, error) {
		desc, _ := in.(*ring.Desc)
		if desc == nil {
			desc = ring.NewDesc()
		}
		desc.AddIngester("dead", "127.0.0.2:1", "", []uint32{42}, ring.ACTIVE, time.Now().Add(-time.Hour), false, time.Time{})
		setHeartbeat(desc, "dead", time.Now().Add(-time.Hour))
		return desc, true, nil
	}))

	done := make(chan error, 1)
	go func() { done <- ts.running(ctx) }()

	require.Eventually(t, func() bool {
		desc := readRing(t, ringStore)
		_, dead := desc.Ingesters["dead"]
		_, live := desc.Ingesters["live"]
		return !dead && live
	}, 5*time.Second, 20*time.Millisecond, "dead instance is not forgotten, or the live one is gone")

	cancel()
	require.NoError(t, <-done)
}

func TestRunning_NoForgetWhenTimeoutDisabled(t *testing.T) {
	var cfg ring.LifecyclerConfig
	flagext.DefaultValues(&cfg)
	cfg.RingConfig.HeartbeatTimeout = 0

	ts := &TPService{cfg: Config{LifecyclerConfig: cfg}, log: gkLog.NewNopLogger()}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	// lifecycler is nil: the loop must not touch it when auto-forget is off.
	go func() { done <- ts.running(ctx) }()
	cancel()
	require.NoError(t, <-done)
}

func setHeartbeat(desc *ring.Desc, id string, at time.Time) {
	inst := desc.Ingesters[id]
	inst.Timestamp = at.Unix()
	desc.Ingesters[id] = inst
}

func readRing(t *testing.T, store *consul.Client) *ring.Desc {
	t.Helper()
	v, err := store.Get(context.Background(), tracepointRingKey)
	require.NoError(t, err)
	desc, _ := v.(*ring.Desc)
	if desc == nil {
		return ring.NewDesc()
	}
	return desc
}
