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

package deepql

import (
	"errors"
	"fmt"

	"golang.org/x/exp/maps"
)

const (
	List       = "list"
	DeleteType = "delete"
)

var commandTypes = map[string]int{
	List:       COMMAND,
	DeleteType: COMMAND,
}

type command struct {
	command string

	rules []configOption
	id    string

	errors []error
}

func (c *command) validate() error {
	// if id is set then we cannot use other values
	if c.id != "" && len(c.rules) != 0 {
		c.errors = append(c.errors, fmt.Errorf("invalid command: cannot use id with query"))
	}

	if len(c.errors) > 0 {
		return errors.Join(c.errors...)
	}
	return nil
}

func (c *command) buildConditions() []Condition {
	mappedToAttr := map[string]Condition{}
	for _, rule := range c.rules {
		key := fmt.Sprintf("%s-%d", rule.lhs, rule.op)
		if v, ok := mappedToAttr[key]; ok {
			mappedToAttr[key] = Condition{
				Attribute: rule.lhs,
				Op:        rule.op,
				Operands:  append(v.Operands, rule.rhs),
			}
		} else {
			mappedToAttr[key] = Condition{
				Attribute: rule.lhs,
				Op:        rule.op,
				Operands:  []Static{rule.rhs},
			}
		}
	}
	return maps.Values(mappedToAttr)
}

func newCommand(typ string, opts []configOption) command {
	var cmd command
	switch typ {
	case List:
		cmd = command{command: List}
	case DeleteType:
		cmd = command{command: DeleteType}
	}

	cmd.rules = opts

	return cmd
}
