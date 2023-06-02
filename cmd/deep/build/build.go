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

package build

import (
	"github.com/prometheus/common/version"
	prom "github.com/prometheus/prometheus/web/api/v1"
)

// Version information passed to Prometheus version package.
// Package path as used by linker changes based on vendoring being used or not,
// so it's easier just to use stable Tempo path, and pass it to
// Prometheus in the code.
var (
	Version   string
	Revision  string
	Branch    string
	BuildUser string
	BuildDate string
)

func init() {
	version.Version = Version
	version.Revision = Revision
	version.Branch = Branch
	version.BuildUser = BuildUser
	version.BuildDate = BuildDate
}

func GetVersion() prom.PrometheusVersion {
	return prom.PrometheusVersion{
		Version:   version.Version,
		Revision:  version.Revision,
		Branch:    version.Branch,
		BuildUser: version.BuildUser,
		BuildDate: version.BuildDate,
		GoVersion: version.GoVersion,
	}
}
