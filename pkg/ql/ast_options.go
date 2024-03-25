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

import "strings"

type configOption struct {
	op  Operator
	lhs string
	rhs Static
	fnc func(c *configOption, target interface{}) error
}

func (c *configOption) apply(cfg interface{}) error {
	return c.fnc(c, cfg)
}

func newConfigOption(op Operator, lhs string, rhs Static) configOption {
	return configOption{
		op:  op,
		lhs: lhs,
		rhs: rhs,
		fnc: applyFuncFor(lhs),
	}
}

func applyFuncFor(lhs string) func(c *configOption, tri interface{}) error {
	return func(c *configOption, tri interface{}) error {
		if v, ok := tri.(*trigger); ok {
			return applyFuncForTrigger(lhs)(c, v)
		}

		if v, ok := tri.(*command); ok {
			return applyFuncForCommand(lhs)(c, v)
		}

		return nil
	}
}

func stripPrefix(lhs string, s string) string {
	after, _ := strings.CutPrefix(lhs, s)
	return after
}
