/*
 * Copyright (C) 2024  Intergral GmbH
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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemove(t *testing.T) {
	type args struct {
		array []*struct {
			name string
		}
		index int
	}
	type testCase struct {
		name string
		args args
		want []*struct {
			name string
		}
	}
	tests := []testCase{
		{
			name: "Remove",
			args: args{
				array: []*struct {
					name string
				}{
					{
						name: "1",
					},
					{
						name: "2",
					},
					{
						name: "3",
					},
				},
				index: 0,
			},
			want: []*struct{ name string }{
				{
					name: "2",
				},
				{
					name: "3",
				},
			},
		},
		{
			name: "Remove",
			args: args{
				array: []*struct {
					name string
				}{
					{
						name: "1",
					},
					{
						name: "2",
					},
					{
						name: "3",
					},
				},
				index: 1,
			},
			want: []*struct{ name string }{
				{
					name: "1",
				},
				{
					name: "3",
				},
			},
		},
		{
			name: "Remove",
			args: args{
				array: []*struct {
					name string
				}{
					{
						name: "1",
					},
					{
						name: "2",
					},
					{
						name: "3",
					},
				},
				index: 2,
			},
			want: []*struct{ name string }{
				{
					name: "1",
				},
				{
					name: "2",
				},
			},
		},
		{
			name: "Remove",
			args: args{
				array: []*struct {
					name string
				}{
					{
						name: "1",
					},
				},
				index: 0,
			},
			want: []*struct{ name string }{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, Remove(tt.args.array, tt.args.index), "Remove(%v, %v)", tt.args.array, tt.args.index)
		})
	}
}

func TestRemoveAll(t *testing.T) {
	type args struct {
		array []*struct {
			name string
		}
		idxs []int
	}
	type testCase struct {
		name string
		args args
		want []*struct {
			name string
		}
	}
	tests := []testCase{
		{
			name: "Remove",
			args: args{
				array: []*struct {
					name string
				}{
					{
						name: "1",
					},
					{
						name: "2",
					},
					{
						name: "3",
					},
				},
				idxs: []int{0, 1, 2},
			},
			want: []*struct{ name string }{},
		},
		{
			name: "Remove",
			args: args{
				array: []*struct {
					name string
				}{
					{
						name: "1",
					},
					{
						name: "2",
					},
					{
						name: "3",
					},
				},
				idxs: []int{0},
			},
			want: []*struct{ name string }{
				{
					name: "2",
				},
				{
					name: "3",
				},
			},
		},
		{
			name: "Remove",
			args: args{
				array: []*struct {
					name string
				}{
					{
						name: "1",
					},
					{
						name: "2",
					},
					{
						name: "3",
					},
				},
				idxs: []int{0, 2},
			},
			want: []*struct{ name string }{
				{
					name: "2",
				},
			},
		},
		{
			name: "Remove",
			args: args{
				array: []*struct {
					name string
				}{
					{
						name: "1",
					},
					{
						name: "2",
					},
					{
						name: "3",
					},
				},
				idxs: []int{2, 0},
			},
			want: []*struct{ name string }{
				{
					name: "2",
				},
			},
		},
		{
			name: "Remove",
			args: args{
				array: []*struct {
					name string
				}{
					{
						name: "1",
					},
					{
						name: "2",
					},
					{
						name: "3",
					},
				},
				idxs: []int{1, 2, 0},
			},
			want: []*struct{ name string }{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			all := RemoveAll(tt.args.array, tt.args.idxs...)
			assert.Equalf(t, tt.want, all, "RemoveAll(%v, %v)", tt.args.array, tt.args.idxs)
		})
	}
}
