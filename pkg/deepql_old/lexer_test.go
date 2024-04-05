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

package deepql_old

import (
	"reflect"
	"testing"
	"time"
)

func TestParseString(t *testing.T) {
	tests := []struct {
		deepql_old string
		trigger    *trigger
		command    *command
		search     *Pipeline
		wantErr    string
	}{
		{
			deepql_old: "trigger{}",
			wantErr:    "trigger missing required property: file\ntrigger missing required property: 'line' or 'method'",
		},
		{
			deepql_old: "log{file=\"simple_test.py\"}",
			wantErr:    "log missing required property: 'line' or 'method'\nlog missing required property: log",
		},
		{
			deepql_old: "metric{line=12}",
			wantErr:    "metric missing required property: file",
		},
		{
			deepql_old: "span{line=12 file=\"SimpleTest.java\" span=\"nest\"}",
			wantErr:    "span option 'span' has invalid value: nest (valid options: line, method)",
		},
		{
			deepql_old: "snapshot{line=12 file=\"SimpleTest.java\" target=\"nest\"}",
			wantErr:    "snapshot option 'target' has invalid value: nest (valid options: start, end, capture)",
		},
		{
			deepql_old: "log{log=\"this is a log message\"}",
			wantErr:    "log missing required property: file\nlog missing required property: 'line' or 'method'",
		},
		{
			deepql_old: "log {log=\"this is a log message\"}",
			wantErr:    "parse error at line 1, col 1 near token 'log': syntax error: unexpected IDENTIFIER",
		},
		{
			deepql_old: "log{log=\"this is a log message\" line=12 file=\"SimpleTest.java\" fire_count=12}",
			trigger:    &trigger{primary: "log", fireCount: 12, rateLimit: 1000, windowStart: "now", windowEnd: "48h", log: "this is a log message", targeting: map[string]string{}, line: 12, file: "SimpleTest.java"},
		},
		{
			deepql_old: "log{log=\"this is a log message\" label_my_label=\"atest\" line=12 file=\"SimpleTest.java\"}",
			trigger:    &trigger{primary: "log", fireCount: -1, rateLimit: 1000, windowStart: "now", windowEnd: "48h", log: "this is a log message", logLabels: []label{{key: "my_label", value: "atest"}}, targeting: map[string]string{}, line: 12, file: "SimpleTest.java"},
		},
		{
			deepql_old: "log{log=\"this is a log message\" label_my_label=\"atest\" file=\"simple_test.py\" line=44 osname=\"windows\"}",
			trigger:    &trigger{primary: "log", fireCount: -1, rateLimit: 1000, windowStart: "now", windowEnd: "48h", log: "this is a log message", logLabels: []label{{key: "my_label", value: "atest"}}, file: "simple_test.py", line: 44, targeting: map[string]string{"osname": "windows"}},
		},
		{
			deepql_old: "log{log=\"this is a log message\" label_my_label=\"atest\" file=\"simple_test.py\" line=44 resource.osname=\"windows\"}",
			trigger:    &trigger{primary: "log", fireCount: -1, rateLimit: 1000, windowStart: "now", windowEnd: "48h", log: "this is a log message", logLabels: []label{{key: "my_label", value: "atest"}}, file: "simple_test.py", line: 44, targeting: map[string]string{"osname": "windows"}},
		},
		{
			deepql_old: "log{log=\"this is a log message\" label_my_label=\"atest\" file=\"simple_test.py\" line=44 os.name=\"windows\"}",
			trigger:    &trigger{primary: "log", fireCount: -1, rateLimit: 1000, windowStart: "now", windowEnd: "48h", log: "this is a log message", logLabels: []label{{key: "my_label", value: "atest"}}, file: "simple_test.py", line: 44, targeting: map[string]string{"os.name": "windows"}},
		},
		{
			deepql_old: "list{}",
			command:    &command{command: list},
		},
		{
			deepql_old: "list{file=\"some_file.py\"}",
			command:    &command{command: list, file: "some_file.py"},
		},
		{
			deepql_old: "delete{file=\"some_file.py\"}",
			command:    &command{command: deleteType, file: "some_file.py"},
		},
		{
			deepql_old: "delete{id=\"alksjdsa89dashd89\"}",
			command:    &command{command: deleteType, id: "alksjdsa89dashd89"},
		},
		{
			deepql_old: "delete{line=88}",
			wantErr:    "parse error unrecognized option: line",
		},
		{
			deepql_old: "{}",
			search:     &Pipeline{},
		},
		{
			deepql_old: "{name=\"asd\"}",
			search:     &Pipeline{},
		},
		{
			deepql_old: "{line!=88}",
			search:     &Pipeline{},
		},
		{
			deepql_old: "{line!=88 && file=\"SimpleTest.java\"}",
			search:     &Pipeline{},
		},
		{
			deepql_old: "{line!=88 && file=~\"SimpleTest.java\"}",
			search:     &Pipeline{},
		},
		{
			deepql_old: "{line!=88 && file!~\"SimpleTest.java\"}",
			search:     &Pipeline{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.deepql_old, func(t *testing.T) {
			got, err := Parse(tt.deepql_old)
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
				if !reflect.DeepEqual(got.Pipeline, tt.search) {
					t.Errorf("ParseString() got = %+v, search %+v", got.Pipeline, tt.search)
				}
			}
		})
	}
}

func asRef(dur time.Duration) *time.Duration {
	return &dur
}
