/*
 * Copyright (C) 2024  Intergral GmbH
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

package deepql

import (
	"fmt"
	"time"

	"golang.org/x/exp/maps"
)

type search struct {
	rules     []configOption
	aggregate string
	window    *time.Duration
}

func (s search) buildConditions() []Condition {
	mappedToAttr := map[string]Condition{}
	for _, rule := range s.rules {
		key := fmt.Sprintf("%s-%d", rule.lhs, rule.op)
		if v, ok := mappedToAttr[key]; ok {
			v.Operands = append(v.Operands, rule.rhs)
		} else {
			mappedToAttr[key] = Condition{
				Attribute: rule.lhs,
				Op:        rule.op,
				Operands:  []Static{rule.rhs},
			}
		}
	}
	return maps.Values(mappedToAttr)
}

func newAggregationSearch(aggregate string, search search, window *time.Duration) search {
	search.aggregate = aggregate
	search.window = window
	return search
}

func newSearch(opts []configOption) search {
	s := search{
		rules: opts,
	}
	return s
}
