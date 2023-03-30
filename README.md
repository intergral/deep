# DEEP

DEEP is an open source, easy-to-use and high-scale distributed dynamic monitoring backend. DEEP is cost-efficient,
requiring only object storage to operate, and is deeply integrated with Grafana, Prometheus, Tempo and Loki.

Deep ingests batches in any supported formats, buffers them and then writes them to Azure, GCS, S3 or local disk. As
such it is robust, cheap and easy to operate!

DEEP implements [DQL](), a monitoring-first query and command language inspired by LogQL and PromQL. This query language
allows users to very precisely and easily select snapshots and jump directly to the snapshots fulfilling the specified
conditions, as well as introduce me dynamic monitoring points into your applications.

## Getting Started

- [Deployment Examples](./example)
    - [Docker Compose](./example/docker-compose)

## Acknowledgements

DEEP is a fork of [Grafana Tempo](https://github.com/grafana/tempo), that has been reworked to work with the unique aspects that DEEP offers.

## License

Grafana Tempo is distributed under [AGPL-3.0-only](LICENSE).
