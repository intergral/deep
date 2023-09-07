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

package registry

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"
)

// disabled this test as it is not reliable
// the logic is fine, but it relies on timing, which periodically fails.
func Test_counter(t *testing.T) {
	var seriesAdded int
	onAdd := func(count uint32) bool {
		seriesAdded++
		return true
	}

	c := newCounter("my_counter", []string{"label"}, onAdd, nil)

	c.Inc(newLabelValues([]string{"value-1"}), 1.0)
	c.Inc(newLabelValues([]string{"value-2"}), 2.0)

	assert.Equal(t, 2, seriesAdded)

	collectionTimeMs := time.Now().UnixMilli()
	offsetCollectionTimeMs := time.UnixMilli(collectionTimeMs).Add(insertOffsetDuration).UnixMilli()
	expectedSamples := []sample{
		newSample(map[string]string{"__name__": "my_counter", "label": "value-1"}, collectionTimeMs, 0),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-1"}, offsetCollectionTimeMs, 1),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-2"}, collectionTimeMs, 0),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-2"}, offsetCollectionTimeMs, 2),
	}
	collectMetricAndAssert(t, c, collectionTimeMs, nil, 2, expectedSamples, nil)

	c.Inc(newLabelValues([]string{"value-2"}), 2.0)
	c.Inc(newLabelValues([]string{"value-3"}), 3.0)

	assert.Equal(t, 3, seriesAdded)

	expectedSamples = []sample{
		newSample(map[string]string{"__name__": "my_counter", "label": "value-1"}, collectionTimeMs, 1),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-2"}, collectionTimeMs, 4),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-3"}, collectionTimeMs, 0),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-3"}, offsetCollectionTimeMs, 3),
	}
	collectMetricAndAssert(t, c, collectionTimeMs, nil, 3, expectedSamples, nil)
}

func Test_counter_invalidLabelValues(t *testing.T) {
	c := newCounter("my_counter", []string{"label"}, nil, nil)

	assert.Panics(t, func() {
		c.Inc(nil, 1.0)
	})
	assert.Panics(t, func() {
		c.Inc(newLabelValues([]string{"value-1", "value-2"}), 1.0)
	})
}

func Test_counter_cantAdd(t *testing.T) {
	canAdd := false
	onAdd := func(count uint32) bool {
		assert.Equal(t, uint32(1), count)
		return canAdd
	}

	c := newCounter("my_counter", []string{"label"}, onAdd, nil)

	// allow adding new series
	canAdd = true

	c.Inc(newLabelValues([]string{"value-1"}), 1.0)
	c.Inc(newLabelValues([]string{"value-2"}), 2.0)

	collectionTimeMs := time.Now().UnixMilli()
	offsetCollectionTimeMs := time.UnixMilli(collectionTimeMs).Add(insertOffsetDuration).UnixMilli()
	expectedSamples := []sample{
		newSample(map[string]string{"__name__": "my_counter", "label": "value-1"}, collectionTimeMs, 0),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-1"}, offsetCollectionTimeMs, 1),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-2"}, collectionTimeMs, 0),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-2"}, offsetCollectionTimeMs, 2),
	}
	collectMetricAndAssert(t, c, collectionTimeMs, nil, 2, expectedSamples, nil)

	// block new series - existing series can still be updated
	canAdd = false

	c.Inc(newLabelValues([]string{"value-2"}), 2.0)
	c.Inc(newLabelValues([]string{"value-3"}), 3.0)

	collectionTimeMs = time.Now().UnixMilli()
	expectedSamples = []sample{
		newSample(map[string]string{"__name__": "my_counter", "label": "value-1"}, collectionTimeMs, 1),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-2"}, collectionTimeMs, 4),
	}
	collectMetricAndAssert(t, c, collectionTimeMs, nil, 2, expectedSamples, nil)
}

func Test_counter_removeStaleSeries(t *testing.T) {
	var removedSeries int
	onRemove := func(count uint32) {
		assert.Equal(t, uint32(1), count)
		removedSeries++
	}

	c := newCounter("my_counter", []string{"label"}, nil, onRemove)

	timeMs := time.Now().UnixMilli()
	c.Inc(newLabelValues([]string{"value-1"}), 1.0)
	c.Inc(newLabelValues([]string{"value-2"}), 2.0)

	c.removeStaleSeries(timeMs)

	assert.Equal(t, 0, removedSeries)

	collectionTimeMs := time.Now().UnixMilli()
	offsetCollectionTimeMs := time.UnixMilli(collectionTimeMs).Add(insertOffsetDuration).UnixMilli()
	expectedSamples := []sample{
		newSample(map[string]string{"__name__": "my_counter", "label": "value-1"}, collectionTimeMs, 0),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-1"}, offsetCollectionTimeMs, 1),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-2"}, collectionTimeMs, 0),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-2"}, offsetCollectionTimeMs, 2),
	}
	collectMetricAndAssert(t, c, collectionTimeMs, nil, 2, expectedSamples, nil)

	time.Sleep(10 * time.Millisecond)
	timeMs = time.Now().UnixMilli()

	// update value-2 series
	c.Inc(newLabelValues([]string{"value-2"}), 2.0)

	c.removeStaleSeries(timeMs)

	assert.Equal(t, 1, removedSeries)

	collectionTimeMs = time.Now().UnixMilli()
	expectedSamples = []sample{
		newSample(map[string]string{"__name__": "my_counter", "label": "value-2"}, collectionTimeMs, 4),
	}
	collectMetricAndAssert(t, c, collectionTimeMs, nil, 1, expectedSamples, nil)
}

func Test_counter_externalLabels(t *testing.T) {
	c := newCounter("my_counter", []string{"label"}, nil, nil)

	c.Inc(newLabelValues([]string{"value-1"}), 1.0)
	c.Inc(newLabelValues([]string{"value-2"}), 2.0)

	collectionTimeMs := time.Now().UnixMilli()
	offsetCollectionTimeMs := time.UnixMilli(collectionTimeMs).Add(insertOffsetDuration).UnixMilli()
	expectedSamples := []sample{
		newSample(map[string]string{"__name__": "my_counter", "label": "value-1", "external_label": "external_value"}, collectionTimeMs, 0),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-1", "external_label": "external_value"}, offsetCollectionTimeMs, 1),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-2", "external_label": "external_value"}, collectionTimeMs, 0),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-2", "external_label": "external_value"}, offsetCollectionTimeMs, 2),
	}
	collectMetricAndAssert(t, c, collectionTimeMs, map[string]string{"external_label": "external_value"}, 2, expectedSamples, nil)
}

func Test_counter_concurrencyDataRace(t *testing.T) {
	c := newCounter("my_counter", []string{"label"}, nil, nil)

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

	for i := 0; i < 4; i++ {
		go accessor(func() {
			c.Inc(newLabelValues([]string{"value-1"}), 1.0)
			c.Inc(newLabelValues([]string{"value-2"}), 1.0)
		})
	}

	// this goroutine constantly creates new series
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	go accessor(func() {
		s := make([]rune, 6)
		for i := range s {
			s[i] = letters[rand.Intn(len(letters))]
		}
		c.Inc(newLabelValues([]string{string(s)}), 1.0)
	})

	go accessor(func() {
		_, err := c.collectMetrics(&noopAppender{}, 0, nil)
		assert.NoError(t, err)
	})

	go accessor(func() {
		c.removeStaleSeries(time.Now().UnixMilli())
	})

	time.Sleep(200 * time.Millisecond)
	close(end)
}

func Test_counter_concurrencyCorrectness(t *testing.T) {
	c := newCounter("my_counter", []string{"label"}, nil, nil)

	var wg sync.WaitGroup
	end := make(chan struct{})

	totalCount := atomic.NewUint64(0)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-end:
					return
				default:
					c.Inc(newLabelValues([]string{"value-1"}), 1.0)
					totalCount.Inc()
				}
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)
	close(end)

	wg.Wait()

	collectionTimeMs := time.Now().UnixMilli()
	offsetCollectionTimeMs := time.UnixMilli(collectionTimeMs).Add(insertOffsetDuration).UnixMilli()
	expectedSamples := []sample{
		newSample(map[string]string{"__name__": "my_counter", "label": "value-1"}, collectionTimeMs, 0),
		newSample(map[string]string{"__name__": "my_counter", "label": "value-1"}, offsetCollectionTimeMs, float64(totalCount.Load())),
	}
	collectMetricAndAssert(t, c, collectionTimeMs, nil, 1, expectedSamples, nil)
}

func collectMetricAndAssert(t *testing.T, m metric, collectionTimeMs int64, externalLabels map[string]string, expectedActiveSeries int, expectedSamples []sample, expectedExemplars []exemplarSample) {
	appender := &capturingAppender{}

	activeSeries, err := m.collectMetrics(appender, collectionTimeMs, externalLabels)
	assert.NoError(t, err)
	assert.Equal(t, expectedActiveSeries, activeSeries)

	assert.False(t, appender.isCommitted)
	assert.False(t, appender.isRolledback)
	assert.ElementsMatch(t, expectedSamples, appender.samples)
	assert.ElementsMatch(t, expectedExemplars, appender.exemplars)
}
