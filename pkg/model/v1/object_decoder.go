package v1

import (
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
)

const Encoding = "v1"

type ObjectDecoder struct {
	SegmentDecoder
}

var staticDecoder = &ObjectDecoder{}

func NewObjectDecoder() *ObjectDecoder {
	return staticDecoder
}

func (o *ObjectDecoder) PrepareForRead(obj []byte) (*deep_tp.Snapshot, error) {
	return o.SegmentDecoder.PrepareForRead(obj)
}

func (o *ObjectDecoder) Combine(objs ...[]byte) ([]byte, error) {
	if len(objs) > 0 {
		return objs[0], nil
	}
	return nil, nil
}

func (o *ObjectDecoder) FastRange(obj []byte) (uint32, error) {
	return o.SegmentDecoder.FastRange(obj)
}
