<!-- main START -->
# main (unreleased)
- **[BUGFIX]**: fix missing data in frame response [#89](https://github.com/intergral/deep/pull/89) [@Umaaz](https://github.com/Umaaz)
<!-- main START -->

<!-- main START -->
# 1.0.5 (08/02/2024)
- **[ENHANCEMENT]**: Update deep proto version to support new properties on tracepoints [#78](https://github.com/intergral/deep/pull/78) [@Umaaz](https://github.com/Umaaz) 
- **[ENHANCEMENT]**: change builds to use goreleaser [#79](https://github.com/intergral/deep/pull/79) [@Umaaz](https://github.com/Umaaz)
<!-- main START -->

<!-- 1.0.4 START -->
# 1.0.4 (04/12/2023)
- **[BUGFIX]**: compactor loop was not running [#76](https://github.com/intergral/deep/pull/76) [@Umaaz](https://github.com/Umaaz)
<!-- 1.0.4 END -->

<!-- 1.0.3 START -->
# 1.0.3 (30/11/2023)
- **[CHANGE]**: add/improve monitoring for retention [#75](https://github.com/intergral/deep/pull/75) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: add docs for run books [#74](https://github.com/intergral/deep/pull/74) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: unify metric namespaces and subsystems [#73](https://github.com/intergral/deep/pull/73) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: unify span tags for tenant [#70](https://github.com/intergral/deep/pull/70) [@Umaaz](https://github.com/Umaaz)
- **[BUGFIX]**: fix port in local docker example [#72](https://github.com/intergral/deep/pull/72) [@Umaaz](https://github.com/Umaaz)
<!-- 1.0.3 END -->

<!-- 1.0.2 START -->
# 1.0.2 (06/11/2023)
- **[CHANGE]**: add log message support in storage engine [#66](https://github.com/intergral/deep/pull/66) [@Umaaz](https://github.com/Umaaz)
- **[BUGFIX]**: tracepoint not removing from ring [#64](https://github.com/intergral/deep/pull/64) [@Umaaz](https://github.com/Umaaz)
<!-- 1.0.2 END -->

<!-- 1.0.1 START -->
# 1.0.1 (09/10/2023)
- **[BUGFIX]**: querier - queries would not select the correct metas [#63](https://github.com/intergral/deep/pull/63) [@Umaaz](https://github.com/Umaaz)

<!-- 1.0.1 END -->

<!-- 1.0.0 START -->
# 1.0.0 (07/09/2023)
- **[CHANGE]**: fix change log order  [#59](https://github.com/intergral/deep/pull/59) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: add fmt check and update CONTRIBUTING.md  [#60](https://github.com/intergral/deep/pull/60) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: fix formatting issues [#61](https://github.com/intergral/deep/pull/61) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: change examples to use official grafana docker image [#58](https://github.com/intergral/deep/pull/58) [@Umaaz](https://github.com/Umaaz)
- **[CHANGE]**: change examples to use docker hub hosted images [#58](https://github.com/intergral/deep/pull/58) [@Umaaz](https://github.com/Umaaz)
<!-- 1.0.0 END -->

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
