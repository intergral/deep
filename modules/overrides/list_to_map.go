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

	"gopkg.in/yaml.v2"
)

type ListToMap map[string]struct{}

var (
	_ yaml.Marshaler   = (*ListToMap)(nil)
	_ yaml.Unmarshaler = (*ListToMap)(nil)
	_ json.Marshaler   = (*ListToMap)(nil)
	_ json.Unmarshaler = (*ListToMap)(nil)
)

// MarshalYAML implements the Marshal interface of the yaml pkg.
func (l ListToMap) MarshalYAML() (interface{}, error) {
	list := make([]string, 0, len(l))
	for k := range l {
		list = append(list, k)
	}

	if len(list) == 0 {
		return nil, nil
	}
	return list, nil
}

// UnmarshalYAML implements the Unmarshaler interface of the yaml pkg.
func (l *ListToMap) UnmarshalYAML(unmarshal func(interface{}) error) error {
	list := make([]string, 0)
	err := unmarshal(&list)
	if err != nil {
		return err
	}

	*l = make(map[string]struct{})
	for _, element := range list {
		(*l)[element] = struct{}{}
	}
	return nil
}

// MarshalJSON implements the Marshal interface of the json pkg.
func (l ListToMap) MarshalJSON() ([]byte, error) {
	list := make([]string, 0, len(l))
	for k := range l {
		list = append(list, k)
	}

	return json.Marshal(&list)
}

// UnmarshalJSON implements the Unmarshal interface of the json pkg.
func (l *ListToMap) UnmarshalJSON(b []byte) error {
	list := make([]string, 0)
	err := json.Unmarshal(b, &list)
	if err != nil {
		return err
	}

	*l = make(map[string]struct{})
	for _, element := range list {
		(*l)[element] = struct{}{}
	}
	return nil
}

func (l *ListToMap) GetMap() map[string]struct{} {
	if *l == nil {
		*l = map[string]struct{}{}
	}
	return *l
}
