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

package deepql

import "fmt"

type Operator int

const (
	OpNone Operator = iota
	OpAdd
	OpSub
	OpDiv
	OpMod
	OpMult
	OpEqual
	OpNotEqual
	OpRegex
	OpNotRegex
	OpGreater
	OpGreaterEqual
	OpLess
	OpLessEqual
	OpPower
	OpAnd
	OpOr
	OpNot
	OpSnapshotChild
	OpSnapshotDescendant
	OpSnapshotAnd
	OpSnapshotUnion
	OpSnapshotSibling
)

func (op Operator) isBoolean() bool {
	return op == OpOr ||
		op == OpAnd ||
		op == OpEqual ||
		op == OpNotEqual ||
		op == OpRegex ||
		op == OpNotRegex ||
		op == OpGreater ||
		op == OpGreaterEqual ||
		op == OpLess ||
		op == OpLessEqual ||
		op == OpNot
}

func (op Operator) binaryTypesValid(lhsT StaticType, rhsT StaticType) bool {
	return binaryTypeValid(op, lhsT) && binaryTypeValid(op, rhsT)
}

func binaryTypeValid(op Operator, t StaticType) bool {
	if t == TypeAttribute {
		return true
	}

	switch t {
	case TypeBoolean:
		return op == OpAnd ||
			op == OpOr ||
			op == OpEqual ||
			op == OpNotEqual
	case TypeFloat:
		fallthrough
	case TypeInt:
		fallthrough
	case TypeDuration:
		return op == OpAdd ||
			op == OpSub ||
			op == OpMult ||
			op == OpDiv ||
			op == OpMod ||
			op == OpPower ||
			op == OpEqual ||
			op == OpNotEqual ||
			op == OpGreater ||
			op == OpGreaterEqual ||
			op == OpLess ||
			op == OpLessEqual
	case TypeString:
		return op == OpEqual ||
			op == OpNotEqual ||
			op == OpRegex ||
			op == OpNotRegex
	case TypeNil:
		return op == OpEqual || op == OpNotEqual
	}

	return false
}

func (op Operator) unaryTypesValid(t StaticType) bool {
	if t == TypeAttribute {
		return true
	}

	switch op {
	case OpSub:
		return t.isNumeric()
	case OpNot:
		return t == TypeBoolean
	}

	return false
}

func (op Operator) String() string {

	switch op {
	case OpAdd:
		return "+"
	case OpSub:
		return "-"
	case OpDiv:
		return "/"
	case OpMod:
		return "%"
	case OpMult:
		return "*"
	case OpEqual:
		return "="
	case OpNotEqual:
		return "!="
	case OpRegex:
		return "=~"
	case OpNotRegex:
		return "!~"
	case OpGreater:
		return ">"
	case OpGreaterEqual:
		return ">="
	case OpLess:
		return "<"
	case OpLessEqual:
		return "<="
	case OpPower:
		return "^"
	case OpAnd:
		return "&&"
	case OpOr:
		return "||"
	case OpNot:
		return "!"
	case OpSnapshotChild:
		return ">"
	case OpSnapshotDescendant:
		return ">>"
	case OpSnapshotAnd:
		return "&&"
	case OpSnapshotSibling:
		return "~"
	case OpSnapshotUnion:
		return "||"
	}

	return fmt.Sprintf("operator(%d)", op)
}
