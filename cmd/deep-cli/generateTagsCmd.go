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
	"context"
	"google.golang.org/grpc"
)

type generateTagsCmd struct {
	generateSnapshotCmd
}

func (cmd *generateTagsCmd) Run(opts *globalOptions) error {
	client := cmd.connectGrpc()
	defer func(connection *grpc.ClientConn) {
		_ = connection.Close()
	}(cmd.connection)

	{
		snapshot := cmd.generateSnapshot(0, generateOptions{attrs: map[string]string{"tag_test": "simple"}, resource: map[string]string{"os_name": "windows"}, serviceName: "tag_test"})
		_, _ = client.Send(context.TODO(), snapshot)
	}

	return nil
}
