<!-- 0.1.6 START -->
# 0.1.6 (xx/xx/2023)
- **[CHANGE]**: change examples to use official grafana docker image [#58](https://github.com/intergral/deep/pull/58) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: change examples to use docker hub hosted images [#58](https://github.com/intergral/deep/pull/58) [@Umaaz](https://github.com/Umaaz)
<!-- 0.1.6 END -->

<!-- 0.1.6 START -->
# 0.1.6 (21/07/2023)
- **[BUGFIX]**: fix issue with parsing config when using kubernetes/helm [#56](https://github.com/intergral/deep/pull/56) [@Umaaz](https://github.com/Umaaz)

<!-- 0.1.6 END -->

<!-- 0.1.6 START -->
# 0.1.5 (07/08/2023)

- **[CHANGE]**: remove gotestsum usage [#47](https://github.com/intergral/deep/pull/47) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: disable metrics gen [#52](https://github.com/intergral/deep/pull/52) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: compactor - clean up trace and Tempo references [#53](https://github.com/intergral/deep/pull/53) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: frontend - clean up trace and Tempo references [#50](https://github.com/intergral/deep/pull/50) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: querier - clean up trace and Tempo references [#49](https://github.com/intergral/deep/pull/49) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: ingester - clean up trace and Tempo references [#32](https://github.com/intergral/deep/pull/32) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: tracepoint - clean up trace and Tempo references [#51](https://github.com/intergral/deep/pull/51) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: overrides - clean up trace and Tempo references [#35](https://github.com/intergral/deep/pull/35) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: storage - clean up trace and Tempo references [#47](https://github.com/intergral/deep/pull/47) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: pkg - clean up trace and Tempo references [#48](https://github.com/intergral/deep/pull/48) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: general - clean up trace and Tempo references [#54](https://github.com/intergral/deep/pull/54) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: ensure consistent naming of tenant ID, Org ID [#36](https://github.com/intergral/deep/pull/36) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: add issue templates [#29](https://github.com/intergral/deep/pull/29) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: add pull request templates [#30](https://github.com/intergral/deep/pull/30) [@Umaaz](https://github.com/Umaaz)
- **[FEATURE]**: add basic distributor metrics [#34](https://github.com/intergral/deep/pull/34) [@Umaaz](https://github.com/Umaaz)
- **[FEATURE]**: add basic deepql support [#47](https://github.com/intergral/deep/pull/47) [@Umaaz](https://github.com/Umaaz)
<!-- 0.1.5 END -->

<!-- 0.1.4 START -->
# 0.1.4 (04/07/2023)

- **[CHANGE]**: Update metrics from ingester [#25](https://github.com/intergral/deep/pull/25) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: Update tenants dashboards [#27](https://github.com/intergral/deep/pull/27) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: Remove references to tempo/traces [#13](https://github.com/intergral/deep/pull/13) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: Disabled usage reports [#28](https://github.com/intergral/deep/pull/28) [@Umaaz](https://github.com/Umaaz)
- **[ENHANCEMENT]**: Add set of dashboards for monitoring deep [#22](https://github.com/intergral/deep/pull/22) [@Umaaz](https://github.com/Umaaz)
- **[BUGFIX]**: Handle error during query, when loading bad blocks [#26](https://github.com/intergral/deep/pull/26) [@Umaaz](https://github.com/Umaaz)
- **[BUGFIX]**: Ensure minio is started in distributed docker example [#24](https://github.com/intergral/deep/pull/24) [@Umaaz](https://github.com/Umaaz)
- **[BUGFIX]**: Handle nil when querier tracepoints, when we have no tracepoints [#23](https://github.com/intergral/deep/pull/23) [@Umaaz](https://github.com/Umaaz)
<!-- 0.1.4 END -->

<!-- 0.1.3 START -->
# 0.1.3 (03/07/2023)

- **[ENHANCEMENT]**: Change distributor to support multi entry points [#20](https://github.com/intergral/deep/pull/20) [@Umaaz](https://github.com/Umaaz)
- **[BUGFIX]**: Handle possible error during tracepoint requests [#21](https://github.com/intergral/deep/pull/21) [@Umaaz](https://github.com/Umaaz)
<!-- 0.1.3 END -->

<!-- 0.1.2 START -->
# 0.1.2 (21/06/2023)

- **[CHANGE]**: Make tracepoint storage use existing services [#14](https://github.com/intergral/deep/pull/14) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: **BREAKING** Remove unnecessary config embedding [#15](https://github.com/intergral/deep/pull/15) [@Umaaz](https://github.com/Umaaz)
- **[ENHANCEMENT]**: Add perma links to docs [#9](https://github.com/intergral/deep/pull/9) [@Umaaz](https://github.com/Umaaz)
- **[ENHANCEMENT]**: Add examples for using kubernetes deployment files [#11](https://github.com/intergral/deep/pull/11) [@Umaaz](https://github.com/Umaaz)
- **[BUGFIX]**: Add exception to middleware auth [#10](https://github.com/intergral/deep/pull/10) [@Umaaz](https://github.com/Umaaz)
- **[BUGFIX]**: Fix tracepoint worker not working in single binary mode [#16](https://github.com/intergral/deep/pull/16) [@Umaaz](https://github.com/Umaaz)
- **[BUGFIX]**: Fix possible nil reference in store [#16](https://github.com/intergral/deep/pull/16) [@Umaaz](https://github.com/Umaaz)
<!-- 0.1.2 END -->

<!-- 0.1.1 START -->
# 0.1.1 (16/06/2023)

- **[FEATURE]**: change the queue/worker system to support tracepoints allowing for distributed mode deployments [#8](https://github.com/intergral/deep/pull/8) [@Umaaz](https://github.com/Umaaz)
<!-- 0.1.1 END -->

<!-- Template START
# 0.1.1 (16/06/2023)

- **[CHANGE]**: description [#PRid](https://github.com/intergral/deep/pull/8) [@user](https://github.com/)
- **[FEATURE]**: description [#PRid](https://github.com/intergral/deep/pull/) [@user](https://github.com/)
- **[ENHANCEMENT]**: description [#PRid](https://github.com/intergral/deep/pull/) [@user](https://github.com/)
- **[BUGFIX]**: description [#PRid](https://github.com/intergral/deep/pull/) [@user](https://github.com/)
Template END -->
