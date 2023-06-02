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

package forwarder

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type mockCountingForwarder struct {
	next               Forwarder
	forwardTracesCount int
}

func (m *mockCountingForwarder) ForwardTraces(ctx context.Context, traces ptrace.Traces) error {
	m.forwardTracesCount++
	return m.next.ForwardTraces(ctx, traces)
}

func (m *mockCountingForwarder) Shutdown(ctx context.Context) error {
	return m.next.Shutdown(ctx)
}

func TestList_ForwardTraces_ReturnsNoErrorAndCallsForwardTracesOnAllUnderlyingWorkingForwarders(t *testing.T) {
	// Given
	forwarder1 := &mockCountingForwarder{next: &mockWorkingForwarder{}, forwardTracesCount: 0}
	forwarder2 := &mockCountingForwarder{next: &mockWorkingForwarder{}, forwardTracesCount: 0}
	forwarder3 := &mockCountingForwarder{next: &mockWorkingForwarder{}, forwardTracesCount: 0}
	list := List([]Forwarder{forwarder1, forwarder2, forwarder3})

	// When
	err := list.ForwardTraces(context.Background(), ptrace.Traces{})

	// Then
	require.NoError(t, err)
	require.Equal(t, 1, forwarder1.forwardTracesCount)
	require.Equal(t, 1, forwarder2.forwardTracesCount)
	require.Equal(t, 1, forwarder3.forwardTracesCount)
}

func TestList_ForwardTraces_ReturnsErrorAndCallsForwardTracesOnAllUnderlyingForwardersWithSingleFailingForwarder(t *testing.T) {
	// Given
	forwarder1 := &mockCountingForwarder{next: &mockWorkingForwarder{}, forwardTracesCount: 0}
	forwarder2 := &mockCountingForwarder{next: &mockWorkingForwarder{}, forwardTracesCount: 0}
	forwarder3 := &mockCountingForwarder{next: &mockFailingForwarder{forwardTracesErr: errors.New("forward batches error")}, forwardTracesCount: 0}
	list := List([]Forwarder{forwarder1, forwarder2, forwarder3})

	// When
	err := list.ForwardTraces(context.Background(), ptrace.Traces{})

	// Then
	require.Error(t, err)
	require.Equal(t, 1, forwarder1.forwardTracesCount)
	require.Equal(t, 1, forwarder2.forwardTracesCount)
	require.Equal(t, 1, forwarder3.forwardTracesCount)
}

func TestList_ForwardTraces_ReturnsErrorAndCallsForwardTracesOnAllUnderlyingForwardersWithAllFailingForwarder(t *testing.T) {
	// Given
	forwarder1 := &mockCountingForwarder{next: &mockFailingForwarder{forwardTracesErr: errors.New("1 forward batches error")}, forwardTracesCount: 0}
	forwarder2 := &mockCountingForwarder{next: &mockFailingForwarder{forwardTracesErr: errors.New("2 forward batches error")}, forwardTracesCount: 0}
	forwarder3 := &mockCountingForwarder{next: &mockFailingForwarder{forwardTracesErr: errors.New("3 forward batches error")}, forwardTracesCount: 0}
	list := List([]Forwarder{forwarder1, forwarder2, forwarder3})

	// When
	err := list.ForwardTraces(context.Background(), ptrace.Traces{})

	// Then
	require.Error(t, err)
	require.ErrorContains(t, err, "1")
	require.ErrorContains(t, err, "2")
	require.ErrorContains(t, err, "3")
	require.Equal(t, 1, forwarder1.forwardTracesCount)
	require.Equal(t, 1, forwarder2.forwardTracesCount)
	require.Equal(t, 1, forwarder3.forwardTracesCount)
}

func TestList_ForwardTraces_DoesNotPanicWhenNil(t *testing.T) {
	// Given
	list := List(nil)

	// When
	panicFunc := func() {
		err := list.ForwardTraces(context.Background(), ptrace.Traces{})
		require.NoError(t, err)
	}

	// Then
	require.NotPanics(t, panicFunc)
}
