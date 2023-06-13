![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/intergral/deep)
[![Build & Test](https://github.com/intergral/deep/actions/workflows/on_push.yml/badge.svg)](https://github.com/intergral/deep/actions/workflows/on_push.yml)

![image](https://github.com/intergral/deep/assets/10787304/33fbd546-1153-433a-9f34-f1f0dd14c5b5)

# Deep - dynamic instrumention engine for runtime logs, metrics, traces and snapshots

Deep is an open source, easy-to-use and high-scale distributed dynamic instrumentation engine that can dynamically add logs, metrics, traces and snapshots your applications at runtime. Deep is cost-efficient, requiring only object storage to operate, and is deeply integrated with Grafana, Prometheus, Grafana Loki and Grafana Tempo.

Deep ingests snapshots and buffers them and then writes them to Azure, GCS, S3 or local disk. As such it is robust, cheap and easy to operate!

Deep implements [DeepQL](), a monitoring-first query and command language inspired by PromQL. This query language
allows users to very precisely and easily select snapshots and jump directly to the snapshots fulfilling the specified
conditions, as well as introduce me dynamic monitoring points into your applications.

## Getting Started


- [Deployment Examples](./examples/README.md)
    - [Docker Compose](./examples/docker-compose/README.md)

## Acknowledgements

Deep is a fork of [Grafana Tempo](https://github.com/grafana/tempo), that has been reworked to work with the unique aspects that Deep offers.

## License

Deep is distributed under [AGPL-3.0-only](LICENSE). For Apache-2.0 exceptions, see [LICENSING.md](LICENSING.md).
