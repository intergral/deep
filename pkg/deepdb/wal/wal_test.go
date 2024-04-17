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

package wal

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/intergral/deep/pkg/deeppb"
	deeptp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/deepql"

	"github.com/go-kit/log" //nolint:all
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/encoding"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/encoding/vparquet"
	"github.com/intergral/deep/pkg/model"
	"github.com/intergral/deep/pkg/util"
	"github.com/intergral/deep/pkg/util/test"
)

const (
	testTenantID = "fake"
)

func TestCompletedDirIsRemoved(t *testing.T) {
	// Create /completed/testfile and verify it is removed.
	tempDir := t.TempDir()

	err := os.MkdirAll(path.Join(tempDir, completedDir), os.ModePerm)
	require.NoError(t, err, "unexpected error creating completedDir")

	_, err = os.Create(path.Join(tempDir, completedDir, "testfile"))
	require.NoError(t, err, "unexpected error creating testfile")

	_, err = New(&Config{
		Filepath: tempDir,
		Version:  encoding.DefaultEncoding().Version(),
	})
	require.NoError(t, err, "unexpected error creating temp wal")

	_, err = os.Stat(path.Join(tempDir, completedDir))
	require.Error(t, err, "completedDir should not exist")
}

func TestAppendBlockStartEnd(t *testing.T) {
	encodings := []encoding.VersionedEncoding{
		vparquet.Encoding{},
	}
	for _, e := range encodings {
		t.Run(e.Version(), func(t *testing.T) {
			testAppendBlockStartEnd(t, e)
		})
	}
}

func testAppendBlockStartEnd(t *testing.T, e encoding.VersionedEncoding) {
	wal, err := New(&Config{
		Filepath:       t.TempDir(),
		Encoding:       backend.EncNone,
		IngestionSlack: 3 * time.Minute,
		Version:        encoding.DefaultEncoding().Version(),
	})
	require.NoError(t, err, "unexpected error creating temp wal")

	blockID := uuid.New()
	block, err := wal.newBlock(blockID, testTenantID, model.CurrentEncoding, e.Version())
	require.NoError(t, err, "unexpected error creating block")

	enc := model.MustNewSegmentDecoder(model.CurrentEncoding)

	// create a new block and confirm start/end times are correct
	blockStart := uint32(time.Now().Add(-time.Minute).Unix())

	for i := 0; i < 10; i++ {
		id := make([]byte, 16)
		_, err := crand.Read(id)
		require.NoError(t, err)
		obj := test.GenerateSnapshot(i, &test.GenerateOptions{Id: id})

		b1, err := enc.PrepareForWrite(obj, blockStart)
		require.NoError(t, err)

		b2, err := enc.ToObject(b1)
		require.NoError(t, err)

		err = block.Append(id, b2, blockStart)
		require.NoError(t, err, "unexpected error writing req")
	}

	require.NoError(t, block.Flush())

	require.Equal(t, blockStart, uint32(block.BlockMeta().StartTime.Unix()))
	require.Equal(t, blockStart, uint32(block.BlockMeta().EndTime.Unix()))

	// rescan the block and make sure the start/end times are the same
	blocks, err := wal.RescanBlocks(time.Hour, log.NewNopLogger())
	require.NoError(t, err, "unexpected error getting blocks")
	require.Len(t, blocks, 1)

	require.Equal(t, blockStart, uint32(blocks[0].BlockMeta().StartTime.Unix()))
	require.Equal(t, blockStart, uint32(blocks[0].BlockMeta().EndTime.Unix()))
}

func TestIngestionSlack(t *testing.T) {
	encodings := []encoding.VersionedEncoding{
		vparquet.Encoding{},
	}
	for _, e := range encodings {
		t.Run(e.Version(), func(t *testing.T) {
			testIngestionSlack(t, e)
		})
	}
}

func testIngestionSlack(t *testing.T, e encoding.VersionedEncoding) {
	wal, err := New(&Config{
		Filepath:       t.TempDir(),
		Encoding:       backend.EncNone,
		IngestionSlack: time.Minute,
		Version:        encoding.DefaultEncoding().Version(),
	})
	require.NoError(t, err, "unexpected error creating temp wal")

	blockID := uuid.New()
	block, err := wal.newBlock(blockID, testTenantID, model.CurrentEncoding, e.Version())
	require.NoError(t, err, "unexpected error creating block")

	enc := model.MustNewSegmentDecoder(model.CurrentEncoding)

	startTs := uint32(time.Now().Add(-2 * time.Minute).Unix()) // Outside of range

	// Append a snapshot
	id := make([]byte, 16)
	_, err = crand.Read(id)
	require.NoError(t, err)
	obj := test.GenerateSnapshot(rand.Int()%10+1, &test.GenerateOptions{Id: id})

	b1, err := enc.PrepareForWrite(obj, startTs)
	require.NoError(t, err)

	b2, err := enc.ToObject(b1)
	require.NoError(t, err)

	err = block.Append(id, b2, startTs)
	require.NoError(t, err, "unexpected error writing req")

	blockStart := uint32(block.BlockMeta().StartTime.Unix())
	blockEnd := uint32(block.BlockMeta().EndTime.Unix())

	require.Equal(t, startTs, blockStart)
	require.Equal(t, startTs, blockEnd)
}

func TestFindBySnapshotID(t *testing.T) {
	for _, e := range encoding.AllEncodings() {
		t.Run(e.Version(), func(t *testing.T) {
			testFindBySnapshotID(t, e)
		})
	}
}

func testFindBySnapshotID(t *testing.T, e encoding.VersionedEncoding) {
	runWALTest(t, e.Version(), func(ids [][]byte, objs []*deeptp.Snapshot, block common.WALBlock) {
		// find all snapshots pushed
		ctx := context.Background()
		for i, id := range ids {
			obj, err := block.FindSnapshotByID(ctx, id, common.DefaultSearchOptions())
			require.NoError(t, err)
			require.Equal(t, objs[i].ID, obj.ID)
		}
	})
}

func TestIterator(t *testing.T) {
	for _, e := range encoding.AllEncodings() {
		t.Run(e.Version(), func(t *testing.T) {
			testIterator(t, e)
		})
	}
}

func testIterator(t *testing.T, e encoding.VersionedEncoding) {
	runWALTest(t, e.Version(), func(ids [][]byte, objs []*deeptp.Snapshot, block common.WALBlock) {
		ctx := context.Background()

		iterator, err := block.Iterator()
		require.NoError(t, err)
		defer iterator.Close()

		i := 0
		for {
			id, obj, err := iterator.Next(ctx)
			if err == io.EOF || id == nil {
				break
			} else {
				require.NoError(t, err)
			}

			found := false
			j := 0
			for ; j < len(ids); j++ {
				if bytes.Equal(ids[j], id) {
					found = true
					break
				}
			}

			require.True(t, found)
			require.Equal(t, objs[j].ID, obj.ID)
			require.Equal(t, ids[j], []byte(id))
			i++
		}

		require.Equal(t, len(objs), i)
	})
}

func TestSearch(t *testing.T) {
	for _, e := range encoding.AllEncodings() {
		t.Run(e.Version(), func(t *testing.T) {
			testSearch(t, e)
		})
	}
}

func testSearch(t *testing.T, e encoding.VersionedEncoding) {
	runWALTest(t, e.Version(), func(ids [][]byte, objs []*deeptp.Snapshot, block common.WALBlock) {
		ctx := context.Background()

		for i, o := range objs {
			k, v := findFirstAttribute(o)
			require.NotEmpty(t, k)
			require.NotEmpty(t, v)

			resp, err := block.Search(ctx, &deeppb.SearchRequest{
				Tags: map[string]string{
					k: v,
				},
				Limit: 1,
			}, common.DefaultSearchOptions())
			if err == common.ErrUnsupported {
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, len(resp.Snapshots))
			require.Equal(t, util.SnapshotIDToHexString(ids[i]), resp.Snapshots[0].SnapshotID)
		}
	})
}

func TestFetch(t *testing.T) {
	for _, e := range encoding.AllEncodings() {
		t.Run(e.Version(), func(t *testing.T) {
			testFetch(t, e)
		})
	}
}

func testFetch(t *testing.T, e encoding.VersionedEncoding) {
	runWALTest(t, e.Version(), func(ids [][]byte, objs []*deeptp.Snapshot, block common.WALBlock) {
		ctx := context.Background()

		for i, o := range objs {
			k, v := findFirstAttribute(o)
			require.NotEmpty(t, k)
			require.NotEmpty(t, v)

			query := fmt.Sprintf("{ id = \"%s\" }", util.SnapshotIDToHexString(o.ID))
			engine := deepql.NewEngine()
			resp, err := engine.ExecuteSearch(ctx, &deeppb.SearchRequest{Query: query}, func(ctx context.Context, request deepql.FetchSnapshotRequest) (deepql.FetchSnapshotResponse, error) {
				return block.Fetch(ctx, request, common.DefaultSearchOptions())
			})

			// not all blocks support fetch
			if errors.Is(err, common.ErrUnsupported) {
				return
			}
			require.NoError(t, err)

			// grab the first result
			ss := resp.Snapshots[0]
			require.NoError(t, err)
			require.NotNil(t, ss)

			// confirm snapshot id matches
			expectedID := ids[i]
			require.NotNil(t, ss)
			require.Equal(t, ss.SnapshotID, util.SnapshotIDToHexString(expectedID))

			// confirm no more matches
			if len(resp.Snapshots) != 1 {
				t.Error("should only get single result")
			}
		}
	})
}

func findFirstAttribute(obj *deeptp.Snapshot) (string, string) {
	for _, a := range obj.Attributes {
		if a.Key == "number" {
			return a.Key, a.Value.GetStringValue()
		}
	}

	return "", ""
}

func TestInvalidFilesAndFoldersAreHandled(t *testing.T) {
	tempDir := t.TempDir()
	wal, err := New(&Config{
		Filepath: tempDir,
		Encoding: backend.EncGZIP,
		Version:  encoding.DefaultEncoding().Version(),
	})
	require.NoError(t, err, "unexpected error creating temp wal")

	// create all valid blocks
	for _, e := range encoding.AllEncodings() {
		block, err := wal.newBlock(uuid.New(), testTenantID, model.CurrentEncoding, e.Version())
		require.NoError(t, err)

		id := make([]byte, 16)
		_, err = crand.Read(id)
		require.NoError(t, err)
		tr := test.GenerateSnapshot(10, &test.GenerateOptions{Id: id})
		b1, err := model.MustNewSegmentDecoder(model.CurrentEncoding).PrepareForWrite(tr, 0)
		require.NoError(t, err)
		b2, err := model.MustNewSegmentDecoder(model.CurrentEncoding).ToObject(b1)
		require.NoError(t, err)
		err = block.Append(id, b2, 0)
		require.NoError(t, err)
		err = block.Flush()
		require.NoError(t, err)
	}

	// create unparseable filename
	err = os.WriteFile(filepath.Join(tempDir, "fe0b83eb-a86b-4b6c-9a74-dc272cd5700e:tenant:vParquet:notanencoding"), []byte{}, 0o644)
	require.NoError(t, err)

	// create empty block
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "fe0b83eb-a86b-4b6c-9a74-dc272cd5700e:blerg:vParquet:gzip"), os.ModePerm))
	err = os.WriteFile(filepath.Join(tempDir, "fe0b83eb-a86b-4b6c-9a74-dc272cd5700e:blerg:vParquet:gzip", "meta.json"), []byte{}, 0o644)
	require.NoError(t, err)

	// create unparseable block
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "fe0b83eb-a86b-4b6c-9a74-dc272cd5700e+tenant+vOther"), os.ModePerm))

	blocks, err := wal.RescanBlocks(0, log.NewNopLogger())
	require.NoError(t, err, "unexpected error getting blocks")
	require.Len(t, blocks, len(encoding.AllEncodings())) // valid blocks created above

	// empty file should have been removed
	require.NoFileExists(t, filepath.Join(tempDir, "fe0b83eb-a86b-4b6c-9a74-dc272cd5700e:blerg:vParquet:gzip"))

	// unparseable files/folder should have been ignored
	require.FileExists(t, filepath.Join(tempDir, "fe0b83eb-a86b-4b6c-9a74-dc272cd5700e:tenant:vParquet:notanencoding"))
	require.DirExists(t, filepath.Join(tempDir, "fe0b83eb-a86b-4b6c-9a74-dc272cd5700e+tenant+vOther"))
}

func runWALTest(t testing.TB, encoding string, runner func([][]byte, []*deeptp.Snapshot, common.WALBlock)) {
	wal, err := New(&Config{
		Filepath: t.TempDir(),
		Encoding: backend.EncNone,
		Version:  encoding,
	})
	require.NoError(t, err, "unexpected error creating temp wal")

	blockID := uuid.New()

	block, err := wal.newBlock(blockID, testTenantID, model.CurrentEncoding, encoding)
	require.NoError(t, err, "unexpected error creating block")

	enc := model.MustNewSegmentDecoder(model.CurrentEncoding)

	objects := 250
	objs := make([]*deeptp.Snapshot, 0, objects)
	ids := make([][]byte, 0, objects)
	for i := 0; i < objects; i++ {
		id := make([]byte, 16)
		_, err = crand.Read(id)
		require.NoError(t, err)
		obj := test.GenerateSnapshot(rand.Int()%10+1, &test.GenerateOptions{Id: id, Attrs: map[string]interface{}{"number": strconv.Itoa(i)}})

		ids = append(ids, id)

		b1, err := enc.PrepareForWrite(obj, 0)
		require.NoError(t, err)

		b2, err := enc.ToObject(b1)
		require.NoError(t, err)

		objs = append(objs, obj)

		err = block.Append(id, b2, 0)
		require.NoError(t, err)

		if i%100 == 0 {
			err = block.Flush()
			require.NoError(t, err)
		}
	}
	err = block.Flush()
	require.NoError(t, err)

	runner(ids, objs, block)

	// rescan blocks
	blocks, err := wal.RescanBlocks(0, log.NewNopLogger())
	require.NoError(t, err, "unexpected error getting blocks")
	require.Len(t, blocks, 1)

	runner(ids, objs, blocks[0])

	err = block.Clear()
	require.NoError(t, err)
}

func BenchmarkAppendFlush(b *testing.B) {
	encodings := []string{
		vparquet.VersionString,
	}
	for _, enc := range encodings {
		b.Run(enc, func(b *testing.B) {
			runWALBenchmark(b, enc, b.N, nil)
		})
	}
}

func BenchmarkFindSnapshotByID(b *testing.B) {
	encodings := []string{
		vparquet.VersionString,
	}
	for _, enc := range encodings {
		b.Run(enc, func(b *testing.B) {
			runWALBenchmark(b, enc, 1, func(ids [][]byte, objs []*deeptp.Snapshot, block common.WALBlock) {
				ctx := context.Background()
				for i := 0; i < b.N; i++ {
					j := i % len(ids)

					obj, err := block.FindSnapshotByID(ctx, ids[j], common.DefaultSearchOptions())
					require.NoError(b, err)
					require.Equal(b, objs[j], obj)
				}
			})
		})
	}
}

func BenchmarkFindUnknownSnapshotID(b *testing.B) {
	encodings := []string{
		vparquet.VersionString,
	}
	for _, enc := range encodings {
		b.Run(enc, func(b *testing.B) {
			runWALBenchmark(b, enc, 1, func(ids [][]byte, objs []*deeptp.Snapshot, block common.WALBlock) {
				for i := 0; i < b.N; i++ {
					_, err := block.FindSnapshotByID(context.Background(), common.ID{}, common.DefaultSearchOptions())
					require.NoError(b, err)
				}
			})
		})
	}
}

func BenchmarkSearch(b *testing.B) {
	encodings := []string{
		vparquet.VersionString,
	}
	for _, enc := range encodings {
		b.Run(enc, func(b *testing.B) {
			runWALBenchmark(b, enc, 1, func(ids [][]byte, objs []*deeptp.Snapshot, block common.WALBlock) {
				ctx := context.Background()

				for i := 0; i < b.N; i++ {
					j := i % len(ids)
					id, o := ids[j], objs[j]

					k, v := findFirstAttribute(o)
					require.NotEmpty(b, k)
					require.NotEmpty(b, v)

					resp, err := block.Search(ctx, &deeppb.SearchRequest{
						Tags: map[string]string{
							k: v,
						},
						Limit: 10,
					}, common.DefaultSearchOptions())
					if err == common.ErrUnsupported {
						return
					}
					require.NoError(b, err)
					require.Equal(b, 1, len(resp.Snapshots))
					require.Equal(b, util.SnapshotIDToHexString(id), resp.Snapshots[0].SnapshotID)
				}
			})
		})
	}
}

func runWALBenchmark(b *testing.B, encoding string, flushCount int, runner func([][]byte, []*deeptp.Snapshot, common.WALBlock)) {
	wal, err := New(&Config{
		Filepath: b.TempDir(),
		Encoding: backend.EncNone,
		Version:  encoding,
	})
	require.NoError(b, err, "unexpected error creating temp wal")

	blockID := uuid.New()

	block, err := wal.newBlock(blockID, testTenantID, model.CurrentEncoding, encoding)
	require.NoError(b, err, "unexpected error creating block")

	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	objects := 250
	snapshots := make([]*deeptp.Snapshot, 0, objects)
	objs := make([][]byte, 0, objects)
	ids := make([][]byte, 0, objects)
	for i := 0; i < objects; i++ {
		id := make([]byte, 16)
		_, err = crand.Read(id)
		require.NoError(b, err)
		obj := test.GenerateSnapshot(rand.Int()%10+1, &test.GenerateOptions{Id: id})

		ids = append(ids, id)
		snapshots = append(snapshots, obj)

		b1, err := dec.PrepareForWrite(obj, 0)
		require.NoError(b, err)

		b2, err := dec.ToObject(b1)
		require.NoError(b, err)

		objs = append(objs, b2)
	}

	b.ResetTimer()

	for flush := 0; flush < flushCount; flush++ {

		for i := range objs {
			require.NoError(b, block.Append(ids[i], objs[i], 0))
		}

		err = block.Flush()
		require.NoError(b, err)
	}

	if runner != nil {
		runner(ids, snapshots, block)
	}
}
