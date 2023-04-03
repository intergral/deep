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
