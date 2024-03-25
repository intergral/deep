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

package ql

import (
	"reflect"
	"testing"
)

func TestParseString(t *testing.T) {
	tests := []struct {
		deepQl  string
		trigger *trigger
		command *command
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
			wantErr: "parse error at line 1, col 1 near token 'log': syntax error",
		},
		{
			deepQl:  "log{log=\"this is a log message\" line=12 file=\"SimpleTest.java\"}",
			trigger: &trigger{primary: "log", fireCount: -1, rateLimit: 1000, windowStart: "now", windowEnd: "48h", log: "this is a log message", targeting: map[string]string{}, line: 12, file: "SimpleTest.java"},
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
			command: &command{command: list},
		},
		{
			deepQl:  "list{file=\"some_file.py\"}",
			command: &command{command: list, file: "some_file.py"},
		},
		{
			deepQl:  "delete{file=\"some_file.py\"}",
			command: &command{command: deleteType, file: "some_file.py"},
		},
		{
			deepQl:  "delete{id=\"alksjdsa89dashd89\"}",
			command: &command{command: deleteType, id: "alksjdsa89dashd89"},
		},
		{
			deepQl:  "delete{line=88}",
			wantErr: "parse error unrecognized option: line",
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
		})
	}
}
