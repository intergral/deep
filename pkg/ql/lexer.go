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

package ql

import (
	"errors"
	"fmt"
	"github.com/prometheus/common/model"
	"strconv"
	"strings"
	"text/scanner"
	"time"
	"unicode"
)

type lexer struct {
	scanner.Scanner
	expr   *RootExpr
	parser *yyParserImpl
	errs   []error
}

var tokens = map[string]int{
	"{":  OPEN_BRACE,
	"}":  CLOSE_BRACE,
	"=":  EQ,
	".":  DOT,
	"!=": NEQ,
	">":  GT,
	">=": GTE,
	"<":  LT,
	"<=": LTE,
}

func (l *lexer) Lex(lval *yySymType) int {
	r := l.Scan()

	switch r {
	case scanner.EOF:
		return 0
	case scanner.String, scanner.RawString:
		var err error
		lval.staticStr, err = strconv.Unquote(l.TokenText())
		if err != nil {
			l.Error(err.Error())
			return 0
		}
		return STRING

	case scanner.Int:
		numberText := l.TokenText()

		// first try to parse as duration
		duration, ok := tryScanDuration(numberText, &l.Scanner)
		if ok {
			lval.staticDuration = duration
			return DURATION
		}

		// if we can't then just try an int
		var err error
		lval.staticInt, err = strconv.Atoi(numberText)
		if err != nil {
			l.Error(err.Error())
			return 0
		}
		return INTEGER

	case scanner.Float:
		var err error
		lval.staticFloat, err = strconv.ParseFloat(l.TokenText(), 64)
		if err != nil {
			l.Error(err.Error())
			return 0
		}
		return FLOAT
	}

	tokStrNext := l.TokenText() + string(l.Peek())
	if tok, ok := tokens[tokStrNext]; ok {
		l.Next()
		return tok
	}

	if tok, ok := tokens[l.TokenText()]; ok {
		return tok
	}

	lval.staticStr = l.TokenText()

	if string(l.Peek()) == "{" {
		if tok, ok := triggerTypes[lval.staticStr]; ok {
			return tok
		}
		if tok, ok := commandTypes[lval.staticStr]; ok {
			return tok
		}
	}

	return IDENTIFIER
}

func (l *lexer) Error(msg string) {
	l.errs = append(l.errs, errors.New(fmt.Sprintf("parse error at line %d, col %d near token '%s': %s", l.Line, l.Column, l.TokenText(), msg)))
}

func ParseString(str string) (*RootExpr, error) {
	l := lexer{
		parser: yyNewParser().(*yyParserImpl),
	}
	l.Init(strings.NewReader(str))
	l.Scanner.Error = func(_ *scanner.Scanner, msg string) {
		l.Error(msg)
	}
	e := l.parser.Parse(&l)
	if len(l.errs) > 0 {
		return nil, l.errs[0]
	}

	if e != 0 {
		return nil, fmt.Errorf("unknown parse error: %d", e)
	}

	err := l.expr.validate()
	if err != nil {
		return nil, err
	}

	return l.expr, nil
}

func tryScanDuration(number string, l *scanner.Scanner) (time.Duration, bool) {
	var sb strings.Builder
	sb.WriteString(number)
	// copy the scanner to avoid advancing it in case it's not a duration.
	s := *l
	consumed := 0
	for r := s.Peek(); r != scanner.EOF && !unicode.IsSpace(r); r = s.Peek() {
		if !unicode.IsNumber(r) && !isDurationRune(r) && r != '.' {
			break
		}
		_, _ = sb.WriteRune(r)
		_ = s.Next()
		consumed++
	}

	if consumed == 0 {
		return 0, false
	}
	// we've found more characters before a whitespace or the end
	d, err := parseDuration(sb.String())
	if err != nil {
		return 0, false
	}
	// we need to consume the scanner, now that we know this is a duration.
	for i := 0; i < consumed; i++ {
		_ = l.Next()
	}
	return d, true
}

func parseDuration(d string) (time.Duration, error) {
	var duration time.Duration
	// Try to parse promql style durations first, to ensure that we support the same duration
	// units as promql
	prometheusDuration, err := model.ParseDuration(d)
	if err != nil {
		// Fall back to standard library's time.ParseDuration if a promql style
		// duration couldn't be parsed.
		duration, err = time.ParseDuration(d)
		if err != nil {
			return 0, err
		}
	} else {
		duration = time.Duration(prometheusDuration)
	}

	return duration, nil
}

func isDurationRune(r rune) bool {
	// "ns", "us" (or "µs"), "ms", "s", "m", "h".
	switch r {
	case 'n', 's', 'u', 'm', 'h', 'µ', 'd', 'w', 'y':
		return true
	default:
		return false
	}
}
