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

type AttributeScope int

const (
	AttributeScopeNone AttributeScope = iota
	AttributeScopeResource
	AttributeScopeSnapshot
)

func (s AttributeScope) String() string {
	switch s {
	case AttributeScopeNone:
		return "none"
	case AttributeScopeSnapshot:
		return "snapshot"
	case AttributeScopeResource:
		return "resource"
	}

	return fmt.Sprintf("att(%d).", s)
}

type Intrinsic int

const (
	IntrinsicNone Intrinsic = iota
	IntrinsicDuration
)

func (i Intrinsic) String() string {
	switch i {
	case IntrinsicNone:
		return "none"
	case IntrinsicDuration:
		return "duration"
	}

	return fmt.Sprintf("intrinsic(%d)", i)
}

// intrinsicFromString returns the matching intrinsic for the given string or -1 if there is none
func intrinsicFromString(s string) Intrinsic {
	switch s {
	case "duration":
		return IntrinsicDuration
	}

	return IntrinsicNone
}

// **********************
// Attributes
// **********************

type Attribute struct {
	Scope     AttributeScope
	Parent    bool
	Name      string
	Intrinsic Intrinsic
}

// NewAttribute creates a new attribute with the given identifier string.
func NewAttribute(att string) Attribute {
	return Attribute{
		Scope:     AttributeScopeNone,
		Parent:    false,
		Name:      att,
		Intrinsic: IntrinsicNone,
	}
}

// nolint: revive
func (Attribute) __fieldExpression() {}

func (a Attribute) impliedType() StaticType {
	switch a.Intrinsic {
	case IntrinsicDuration:
		return TypeDuration
	}

	return TypeAttribute
}

func (Attribute) referencesSnapshot() bool {
	return true
}

func (a Attribute) extractConditions(request *FetchSnapshotRequest) {
	request.appendCondition(Condition{
		Attribute: a,
		Op:        OpNone,
		Operands:  nil,
	})
}

// NewScopedAttribute creates a new scopedattribute with the given identifier string.
// this handles parent, snapshot, and resource scopes.
func NewScopedAttribute(scope AttributeScope, parent bool, att string) Attribute {
	intrinsic := IntrinsicNone
	// if we are explicitly passed a resource or snapshot scopes then we shouldn't parse for intrinsic
	if scope != AttributeScopeResource && scope != AttributeScopeSnapshot {
		intrinsic = intrinsicFromString(att)
	}

	return Attribute{
		Scope:     scope,
		Parent:    parent,
		Name:      att,
		Intrinsic: intrinsic,
	}
}
