# Labels

When creating a tracepoint it is possible to attach labels to the resulting data (span, metrics, log, snapshot). To
define a label simply follow these rules:

1. prefix with the type e.g. `log_label_`
2. define the label key as a static value e.g. `cluster`
3. define the value as an expression e.g. `{self.cluster.nane}` or `development`
4. put them all together `log_label_cluster={self.cluster.nane}`

When creating a tracepoint with [DeepQL](./deepql.md) you can omit the type prefix for the default type.
e.g. `log{label_cluster="name"}"` will create a log label, while `span{label_cluster="name"}` will create a span name.

It is still possible to combine them when creating logs with spans (or other combinations) e.g. these two statements are
equivalent. `log{label_cluster="name" span_label_name="span_name"}` and `log{log_label_cluster="name" span_label_name="span_name"}` 

This patten will work for any label type, below are some examples to create the label `cluster=cluster`:

```
// some values are omitted for brevity
log{... label_cluster="cluster"}

span{... label_cluster="cluster"}

snapshot{... label_cluster="cluster"}

metric{... label_cluster="cluster"}
```

{!_sections/expression.md!}