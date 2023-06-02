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

package processor

import (
	"context"

	"github.com/intergral/deep/pkg/deeppb"
)

type Processor interface {
	// Name returns the name of the processor.
	Name() string

	// PushSnapshot processes a snapshot and updates the metrics registered in RegisterMetrics.
	PushSnapshot(ctx context.Context, req *deeppb.PushSnapshotRequest)

	// Shutdown releases any resources allocated by the processor. Once the processor is shut down,
	// PushSnapshot should not be called anymore.
	Shutdown(ctx context.Context)
}
