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

package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Encoding is the identifier for a chunk encoding.
type Encoding byte

// The different available encodings.
// Make sure to preserve the order, as these numeric values are written to the chunks!
const (
	EncNone Encoding = iota
	EncGZIP
	EncLZ4_64k
	EncLZ4_256k
	EncLZ4_1M
	EncLZ4_4M
	EncSnappy
	EncZstd
	EncS2
)

// SupportedEncoding is a slice of all supported encodings
var SupportedEncoding = []Encoding{
	EncNone,
	EncGZIP,
	EncLZ4_64k,
	EncLZ4_256k,
	EncLZ4_1M,
	EncLZ4_4M,
	EncSnappy,
	EncZstd,
	EncS2,
}

func (e Encoding) String() string {
	switch e {
	case EncNone:
		return "none"
	case EncGZIP:
		return "gzip"
	case EncLZ4_64k:
		return "lz4-64k"
	case EncLZ4_256k:
		return "lz4-256k"
	case EncLZ4_1M:
		return "lz4-1M"
	case EncLZ4_4M:
		return "lz4"
	case EncSnappy:
		return "snappy"
	case EncZstd:
		return "zstd"
	case EncS2:
		return "s2"
	default:
		return "unsupported"
	}
}

// UnmarshalYAML implements the Unmarshaler interface of the yaml pkg.
func (e *Encoding) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var encString string
	err := unmarshal(&encString)
	if err != nil {
		return err
	}

	*e, err = ParseEncoding(encString)
	if err != nil {
		return err
	}

	return nil
}

// MarshalYAML implements the Marshaler interface of the yaml pkg
func (e Encoding) MarshalYAML() (interface{}, error) {
	return e.String(), nil
}

// UnmarshalJSON implements the Unmarshaler interface of the json pkg.
func (e *Encoding) UnmarshalJSON(b []byte) error {
	var encString string
	err := json.Unmarshal(b, &encString)
	if err != nil {
		return err
	}

	*e, err = ParseEncoding(encString)
	if err != nil {
		return err
	}

	return nil
}

// MarshalJSON implements the marshaler interface of the json pkg.
func (e Encoding) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString("\"" + e.String() + "\"")
	return buffer.Bytes(), nil
}

// ParseEncoding parses a chunk encoding (compression algorithm) by its name.
func ParseEncoding(enc string) (Encoding, error) {
	for _, e := range SupportedEncoding {
		if strings.EqualFold(e.String(), enc) {
			return e, nil
		}
	}
	return 0, fmt.Errorf("invalid encoding: %s, supported: %s", enc, SupportedEncodingString())
}

// SupportedEncodingString returns the list of supported Encoding.
func SupportedEncodingString() string {
	var sb strings.Builder
	for i := range SupportedEncoding {
		sb.WriteString(SupportedEncoding[i].String())
		if i != len(SupportedEncoding)-1 {
			sb.WriteString(", ")
		}
	}
	return sb.String()
}
