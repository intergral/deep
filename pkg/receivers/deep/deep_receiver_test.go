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

package deep

import (
	"reflect"
	"testing"

	"github.com/intergral/deep/pkg/receivers/config/configgrpc"
	"github.com/intergral/deep/pkg/receivers/config/confignet"
)

func TestCreateConfig(t *testing.T) {
	type args struct {
		cfg interface{}
	}
	tests := []struct {
		name string
		args args
		want *DeepConfig
	}{
		{
			name: "empty config gets default",
			args: args{
				cfg: nil,
			},
			want: &DeepConfig{
				Debug: false,
				Protocols: &Protocols{
					GRPC: &configgrpc.GRPCServerSettings{
						NetAddr: confignet.NetAddr{
							Endpoint:  "0.0.0.0:43315",
							Transport: "tcp",
						},
					},
				},
			},
		},
		{
			name: "will default protocols if not set",
			args: args{
				cfg: map[string]interface{}{},
			},
			want: &DeepConfig{
				Protocols: &Protocols{
					GRPC: &configgrpc.GRPCServerSettings{
						NetAddr: confignet.NetAddr{
							Endpoint:  "0.0.0.0:43315",
							Transport: "tcp",
						},
					},
				},
			},
		},
		{
			name: "can set new port",
			args: args{
				cfg: map[string]interface{}{
					"protocols": map[string]interface{}{
						"grpc": map[string]interface{}{
							"Endpoint": "4444",
						},
					},
				},
			},
			want: &DeepConfig{
				Debug: false,
				Protocols: &Protocols{
					GRPC: &configgrpc.GRPCServerSettings{
						NetAddr: confignet.NetAddr{
							Endpoint: "4444",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := CreateConfig(tt.args.cfg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateConfig() = got: %v, expected: %v", got, tt.want)
			}
		})
	}
}
