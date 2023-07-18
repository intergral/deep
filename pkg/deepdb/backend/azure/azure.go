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

package azure

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	backend2 "github.com/intergral/deep/pkg/deepdb/backend"
	"io"
	"path"
	"strings"

	blob "github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

const (
	// dir represents the char separator used by the blob virtual directory structure
	dir = "/"
	// max parallelism on uploads
	maxParallelism = 3
)

type readerWriter struct {
	cfg                *Config
	containerURL       blob.ContainerURL
	hedgedContainerURL blob.ContainerURL
}

type appendTracker struct {
	Name string
}

// New gets the Azure blob container
func New(cfg *Config) (backend2.RawReader, backend2.RawWriter, backend2.Compactor, error) {
	return internalNew(cfg, true)
}

func internalNew(cfg *Config, confirm bool) (backend2.RawReader, backend2.RawWriter, backend2.Compactor, error) {
	ctx := context.Background()

	container, err := GetContainer(ctx, cfg, false)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "getting storage container")
	}

	hedgedContainer, err := GetContainer(ctx, cfg, true)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "getting hedged storage container")
	}

	if confirm {
		// Getting container properties to check if container exists
		_, err = container.GetProperties(ctx, blob.LeaseAccessConditions{})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to GetProperties: %w", err)
		}
	}

	rw := &readerWriter{
		cfg:                cfg,
		containerURL:       container,
		hedgedContainerURL: hedgedContainer,
	}

	return rw, rw, rw, nil
}

// Write implements backend.Writer
func (rw *readerWriter) Write(ctx context.Context, name string, keypath backend2.KeyPath, data io.Reader, _ int64, _ bool) error {
	span, derivedCtx := opentracing.StartSpanFromContext(ctx, "azure.Write")
	defer span.Finish()

	return rw.writer(derivedCtx, bufio.NewReader(data), backend2.ObjectFileName(keypath, name))
}

// Append implements backend.Writer
func (rw *readerWriter) Append(ctx context.Context, name string, keypath backend2.KeyPath, tracker backend2.AppendTracker, buffer []byte) (backend2.AppendTracker, error) {
	var a appendTracker
	if tracker == nil {
		a.Name = backend2.ObjectFileName(keypath, name)

		err := rw.writeAll(ctx, a.Name, buffer)
		if err != nil {
			return nil, err
		}
	} else {
		a = tracker.(appendTracker)

		err := rw.append(ctx, buffer, a.Name)
		if err != nil {
			return nil, err
		}
	}

	return a, nil
}

// CloseAppend implements backend.Writer
func (rw *readerWriter) CloseAppend(context.Context, backend2.AppendTracker) error {
	return nil
}

// List implements backend.Reader
func (rw *readerWriter) List(ctx context.Context, keypath backend2.KeyPath) ([]string, error) {
	marker := blob.Marker{}
	prefix := path.Join(keypath...)

	if len(prefix) > 0 {
		prefix = prefix + dir
	}

	objects := make([]string, 0)
	for {
		list, err := rw.containerURL.ListBlobsHierarchySegment(ctx, marker, dir, blob.ListBlobsSegmentOptions{
			Prefix:  prefix,
			Details: blob.BlobListingDetails{},
		})
		if err != nil {
			return objects, errors.Wrap(err, "iterating tenants")

		}
		marker = list.NextMarker

		for _, objBlob := range list.Segment.BlobPrefixes {
			objects = append(objects, strings.TrimPrefix(strings.TrimSuffix(objBlob.Name, dir), prefix))
		}

		// Continue iterating if we are not done.
		if !marker.NotDone() {
			break
		}
	}
	return objects, nil
}

// Read implements backend.Reader
func (rw *readerWriter) Read(ctx context.Context, name string, keypath backend2.KeyPath, _ bool) (io.ReadCloser, int64, error) {
	span, derivedCtx := opentracing.StartSpanFromContext(ctx, "azure.Read")
	defer span.Finish()

	object := backend2.ObjectFileName(keypath, name)
	b, err := rw.readAll(derivedCtx, object)
	if err != nil {
		return nil, 0, readError(err)
	}

	return io.NopCloser(bytes.NewReader(b)), int64(len(b)), nil
}

// ReadRange implements backend.Reader
func (rw *readerWriter) ReadRange(ctx context.Context, name string, keypath backend2.KeyPath, offset uint64, buffer []byte, _ bool) error {
	span, derivedCtx := opentracing.StartSpanFromContext(ctx, "azure.ReadRange", opentracing.Tags{
		"len":    len(buffer),
		"offset": offset,
	})
	defer span.Finish()

	object := backend2.ObjectFileName(keypath, name)
	err := rw.readRange(derivedCtx, object, int64(offset), buffer)
	if err != nil {
		return readError(err)
	}

	return nil
}

// Shutdown implements backend.Reader
func (rw *readerWriter) Shutdown() {
}

func (rw *readerWriter) writeAll(ctx context.Context, name string, b []byte) error {
	err := rw.writer(ctx, bytes.NewReader(b), name)
	if err != nil {
		return err
	}

	return nil
}

func (rw *readerWriter) append(ctx context.Context, src []byte, name string) error {
	appendBlobURL := rw.containerURL.NewBlockBlobURL(name)

	// These helper functions convert a binary block ID to a base-64 string and vice versa
	// NOTE: The blockID must be <= 64 bytes and ALL blockIDs for the block must be the same length
	blockIDBinaryToBase64 := func(blockID []byte) string { return base64.StdEncoding.EncodeToString(blockID) }

	blockIDIntToBase64 := func(blockID int) string {
		binaryBlockID := (&[64]byte{})[:]
		binary.LittleEndian.PutUint32(binaryBlockID, uint32(blockID))
		return blockIDBinaryToBase64(binaryBlockID)
	}

	l, err := appendBlobURL.GetBlockList(ctx, blob.BlockListAll, blob.LeaseAccessConditions{})
	if err != nil {
		return err
	}

	// generate the next block id
	id := blockIDIntToBase64(len(l.CommittedBlocks) + 1)

	_, err = appendBlobURL.StageBlock(ctx, id, bytes.NewReader(src), blob.LeaseAccessConditions{}, nil, blob.ClientProvidedKeyOptions{})
	if err != nil {
		return err
	}

	base64BlockIDs := make([]string, len(l.CommittedBlocks)+1)
	for i := 0; i < len(l.CommittedBlocks); i++ {
		base64BlockIDs[i] = l.CommittedBlocks[i].Name
	}

	base64BlockIDs[len(l.CommittedBlocks)] = id

	// After all the blocks are uploaded, atomically commit them to the blob.
	_, err = appendBlobURL.CommitBlockList(ctx, base64BlockIDs, blob.BlobHTTPHeaders{}, blob.Metadata{}, blob.BlobAccessConditions{}, blob.DefaultAccessTier, blob.BlobTagsMap{}, blob.ClientProvidedKeyOptions{}, blob.ImmutabilityPolicyOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (rw *readerWriter) writer(ctx context.Context, src io.Reader, name string) error {
	blobURL := rw.containerURL.NewBlockBlobURL(name)

	if _, err := blob.UploadStreamToBlockBlob(ctx, src, blobURL,
		blob.UploadStreamToBlockBlobOptions{
			BufferSize: rw.cfg.BufferSize,
			MaxBuffers: rw.cfg.MaxBuffers,
		},
	); err != nil {
		return errors.Wrapf(err, "cannot upload blob, name: %s", name)
	}
	return nil
}

func (rw *readerWriter) readRange(ctx context.Context, name string, offset int64, destBuffer []byte) error {
	blobURL := rw.hedgedContainerURL.NewBlockBlobURL(name)

	var props *blob.BlobGetPropertiesResponse
	props, err := blobURL.GetProperties(ctx, blob.BlobAccessConditions{}, blob.ClientProvidedKeyOptions{})
	if err != nil {
		return err
	}

	length := int64(len(destBuffer))
	var size int64

	if length > 0 && length <= props.ContentLength()-offset {
		size = length
	} else {
		size = props.ContentLength() - offset
	}

	if err := blob.DownloadBlobToBuffer(context.Background(), blobURL.BlobURL, offset, size,
		destBuffer, blob.DownloadFromBlobOptions{
			BlockSize:   blob.BlobDefaultDownloadBlockSize,
			Parallelism: maxParallelism,
			Progress:    nil,
			RetryReaderOptionsPerBlock: blob.RetryReaderOptions{
				MaxRetryRequests: maxRetries,
			},
		},
	); err != nil {
		return err
	}

	_, err = bytes.NewReader(destBuffer).Read(destBuffer)
	if err != nil {
		return err
	}

	return nil
}

func (rw *readerWriter) readAll(ctx context.Context, name string) ([]byte, error) {
	blobURL := rw.hedgedContainerURL.NewBlockBlobURL(name)

	var props *blob.BlobGetPropertiesResponse
	props, err := blobURL.GetProperties(ctx, blob.BlobAccessConditions{}, blob.ClientProvidedKeyOptions{})
	if err != nil {
		return nil, err
	}

	destBuffer := make([]byte, props.ContentLength())

	if err := blob.DownloadBlobToBuffer(context.Background(), blobURL.BlobURL, 0, props.ContentLength(),
		destBuffer, blob.DownloadFromBlobOptions{
			BlockSize:   blob.BlobDefaultDownloadBlockSize,
			Parallelism: uint16(maxParallelism),
			Progress:    nil,
			RetryReaderOptionsPerBlock: blob.RetryReaderOptions{
				MaxRetryRequests: maxRetries,
			},
		},
	); err != nil {
		return nil, err
	}

	return destBuffer, nil
}

func readError(err error) error {
	var storageError blob.StorageError
	errors.As(err, &storageError)

	if storageError == nil {
		return errors.Wrap(err, "reading storage container")
	}
	if storageError.ServiceCode() == blob.ServiceCodeBlobNotFound {
		return backend2.ErrDoesNotExist
	}
	return errors.Wrap(storageError, "reading Azure blob container")
}
