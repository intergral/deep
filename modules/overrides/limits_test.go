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

package overrides

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Copied from Cortex
func TestLimitsTagsYamlMatchJson(t *testing.T) {
	limits := reflect.TypeOf(Limits{})
	n := limits.NumField()
	var mismatch []string

	for i := 0; i < n; i++ {
		field := limits.Field(i)

		// Note that we aren't requiring YAML and JSON tags to match, just that
		// they either both exist or both don't exist.
		hasYAMLTag := field.Tag.Get("yaml") != ""
		hasJSONTag := field.Tag.Get("json") != ""

		if hasYAMLTag != hasJSONTag {
			mismatch = append(mismatch, field.Name)
		}
	}

	assert.Empty(t, mismatch, "expected no mismatched JSON and YAML tags")
}

// Copied from Cortex and modified
func TestLimitsYamlMatchJson(t *testing.T) {
	inputYAML := `
ingestion_rate_strategy: global
ingestion_rate_limit_bytes: 100_000
ingestion_burst_size_bytes: 100_000

max_snapshots_per_tenant: 1000
max_global_snapshots_per_tenant: 1000
max_bytes_per_snapshot: 100_000

block_retention: 24h

per_tenant_override_config: /etc/overrides.yaml
per_tenant_override_period: 1m

metrics_generator_send_queue_size: 10
metrics_generator_send_workers: 1

max_search_duration: 5m
`
	inputJSON := `
{
	"ingestion_rate_strategy": "global",
	"ingestion_rate_limit_bytes": 100000,
	"ingestion_burst_size_bytes": 100000,

	"max_snapshots_per_tenant": 1000,
	"max_global_snapshots_per_tenant": 1000,
	"max_bytes_per_snapshot": 100000,

	"block_retention": "24h",

	"per_tenant_override_config": "/etc/overrides.yaml",
	"per_tenant_override_period": "1m",

	"metrics_generator_send_queue_size": 10,
	"metrics_generator_send_workers": 1,

	"max_search_duration": "5m"
}`

	limitsYAML := Limits{}
	err := yaml.Unmarshal([]byte(inputYAML), &limitsYAML)
	require.NoError(t, err, "expected to be able to unmarshal from YAML")

	limitsJSON := Limits{}
	err = json.Unmarshal([]byte(inputJSON), &limitsJSON)
	require.NoError(t, err, "expected to be able to unmarshal from JSON")

	assert.Equal(t, limitsYAML, limitsJSON)
}
