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

package deepql

import (
	"reflect"
	"testing"
	"time"
)

func TestParseString(t *testing.T) {
	tests := []struct {
		deepQl  string
		trigger *trigger
		command *command
		search  *search
		wantErr string
	}{
		{
			deepQl:  "trigger{}",
			wantErr: "trigger missing required property: file\ntrigger missing required property: 'line' or 'method'",
		},
		{
			deepQl:  "log{file=\"simple_test.py\"}",
			wantErr: "log missing required property: 'line' or 'method'\nlog missing required property: log",
		},
		{
			deepQl:  "metric{line=12}",
			wantErr: "metric missing required property: file",
		},
		{
			deepQl:  "span{line=12 file=\"SimpleTest.java\" span=\"nest\"}",
			wantErr: "span option 'span' has invalid value: nest (valid options: line, method)",
		},
		{
			deepQl:  "snapshot{line=12 file=\"SimpleTest.java\" target=\"nest\"}",
			wantErr: "snapshot option 'target' has invalid value: nest (valid options: start, end, capture)",
		},
		{
			deepQl:  "log {log=\"this is a log message\"}",
			wantErr: "parse error at line 1, col 5 near token '{': syntax error",
		},
		{
			deepQl:  "log{log=\"this is a log message\" line=12 file=\"SimpleTest.java\" fire_count=12}",
			trigger: &trigger{primary: "log", fireCount: 12, rateLimit: 1000, windowStart: "now", windowEnd: "48h", log: "this is a log message", targeting: map[string]string{}, line: 12, file: "SimpleTest.java"},
		},
		{
			deepQl:  "log{log=\"this is a log message\" label_my_label=\"atest\" line=12 file=\"SimpleTest.java\"}",
			trigger: &trigger{primary: "log", fireCount: -1, rateLimit: 1000, windowStart: "now", windowEnd: "48h", log: "this is a log message", logLabels: []label{{key: "my_label", value: "atest"}}, targeting: map[string]string{}, line: 12, file: "SimpleTest.java"},
		},
		{
			deepQl:  "log{log=\"this is a log message\" label_my_label=\"atest\" file=\"simple_test.py\" line=44 osname=\"windows\"}",
			trigger: &trigger{primary: "log", fireCount: -1, rateLimit: 1000, windowStart: "now", windowEnd: "48h", log: "this is a log message", logLabels: []label{{key: "my_label", value: "atest"}}, file: "simple_test.py", line: 44, targeting: map[string]string{"osname": "windows"}},
		},
		{
			deepQl:  "log{log=\"this is a log message\" label_my_label=\"atest\" file=\"simple_test.py\" line=44 resource.osname=\"windows\"}",
			trigger: &trigger{primary: "log", fireCount: -1, rateLimit: 1000, windowStart: "now", windowEnd: "48h", log: "this is a log message", logLabels: []label{{key: "my_label", value: "atest"}}, file: "simple_test.py", line: 44, targeting: map[string]string{"osname": "windows"}},
		},
		{
			deepQl:  "log{log=\"this is a log message\" label_my_label=\"atest\" file=\"simple_test.py\" line=44 os.name=\"windows\"}",
			trigger: &trigger{primary: "log", fireCount: -1, rateLimit: 1000, windowStart: "now", windowEnd: "48h", log: "this is a log message", logLabels: []label{{key: "my_label", value: "atest"}}, file: "simple_test.py", line: 44, targeting: map[string]string{"os.name": "windows"}},
		},
		{
			deepQl:  "list{}",
			command: &command{command: List},
		},
		{
			deepQl:  "list{path=\"some_file.py\"}",
			command: &command{command: List, rules: []configOption{newConfigOption(OpEqual, "path", Static{Type: TypeString, S: "some_file.py"})}},
		},
		{
			deepQl:  "delete{path=\"some_file.py\"}",
			command: &command{command: DeleteType, rules: []configOption{newConfigOption(OpEqual, "path", Static{Type: TypeString, S: "some_file.py"})}},
		},
		{
			deepQl:  "delete{id=\"alksjdsa89dashd89\"}",
			command: &command{command: DeleteType, rules: []configOption{newConfigOption(OpEqual, "id", Static{Type: TypeString, S: "alksjdsa89dashd89"})}},
		},
		{
			deepQl:  "delete{id=\"alksjdsa89dashd89\" id=\"otherid\"}",
			command: &command{command: DeleteType, rules: []configOption{newConfigOption(OpEqual, "id", Static{Type: TypeString, S: "otherid"}), newConfigOption(OpEqual, "id", Static{Type: TypeString, S: "alksjdsa89dashd89"})}},
		},
		{
			deepQl:  "delete{line=88}",
			command: &command{command: DeleteType, rules: []configOption{newConfigOption(OpEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "{}",
			search: &search{},
		},
		{
			deepQl: "{line=88}",
			search: &search{rules: []configOption{newConfigOption(OpEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "{somevalue = false}",
			search: &search{rules: []configOption{newConfigOption(OpEqual, "somevalue", Static{Type: TypeBoolean, B: false})}},
		},
		{
			deepQl: "{line!=88}",
			search: &search{rules: []configOption{newConfigOption(OpNotEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "{line!=88 file=\"SimpleTest.java\"}",
			search: &search{rules: []configOption{newConfigOption(OpEqual, "file", Static{Type: TypeString, S: "SimpleTest.java"}), newConfigOption(OpNotEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "{line!=88 file=~\"SimpleTest.java\"}",
			search: &search{rules: []configOption{newConfigOption(OpRegex, "file", Static{Type: TypeString, S: "SimpleTest.java"}), newConfigOption(OpNotEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "{line!=88 file!~\"SimpleTest.java\"}",
			search: &search{rules: []configOption{newConfigOption(OpNotRegex, "file", Static{Type: TypeString, S: "SimpleTest.java"}), newConfigOption(OpNotEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "sum({line=88})",
			search: &search{aggregate: "sum", rules: []configOption{newConfigOption(OpEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "count({line!=88})",
			search: &search{aggregate: "count", rules: []configOption{newConfigOption(OpNotEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "max({line!=88 file=\"SimpleTest.java\"})",
			search: &search{aggregate: "max", rules: []configOption{newConfigOption(OpEqual, "file", Static{Type: TypeString, S: "SimpleTest.java"}), newConfigOption(OpNotEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "min({line!=88 file=~\"SimpleTest.java\"})",
			search: &search{aggregate: "min", rules: []configOption{newConfigOption(OpRegex, "file", Static{Type: TypeString, S: "SimpleTest.java"}), newConfigOption(OpNotEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "mean({line!=88 file!~\"SimpleTest.java\"})",
			search: &search{aggregate: "mean", rules: []configOption{newConfigOption(OpNotRegex, "file", Static{Type: TypeString, S: "SimpleTest.java"}), newConfigOption(OpNotEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "sum({line=88})[5m]",
			search: &search{window: asRef(time.Minute * 5), aggregate: "sum", rules: []configOption{newConfigOption(OpEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "count({line!=88})[1m]",
			search: &search{window: asRef(time.Minute * 1), aggregate: "count", rules: []configOption{newConfigOption(OpNotEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "max({line!=88 file=\"SimpleTest.java\"})[1d]",
			search: &search{window: asRef(time.Hour * 24), aggregate: "max", rules: []configOption{newConfigOption(OpEqual, "file", Static{Type: TypeString, S: "SimpleTest.java"}), newConfigOption(OpNotEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "min({line!=88 file=~\"SimpleTest.java\"})[7d]",
			search: &search{window: asRef(time.Hour * 24 * 7), aggregate: "min", rules: []configOption{newConfigOption(OpRegex, "file", Static{Type: TypeString, S: "SimpleTest.java"}), newConfigOption(OpNotEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
		{
			deepQl: "mean({line!=88 file!~\"SimpleTest.java\"})[14d]",
			search: &search{window: asRef(time.Hour * 24 * 14), aggregate: "mean", rules: []configOption{newConfigOption(OpNotRegex, "file", Static{Type: TypeString, S: "SimpleTest.java"}), newConfigOption(OpNotEqual, "line", Static{Type: TypeInt, N: 88})}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.deepQl, func(t *testing.T) {
			got, err := ParseString(tt.deepQl)
			if err != nil {
				if tt.wantErr != err.Error() {
					t.Errorf("ParseString() error = %+v, wantErr %+v", err, tt.wantErr)
				}
				return
			}
			if tt.wantErr != "" && err == nil {
				t.Errorf("ParseString() wantErr %+v", tt.wantErr)
				return
			}
			if tt.trigger != nil {
				if !reflect.DeepEqual(got.trigger, tt.trigger) {
					t.Errorf("ParseString() got = %+v, trigger %+v", got.trigger, tt.trigger)
				}
			}
			if tt.command != nil {
				if !reflect.DeepEqual(got.command, tt.command) {
					t.Errorf("ParseString() got = %+v, command %+v", got.command, tt.command)
				}
			}
			if tt.search != nil {
				if !reflect.DeepEqual(got.search, tt.search) {
					t.Errorf("ParseString() got = %+v, search %+v", got.search, tt.search)
				}
			}
		})
	}
}

func asRef[T any](dur T) *T {
	return &dur
}
