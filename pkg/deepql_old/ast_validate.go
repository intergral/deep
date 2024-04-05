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
)

// unsupportedError is returned for deepql_old features that are not yet supported.
type unsupportedError struct {
	feature string
}

func newUnsupportedError(feature string) unsupportedError {
	return unsupportedError{feature: feature}
}

func (e unsupportedError) Error() string {
	return e.feature + " not yet supported"
}

func (r RootExpr) validate() error {

	var errs []error
	if r.trigger != nil && r.command != nil {
		return errors.New("fatal error: cannot define a trigger and a command")
	}
	if r.trigger != nil {
		err := r.trigger.validate()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if r.command != nil {
		err := r.command.validate()
		if err != nil {
			errs = append(errs, err)
		}
	}
	err := r.Pipeline.validate()
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (p Pipeline) validate() error {
	for _, p := range p.Elements {
		err := p.validate()
		if err != nil {
			return err
		}
	}
	return nil
}

func (o GroupOperation) validate() error {
	return newUnsupportedError("by()")

	// todo: once grouping is supported the below validation will apply
	// if !o.Expression.referencesSnapshot() {
	// 	return fmt.Errorf("grouping field expressions must reference the snapshot: %s", o.String())
	// }

	// return o.Expression.validate()
}

func (o CoalesceOperation) validate() error {
	return newUnsupportedError("coalesce()")
}

func (o ScalarOperation) validate() error {
	if err := o.LHS.validate(); err != nil {
		return err
	}
	if err := o.RHS.validate(); err != nil {
		return err
	}

	lhsT := o.LHS.impliedType()
	rhsT := o.RHS.impliedType()
	if !lhsT.isMatchingOperand(rhsT) {
		return fmt.Errorf("binary operations must operate on the same type: %s", o.String())
	}

	if !o.Op.binaryTypesValid(lhsT, rhsT) {
		return fmt.Errorf("illegal operation for the given types: %s", o.String())
	}

	return nil
}

func (a Aggregate) validate() error {
	if a.e == nil {
		return nil
	}

	if err := a.e.validate(); err != nil {
		return err
	}

	// aggregate field expressions require a type of a number or attribute
	t := a.e.impliedType()
	if t != TypeAttribute && !t.isNumeric() {
		return fmt.Errorf("aggregate field expressions must resolve to a number type: %s", a.String())
	}

	if !a.e.referencesSnapshot() {
		return fmt.Errorf("aggregate field expressions must reference the snapshot: %s", a.String())
	}

	switch a.op {
	case aggregateCount, aggregateAvg, aggregateMin, aggregateMax, aggregateSum:
	default:
		return newUnsupportedError(fmt.Sprintf("aggregate operation (%v)", a.op))
	}

	return nil
}

func (o SnapshotOperation) validate() error {
	// TODO validate operator is a SnapshotSetOperator
	if err := o.LHS.validate(); err != nil {
		return err
	}
	if err := o.RHS.validate(); err != nil {
		return err
	}

	// supported snapshotset operations
	switch o.Op {
	case OpSnapshotChild, OpSnapshotDescendant, OpSnapshotSibling:
		return newUnsupportedError(fmt.Sprintf("snapshotset operation (%v)", o.Op))
	}

	return nil
}

func (f SnapshotFilter) validate() error {
	if err := f.Expression.validate(); err != nil {
		return err
	}

	t := f.Expression.impliedType()
	if t != TypeAttribute && t != TypeBoolean {
		return fmt.Errorf("snapshot filter field expressions must resolve to a boolean: %s", f.String())
	}

	return nil
}

func (f ScalarFilter) validate() error {
	if err := f.lhs.validate(); err != nil {
		return err
	}
	if err := f.rhs.validate(); err != nil {
		return err
	}

	lhsT := f.lhs.impliedType()
	rhsT := f.rhs.impliedType()
	if !lhsT.isMatchingOperand(rhsT) {
		return fmt.Errorf("binary operations must operate on the same type: %s", f.String())
	}

	if !f.op.binaryTypesValid(lhsT, rhsT) {
		return fmt.Errorf("illegal operation for the given types: %s", f.String())
	}

	// Only supported expression types
	switch f.lhs.(type) {
	case Aggregate:
	default:
		return newUnsupportedError("scalar filter lhs of type (%v)")
	}

	switch f.rhs.(type) {
	case Static:
	default:
		return newUnsupportedError("scalar filter rhs of type (%v)")
	}

	return nil
}

func (o BinaryOperation) validate() error {
	if err := o.LHS.validate(); err != nil {
		return err
	}
	if err := o.RHS.validate(); err != nil {
		return err
	}

	lhsT := o.LHS.impliedType()
	rhsT := o.RHS.impliedType()
	if !lhsT.isMatchingOperand(rhsT) {
		return fmt.Errorf("binary operations must operate on the same type: %s", o.String())
	}

	if !o.Op.binaryTypesValid(lhsT, rhsT) {
		return fmt.Errorf("illegal operation for the given types: %s", o.String())
	}

	switch o.Op {
	case OpNotRegex,
		OpSnapshotChild,
		OpSnapshotDescendant,
		OpSnapshotSibling:
		return newUnsupportedError(fmt.Sprintf("binary operation (%v)", o.Op))
	}

	return nil
}

func (o UnaryOperation) validate() error {
	if err := o.Expression.validate(); err != nil {
		return err
	}

	t := o.Expression.impliedType()
	if t == TypeAttribute {
		return nil
	}

	if !o.Op.unaryTypesValid(t) {
		return fmt.Errorf("illegal operation for the given type: %s", o.String())
	}

	return nil
}

func (n Static) validate() error {
	if n.Type == TypeNil {
		return newUnsupportedError("nil")
	}

	return nil
}

func (a Attribute) validate() error {
	if a.Parent {
		return newUnsupportedError("parent")
	}
	return nil
}
