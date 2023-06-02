## Local Storage

In this example all data is stored locally in the `deep-data` folder. Local storage is fine for experimenting with DEEP
or when using the single binary, but does not work in a distributed/microservices scenario.

1. First start up the local stack.

```console
docker compose up -d
```

At this point, the following containers should be spun up -

```console
docker compose ps
```

```
       Name                     Command               State                                   Ports                                 
-----------------------------------------------------------------------------------------------------------
local_grafana_1      /run.sh                          Up      0.0.0.0:3000->3000/tcp,:::3000->3000/tcp                              
local_test_app_1     /python src/simple-a...          Up                                                                            
local_prometheus_1   /bin/prometheus --config.f ...   Up      0.0.0.0:9090->9090/tcp,:::9090->9090/tcp                              
local_deep_1         /deep -config.file=/etc/d ...    Up      0.0.0.0:43315->43315/tcp,:::43315->43315/tcp                         
                                                              0.0.0.0:3300->3300/tcp,:::3300->3300/tcp                             
```

2. If you're interested you can see the wal/blocks as they are being created.

```console
ls deep-data/
```

3. Navigate to [Grafana](http://localhost:3000/explore) select the Deep data source and use the "Search"
   tab to find snapshots. Also notice that you can query Deep metrics from the Prometheus data source setup in
   Grafana.

4. To stop the setup use -

```console
docker compose down -v
```
