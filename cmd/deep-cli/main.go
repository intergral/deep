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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/intergral/deep/cmd/deep/app"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/azure"
	"github.com/intergral/deep/pkg/deepdb/backend/gcs"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/backend/s3"
	"gopkg.in/yaml.v2"
)

type globalOptions struct {
	ConfigFile string `type:"path" short:"c" help:"Path to deep config file"`
}

type frontendOptions struct {
	Endpoint string `help:"The url to send data to" default:"localhost:43315"`
}

type apiOptions struct {
	Endpoint string `help:"The url to use for the api" default:"localhost:3300"`
}

type backendOptions struct {
	Backend string `help:"backend to connect to (s3/gcs/local/azure), optional, overrides backend in config file" enum:",s3,gcs,local,azure"`
	Bucket  string `help:"bucket (or path on local backend) to scan, optional, overrides bucket in config file"`

	S3Endpoint string `name:"s3-endpoint" help:"s3 endpoint (s3.dualstack.us-east-2.amazonaws.com), optional, overrides endpoint in config file"`
	S3User     string `name:"s3-user" help:"s3 username, optional, overrides username in config file"`
	S3Pass     string `name:"s3-pass" help:"s3 password, optional, overrides password in config file"`
}

var cli struct {
	globalOptions

	List struct {
		Blocks      listBlocksCmd     `cmd:"" help:"List information about all blocks in a bucket"`
		Column      listColumnCmd     `cmd:"" help:"List values in a given column"`
		Objects     listObjectsCmd    `cmd:"" help:"List values in a block"`
		Tracepoints listTracepointCmd `cmd:"" help:"List configured tracepoints"`
	} `cmd:""`

	Generate struct {
		Snapshot   generateSnapshotCmd `cmd:"snapshot" aliases:"snap" help:"Can generate a snapshot and send to configured endpoint"`
		Tags       generateTagsCmd     `cmd:"tags" help:"Generate basic snapshots with a variety of tags"`
		Tracepoint cmdCreateTracepoint `cmd:"tp" help:"Generate a new tracepoint"`
		Block      cmdGenerateBlock    `cmd:"block" help:"Generate a test block"`
	} `cmd:"generate" aliases:"gen"`

	Delete struct {
		Tracepoint cmdDeleteTracepoint `cmd:"tp" help:"Delete a tracepoint"`
	} `cmd:"delete"`
}

func main() {
	ctx := kong.Parse(&cli,
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			// Compact: true,
		}),
	)
	err := ctx.Run(&cli.globalOptions)
	ctx.FatalIfErrorf(err)
}

func loadBackend(b *backendOptions, g *globalOptions) (backend.Reader, backend.Writer, backend.Compactor, error) {
	// Defaults
	cfg := app.Config{}
	cfg.RegisterFlagsAndApplyDefaults("", &flag.FlagSet{})

	// Existing config
	if g.ConfigFile != "" {
		buff, err := os.ReadFile(g.ConfigFile)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to read configFile %s: %w", g.ConfigFile, err)
		}

		err = yaml.UnmarshalStrict(buff, &cfg)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse configFile %s: %w", g.ConfigFile, err)
		}
	}

	// cli overrides
	if b.Backend != "" {
		cfg.StorageConfig.TracePoint.Backend = b.Backend
	}

	if b.Bucket != "" {
		cfg.StorageConfig.TracePoint.Local.Path = b.Bucket
		cfg.StorageConfig.TracePoint.GCS.BucketName = b.Bucket
		cfg.StorageConfig.TracePoint.S3.Bucket = b.Bucket
		cfg.StorageConfig.TracePoint.Azure.ContainerName = b.Bucket
	}

	if b.S3Endpoint != "" {
		cfg.StorageConfig.TracePoint.S3.Endpoint = b.S3Endpoint
	}

	var err error
	var r backend.RawReader
	var w backend.RawWriter
	var c backend.Compactor

	switch cfg.StorageConfig.TracePoint.Backend {
	case "local":
		r, w, c, err = local.New(cfg.StorageConfig.TracePoint.Local)
	case "gcs":
		r, w, c, err = gcs.New(cfg.StorageConfig.TracePoint.GCS)
	case "s3":
		r, w, c, err = s3.New(cfg.StorageConfig.TracePoint.S3)
	case "azure":
		r, w, c, err = azure.New(cfg.StorageConfig.TracePoint.Azure)
	default:
		err = fmt.Errorf("unknown backend %s", cfg.StorageConfig.TracePoint.Backend)
	}

	if err != nil {
		return nil, nil, nil, err
	}

	return backend.NewReader(r), backend.NewWriter(w), c, nil
}
