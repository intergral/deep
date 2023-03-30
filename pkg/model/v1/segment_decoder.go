package v1

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/intergral/deep/pkg/deeppb"
	"github.com/intergral/deep/pkg/model/decoder"
	"github.com/intergral/deep/pkg/model/trace"
)

type SegmentDecoder struct {
}

var segmentDecoder = &SegmentDecoder{}

// NewSegmentDecoder() returns a v1 segment decoder.
func NewSegmentDecoder() *SegmentDecoder {
	return segmentDecoder
}

func (d *SegmentDecoder) PrepareForWrite(trace *deeppb.Trace, start uint32, end uint32) ([]byte, error) {
	// v1 encoding doesn't support start/end
	return proto.Marshal(trace)
}

func (d *SegmentDecoder) PrepareForRead(segments [][]byte) (*deeppb.Trace, error) {
	// each slice is a marshalled deeppb.Trace, unmarshal and combine
	combiner := trace.NewCombiner()
	for i, s := range segments {
		t := &deeppb.Trace{}
		err := proto.Unmarshal(s, t)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling trace: %w", err)
		}

		combiner.ConsumeWithFinal(t, i == len(segments)-1)
	}

	combinedTrace, _ := combiner.Result()

	return combinedTrace, nil
}

func (d *SegmentDecoder) ToObject(segments [][]byte) ([]byte, error) {
	// wrap byte slices in a deeppb.TraceBytes and marshal
	wrapper := &deeppb.TraceBytes{
		Traces: append([][]byte(nil), segments...),
	}
	return proto.Marshal(wrapper)
}

func (d *SegmentDecoder) FastRange([]byte) (uint32, uint32, error) {
	return 0, 0, decoder.ErrUnsupported
}
