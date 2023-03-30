package registry

import (
	"time"

	"github.com/intergral/deep/modules/overrides"
)

type Overrides interface {
	MetricsGeneratorMaxActiveSeries(userID string) uint32
	MetricsGeneratorCollectionInterval(userID string) time.Duration
	MetricsGeneratorDisableCollection(userID string) bool
}

var _ Overrides = (*overrides.Overrides)(nil)
