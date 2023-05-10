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

package main

import (
	"bytes"
	"fmt"
	"net/http"
)

type cmdCreateTracepoint struct {
	apiOptions

	Path       string `arg:"" help:"Path of the source file" default:"simple_test.py"`
	LineNumber int    `arg:"" help:"Line in the source file" default:"31"`
	FireCount  string `arg:"" help:"The number of times to fire" default:"-1"`
}

func (cmd *cmdCreateTracepoint) Run(ctx *globalOptions) error {

	var jsonData = []byte(fmt.Sprintf("{\"Tracepoint\":{\"path\":\"%s\",\"line_number\":%d, \"args\":{\"fire_count\":\"%s\"}}}", cmd.Path, cmd.LineNumber, cmd.FireCount))

	resp, err := http.Post(fmt.Sprintf("http://%s/tracepoints/api/tracepoints", cmd.Endpoint), "application/json", bytes.NewReader(jsonData))
	defer resp.Body.Close()

	return err

}
