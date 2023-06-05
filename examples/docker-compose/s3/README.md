## S3

In this example deep is configured to write data to S3 via MinIO which presents an S3 compatible API.

1. First start up the S3 stack.

```console
docker-compose up -d
```

At this point, the following containers should be spun up -

```console
docker-compose ps
```
```
     Name                    Command               State                                  Ports                               
------------------------------------------------------------------------------------------------------------------------------
s3_grafana_1      /run.sh                          Up      0.0.0.0:3000->3000/tcp,:::3000->3000/tcp                           
s3_test-app_1     python src/simpl_t ...           Up                                                                         
s3_minio_1        sh -euc mkdir -p /data/dee ...   Up      9000/tcp, 0.0.0.0:9001->9001/tcp,:::9001->9001/tcp                 
s3_prometheus_1   /bin/prometheus --config.f ...   Up      0.0.0.0:9090->9090/tcp,:::9090->9090/tcp                           
s3_deep_1         /deep -config.file=/etc/d  ...   Up      0.0.0.0:43315->43315/tcp,:::43315->43315/tcp,                      
                                                           0.0.0.0:3300->3300/tcp,:::3300->3300/tcp 
```

2. If you're interested you can see the wal/blocks as they are being created.  Navigate to minio at
   http://localhost:9001 and use the username/password of `deep`/`supersecret`.

3. Navigate to [Grafana](http://localhost:3000/explore) select the Deep data source and use the "Search"
   tab to find traces. Also notice that you can query Deep metrics from the Prometheus data source setup in
   Grafana.

4. To stop the setup use -

```console
docker-compose down -v
```
