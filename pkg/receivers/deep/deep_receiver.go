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

package deep

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/gogo/protobuf/proto"
	"github.com/intergral/deep/pkg/receivers/config/configgrpc"
	"github.com/intergral/deep/pkg/receivers/config/confignet"
	"github.com/intergral/deep/pkg/receivers/types"
	pb "github.com/intergral/go-deep-proto/poll/v1"
	tp "github.com/intergral/go-deep-proto/tracepoint/v1"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type Protocols struct {
	GRPC *configgrpc.GRPCServerSettings `mapstructure:"grpc"`
}

type DeepConfig struct {
	Protocols *Protocols `mapstructure:"protocols"`
	Debug     bool       `mapstructure:"debug"`
}

type deepReceiver struct {
	tp.UnimplementedSnapshotServiceServer
	pb.UnimplementedPollConfigServer

	cfg        *DeepConfig
	next       types.ProcessSnapshots
	logger     log.Logger
	pollNext   types.ProcessPoll
	serverGRPC *grpc.Server
	shutdownWG sync.WaitGroup
}

func CreateConfig(cfg interface{}) (*DeepConfig, error) {
	defaultCfg := &DeepConfig{
		Protocols: &Protocols{
			GRPC: &configgrpc.GRPCServerSettings{
				NetAddr: confignet.NetAddr{
					Endpoint:  "0.0.0.0:43315",
					Transport: "tcp",
				},
			},
		},
		Debug: false,
	}
	if cfg == nil {
		return defaultCfg, nil
	}
	var result DeepConfig
	err := mapstructure.Decode(cfg, &result)
	// if there is a config that is empty or has set debug, but not changed the protocols
	// then set the protocols to the default
	if result.Protocols == nil {
		result.Protocols = defaultCfg.Protocols
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func NewDeepReceiver(cfg *DeepConfig, next types.ProcessSnapshots, pollNext types.ProcessPoll, logger log.Logger) (types.Receiver, error) {
	return &deepReceiver{cfg: cfg, next: next, pollNext: pollNext, logger: logger}, nil
}

func (d *deepReceiver) Start(ctx context.Context, host types.Host) error {
	return d.startProtocolServers(host)
}

func (d *deepReceiver) Shutdown(ctx context.Context) error {
	if d.serverGRPC != nil {
		d.serverGRPC.GracefulStop()
	}

	d.shutdownWG.Wait()
	return nil
}

func (d *deepReceiver) startProtocolServers(host types.Host) error {
	var err error
	if d.cfg.Protocols.GRPC != nil {
		opts, err := d.cfg.Protocols.GRPC.ToServerOption()
		if err != nil {
			return err
		}
		d.serverGRPC = grpc.NewServer(opts...)

		tp.RegisterSnapshotServiceServer(d.serverGRPC, d)
		pb.RegisterPollConfigServer(d.serverGRPC, d)

		err = d.startGRPCServer(host)
	}
	return err
}

func (d *deepReceiver) Send(ctx context.Context, in *tp.Snapshot) (*tp.SnapshotResponse, error) {
	if d.cfg.Debug {
		d.logMessage("Received snapshot", in)
	}
	return d.next(ctx, in)
}

func (d *deepReceiver) Poll(ctx context.Context, pollRequest *pb.PollRequest) (*pb.PollResponse, error) {
	if d.cfg.Debug {
		d.logMessage("Received poll", pollRequest)
	}
	return d.pollNext(ctx, pollRequest)
}

func (d *deepReceiver) startGRPCServer(host types.Host) error {
	level.Info(d.logger).Log("msg", "Starting GRPC server on endpoint "+d.cfg.Protocols.GRPC.NetAddr.Endpoint)

	gln, err := d.cfg.Protocols.GRPC.ToListener()
	if err != nil {
		return err
	}
	d.shutdownWG.Add(1)
	go func() {
		defer d.shutdownWG.Done()

		if errGrpc := d.serverGRPC.Serve(gln); errGrpc != nil && !errors.Is(errGrpc, grpc.ErrServerStopped) {
			host.ReportFatalError(errGrpc)
		}
	}()
	return nil
}

func (d *deepReceiver) logMessage(s string, in proto.Message) {
	level.Info(d.logger).Log("msg", s, "proto", fmt.Sprintf("%+v", in))
}
