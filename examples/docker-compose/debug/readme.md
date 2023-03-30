## Remote Debugging

How to use remote debugging with Deep. This example builds on the [local example](../local), for 
questions about the general setup, please refer to its readme file.

Although it is possible to debug Deep with [`dlv debug`](https://github.com/go-delve/delve/blob/master/Documentation/usage/dlv_debug.md), 
this approach also has disadvantages in scenarios where it is desirable to run Deep inside a container. 
This example demonstrates how to debug Deep running in docker-compose.

The make target `docker-deep-debug` compiles deep without optimizations and creates a docker 
image that runs Deep using [`dlv exec`](https://github.com/go-delve/delve/blob/master/Documentation/usage/dlv_exec.md).

1. Build the deep debug image:

```console
make docker-deep-debug
```

To distinguish the debug image from the conventional Deep image its is tagged with `deep/deep-debug`. Let’s see if the image is present:

```console
docker images | grep deep/deep-debug
```
```
deep/deep-debug                            latest                         3d6789d20dc3   2 days ago      112MB
```

2. Take a look at deep service in the [docker-compose.yaml](./docker-compose.yaml). The environment
variable `DEBUG_BLOCK` controls whether delve halts the execution of Deep until a debugger is connected.
Setting this option to `1` is helpful to debug errors during the start-up phase.

3. Now start up the stack.

```console
docker-compose up -d
```

At this point, the following containers should be running.

```console
docker-compose ps
```
```
       Name                     Command               State                            Ports                         
---------------------------------------------------------------------------------------------------------------------
debug_grafana_1      /run.sh                          Up      0.0.0.0:3000->3000/tcp,:::3000->3000/tcp               
debug_k6-tracing_1   /k6-tracing run /example-s ...   Up                                                             
debug_prometheus_1   /bin/prometheus --config.f ...   Up      0.0.0.0:9090->9090/tcp,:::9090->9090/tcp               
debug_deep_1         /entrypoint-debug.sh -conf ...   Up      0.0.0.0:14268->14268/tcp,:::14268->14268/tcp,          
                                                              0.0.0.0:2345->2345/tcp,:::2345->2345/tcp,              
                                                              0.0.0.0:3200->3200/tcp,:::3200->3200/tcp,              
                                                              0.0.0.0:4317->4317/tcp,:::4317->4317/tcp,              
                                                              0.0.0.0:4318->4318/tcp,:::4318->4318/tcp,              
                                                              0.0.0.0:9411->9411/tcp,:::9411->9411/tcp 
```

4. The deep container exposes delves debugging server at port `2345` and it is now possible to 
connect to it and debug the program. If you prefer to operate delve from the command line you can connect to it via:

```console
dlv connect localhost:2345
```

Goland users can connect with the [Go Remote](https://www.jetbrains.com/help/go/go-remote.html) run 
configuration:

![Go Remote](./goland-remote-debug.png)

5. To stop the setup use.

```console
docker-compose down -v
```
