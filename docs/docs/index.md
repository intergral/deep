# Deep

DEEP is a Dynamic Exploration Evaluation Protocol, that allow the gathering of application data as and when it is
needed.

NOTE: This project is in early beta. Feedback is welcome please submit an issue
on [GitHub](https://github.com/intergra/deep)

## Usage

To use deep you need to deploy the service, there are a few options for this:

- [Local as a docker compose stack](./deploy/local.md)
- [Using AWS EKS](deploy/aws/aws-eks.md)

Once the service is deployed you will need to setup a client and connect Grafana to the service.

### Grafana

To connect grafana to DEEP you will need to install the deep plugins.

- [Datasource Plugin](https://github.com/intergral/grafana-deep-datasource)
- [Panel Plugin](https://github.com/intergral/grafana-deep-panel)

These plugins are planned for release, pending review from Grafana.

### Client

To set up the client you will need to pick the appropriate language and follow the instructions for that client. The
available clients are:

- [Python Client](https://github.com/intergral/deep-python-client)
- [Java Client](https://github.com/intergral/deep-java-client)
- [ColdFusion Client](https://github.com/intergral/deep-java-client)
