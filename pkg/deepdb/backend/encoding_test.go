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

package backend

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

type marshalTest struct {
	Test Encoding
}

func TestUnmarshalMarshalYaml(t *testing.T) {
	for _, enc := range SupportedEncoding {
		expected := marshalTest{
			Test: enc,
		}
		actual := marshalTest{}

		buff, err := yaml.Marshal(expected)
		assert.NoError(t, err)
		err = yaml.Unmarshal(buff, &actual)
		assert.NoError(t, err)

		assert.Equal(t, expected, actual)
	}
}

func TestUnmarshalMarshalJson(t *testing.T) {
	for _, enc := range SupportedEncoding {
		expected := marshalTest{
			Test: enc,
		}
		actual := marshalTest{}

		buff, err := json.Marshal(expected)
		assert.NoError(t, err)
		err = json.Unmarshal(buff, &actual)
		assert.NoError(t, err)

		assert.Equal(t, expected, actual)
	}
}
