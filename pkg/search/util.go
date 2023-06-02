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

package search

import (
	"github.com/intergral/deep/pkg/deeppb"
)

const (
	ErrorTag        = "error"
	StatusCodeTag   = "status.code"
	StatusCodeUnset = "unset"
	StatusCodeOK    = "ok"
	StatusCodeError = "error"

	RootSpanNotYetReceivedText = "<root span not yet received>"
)

func GetVirtualTagValues(tagName string) []string {
	switch tagName {

	case StatusCodeTag:
		return []string{StatusCodeUnset, StatusCodeOK, StatusCodeError}

	case ErrorTag:
		return []string{"true"}
	}

	return nil
}

func GetVirtualTagValuesV2(tagName string) []*deeppb.TagValue {
	return []*deeppb.TagValue{}
}

// CombineSearchResults overlays the incoming search result with the existing result. This is required
// for the following reason:  a trace may be present in multiple blocks, or in partial segments
// in live traces.  The results should reflect elements of all segments.
func CombineSearchResults(existing *deeppb.SnapshotSearchMetadata, incoming *deeppb.SnapshotSearchMetadata) {
	if existing.SnapshotID == "" {
		existing.SnapshotID = incoming.SnapshotID
	}

	if existing.ServiceName == "" {
		existing.ServiceName = incoming.ServiceName
	}

	if existing.FilePath == "" {
		existing.FilePath = incoming.FilePath
	}

	if existing.LineNo == 0 {
		existing.LineNo = incoming.LineNo
	}

	// Earliest start time.
	if existing.StartTimeUnixNano > incoming.StartTimeUnixNano {
		existing.StartTimeUnixNano = incoming.StartTimeUnixNano
	}

	// Longest duration
	if existing.DurationNano < incoming.DurationNano {
		existing.DurationNano = incoming.DurationNano
	}
}
