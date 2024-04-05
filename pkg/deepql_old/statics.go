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
	"fmt"
	"math"
	"time"
)

type StaticType int

const (
	TypeNil       StaticType = iota
	TypeSnapshot             // type used by snapshot pipelines
	TypeAttribute            // a special constant that indicates the type is determined at query time by the attribute
	TypeInt
	TypeFloat
	TypeString
	TypeBoolean
	TypeDuration
)

// isMatchingOperand returns whether two types can be combined with a binary operator. the kind of operator is
// immaterial. see Operator.typesValid() for code that determines if the passed types are valid for the given
// operator.
func (t StaticType) isMatchingOperand(otherT StaticType) bool {
	if t == TypeAttribute || otherT == TypeAttribute {
		return true
	}

	if t == otherT {
		return true
	}

	if t.isNumeric() && otherT.isNumeric() {
		return true
	}

	return false
}

func (t StaticType) isNumeric() bool {
	return t == TypeInt || t == TypeFloat || t == TypeDuration
}

// Status represents valid static values of typeStatus
type Status int

const (
	StatusError Status = iota
	StatusOk
	StatusUnset
)

func (s Status) String() string {
	switch s {
	case StatusError:
		return "error"
	case StatusOk:
		return "ok"
	case StatusUnset:
		return "unset"
	}

	return fmt.Sprintf("status(%d)", s)
}

type Kind int

const (
	KindUnspecified Kind = iota
	KindInternal
	KindClient
	KindServer
	KindProducer
	KindConsumer
)

func (k Kind) String() string {
	switch k {
	case KindUnspecified:
		return "unspecified"
	case KindInternal:
		return "internal"
	case KindClient:
		return "client"
	case KindServer:
		return "server"
	case KindProducer:
		return "producer"
	case KindConsumer:
		return "consumer"
	}

	return fmt.Sprintf("kind(%d)", k)
}

// **********************
// Statics
// **********************
type Static struct {
	Type StaticType
	N    int
	F    float64
	S    string
	B    bool
	D    time.Duration
}

func (Static) __fieldExpression() {}

func (Static) __scalarExpression() {}

func (Static) referencesSnapshot() bool {
	return false
}

func (s Static) extractConditions(request *FetchSnapshotRequest) {
}

func (s Static) impliedType() StaticType {
	return s.Type
}

func (s Static) Equals(other Static) bool {
	// if they are different number types. compare them as floats. however, if they are the same type just fall through to
	// a normal comparison which should be more efficient
	differentNumberTypes := (s.Type == TypeInt || s.Type == TypeFloat || s.Type == TypeDuration) &&
		(other.Type == TypeInt || other.Type == TypeFloat || other.Type == TypeDuration) &&
		s.Type != other.Type
	if differentNumberTypes {
		return s.asFloat() == other.asFloat()
	}

	// no special cases, just compare directly
	return s == other
}

func (s Static) asFloat() float64 {
	switch s.Type {
	case TypeInt:
		return float64(s.N)
	case TypeFloat:
		return s.F
	case TypeDuration:
		return float64(s.D.Nanoseconds())
	default:
		return math.NaN()
	}
}

func NewStaticInt(n int) Static {
	return Static{
		Type: TypeInt,
		N:    n,
	}
}

func NewStaticFloat(f float64) Static {
	return Static{
		Type: TypeFloat,
		F:    f,
	}
}

func NewStaticString(s string) Static {
	return Static{
		Type: TypeString,
		S:    s,
	}
}

func NewStaticBool(b bool) Static {
	return Static{
		Type: TypeBoolean,
		B:    b,
	}
}

func NewStaticNil() Static {
	return Static{
		Type: TypeNil,
	}
}

func NewStaticDuration(d time.Duration) Static {
	return Static{
		Type: TypeDuration,
		D:    d,
	}
}
