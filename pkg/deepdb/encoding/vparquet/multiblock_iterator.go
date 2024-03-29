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

package vparquet

import (
	"bytes"
	"context"
	"io"

	"github.com/segmentio/parquet-go"

	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/pkg/errors"
)

type iterable interface {
	parquet.Row | *Snapshot | *uint8
}

type combineFn[T iterable] func([]T) (T, error)

type MultiBlockIterator[T iterable] struct {
	bookmarks []*bookmark[T]
	combine   combineFn[T]
}

func newMultiblockIterator[T iterable](bookmarks []*bookmark[T]) *MultiBlockIterator[T] {
	return &MultiBlockIterator[T]{
		bookmarks: bookmarks,
	}
}

func (m *MultiBlockIterator[T]) Next(ctx context.Context) (common.ID, T, error) {
	if m.done(ctx) {
		return nil, nil, io.EOF
	}

	var (
		lowestID       common.ID
		lowestBookmark *bookmark[T]
	)

	// find the lowest ID of the new object
	for _, b := range m.bookmarks {
		id, err := b.peekID(ctx)
		if err != nil && err != io.EOF {
			return nil, nil, err
		}
		if id == nil {
			continue
		}

		comparison := bytes.Compare(id, lowestID)

		if len(lowestID) == 0 || comparison == -1 {
			lowestID = id
			lowestBookmark = b
		}
	}

	_, obj, err := lowestBookmark.current(ctx)
	if err != nil && err != io.EOF {
		return nil, nil, err
	}
	if obj == nil {
		// this should never happen. id was non-nil above
		return nil, nil, errors.New("unexpected nil object from lowest bookmark")
	}

	lowestBookmark.clear()

	return lowestID, obj, nil
}

func (m *MultiBlockIterator[T]) Close() {
	for _, b := range m.bookmarks {
		b.close()
	}
}

func (m *MultiBlockIterator[T]) done(ctx context.Context) bool {
	for _, b := range m.bookmarks {
		if !b.done(ctx) {
			return false
		}
	}
	return true
}

type bookmark[T iterable] struct {
	iter bookmarkIterator[T]

	currentID     common.ID
	currentObject T
	currentErr    error
}

type bookmarkIterator[T iterable] interface {
	Next(ctx context.Context) (common.ID, T, error)
	Close()
	peekNextID(ctx context.Context) (common.ID, error)
}

func newBookmark[T iterable](iter bookmarkIterator[T]) *bookmark[T] {
	return &bookmark[T]{
		iter: iter,
	}
}

func (b *bookmark[T]) peekID(ctx context.Context) (common.ID, error) {
	nextID, err := b.iter.peekNextID(ctx)
	if err != common.ErrUnsupported {
		return nextID, err
	}

	id, _, err := b.current(ctx)
	return id, err
}

func (b *bookmark[T]) current(ctx context.Context) (common.ID, T, error) {
	if b.currentErr != nil {
		return nil, nil, b.currentErr
	}

	if b.currentObject != nil {
		return b.currentID, b.currentObject, nil
	}

	b.currentID, b.currentObject, b.currentErr = b.iter.Next(ctx)
	return b.currentID, b.currentObject, b.currentErr
}

func (b *bookmark[T]) done(ctx context.Context) bool {
	nextID, err := b.iter.peekNextID(ctx)
	if err != common.ErrUnsupported {
		return nextID == nil || err != nil
	}

	_, obj, err := b.current(ctx)

	return obj == nil || err != nil
}

func (b *bookmark[T]) clear() {
	b.currentID = nil
	b.currentObject = nil
}

func (b *bookmark[T]) close() {
	b.iter.Close()
}
