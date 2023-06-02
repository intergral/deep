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

package cache_test

import (
	"fmt"
	"testing"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/facette/natsort"
	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/pkg/cache"
)

func TestNatSort(t *testing.T) {
	// Validate that the order of SRV records returned by a DNS
	// lookup for a k8s StatefulSet are ordered as expected when
	// a natsort is done.
	input := []string{
		"memcached-10.memcached.cortex.svc.cluster.local.",
		"memcached-1.memcached.cortex.svc.cluster.local.",
		"memcached-6.memcached.cortex.svc.cluster.local.",
		"memcached-3.memcached.cortex.svc.cluster.local.",
		"memcached-25.memcached.cortex.svc.cluster.local.",
	}

	expected := []string{
		"memcached-1.memcached.cortex.svc.cluster.local.",
		"memcached-3.memcached.cortex.svc.cluster.local.",
		"memcached-6.memcached.cortex.svc.cluster.local.",
		"memcached-10.memcached.cortex.svc.cluster.local.",
		"memcached-25.memcached.cortex.svc.cluster.local.",
	}

	natsort.Sort(input)
	require.Equal(t, expected, input)
}

func TestMemcachedJumpHashSelector_PickSever(t *testing.T) {
	s := cache.MemcachedJumpHashSelector{}
	err := s.SetServers("google.com:80", "microsoft.com:80", "duckduckgo.com:80")
	require.NoError(t, err)

	// We store the string representation instead of the net.Addr
	// to make sure different IPs were discovered during SetServers
	distribution := make(map[string]int)

	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		addr, err := s.PickServer(key)
		require.NoError(t, err)
		distribution[addr.String()]++
	}

	// All of the servers should have been returned at least
	// once
	require.Len(t, distribution, 3)
	for _, v := range distribution {
		require.NotZero(t, v)
	}
}

func TestMemcachedJumpHashSelector_PickSever_ErrNoServers(t *testing.T) {
	s := cache.MemcachedJumpHashSelector{}
	_, err := s.PickServer("foo")
	require.Error(t, memcache.ErrNoServers, err)
}
