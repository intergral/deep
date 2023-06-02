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

package test

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func GetCounterValue(metric prometheus.Counter) (float64, error) {
	var m = &dto.Metric{}
	err := metric.Write(m)
	if err != nil {
		return 0, err
	}
	return m.Counter.GetValue(), nil
}

func GetGaugeValue(metric prometheus.Gauge) (float64, error) {
	var m = &dto.Metric{}
	err := metric.Write(m)
	if err != nil {
		return 0, err
	}
	return m.Gauge.GetValue(), nil
}

func GetGaugeVecValue(metric *prometheus.GaugeVec, labels ...string) (float64, error) {
	var m = &dto.Metric{}
	err := metric.WithLabelValues(labels...).Write(m)
	if err != nil {
		return 0, err
	}
	return m.Gauge.GetValue(), nil
}

func GetCounterVecValue(metric *prometheus.CounterVec, label string) (float64, error) {
	var m = &dto.Metric{}
	if err := metric.WithLabelValues(label).Write(m); err != nil {
		return 0, err
	}
	return m.Counter.GetValue(), nil
}
