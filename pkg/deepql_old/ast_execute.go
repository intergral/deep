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

import (
	"errors"
	"fmt"
	"math"
	"regexp"
)

func (o SnapshotOperation) evaluate(input []*SnapshotResult) (output []*SnapshotResult, err error) {
	for i := range input {
		curr := input[i : i+1]

		lhs, err := o.LHS.evaluate(curr)
		if err != nil {
			return nil, err
		}

		rhs, err := o.RHS.evaluate(curr)
		if err != nil {
			return nil, err
		}

		switch o.Op {
		case OpSnapshotAnd:
			if len(lhs) > 0 && len(rhs) > 0 {
				matchingSnapshot := input[i].clone()
				output = append(output, matchingSnapshot)
			}

		case OpSnapshotUnion:
			if len(lhs) > 0 || len(rhs) > 0 {
				matchingSnapshot := input[i].clone()
				output = append(output, matchingSnapshot)
			}

		default:
			return nil, fmt.Errorf("Snapshot operation (%v) not supported", o.Op)
		}
	}

	return output, nil
}

func (f ScalarFilter) evaluate(input []*SnapshotResult) (output []*SnapshotResult, err error) {
	// TODO we solve this gap where pipeline elements and scalar binary
	// operations meet in a generic way. For now we only support well-defined
	// case: aggregate binop static
	switch l := f.lhs.(type) {
	case Aggregate:
		switch r := f.rhs.(type) {
		case Static:
			input, err = l.evaluate(input)
			if err != nil {
				return nil, err
			}

			for _, ss := range input {
				res, err := binOp(f.op, ss.Scalar, r)
				if err != nil {
					return nil, fmt.Errorf("scalar filter (%v) failed: %v", f, err)
				}
				if res {
					output = append(output, ss)
				}
			}

		default:
			return nil, fmt.Errorf("scalar filter lhs (%v) not supported", f.lhs)
		}

	default:
		return nil, fmt.Errorf("scalar filter lhs (%v) not supported", f.lhs)
	}

	return output, nil
}

func (a Aggregate) evaluate(input []*SnapshotResult) (output []*SnapshotResult, err error) {
	for _, ss := range input {
		switch a.op {
		case aggregateCount:
			copy := ss.clone()
			copy.Scalar = NewStaticInt(1)
			output = append(output, copy)

		case aggregateAvg:
			sum := 0.0
			count := 1
			val, err := a.e.execute(ss.Snapshot)
			if err != nil {
				return nil, err
			}

			sum += val.asFloat()

			copy := ss.clone()
			copy.Scalar = NewStaticFloat(sum / float64(count))
			output = append(output, copy)

		case aggregateMax:
			max := math.Inf(-1)
			val, err := a.e.execute(ss.Snapshot)
			if err != nil {
				return nil, err
			}
			if val.asFloat() > max {
				max = val.asFloat()
			}
			copy := ss.clone()
			copy.Scalar = NewStaticFloat(max)
			output = append(output, copy)

		case aggregateMin:
			min := math.Inf(1)
			val, err := a.e.execute(ss.Snapshot)
			if err != nil {
				return nil, err
			}
			if val.asFloat() < min {
				min = val.asFloat()
			}
			copy := ss.clone()
			copy.Scalar = NewStaticFloat(min)
			output = append(output, copy)

		case aggregateSum:
			sum := 0.0
			val, err := a.e.execute(ss.Snapshot)
			if err != nil {
				return nil, err
			}
			sum += val.asFloat()
			copy := ss.clone()
			copy.Scalar = NewStaticFloat(sum)
			output = append(output, copy)

		default:
			return nil, fmt.Errorf("aggregate operation (%v) not supported", a.op)
		}
	}

	return output, nil
}

func (o BinaryOperation) execute(snapshot Snapshot) (Static, error) {
	lhs, err := o.LHS.execute(snapshot)
	if err != nil {
		return NewStaticNil(), err
	}

	rhs, err := o.RHS.execute(snapshot)
	if err != nil {
		return NewStaticNil(), err
	}

	// Ensure the resolved types are still valid
	lhsT := lhs.impliedType()
	rhsT := rhs.impliedType()
	if !lhsT.isMatchingOperand(rhsT) {
		return NewStaticBool(false), nil
	}

	if !o.Op.binaryTypesValid(lhsT, rhsT) {
		return NewStaticBool(false), nil
	}

	switch o.Op {
	case OpAdd:
		return NewStaticFloat(lhs.asFloat() + rhs.asFloat()), nil
	case OpSub:
		return NewStaticFloat(lhs.asFloat() - rhs.asFloat()), nil
	case OpDiv:
		return NewStaticFloat(lhs.asFloat() / rhs.asFloat()), nil
	case OpMod:
		return NewStaticFloat(math.Mod(lhs.asFloat(), rhs.asFloat())), nil
	case OpMult:
		return NewStaticFloat(lhs.asFloat() * rhs.asFloat()), nil
	case OpGreater:
		return NewStaticBool(lhs.asFloat() > rhs.asFloat()), nil
	case OpGreaterEqual:
		return NewStaticBool(lhs.asFloat() >= rhs.asFloat()), nil
	case OpLess:
		return NewStaticBool(lhs.asFloat() < rhs.asFloat()), nil
	case OpLessEqual:
		return NewStaticBool(lhs.asFloat() <= rhs.asFloat()), nil
	case OpPower:
		return NewStaticFloat(math.Pow(lhs.asFloat(), rhs.asFloat())), nil
	case OpEqual:
		return NewStaticBool(lhs.Equals(rhs)), nil
	case OpNotEqual:
		return NewStaticBool(!lhs.Equals(rhs)), nil
	case OpRegex:
		matched, err := regexp.MatchString(rhs.S, lhs.S)
		return NewStaticBool(matched), err
	case OpNotRegex:
		matched, err := regexp.MatchString(rhs.S, lhs.S)
		return NewStaticBool(!matched), err
	case OpAnd:
		return NewStaticBool(lhs.B && rhs.B), nil
	case OpOr:
		return NewStaticBool(lhs.B || rhs.B), nil
	default:
		return NewStaticNil(), errors.New("unexpected operator " + o.Op.String())
	}
}

// why does this and the above exist?
func binOp(op Operator, lhs, rhs Static) (bool, error) {
	lhsT := lhs.impliedType()
	rhsT := rhs.impliedType()
	if !lhsT.isMatchingOperand(rhsT) {
		return false, nil
	}

	if !op.binaryTypesValid(lhsT, rhsT) {
		return false, nil
	}

	switch op {
	case OpGreater:
		return lhs.asFloat() > rhs.asFloat(), nil
	case OpGreaterEqual:
		return lhs.asFloat() >= rhs.asFloat(), nil
	case OpLess:
		return lhs.asFloat() < rhs.asFloat(), nil
	case OpLessEqual:
		return lhs.asFloat() <= rhs.asFloat(), nil
	case OpEqual:
		return lhs.Equals(rhs), nil
	case OpNotEqual:
		return !lhs.Equals(rhs), nil
	case OpAnd:
		return lhs.B && rhs.B, nil
	case OpOr:
		return lhs.B || rhs.B, nil
	}

	return false, errors.New("unexpected operator " + op.String())
}

func (o UnaryOperation) execute(snapshot Snapshot) (Static, error) {
	static, err := o.Expression.execute(snapshot)
	if err != nil {
		return NewStaticNil(), err
	}

	if o.Op == OpNot {
		if static.Type != TypeBoolean {
			return NewStaticNil(), fmt.Errorf("expression (%v) expected a boolean, but got %v", o, static.Type)
		}
		return NewStaticBool(!static.B), nil
	}
	if o.Op == OpSub {
		if !static.Type.isNumeric() {
			return NewStaticNil(), fmt.Errorf("expression (%v) expected a numeric, but got %v", o, static.Type)
		}
		switch static.Type {
		case TypeInt:
			return NewStaticInt(-1 * static.N), nil
		case TypeFloat:
			return NewStaticFloat(-1 * static.F), nil
		case TypeDuration:
			return NewStaticDuration(-1 * static.D), nil
		}
	}

	return NewStaticNil(), errors.New("UnaryOperation has Op different from Not and Sub")
}

func (s Static) execute(snapshot Snapshot) (Static, error) {
	return s, nil
}

func (a Attribute) execute(snapshot Snapshot) (Static, error) {
	atts := snapshot.Attributes()
	static, ok := atts[a]
	if ok {
		return static, nil
	}

	if a.Scope == AttributeScopeNone {
		for attribute, static := range atts {
			if a.Name == attribute.Name && attribute.Scope == AttributeScopeSnapshot {
				return static, nil
			}
		}
		for attribute, static := range atts {
			if a.Name == attribute.Name {
				return static, nil
			}
		}
	}

	return NewStaticNil(), nil
}

func uniqueSnapshots(ss1 []*SnapshotResult, ss2 []*SnapshotResult) []Snapshot {
	ss1Count := len(ss1)
	ss2Count := len(ss2)

	output := make([]Snapshot, 0, ss1Count+ss2Count)

	ssCount := ss2Count
	ssSmaller := ss2
	ssLarger := ss1
	if ss1Count < ss2Count {
		ssCount = ss1Count
		ssSmaller = ss1
		ssLarger = ss2
	}

	snaps := make(map[Snapshot]struct{}, ssCount)
	for _, ss := range ssSmaller {
		snaps[ss.Snapshot] = struct{}{}
		output = append(output, ss.Snapshot)
	}

	// only add the snapshots from ssLarger that aren't in the map
	for _, ss := range ssLarger {
		if _, ok := snaps[ss.Snapshot]; !ok {
			output = append(output, ss.Snapshot)
		}
	}

	return output
}
