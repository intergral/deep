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
	"strings"
	"text/scanner"
)

func init() {
	yyErrorVerbose = true
	// yyDebug = 3
	// replaces constants with actual identifiers in error messages
	//   i.e. "expecting OPEN_BRACE" => "expecting {"
	for str, tok := range tokens {
		yyToknames[tok-yyPrivate+1] = str
	}
}

func Parse(s string) (expr *RootExpr, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			if err, ok = r.(error); ok {
				if errors.Is(err, ParseError{}) {
					return
				}
				err = newParseError(err.Error(), 0, 0)
			}
		}
	}()
	l := lexer{
		parser: yyNewParser().(*yyParserImpl),
	}
	l.Init(strings.NewReader(s))
	l.Scanner.Error = func(_ *scanner.Scanner, msg string) {
		l.Error(msg)
	}
	e := l.parser.Parse(&l)
	if len(l.errs) > 0 {
		return nil, errors.Join(l.errs...)
	}
	if e != 0 {
		return nil, fmt.Errorf("unknown parse error: %d", e)
	}

	err = l.expr.validate()
	return l.expr, err
}

func ParseIdentifier(s string) (Attribute, error) {
	if i := intrinsicFromString(s); i != IntrinsicNone {
		return NewIntrinsic(i), nil
	}

	switch {
	case strings.HasPrefix(s, "."):
		return NewAttribute(strings.TrimPrefix(s, ".")), nil
	case strings.HasPrefix(s, "resource."):
		return NewScopedAttribute(AttributeScopeResource, false, strings.TrimPrefix(s, "resource.")), nil
	case strings.HasPrefix(s, "span."):
		return NewScopedAttribute(AttributeScopeSnapshot, false, strings.TrimPrefix(s, "span.")), nil
	default:
		return Attribute{}, fmt.Errorf("tag name is not valid intrinsic or scoped attribute: %s", s)
	}
}

func MustParseIdentifier(s string) Attribute {
	a, err := ParseIdentifier(s)
	if err != nil {
		panic(err)
	}
	return a
}

// ParseError is what is returned when we failed to parse.
type ParseError struct {
	msg       string
	line, col int
}

func (p ParseError) Error() string {
	if p.col == 0 && p.line == 0 {
		return fmt.Sprintf("parse error : %s", p.msg)
	}
	return fmt.Sprintf("parse error at line %d, col %d: %s", p.line, p.col, p.msg)
}

func newParseError(msg string, line, col int) ParseError {
	return ParseError{
		msg:  msg,
		line: line,
		col:  col,
	}
}
