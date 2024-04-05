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
)

const (
	list       = "list"
	deleteType = "delete"
)

var commandTypes = map[string]int{
	list:       COMMAND,
	deleteType: COMMAND,
}

type command struct {
	command string

	file string
	id   string

	errors []error
}

func (c *command) validate() error {
	if len(c.errors) > 0 {
		return errors.Join(c.errors...)
	}
	return nil
}

func newCommand(typ string, opts []configOption) command {
	var cmd command
	switch typ {
	case list:
		cmd = command{command: list}
	case deleteType:
		cmd = command{command: deleteType}
	}

	for _, cfg := range opts {
		err := cfg.apply(&cmd)
		if err != nil {
			cmd.errors = append(cmd.errors, err)
		}
	}

	return cmd
}

func applyFuncForCommand(lhs string) func(c *configOption, cmd *command) error {
	switch lhs {
	case "file":
		return func(c *configOption, cmd *command) error {
			cmd.file = c.rhs.S
			return nil
		}
	case "id":
		return func(c *configOption, cmd *command) error {
			cmd.id = c.rhs.S
			return nil
		}
	}
	return func(c *configOption, cmd *command) error {
		return errors.New(fmt.Sprintf("parse error unrecognized option: %s", lhs))
	}
}
