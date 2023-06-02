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
	"fmt"
	tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"time"

	"github.com/go-kit/log"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/intergral/deep/modules/distributor/forwarder/otlpgrpc"
)

type Forwarder interface {
	ForwardTraces(ctx context.Context, traces ptrace.Traces) error
	Shutdown(ctx context.Context) error
}

type List []Forwarder

func (l List) ForwardTraces(ctx context.Context, traces ptrace.Traces) error {
	var errs []error

	for _, forwarder := range l {
		if err := forwarder.ForwardTraces(ctx, traces); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.Combine(errs...)
}

func (l List) ForwardSnapshot(ctx context.Context, snapshot *tp.Snapshot) error {
	print("ForwardSnapshot")
	return nil
}

func New(cfg Config, logger log.Logger) (Forwarder, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	switch cfg.Backend {
	case OTLPGRPCBackend:
		f, err := otlpgrpc.NewForwarder(cfg.OTLPGRPC, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create new otlpgrpc forwarder: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := f.Dial(ctx); err != nil {
			return nil, fmt.Errorf("failed to dial: %w", err)
		}

		return f, nil
	default:
		return nil, fmt.Errorf("%s backend is not supported", cfg.Backend)
	}
}
