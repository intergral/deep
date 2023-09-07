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

package distributor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"

	receiver "github.com/intergral/deep/modules/distributor/snapshotreceiver"
	deepTP "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	deep "github.com/intergral/go-deep-proto/tracepoint/v1"

	"github.com/go-kit/log"
	kitlog "github.com/go-kit/log"
	"github.com/golang/protobuf/proto" // nolint: all  //ProtoReflect
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/dskit/kv"
	"github.com/grafana/dskit/ring"
	ring_client "github.com/grafana/dskit/ring/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/common/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"

	generator_client "github.com/intergral/deep/modules/generator/client"
	ingester_client "github.com/intergral/deep/modules/ingester/client"
	"github.com/intergral/deep/modules/overrides"
	"github.com/intergral/deep/pkg/deeppb"
	"github.com/intergral/deep/pkg/util"
	"github.com/intergral/deep/pkg/util/test"
)

const (
	numIngesters = 5
)

var ctx = util.InjectTenantID(context.Background(), "test")

func TestDistributor(t *testing.T) {
	for i, tc := range []struct {
		lines         int
		expectedError error
	}{
		{
			lines:         10,
			expectedError: nil,
		},
		{
			lines:         100,
			expectedError: nil,
		},
	} {
		t.Run(fmt.Sprintf("[%d](samples=%v)", i, tc.lines), func(t *testing.T) {
			limits := &overrides.Limits{}
			flagext.DefaultValues(limits)

			// todo:  test limits
			d := prepare(t, limits, nil, nil)

			b := test.GenerateSnapshot(tc.lines, nil)

			convert, err := proto.Marshal(b)
			assert.NoError(t, err)

			snapshot := &deep.Snapshot{}
			err = proto.Unmarshal(convert, snapshot)
			assert.NoError(t, err)

			_, err = d.PushSnapshot(ctx, snapshot)

			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestLogSnapshots(t *testing.T) {
	for i, tc := range []struct {
		LogReceivedSnapshots        bool // Backwards compatibility with old config
		LogReceivedSnapshotsEnabled bool
		filterByStatusError         bool
		includeAllAttributes        bool
		batches                     *deepTP.Snapshot
		expectedLogsSnapshots       []logSnapShot
	}{
		{
			LogReceivedSnapshotsEnabled: false,
			batches:                     test.GenerateSnapshot(1, nil),
			expectedLogsSnapshots:       []logSnapShot{},
		},
		{
			LogReceivedSnapshotsEnabled: true,
			batches:                     test.GenerateSnapshot(1, &test.GenerateOptions{Id: util.HexStringToSnapshotIDUnsafe("123")}),
			expectedLogsSnapshots: []logSnapShot{
				{
					Msg:        "received",
					Level:      "info",
					SnapshotId: "123",
				},
			},
		},
	} {
		t.Run(fmt.Sprintf("[%d] TestLogSnapshots LogReceivedSnapshots=%v LogReceivedSnapshotsEnabled=%v filterByStatusError=%v includeAllAttributes=%v", i, tc.LogReceivedSnapshots, tc.LogReceivedSnapshotsEnabled, tc.filterByStatusError, tc.includeAllAttributes), func(t *testing.T) {
			limits := &overrides.Limits{}
			flagext.DefaultValues(limits)

			buf := &bytes.Buffer{}
			logger := kitlog.NewJSONLogger(kitlog.NewSyncWriter(buf))

			d := prepare(t, limits, nil, logger)
			d.cfg.LogReceivedSnapshots = LogReceivedSnapshotsConfig{
				Enabled:              tc.LogReceivedSnapshotsEnabled,
				IncludeAllAttributes: tc.includeAllAttributes,
			}

			convert, err := proto.Marshal(tc.batches)
			assert.NoError(t, err)

			snapshot := &deep.Snapshot{}
			err = proto.Unmarshal(convert, snapshot)
			assert.NoError(t, err)

			_, err = d.PushSnapshot(ctx, snapshot)
			if err != nil {
				t.Fatal(err)
			}

			bufJSON := "[" + strings.TrimRight(strings.ReplaceAll(buf.String(), "\n", ","), ",") + "]"
			var actualLogs []logSnapShot
			err = json.Unmarshal([]byte(bufJSON), &actualLogs)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, len(tc.expectedLogsSnapshots), len(actualLogs))
			for i, expectedLog := range tc.expectedLogsSnapshots {
				assert.EqualValues(t, expectedLog, actualLogs[i])
			}
		})
	}
}

type logSnapShot struct {
	Msg        string `json:"msg"`
	Level      string `json:"level"`
	SnapshotId string `json:"snapshotId"`
}

func prepare(t *testing.T, limits *overrides.Limits, kvStore kv.Client, logger log.Logger) *Distributor {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	var (
		distributorConfig Config
		clientConfig      ingester_client.Config
	)
	flagext.DefaultValues(&clientConfig)

	newOverrides, err := overrides.NewOverrides(*limits)
	require.NoError(t, err)

	// Mock the ingesters ring
	ingesters := map[string]*mockIngester{}
	for i := 0; i < numIngesters; i++ {
		ingesters[fmt.Sprintf("ingester%d", i)] = &mockIngester{}
	}

	ingestersRing := &mockRing{
		replicationFactor: 3,
	}
	for addr := range ingesters {
		ingestersRing.ingesters = append(ingestersRing.ingesters, ring.InstanceDesc{
			Addr: addr,
		})
	}

	distributorConfig.DistributorRing.HeartbeatPeriod = 100 * time.Millisecond
	distributorConfig.DistributorRing.InstanceID = strconv.Itoa(rand.Int())
	distributorConfig.DistributorRing.KVStore.Mock = kvStore
	distributorConfig.DistributorRing.InstanceInterfaceNames = []string{"eth0", "en0", "lo0"}
	distributorConfig.factory = func(addr string) (ring_client.PoolClient, error) {
		return ingesters[addr], nil
	}

	l := logging.Level{}
	_ = l.Set("error")
	mw := receiver.MultiTenancyMiddleware()
	d, err := New(distributorConfig, nil, clientConfig, mw, ingestersRing, generator_client.Config{}, nil, newOverrides, logger, prometheus.NewPedanticRegistry())
	require.NoError(t, err)

	return d
}

type mockIngester struct {
	grpc_health_v1.HealthClient
}

var _ deeppb.IngesterServiceClient = (*mockIngester)(nil)

func (i *mockIngester) PushBytes(ctx context.Context, in *deeppb.PushBytesRequest, opts ...grpc.CallOption) (*deeppb.PushBytesResponse, error) {
	return nil, nil
}

func (i *mockIngester) Close() error {
	return nil
}

// Copied from Cortex; TODO(twilkie) - factor this our and share it.
// mockRing doesn't do virtual nodes, just returns mod(key) + replicationFactor
// ingesters.
type mockRing struct {
	prometheus.Counter
	ingesters         []ring.InstanceDesc
	replicationFactor uint32
}

var _ ring.ReadRing = (*mockRing)(nil)

func (r mockRing) Get(key uint32, op ring.Operation, buf []ring.InstanceDesc, _, _ []string) (ring.ReplicationSet, error) {
	result := ring.ReplicationSet{
		MaxErrors: 1,
		Instances: buf[:0],
	}
	for i := uint32(0); i < r.replicationFactor; i++ {
		n := (key + i) % uint32(len(r.ingesters))
		result.Instances = append(result.Instances, r.ingesters[n])
	}
	return result, nil
}

func (r mockRing) GetAllHealthy(op ring.Operation) (ring.ReplicationSet, error) {
	return ring.ReplicationSet{
		Instances: r.ingesters,
		MaxErrors: 1,
	}, nil
}

func (r mockRing) GetReplicationSetForOperation(op ring.Operation) (ring.ReplicationSet, error) {
	return r.GetAllHealthy(op)
}

func (r mockRing) ReplicationFactor() int {
	return int(r.replicationFactor)
}

func (r mockRing) ShuffleShard(identifier string, size int) ring.ReadRing {
	return r
}

func (r mockRing) ShuffleShardWithLookback(string, int, time.Duration, time.Time) ring.ReadRing {
	return r
}

func (r mockRing) InstancesCount() int {
	return len(r.ingesters)
}

func (r mockRing) HasInstance(instanceID string) bool {
	return true
}

func (r mockRing) CleanupShuffleShardCache(identifier string) {
}

func (r mockRing) GetInstanceState(instanceID string) (ring.InstanceState, error) {
	return ring.ACTIVE, nil
}
