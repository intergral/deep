package ingester

import (
	"context"
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"testing"

	"github.com/intergral/deep/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTraceStartEndTime(t *testing.T) {
	s := model.MustNewSegmentDecoder(model.CurrentEncoding)

	tr := newLiveSnapshot(nil, 0)

	// initial push
	buff, err := s.PrepareForWrite(&deep_tp.Snapshot{}, 10)
	require.NoError(t, err)
	err = tr.Push(context.Background(), "test", buff)
	require.NoError(t, err)

	assert.Equal(t, uint32(10), tr.start)

	// overwrite start
	buff, err = s.PrepareForWrite(&deep_tp.Snapshot{}, 5)
	require.NoError(t, err)
	err = tr.Push(context.Background(), "test", buff)
	require.NoError(t, err)

	assert.Equal(t, uint32(5), tr.start)

	// overwrite end
	buff, err = s.PrepareForWrite(&deep_tp.Snapshot{}, 15)
	require.NoError(t, err)
	err = tr.Push(context.Background(), "test", buff)
	require.NoError(t, err)

	assert.Equal(t, uint32(5), tr.start)
}
