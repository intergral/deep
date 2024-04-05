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

package vparquet

import (
	"fmt"

	"github.com/intergral/deep/pkg/deepql"
	"github.com/intergral/deep/pkg/parquetquery"
	"github.com/segmentio/parquet-go"
)

func createPredicate(op deepql.Operator, operands deepql.Operands) (parquetquery.Predicate, error) {
	if op == deepql.OpNone {
		return nil, nil
	}

	switch operands[0].Type {
	case deepql.TypeString:
		return createStringPredicate(op, operands)
	case deepql.TypeInt:
		return createIntPredicate(op, operands)
	case deepql.TypeFloat:
		return createFloatPredicate(op, operands)
	case deepql.TypeBoolean:
		return createBoolPredicate(op, operands)
	default:
		return nil, fmt.Errorf("cannot create predicate for operand: %v", operands[0])
	}
}

func createIntPredicate(op deepql.Operator, operands deepql.Operands) (parquetquery.Predicate, error) {
	if op == deepql.OpNone {
		return nil, nil
	}

	var i int64
	switch operands[0].Type {
	case deepql.TypeInt:
		i = int64(operands[0].N)
	case deepql.TypeDuration:
		i = operands[0].D.Nanoseconds()
	default:
		return nil, fmt.Errorf("operand is not int, duration, status or kind: %+v", operands[0])
	}

	var fn func(v int64) bool
	var rangeFn func(min, max int64) bool

	switch op {
	case deepql.OpEqual:
		fn = func(v int64) bool { return v == i }
		rangeFn = func(min, max int64) bool { return min <= i && i <= max }
	case deepql.OpNotEqual:
		fn = func(v int64) bool { return v != i }
		rangeFn = func(min, max int64) bool { return min != i || max != i }
	case deepql.OpGreater:
		fn = func(v int64) bool { return v > i }
		rangeFn = func(min, max int64) bool { return max > i }
	case deepql.OpGreaterEqual:
		fn = func(v int64) bool { return v >= i }
		rangeFn = func(min, max int64) bool { return max >= i }
	case deepql.OpLess:
		fn = func(v int64) bool { return v < i }
		rangeFn = func(min, max int64) bool { return min < i }
	case deepql.OpLessEqual:
		fn = func(v int64) bool { return v <= i }
		rangeFn = func(min, max int64) bool { return min <= i }
	default:
		return nil, fmt.Errorf("operand not supported for integers: %+v", op)
	}

	return parquetquery.NewIntPredicate(fn, rangeFn), nil
}

func createStringPredicate(op deepql.Operator, operands deepql.Operands) (parquetquery.Predicate, error) {
	if op == deepql.OpNone {
		return nil, nil
	}

	for _, op := range operands {
		if op.Type != deepql.TypeString {
			return nil, fmt.Errorf("operand is not string: %+v", op)
		}
	}

	s := operands[0].S

	switch op {
	case deepql.OpEqual:
		return parquetquery.NewStringInPredicate([]string{s}), nil

	case deepql.OpNotEqual:
		return parquetquery.NewGenericPredicate(
			func(v string) bool {
				return v != s
			},
			func(min, max string) bool {
				return min != s || max != s
			},
			func(v parquet.Value) string {
				return v.String()
			},
		), nil

	case deepql.OpRegex:
		return parquetquery.NewRegexInPredicate([]string{s})

	case deepql.OpNotRegex:
		return parquetquery.NewRegexNotInPredicate([]string{s})

	default:
		return nil, fmt.Errorf("operand not supported for strings: %+v", op)
	}
}

func createFloatPredicate(op deepql.Operator, operands deepql.Operands) (parquetquery.Predicate, error) {
	if op == deepql.OpNone {
		return nil, nil
	}

	// Ensure operand is float
	if operands[0].Type != deepql.TypeFloat {
		return nil, fmt.Errorf("operand is not float: %+v", operands[0])
	}

	i := operands[0].F

	var fn func(v float64) bool
	var rangeFn func(min, max float64) bool

	switch op {
	case deepql.OpEqual:
		fn = func(v float64) bool { return v == i }
		rangeFn = func(min, max float64) bool { return min <= i && i <= max }
	case deepql.OpNotEqual:
		fn = func(v float64) bool { return v != i }
		rangeFn = func(min, max float64) bool { return min != i || max != i }
	case deepql.OpGreater:
		fn = func(v float64) bool { return v > i }
		rangeFn = func(min, max float64) bool { return max > i }
	case deepql.OpGreaterEqual:
		fn = func(v float64) bool { return v >= i }
		rangeFn = func(min, max float64) bool { return max >= i }
	case deepql.OpLess:
		fn = func(v float64) bool { return v < i }
		rangeFn = func(min, max float64) bool { return min < i }
	case deepql.OpLessEqual:
		fn = func(v float64) bool { return v <= i }
		rangeFn = func(min, max float64) bool { return min <= i }
	default:
		return nil, fmt.Errorf("operand not supported for floats: %+v", op)
	}

	return parquetquery.NewFloatPredicate(fn, rangeFn), nil
}

func createBoolPredicate(op deepql.Operator, operands deepql.Operands) (parquetquery.Predicate, error) {
	if op == deepql.OpNone {
		return nil, nil
	}

	// Ensure operand is bool
	if operands[0].Type != deepql.TypeBoolean {
		return nil, fmt.Errorf("operand is not bool: %+v", operands[0])
	}

	switch op {
	case deepql.OpEqual:
		return parquetquery.NewBoolPredicate(operands[0].B), nil

	case deepql.OpNotEqual:
		return parquetquery.NewBoolPredicate(!operands[0].B), nil

	default:
		return nil, fmt.Errorf("operand not supported for booleans: %+v", op)
	}
}
