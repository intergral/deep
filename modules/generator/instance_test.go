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

package generator

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	prometheus_storage "github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"

	"github.com/intergral/deep/modules/generator/processor/spanmetrics"
	"github.com/intergral/deep/modules/generator/storage"
	"github.com/intergral/deep/pkg/deeppb"
	"github.com/intergral/deep/pkg/util/test"
)

func Test_instance_concurrency(t *testing.T) {
	overrides := &mockOverrides{}
	instance, err := newInstance(&Config{}, "test", overrides, &noopStorage{}, prometheus.DefaultRegisterer, log.NewNopLogger())
	assert.NoError(t, err)

	end := make(chan struct{})

	accessor := func(f func()) {
		for {
			select {
			case <-end:
				return
			default:
				f()
			}
		}
	}

	go accessor(func() {
		req := test.GenerateSnapshot(1, nil)
		instance.PushSnapshot(context.Background(), &deeppb.PushSnapshotRequest{Snapshot: req})
	})

	go accessor(func() {
		overrides.processors = map[string]struct{}{
			"span-metrics": {},
		}
		err := instance.updateProcessors()
		assert.NoError(t, err)
	})

	time.Sleep(100 * time.Millisecond)

	instance.shutdown()

	time.Sleep(10 * time.Millisecond)
	close(end)
}

func Test_instance_updateProcessors(t *testing.T) {
	cfg := Config{}
	cfg.RegisterFlagsAndApplyDefaults("", &flag.FlagSet{})
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	overrides := mockOverrides{}

	instance, err := newInstance(&cfg, "test", &overrides, &noopStorage{}, prometheus.DefaultRegisterer, logger)
	assert.NoError(t, err)

	// stop the update goroutine
	close(instance.shutdownCh)

	// no processors should be present initially
	assert.Len(t, instance.processors, 0)

	t.Run("add spanmetrics processor", func(t *testing.T) {
		overrides.processors = map[string]struct{}{
			spanmetrics.Name: {},
		}
		err := instance.updateProcessors()
		assert.NoError(t, err)

		assert.Len(t, instance.processors, 1)
		assert.Equal(t, instance.processors[spanmetrics.Name].Name(), spanmetrics.Name)
	})

	t.Run("replace spanmetrics processor", func(t *testing.T) {
		overrides.processors = map[string]struct{}{
			spanmetrics.Name: {},
		}
		overrides.spanMetricsDimensions = []string{"namespace"}
		overrides.spanMetricsIntrinsicDimensions = map[string]bool{"service": true}

		err := instance.updateProcessors()
		assert.NoError(t, err)

		var expectedConfig spanmetrics.Config
		expectedConfig.RegisterFlagsAndApplyDefaults("", &flag.FlagSet{})
		expectedConfig.Dimensions = []string{"namespace"}
		expectedConfig.IntrinsicDimensions.Service = true

		assert.Equal(t, expectedConfig, instance.processors[spanmetrics.Name].(*spanmetrics.Processor).Cfg)
	})

	t.Run("remove processor", func(t *testing.T) {
		overrides.processors = nil
		err := instance.updateProcessors()
		assert.NoError(t, err)

		assert.Len(t, instance.processors, 0)
	})
}

type noopStorage struct{}

var _ storage.Storage = (*noopStorage)(nil)

func (m noopStorage) Appender(context.Context) prometheus_storage.Appender {
	return &noopAppender{}
}

func (m noopStorage) Close() error { return nil }

type noopAppender struct{}

var _ prometheus_storage.Appender = (*noopAppender)(nil)

func (n noopAppender) Append(prometheus_storage.SeriesRef, labels.Labels, int64, float64) (prometheus_storage.SeriesRef, error) {
	return 0, nil
}

func (n noopAppender) AppendExemplar(prometheus_storage.SeriesRef, labels.Labels, exemplar.Exemplar) (prometheus_storage.SeriesRef, error) {
	return 0, nil
}

func (n noopAppender) Commit() error { return nil }

func (n noopAppender) Rollback() error { return nil }

func (n noopAppender) UpdateMetadata(prometheus_storage.SeriesRef, labels.Labels, metadata.Metadata) (prometheus_storage.SeriesRef, error) {
	return 0, nil
}
