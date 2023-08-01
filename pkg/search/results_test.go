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
	"context"
	"errors"
	"github.com/intergral/deep/pkg/deeppb"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResultsDoesNotRace(t *testing.T) {

	testCases := []struct {
		name           string
		consumeResults bool
		error          bool
	}{
		{"default", true, false},
		{"exit early", false, false},
		{"exit early due to error", true, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sr := NewResults()
			defer sr.Close()

			for i := 0; i < 100; i++ {
				sr.StartWorker()
				go func() {
					defer sr.FinishWorker()
					for j := 0; j < 10_000; j++ {
						if sr.AddResult(ctx, &deeppb.SnapshotSearchMetadata{}) {
							break
						}
					}

					if tc.error {
						sr.SetError(errors.New("test error"))
					}
				}()
			}

			sr.AllWorkersStarted()

			var resultsCount int
			var err error
			for range sr.Results() {
				if sr.Error() != nil {
					err = sr.Error() // capture err to assert below
					break            // exit early
				}
				resultsCount++
			}

			if tc.error {
				require.Error(t, err)
				if tc.consumeResults {
					// in case of error, we will bail out early
					require.NotEqual(t, 10_000_00, resultsCount)
					// will read at-least something by the time we have first error
					require.NotEqual(t, 0, resultsCount)
				}
			} else {
				require.NoError(t, err)
				if tc.consumeResults {
					require.Equal(t, 10_000_00, resultsCount)
				}
			}
		})
	}
}
