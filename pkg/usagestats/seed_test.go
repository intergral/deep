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

package usagestats

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/dskit/kv"
	"github.com/grafana/dskit/kv/codec"
	"github.com/grafana/dskit/kv/memberlist"
	"github.com/grafana/dskit/services"
	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/pkg/deepdb/backend/local"
)

type dnsProviderMock struct {
	resolved []string
}

func (p *dnsProviderMock) Resolve(ctx context.Context, addrs []string) error {
	p.resolved = addrs
	return nil
}

func (p dnsProviderMock) Addresses() []string {
	return p.resolved
}

func createMemberlist(t *testing.T, port, memberID int) *memberlist.KV {
	t.Helper()
	var cfg memberlist.KVConfig
	flagext.DefaultValues(&cfg)
	cfg.TCPTransport = memberlist.TCPTransportConfig{
		BindAddrs: []string{"localhost"},
		BindPort:  0,
	}
	cfg.GossipInterval = 100 * time.Millisecond
	cfg.GossipNodes = 3
	cfg.PushPullInterval = 5 * time.Second
	cfg.NodeName = fmt.Sprintf("Member-%d", memberID)
	cfg.Codecs = []codec.Codec{JSONCodec}

	mkv := memberlist.NewKV(cfg, log.NewNopLogger(), &dnsProviderMock{}, nil)
	require.NoError(t, services.StartAndAwaitRunning(context.Background(), mkv))
	if port != 0 {
		_, err := mkv.JoinMembers([]string{fmt.Sprintf("127.0.0.1:%d", port)})
		require.NoError(t, err, "%s failed to join the cluster: %v", memberID, err)
	}
	t.Cleanup(func() {
		_ = services.StopAndAwaitTerminated(context.TODO(), mkv)
	})
	return mkv
}

func Test_Memberlist(t *testing.T) {
	stabilityCheckInterval = time.Second

	objectClient, err := local.NewBackend(&local.Config{
		Path: t.TempDir(),
	})

	require.NoError(t, err)
	result := make(chan *ClusterSeed, 10)

	// create a first memberlist to get a valid listening port.
	initMKV := createMemberlist(t, 0, -1)

	for i := 0; i < 10; i++ {
		go func(j int) {
			leader, err := NewReporter(Config{
				Leader:  true,
				Enabled: true,
			}, kv.Config{
				Store: "memberlist",
				StoreConfig: kv.StoreConfig{
					MemberlistKV: func() (*memberlist.KV, error) {
						return createMemberlist(t, initMKV.GetListeningPort(), j), nil
					},
				},
			}, objectClient, objectClient, log.NewLogfmtLogger(os.Stdout), nil)
			require.NoError(t, err)
			leader.init(context.Background())
			result <- leader.cluster
		}(i)
	}

	var UID []string
	for i := 0; i < 10; i++ {
		cluster := <-result
		require.NotNil(t, cluster)
		UID = append(UID, cluster.UID)
	}
	first := UID[0]
	for _, uid := range UID {
		require.Equal(t, first, uid)
	}
}
