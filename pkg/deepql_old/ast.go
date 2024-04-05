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

package deepql_old

import (
	"fmt"
)

// region RootExpr
type RootExpr struct {
	Pipeline Pipeline
	trigger  *trigger
	command  *command
}

func newRootExpr(e pipelineElement) *RootExpr {
	p, ok := e.(Pipeline)
	if !ok {
		p = newPipeline(e)
	}

	return &RootExpr{
		Pipeline: p,
	}
}

// endregion

// region Pipeline
type Pipeline struct {
	Elements []pipelineElement
}

func (Pipeline) __scalarExpression() {}

func (Pipeline) __snapshotExpression() {}

func newPipeline(i ...pipelineElement) Pipeline {
	return Pipeline{
		Elements: i,
	}
}

func (p Pipeline) addItem(i pipelineElement) Pipeline {
	p.Elements = append(p.Elements, i)
	return p
}

func (p Pipeline) impliedType() StaticType {
	if len(p.Elements) == 0 {
		return TypeSnapshot
	}

	finalItem := p.Elements[len(p.Elements)-1]
	aggregate, ok := finalItem.(Aggregate)
	if ok {
		return aggregate.impliedType()
	}

	return TypeSnapshot
}

func (p Pipeline) extractConditions(req *FetchSnapshotRequest) {
	for _, element := range p.Elements {
		element.extractConditions(req)
	}
	// TODO this needs to be fine-tuned a bit, e.g. { .foo = "bar" } | by(.namespace), AllConditions can still be true
	if len(p.Elements) > 1 {
		req.AllConditions = false
	}
}

func (p Pipeline) evaluate(input []*SnapshotResult) (result []*SnapshotResult, err error) {
	result = input

	for _, element := range p.Elements {
		result, err = element.evaluate(result)
		if err != nil {
			return nil, err
		}

		if len(result) == 0 {
			return []*SnapshotResult{}, nil
		}
	}

	return result, nil
}

type pipelineElement interface {
	Element

	extractConditions(request *FetchSnapshotRequest)
	evaluate([]*SnapshotResult) ([]*SnapshotResult, error)
}

// endregion
type typedExpression interface {
	impliedType() StaticType
}

type Element interface {
	fmt.Stringer
	validate() error
}

// region GroupOperation
type GroupOperation struct {
	Expression FieldExpression
}

func (o GroupOperation) extractConditions(request *FetchSnapshotRequest) {
	o.Expression.extractConditions(request)
}

func (GroupOperation) evaluate(ss []*SnapshotResult) ([]*SnapshotResult, error) {
	return ss, nil
}

func newGroupOperation(e FieldExpression) GroupOperation {
	return GroupOperation{
		Expression: e,
	}
}

// endregion

// region CoalesceOperation
type CoalesceOperation struct{}

func (CoalesceOperation) evaluate(ss []*SnapshotResult) ([]*SnapshotResult, error) {
	return ss, nil
}

func (o CoalesceOperation) extractConditions(request *FetchSnapshotRequest) {
}

func newCoalesceOperation() CoalesceOperation {
	return CoalesceOperation{}
}

// endregion

// **********************
// Expressions
// **********************
type FieldExpression interface {
	Element
	typedExpression

	// referencesSnapshot returns true if this field expression has any attributes or intrinsics. i.e. it references the snapshot itself
	referencesSnapshot() bool
	extractConditions(request *FetchSnapshotRequest)
	execute(snapshots Snapshot) (Static, error)

	__fieldExpression()
}

type ScalarFilter struct {
	op  Operator
	lhs ScalarExpression
	rhs ScalarExpression
}

func newScalarFilter(op Operator, lhs ScalarExpression, rhs ScalarExpression) ScalarFilter {
	return ScalarFilter{
		op:  op,
		lhs: lhs,
		rhs: rhs,
	}
}

func (f ScalarFilter) extractConditions(request *FetchSnapshotRequest) {
	f.lhs.extractConditions(request)
	f.rhs.extractConditions(request)
}

// region Scalars

type ScalarExpression interface {
	Element
	typedExpression

	extractConditions(request *FetchSnapshotRequest)

	__scalarExpression()
}

type ScalarOperation struct {
	Op  Operator
	LHS ScalarExpression
	RHS ScalarExpression
}

func (ScalarOperation) __scalarExpression() {}

func (o ScalarOperation) impliedType() StaticType {
	if o.Op.isBoolean() {
		return TypeBoolean
	}

	// remaining operators will be based on the operands
	// opAdd, opSub, opDiv, opMod, opMult
	t := o.LHS.impliedType()
	if t != TypeAttribute {
		return t
	}

	return o.RHS.impliedType()
}

func (o ScalarOperation) extractConditions(request *FetchSnapshotRequest) {
	o.LHS.extractConditions(request)
	o.RHS.extractConditions(request)
	request.AllConditions = false
}

func newScalarOperation(op Operator, lhs ScalarExpression, rhs ScalarExpression) ScalarOperation {
	return ScalarOperation{
		Op:  op,
		LHS: lhs,
		RHS: rhs,
	}
}

// endregion

// region Aggregate
type Aggregate struct {
	op AggregateOp
	e  FieldExpression
}

func (Aggregate) __scalarExpression() {}

func (a Aggregate) impliedType() StaticType {
	if a.op == aggregateCount || a.e == nil {
		return TypeInt
	}

	return a.e.impliedType()
}

func (a Aggregate) extractConditions(request *FetchSnapshotRequest) {
	if a.e != nil {
		a.e.extractConditions(request)
	}
}

func newAggregate(agg AggregateOp, e FieldExpression) Aggregate {
	return Aggregate{
		op: agg,
		e:  e,
	}
}

// endregion
type SnapshotExpression interface {
	pipelineElement

	__snapshotExpression()
}

// region SnapshotFilter

type SnapshotFilter struct {
	Expression FieldExpression
}

func (SnapshotFilter) __snapshotExpression() {}

func (f SnapshotFilter) extractConditions(request *FetchSnapshotRequest) {
	f.Expression.extractConditions(request)
}

func (f SnapshotFilter) evaluate(input []*SnapshotResult) ([]*SnapshotResult, error) {
	var output []*SnapshotResult

	for _, ss := range input {
		result, err := f.Expression.execute(ss.Snapshot)
		if err != nil {
			return nil, err
		}

		if result.Type != TypeBoolean {
			continue
		}

		if !result.B {
			continue
		}

		output = append(output, ss)
	}

	return output, nil
}

func newSnapshotFilter(e FieldExpression) SnapshotFilter {
	return SnapshotFilter{e}
}

// endregion

// region Snapshots

type SnapshotOperation struct {
	Op  Operator
	LHS SnapshotExpression
	RHS SnapshotExpression
}

func (SnapshotOperation) __snapshotExpression() {}

func (o SnapshotOperation) extractConditions(request *FetchSnapshotRequest) {
	o.LHS.extractConditions(request)
	o.RHS.extractConditions(request)
	request.AllConditions = false
}

func newSnapshotOperation(op Operator, lhs SnapshotExpression, rhs SnapshotExpression) SnapshotOperation {
	return SnapshotOperation{
		Op:  op,
		LHS: lhs,
		RHS: rhs,
	}
}

// endregion

// region BinaryOperation
type BinaryOperation struct {
	Op  Operator
	LHS FieldExpression
	RHS FieldExpression
}

func (BinaryOperation) __fieldExpression() {}

func (o BinaryOperation) impliedType() StaticType {
	if o.Op.isBoolean() {
		return TypeBoolean
	}

	// remaining operators will be based on the operands
	// opAdd, opSub, opDiv, opMod, opMult
	t := o.LHS.impliedType()
	if t != TypeAttribute {
		return t
	}

	return o.RHS.impliedType()
}

func (o BinaryOperation) referencesSnapshot() bool {
	return o.LHS.referencesSnapshot() || o.RHS.referencesSnapshot()
}

func (o BinaryOperation) extractConditions(request *FetchSnapshotRequest) {
	// TODO we can further optimise this by attempting to execute every FieldExpression, if they only contain statics it should resolve
	switch o.LHS.(type) {
	case Attribute:
		switch o.RHS.(type) {
		case Static:
			request.appendCondition(Condition{
				Attribute: o.LHS.(Attribute),
				Op:        o.Op,
				Operands:  []Static{o.RHS.(Static)},
			})
		case Attribute:
			// Both sides are attributes, just fetch both
			request.appendCondition(Condition{
				Attribute: o.LHS.(Attribute),
				Op:        OpNone,
				Operands:  nil,
			})
			request.appendCondition(Condition{
				Attribute: o.RHS.(Attribute),
				Op:        OpNone,
				Operands:  nil,
			})
		default:
			// Just fetch LHS and try to do something smarter with RHS
			request.appendCondition(Condition{
				Attribute: o.LHS.(Attribute),
				Op:        OpNone,
				Operands:  nil,
			})
			o.RHS.extractConditions(request)
		}
	case Static:
		switch o.RHS.(type) {
		case Static:
			// 2 statics, don't need to send any conditions
			return
		case Attribute:
			request.appendCondition(Condition{
				Attribute: o.RHS.(Attribute),
				Op:        o.Op,
				Operands:  []Static{o.LHS.(Static)},
			})
		default:
			o.RHS.extractConditions(request)
		}
	default:
		o.LHS.extractConditions(request)
		o.RHS.extractConditions(request)
		request.AllConditions = request.AllConditions && (o.Op != OpOr)
	}
}

func newBinaryOperation(op Operator, lhs FieldExpression, rhs FieldExpression) BinaryOperation {
	return BinaryOperation{
		Op:  op,
		LHS: lhs,
		RHS: rhs,
	}
}

// endregion

// region UnaryOperation
type UnaryOperation struct {
	Op         Operator
	Expression FieldExpression
}

func (UnaryOperation) __fieldExpression() {}

func (o UnaryOperation) impliedType() StaticType {
	// both operators (opPower and opNot) will just be based on the operand type
	return o.Expression.impliedType()
}

func (o UnaryOperation) referencesSnapshot() bool {
	return o.Expression.referencesSnapshot()
}

func (o UnaryOperation) extractConditions(request *FetchSnapshotRequest) {
	// TODO when Op is Not we should just either negate all inner Operands or just fetch the columns with OpNone
	o.Expression.extractConditions(request)
}

func newUnaryOperation(op Operator, e FieldExpression) UnaryOperation {
	return UnaryOperation{
		Op:         op,
		Expression: e,
	}
}

// endregion
func NewIntrinsic(n Intrinsic) Attribute {
	return Attribute{
		Scope:     AttributeScopeNone,
		Parent:    false,
		Name:      n.String(),
		Intrinsic: n,
	}
}
