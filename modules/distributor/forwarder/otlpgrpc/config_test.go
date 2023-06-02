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

package otlpgrpc

import (
	"testing"

	"github.com/grafana/dskit/flagext"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	type fields struct {
		Endpoints flagext.StringSlice
		TLS       TLSConfig
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "ReturnsNoErrorForValidInsecureConfig",
			fields: fields{
				Endpoints: nil,
				TLS: TLSConfig{
					Insecure: true,
				},
			},
			wantErr: false,
		},
		{
			name: "ReturnsNoErrorForValidSecureConfig",
			fields: fields{
				Endpoints: nil,
				TLS: TLSConfig{
					Insecure: false,
					CertFile: "/test/path",
				},
			},
			wantErr: false,
		},
		{
			name: "ReturnsErrorWithInsecureFalseAndNoCertFile",
			fields: fields{
				Endpoints: nil,
				TLS: TLSConfig{
					Insecure: false,
					CertFile: "",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Endpoints: tt.fields.Endpoints,
				TLS:       tt.fields.TLS,
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
