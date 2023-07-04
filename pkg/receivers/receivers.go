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

package receivers

import (
	"github.com/go-kit/log"
	"github.com/intergral/deep/pkg/receivers/deep"
	"github.com/intergral/deep/pkg/receivers/types"
)

func ForConfig(receiverCfg map[string]interface{}, snapshotNext types.ProcessSnapshots, pollNext types.ProcessPoll, logger log.Logger) ([]types.Receiver, error) {
	var receivers []types.Receiver
	for key, cfg := range receiverCfg {
		switch key {
		case "deep":
			deepCfg, err := deep.CreateConfig(cfg)
			if err != nil {
				return nil, err
			}
			receiver, err := deep.NewDeepReceiver(deepCfg, snapshotNext, pollNext, logger)
			if err != nil {
				return nil, err
			}
			receivers = append(receivers, receiver)
		}
	}
	return receivers, nil
}
