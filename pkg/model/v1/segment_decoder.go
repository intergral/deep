package v1

import (
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	deeptp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
)

type SegmentDecoder struct {
}

func (s *SegmentDecoder) PrepareForWrite(trace *deeptp.Snapshot, start uint32) ([]byte, error) {
	return marshalWithStart(trace, start)
}

func (s *SegmentDecoder) PrepareForRead(segment []byte) (*deeptp.Snapshot, error) {
	obj, _, err := stripStart(segment)
	if err != nil {
		return nil, fmt.Errorf("error stripping start: %w", err)
	}

	snapshot := &deeptp.Snapshot{}
	err = proto.Unmarshal(obj, snapshot)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling snapshot: %w", err)
	}

	return snapshot, nil
}

func (s *SegmentDecoder) ToObject(segment []byte) ([]byte, error) {
	return segment, nil
}

func (s *SegmentDecoder) FastRange(segment []byte) (uint32, error) {
	_, start, err := stripStart(segment)

	return start, err
}

var segmentDecoder = &SegmentDecoder{}

// NewSegmentDecoder() returns a v1 segment decoder.
func NewSegmentDecoder() *SegmentDecoder {
	return segmentDecoder
}

func marshalWithStart(pb proto.Message, start uint32) ([]byte, error) {
	const uint32Size = 4

	sz := proto.Size(pb)
	buff := make([]byte, 0, sz+uint32Size) // proto buff size + start/end uint32s

	buffer := proto.NewBuffer(buff)

	_ = buffer.EncodeFixed32(uint64(start)) // EncodeFixed32 can't return an error
	err := buffer.Marshal(pb)
	if err != nil {
		return nil, err
	}

	buff = buffer.Bytes()

	return buff, nil
}

func stripStart(buff []byte) ([]byte, uint32, error) {
	if len(buff) < 4 {
		return nil, 0, errors.New("buffer too short to have start/end")
	}

	buffer := proto.NewBuffer(buff)
	start, err := buffer.DecodeFixed32()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read start from buffer %w", err)
	}

	return buff[4:], uint32(start), nil
}
