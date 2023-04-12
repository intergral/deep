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

package snapshot

import (
	"errors"
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
)

type ResultHandler struct {
	result *deep_tp.Snapshot
}

func NewResultHandler() *ResultHandler {
	return &ResultHandler{}
}

func (rh *ResultHandler) Complete(result *deep_tp.Snapshot) error {
	if rh.result == nil {
		rh.result = result
		return nil
	}
	return errors.New("result already complete")
}

func (rh *ResultHandler) Result() *deep_tp.Snapshot {
	return rh.result
}

func (rh *ResultHandler) IsComplete() bool {
	if rh.result != nil {
		return true
	}
	return false
}
