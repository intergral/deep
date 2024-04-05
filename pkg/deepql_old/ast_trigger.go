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
	"errors"
	"fmt"
	"strings"
)

const (
	log         = "log"
	snapshot    = "snapshot"
	metric      = "metric"
	span        = "span"
	triggerType = "trigger"

	line   = "line"
	method = "method"

	start   = "start"
	end     = "end"
	capture = "capture"
)

var triggerTypes = map[string]int{
	log:         TRIGGER,
	metric:      TRIGGER,
	triggerType: TRIGGER,
	span:        TRIGGER,
	snapshot:    TRIGGER,
}

type label struct {
	key   string
	value string
}

type trigger struct {
	primary string

	// file is the source file path
	file string

	// line of method is required but not both
	line   uint
	method string

	// control options
	fireCount   int
	rateLimit   uint
	windowStart string
	windowEnd   string
	condition   string

	// snapshot options
	snapshot       bool
	target         string
	watch          []string
	snapshotLabels []label

	// metric options
	metric       bool
	metricName   string
	metricLabels []label

	// span options
	span       string
	spanName   string
	spanLabels []label

	// log options
	log       string
	logLabels []label
	targeting map[string]string

	errors []error
}

func missingRequired(tname, name string) error {
	return errors.New(fmt.Sprintf("%s missing required property: %s", tname, name))
}

func invalidOptionWithOptions(tname, name, option string, opts ...string) error {
	return errors.New(fmt.Sprintf("%s option '%s' has invalid value: %s (valid options: %s)", tname, name, option, strings.Join(opts, ", ")))
}

func (t trigger) validate() error {
	var errs []error
	if t.file == "" {
		errs = append(errs, missingRequired(t.primary, "file"))
	}

	if t.line == 0 && t.method == "" {
		errs = append(errs, missingRequired(t.primary, "'line' or 'method'"))
	}

	if t.primary == log && t.log == "" {
		errs = append(errs, missingRequired(t.primary, "log"))
	}

	if t.span != "" && t.span != line && t.span != method {
		errs = append(errs, invalidOptionWithOptions(t.primary, "span", t.span, line, method))
	}

	if t.target != "" && t.target != start && t.target != end && t.target != capture {
		errs = append(errs, invalidOptionWithOptions(t.primary, "target", t.target, start, end, capture))
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func newTrigger(typ string, opts []configOption) trigger {
	var cfg trigger
	switch typ {
	case triggerType:
		cfg = trigger{
			primary:     triggerType,
			fireCount:   1,
			rateLimit:   1000,
			windowEnd:   "48h",
			windowStart: "now",
			condition:   "",

			snapshot: false,

			metric: false,

			span: "",

			log:       "",
			targeting: make(map[string]string),
		}
	case log:
		cfg = trigger{
			primary:     log,
			fireCount:   -1,
			rateLimit:   1000,
			windowEnd:   "48h",
			windowStart: "now",
			condition:   "",

			snapshot: false,

			metric: false,

			span: "",

			log:       "",
			targeting: make(map[string]string),
		}
	case snapshot:
		cfg = trigger{
			primary:     snapshot,
			fireCount:   100,
			rateLimit:   10000,
			windowEnd:   "7d",
			windowStart: "now",
			condition:   "",

			snapshot: true,
			target:   start,

			metric: false,

			span: "",

			log:       "",
			targeting: make(map[string]string),
		}
	case metric:
		cfg = trigger{
			primary:     metric,
			fireCount:   100,
			rateLimit:   10000,
			windowEnd:   "28d",
			windowStart: "now",
			condition:   "",

			snapshot: false,

			metric: true,

			span: "",

			log:       "",
			targeting: make(map[string]string),
		}
	case span:
		cfg = trigger{
			primary:     span,
			fireCount:   -1,
			rateLimit:   100,
			windowEnd:   "0",
			windowStart: "now",
			condition:   "",

			snapshot: false,

			metric: false,

			span: "",

			log:       "",
			targeting: make(map[string]string),
		}
	}

	for _, opt := range opts {
		err := opt.apply(&cfg)
		if err != nil {
			cfg.errors = append(cfg.errors, err)
		}
	}

	return cfg
}

func applyFuncForTrigger(lhs string) func(c *configOption, tri *trigger) error {
	switch lhs {
	case "file":
		return func(c *configOption, tri *trigger) error {
			tri.file = c.rhs.S
			return nil
		}
	case "line":
		return func(c *configOption, tri *trigger) error {
			tri.line = uint(c.rhs.N)
			return nil
		}
	case "method":
		return func(c *configOption, tri *trigger) error {
			tri.method = c.rhs.S
			return nil
		}
	case "fireCount", "fire_count":
		return func(c *configOption, tri *trigger) error {
			tri.fireCount = c.rhs.N
			return nil
		}
	case "rateLimit", "rate_limit":
		return func(c *configOption, tri *trigger) error {
			tri.rateLimit = uint(c.rhs.N)
			return nil
		}
	case "windowStart", "window_start":
		return func(c *configOption, tri *trigger) error {
			tri.windowStart = c.rhs.S
			return nil
		}
	case "windowEnd", "window_end":
		return func(c *configOption, tri *trigger) error {
			tri.windowEnd = c.rhs.S
			return nil
		}
	case "condition":
		return func(c *configOption, tri *trigger) error {
			tri.condition = c.rhs.S
			return nil
		}
	case "snapshot":
		return func(c *configOption, tri *trigger) error {
			tri.snapshot = c.rhs.B
			if tri.target == "" {
				tri.target = start
			}
			return nil
		}
	case "target":
		return func(c *configOption, tri *trigger) error {
			tri.target = c.rhs.S
			return nil
		}
	case "metric":
		return func(c *configOption, tri *trigger) error {
			tri.metric = c.rhs.B
			return nil
		}
	case "metricName", "metric_name":
		return func(c *configOption, tri *trigger) error {
			tri.metricName = c.rhs.S
			return nil
		}
	case "span":
		return func(c *configOption, tri *trigger) error {
			tri.span = c.rhs.S
			return nil
		}
	case "spanName", "span_name":
		return func(c *configOption, tri *trigger) error {
			tri.spanName = c.rhs.S
			return nil
		}
	case "log":
		return func(c *configOption, tri *trigger) error {
			tri.log = c.rhs.S
			return nil
		}
	case "watch":
		return func(c *configOption, tri *trigger) error {
			tri.watch = append(tri.watch, c.rhs.S)
			return nil
		}
	}

	// generic label prefix attaches labels to the primary type for the trigger
	if strings.HasPrefix(lhs, "label_") {
		return func(c *configOption, tri *trigger) error {
			key := stripPrefix(lhs, "label_")
			switch tri.primary {
			case snapshot, triggerType:
				tri.snapshotLabels = append(tri.snapshotLabels, label{key: key, value: c.rhs.S})
			case log:
				tri.logLabels = append(tri.snapshotLabels, label{key: key, value: c.rhs.S})
			case metric:
				tri.metricLabels = append(tri.snapshotLabels, label{key: key, value: c.rhs.S})
			case span:
				tri.spanLabels = append(tri.snapshotLabels, label{key: key, value: c.rhs.S})
			}
			return nil
		}
	}
	if strings.HasPrefix(lhs, "snapshot_label_") {
		return func(c *configOption, tri *trigger) error {
			key := stripPrefix(lhs, "snapshot_label_")
			tri.snapshotLabels = append(tri.snapshotLabels, label{key: key, value: c.rhs.S})
			return nil
		}
	}
	if strings.HasPrefix(lhs, "log_label_") {
		return func(c *configOption, tri *trigger) error {
			key := stripPrefix(lhs, "log_label_")
			tri.logLabels = append(tri.logLabels, label{key: key, value: c.rhs.S})
			return nil
		}
	}
	if strings.HasPrefix(lhs, "metric_label_") {
		return func(c *configOption, tri *trigger) error {
			key := stripPrefix(lhs, "metric_label_")
			tri.metricLabels = append(tri.metricLabels, label{key: key, value: c.rhs.S})
			return nil
		}
	}
	if strings.HasPrefix(lhs, "span_label_") {
		return func(c *configOption, tri *trigger) error {
			key := stripPrefix(lhs, "span_label_")
			tri.spanLabels = append(tri.spanLabels, label{key: key, value: c.rhs.S})
			return nil
		}
	}
	if strings.HasPrefix(lhs, "resource.") {
		lhs = stripPrefix(lhs, "resource.")
	}
	return func(cfg *configOption, tri *trigger) error {
		tri.targeting[lhs] = cfg.rhs.S
		return nil
	}
}
