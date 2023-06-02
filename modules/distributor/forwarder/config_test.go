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

package forwarder

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/modules/distributor/forwarder/otlpgrpc"
)

func TestConfig_Validate(t *testing.T) {
	type fields struct {
		Name     string
		Backend  string
		OTLPGRPC otlpgrpc.Config
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "ReturnsNoErrorWithValidArguments",
			fields: fields{
				Name:    "test",
				Backend: OTLPGRPCBackend,
				OTLPGRPC: otlpgrpc.Config{
					Endpoints: nil,
					TLS: otlpgrpc.TLSConfig{
						Insecure: true,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ReturnsErrorWithEmptyName",
			fields: fields{
				Name:    "",
				Backend: OTLPGRPCBackend,
				OTLPGRPC: otlpgrpc.Config{
					Endpoints: nil,
					TLS: otlpgrpc.TLSConfig{
						Insecure: false,
						CertFile: "",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "ReturnsErrorWithUnsupportedBackendName",
			fields: fields{
				Name:    "test",
				Backend: "unsupported",
				OTLPGRPC: otlpgrpc.Config{
					Endpoints: nil,
					TLS: otlpgrpc.TLSConfig{
						Insecure: true,
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Name:     tt.fields.Name,
				Backend:  tt.fields.Backend,
				OTLPGRPC: tt.fields.OTLPGRPC,
			}

			err := cfg.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
