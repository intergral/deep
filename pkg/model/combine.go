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

package model

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
)

type objectCombiner struct{}

type ObjectCombiner interface {
	Combine(dataEncoding string, objs ...[]byte) ([]byte, bool, error)
}

var StaticCombiner = objectCombiner{}

// Combine implements deepdb/encoding/common.ObjectCombiner
func (o objectCombiner) Combine(dataEncoding string, objs ...[]byte) ([]byte, bool, error) {
	if len(objs) <= 0 {
		return nil, false, errors.New("no objects provided")
	}

	// check to see if we need to combine
	needCombine := false
	for i := 1; i < len(objs); i++ {
		if !bytes.Equal(objs[0], objs[i]) {
			needCombine = true
			break
		}
	}

	if !needCombine {
		return objs[0], false, nil
	}

	encoding, err := NewObjectDecoder(dataEncoding)
	if err != nil {
		return nil, false, fmt.Errorf("error getting decoder: %w", err)
	}

	combinedBytes, err := encoding.Combine(objs...)
	if err != nil {
		return nil, false, fmt.Errorf("error combining: %w", err)
	}

	return combinedBytes, true, nil
}
