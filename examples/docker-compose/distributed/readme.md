## Distributed

In this example deep is configured in a distributed manner, where modules are
running as separate components and configured to write data to S3 via MinIO,
which presents an S3 compatible API.

1. First start up the distributed stack.

```console
docker-compose up -d
```

At this point, the following containers should be spun up -

```console
docker-compose ps
```
```
             Name                            Command               State                         Ports                       
------------------------------------------------------------------------------------------------------------------------
distributed_tracepoint_1          /deep -target=tracepoint   ...   Up      0.0.0.0:32785->3200/tcp,:::32785->3200/tcp        
distributed_tracepoint-api_1      /deep -target=tracepoint   ...   Up      0.0.0.0:32785->3200/tcp,:::32785->3200/tcp        
distributed_compactor_1           /deep -target=compactor    ...   Up      0.0.0.0:32785->3200/tcp,:::32785->3200/tcp        
distributed_distributor_1         /deep -target=distributor  ...   Up      0.0.0.0:32787->3200/tcp,:::32787->3200/tcp        
distributed_grafana_1             /run.sh                          Up      0.0.0.0:3000->3000/tcp,:::3000->3000/tcp          
distributed_ingester-0_1          /deep -target=ingester -c  ...   Up      0.0.0.0:32789->3200/tcp,:::32789->3200/tcp        
distributed_ingester-1_1          /deep -target=ingester -c  ...   Up      0.0.0.0:32783->3200/tcp,:::32783->3200/tcp        
distributed_ingester-2_1          /deep -target=ingester -c  ...   Up      0.0.0.0:32784->3200/tcp,:::32784->3200/tcp        
distributed_metrics-generator_1   /deep -target=metrics-gen  ...   Up      0.0.0.0:32790->3200/tcp,:::32790->3200/tcp        
distributed_minio_1               sh -euc mkdir -p /data/dee ...   Up      9000/tcp, 0.0.0.0:9001->9001/tcp,:::9001->9001/tcp
distributed_prometheus_1          /bin/prometheus --config.f ...   Up      0.0.0.0:9090->9090/tcp,:::9090->9090/tcp          
distributed_querier_1             /deep -target=querier -co  ...   Up      0.0.0.0:32788->3200/tcp,:::32788->3200/tcp        
distributed_query-frontend_1      /deep -target=query-front  ...   Up      0.0.0.0:3200->3200/tcp,:::3200->3200/tcp
distributed_test-app-1            pythong src/simple_te      ...   Up 
```

2. If you're interested you can see the wal/blocks as they are being created.  Navigate to minio at
http://localhost:9001 and use the username/password of `deep`/`supersecret`.

3. Navigate to [Grafana](http://localhost:3000/explore) select the Deep data source and use the "Search"
tab to find snapshots.

4. To stop the setup use -

```console
docker-compose down -v
```
