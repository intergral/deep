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

package deepql_old

import "fmt"

type AggregateOp int

const (
	aggregateCount AggregateOp = iota
	aggregateMax
	aggregateMin
	aggregateSum
	aggregateAvg
)

func (a AggregateOp) String() string {
	switch a {
	case aggregateCount:
		return "count"
	case aggregateMax:
		return "max"
	case aggregateMin:
		return "min"
	case aggregateSum:
		return "sum"
	case aggregateAvg:
		return "avg"
	}

	return fmt.Sprintf("aggregate(%d)", a)
}
