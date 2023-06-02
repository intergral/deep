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

import (
	"fmt"
	"strconv"
	"strings"
)

func (r RootExpr) String() string {
	return r.Pipeline.String()
}

func (p Pipeline) String() string {
	s := make([]string, 0, len(p.Elements))
	for _, p := range p.Elements {
		s = append(s, p.String())
	}
	return strings.Join(s, "|")
}

func (o GroupOperation) String() string {
	return "by(" + o.Expression.String() + ")"
}

func (o CoalesceOperation) String() string {
	return "coalesce()"
}

func (o ScalarOperation) String() string {
	return binaryOp(o.Op, o.LHS, o.RHS)
}

func (a Aggregate) String() string {
	if a.e == nil {
		return a.op.String() + "()"
	}

	return a.op.String() + "(" + a.e.String() + ")"
}

func (o SnapshotOperation) String() string {
	return binaryOp(o.Op, o.LHS, o.RHS)
}

func (f SnapshotFilter) String() string {
	return "{ " + f.Expression.String() + " }"
}

func (f ScalarFilter) String() string {
	return binaryOp(f.op, f.lhs, f.rhs)
}

func (o BinaryOperation) String() string {
	return binaryOp(o.Op, o.LHS, o.RHS)
}

func (o UnaryOperation) String() string {
	return unaryOp(o.Op, o.Expression)
}

func (n Static) String() string {
	return n.EncodeToString(true)
}

func (n Static) EncodeToString(quotes bool) string {
	switch n.Type {
	case TypeInt:
		return strconv.Itoa(n.N)
	case TypeFloat:
		return strconv.FormatFloat(n.F, 'f', 5, 64)
	case TypeString:
		if quotes {
			return "`" + n.S + "`"
		}
		return n.S
	case TypeBoolean:
		return strconv.FormatBool(n.B)
	case TypeNil:
		return "nil"
	case TypeDuration:
		return n.D.String()
	}

	return fmt.Sprintf("static(%d)", n.Type)
}

func (a Attribute) String() string {
	scopes := []string{}
	if a.Parent {
		scopes = append(scopes, "parent")
	}

	if a.Scope != AttributeScopeNone {
		attributeScope := a.Scope.String()
		scopes = append(scopes, attributeScope)
	}

	att := a.Name
	if a.Intrinsic != IntrinsicNone {
		att = a.Intrinsic.String()
	}

	scope := ""
	if len(scopes) > 0 {
		scope = strings.Join(scopes, ".") + "."
	}

	// Top-level attributes get a "." but top-level intrinsics don't
	if scope == "" && a.Intrinsic == IntrinsicNone {
		scope += "."
	}

	return scope + att
}

func binaryOp(op Operator, lhs Element, rhs Element) string {
	return wrapElement(lhs) + " " + op.String() + " " + wrapElement(rhs)
}

func unaryOp(op Operator, e Element) string {
	return op.String() + wrapElement(e)
}

func wrapElement(e Element) string {
	static, ok := e.(Static)
	if ok {
		return static.String()
	}
	att, ok := e.(Attribute)
	if ok {
		return att.String()
	}
	return "(" + e.String() + ")"
}
