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
