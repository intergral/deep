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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/intergral/deep/pkg/deeppb"
	"github.com/olekukonko/tablewriter"
)

type listTracepointCmd struct {
	apiOptions
}

func (cmd *listTracepointCmd) Run(ctx *globalOptions) error {
	request, err := http.NewRequest("GET", fmt.Sprintf("http://%s/tracepoints/api/tracepoints", cmd.Endpoint), nil)
	request.Header.Add("Accept", "application/protobuf")
	if err != nil {
		return err
	}

	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	all, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Path", "Line", "Args"})
	var data deeppb.LoadTracepointResponse

	err = proto.Unmarshal(all, &data)
	if err != nil {
		return err
	}

	tps := data.Response.Response

	for _, tp := range tps {
		marshal, err := json.Marshal(tp.Args)
		if err != nil {
			return err
		}

		table.Append([]string{tp.ID, tp.Path, fmt.Sprintf("%d", tp.LineNumber), string(marshal)})
	}

	table.Render()

	return nil
}
