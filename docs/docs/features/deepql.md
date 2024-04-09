# DeepQL

Deep offers a query language (DeepQL), to query data and create tracepoints. DeepQL is broken down into 2 main
structures
commands and search.

- **commands** are actions to perform on tracepoints and include list, delete, create (broken down into further types
  see below)
    - **list** allows you to filter for tracepoints
    - **delete** allows you to delete tracepoints
    - **create** allows you to create tracepoints, this is further seperated into tracepoint types.
        - **trigger** - a generic form to create any combined tracepoint
        - **log** - creates a log focus tracepoint
        - **span** - creates a span focus tracepoint
        - **snapshot** - creates a snapshot focus tracepoint
        - **metric** - creates a metric focus tracepoint
- **search** allows searching of collected snapshot data

## Operators

Supported operators are:

- Equals `=` value must be an exact match
- Not Equals `!=` value must not be an exact match
- Regex `=~` value must match regex expression
- Not Regex `!~` value must not match the regex expression
- Greater `>` value must be greater (number value only)
- Greater Equal `>=` value must be greater or equal (number value only)
- Less `<` value must be less (number value only)
- Less Equal `<=` value must be less or equal (number value only)

## Commands

The `list` and `delete` commands work in the same way. The values are used to find all matching tracepoints and then
either list or delete those tracepoints. The command `list{}` will show all tracepoints, and `delete{}` will delete all
tracepoints.

All value 'and' so combinations `list{id="one" id="two"}` will never find anything at the id cannot be both `one`
and `two` at the same time. However `list{id=~"one|two"}` will find tracepoints with the id `one` and `two`.

```
# list triggers
list{}

# list triggers by file
list{ file="some_file.py" }

# delete trigger by id
delete{ id="atracepointid" }

# delete triggers on file
delete{ file="some_file.py" }
```

## Generic Trigger

A generic trigger can be created which allows you full control on the actions. This can be useful when creating more
complex actions.

The defaults for a trigger are:

- fire count - always
- rate limit - 1s
- window - now - 48h
- snapshot - disabled
- span - disabled
- snapshot - disabled
- metric - disabled

```
# create snapshot and log on a line
trigger{ file="some_file.py" line=12 snapshot=true watch="self.name" log="inject a log" }
```

<details>
    <summary>Full options</summary>

### Full options

```
trigger{
 file="some_test.py"                           # the file name/path of the source file
 line=12                                       # the source line number
 method="name"                                 # the method name to look for in the file (cannot define line and method)
 
 # Optional Values
 
 # control options
 fire_count=-1                                 # the number of times to fire (-1 represents forever)
 rate_limit=1000                               # the minimum time in ms between fires
 window_end=48h                                # the relative or absolute time to stop collecting (0 means never end)
 window_start=now                              # the relative or absolute time to start collecting
 condition="{expression}"                      # conditional statement to control fire
 
 # snapshot options
 snapshot=false                                # should this log also capture a snapshot (default false)
 watch="expression"                            # define watch expressions to attach to the snapshot add multiple expressions as required
 watch="expression"
 snapshot_label_my_label="{expression}"        # snapshot labels for collected snapshot follow the same pattern as log labels
 snapshot_label_other_label="static"

 # metric options
 metric=false                                  # should this log also capture a metric (default false)
 metric_name="log_some_test_py_12"             # metric name for collected metric
 metric_label_my_label="{expression}"          # metric labels for collected metric follow the same pattern as log labels
 metric_label_other_label="static"
 
 # span options
 span=None                                     # should this log also create a span (default None)
 span_name="log_some_test_py_12"               # the name of the span
 span_label_my_label="{expression}"            # span labels for create span follw the same pattern as log labels
 span_label_other_label="static"
 
 # optional log values
 log="some injected log with {expression}"     # the injected log message with expressions
 log_label_my_label="{expression}"             # labels to attach to the log, `label_` is remove from the key to 
 log_label_other_label="static value"          # form the real key, value can be expression of static value
}
```

</details>

## Logs

Logs will create a log message without a snapshot.

The defaults for a log are:

- fire count - always
- rate limit - 1s
- window - now - 48h
- span - disabled
- snapshot - disabled
- metric - disabled

[see logging](./logging.md)

```
log{file="some_test.py" line=12 msg="some injected log with {expression}"}
```

<details>
    <summary>Full options</summary>

### Full options

```
log{
 file="some_test.py"                           # the file name/path of the source file
 line=12                                       # the source line number
 msg="some injected log with {expression}"     # the injected log message with expressions
 
 # optional log values
 label_my_label="{expression}"                 # labels to attach to the log, `label_` is remove from the key to 
 label_other_label="static value"              # form the real key, value can be expression of static value

 # Optional Values
 
 # control options
 fire_count=-1                                 # the number of times to fire (-1 represents forever)
 rate_limit=1000                               # the minimum time in ms between fires
 window_end=48h                                # the relative or absolute time to stop collecting (0 means never end)
 window_start=now                              # the relative or absolute time to start collecting
 condition="expression"                        # conditional statement to control fire
 
 # snapshot options
 snapshot=false                                # should this log also capture a snapshot (default false)
 watch="expression"                            # define watch expressions to attach to the snapshot add multiple expressions as required
 watch="expression"
 
 # metric options
 metric=false                                  # should this log also capture a metric (default false)
 metric_name="log_some_test_py_12"             # metric name for collected metric
 metric_label_my_label="{expression}"          # metric labels for collected metric follow the same pattern as log labels
 metric_label_other_label="static"
 
 # span options
 span=None                                     # should this log also create a span (default None)
 span_name="log_some_test_py_12"               # the name of the span
 span_label_my_label="{expression}"            # span labels for create span follw the same pattern as log labels
 span_label_other_label="static"
}
```

</details>

## Snapshot

A snapshot will collect the local app state, including frames, variables and watches.

The defaults for a snapshot are:

- fire count - 100
- rate limit - 10s
- window - now - 7d
- span - disabled
- log - disabled
- metric - disabled

The `target` attribute can be used to change the snapshot location into one of:

- start (default) - This will capture the data at the start of the line or method.
- capture - This will capture the data at the start of the line or method, and will also collect the exception or return
  and execution time of the line or method.
- end - This will capture the data at the end of the line, and will collector the exception or return and execution time
  of the line or method.

[see snapshot](./snapshots.md)

```promql
# line snapshot - capture data at start of line
snapshot{file="some_file.py" line=12}

# method entry snapshot - capture data at start of method
snapshot{file="some_file.py" method="my_method"}

# line capture snapshot - capture line exception and runtime
snapshot{file="some_file.py" line=12 target="capture"}

# line end snapshot - capture data at end of line and exception and runtime
snapshot{file="some_file.py" line=12 target="end"}

# method exit snapshot - capture data at end of method with return/exception and runtime
snapshot{file="some_file.py" method="my_method" target="end"}

# method entry snapshot - capture data at start of method with return/exception and runtime
snapshot{file="some_file.py" method="my_method" target="capture"}
```

<details>
    <summary>Full options</summary>

### Full options

```
snapshot{
 file="some_test.py"                           # the file name/path of the source file
 line=12                                       # the source line number (cannot define line and method)
 method="some_method"                          # the method name to look for in the file (cannot define line and method)
 
 # Optional snapshot values
 watch="expression"                            # define watch expressions to attach to the snapshot add multiple expressions as required
 watch="expression"
 
 # Optional Values
 
 # control options
 fire_count=100                                # the number of times to fire (-1 represents forever)
 rate_limit=10000                              # the minimum time in ms between fires
 window_end=7d                                 # the relative or absolute time to stop collecting (0 means never end)
 window_start=now                              # the relative or absolute time to start collecting
 condition="expression"                        # conditional statement to control fire
 
 # log options
 log="some injected log with {expression}"     # the injected log message with expressions (default null)
 log_label_my_label="{expression}"             # labels to attach to the log, `label_` is remove from the key to 
 log_label_other_label="static value"          # form the real key, value can be expression of static value
 
 # metric options
 metric=false                                  # should this log also capture a metric (default false)
 metric_name="log_some_test_py_12"             # metric name for collected metric
 metric_label_my_label="{expression}"          # metric labels for collected metric follow the same pattern as log labels
 metric_label_other_label="static"
 
 # span options
 span=None                                     # should this log also create a span (default None)
 span_name="log_some_test_py_12"               # the name of the span
 span_label_my_label="{expression}"            # span labels for create span follw the same pattern as log labels
 span_label_other_label="static"
}
```

</details>

## Metric

Metrics allow you to capture variables into metrics.

The defaults for a metric are:

- fire count - forever
- rate limit - 500ms
- window - now - 28d
- span - disabled
- log - disabled
- snapshot - disabled

[see metrics](./metrics.md)

```
# Create guage of LRU caceh size
metric{file="some_file.py" line=12 value="len(self.cache)" name="lru_cache_size" type="gauge"}

# Track calls to endpoints by users
metric{file="some_file.py line=12 name="calls_to_endpoint" label_user_id="{user.id}" label_endpoint="login" type="counter"}
metric{file="some_other_file.py line=22 name="calls_to_endpoint" label_user_id="{user.id}" label_endpoint="logout" type="counter"}

# Track function performance by user
metric{file="some_file.py" method="some_method" value="deep.target.runtime" type="histogram", label_user="{user.id}"}
```

<details>
    <summary>Full options</summary>

### Full options

```
metric{
 file="some_test.py"                           # the file name/path of the source file
 line=12                                       # the source line number
 method="some_method"                          # the method name to look for in the file (cannot define line and method)
 
 # Optional metric values
 type="counter"                                # the type of metric to create (default coutner)
 value="expression"                            # the value to use for the metric, can be expression or fixed value (default 1)
 label_my_label="{expression}"                 # metric labels for collected metric follow the same pattern as log labels
 label_other_label="static"
 
 # Optional Values
 
 # control options
 fire_count=-1                                 # the number of times to fire (-1 represents forever)
 rate_limit=500                                # the minimum time in ms between fires
 window_end=28d                                # the relative or absolute time to stop collecting (0 means never end)
 window_start=now                              # the relative or absolute time to start collecting
 condition="expression"                        # conditional statement to control fire
 
 # log options
 log="some injected log with {expression}"     # the injected log message with expressions (default null)
 log_label_my_label="{expression}"             # labels to attach to the log, `label_` is remove from the key to 
 log_label_other_label="static value"          # form the real key, value can be expression of static value

 # snapshot options
 snapshot=false                                # should this log also capture a snapshot (default false)
 watch="expression"                            # define watch expressions to attach to the snapshot add multiple expressions as required
 watch="expression"
 
 # span options
 span=None                                     # should this log also create a span (default None)
 span_name="log_some_test_py_12"               # the name of the span
 span_label_my_label="{expression}"            # span labels for create span follw the same pattern as log labels
 span_label_other_label="static"
}
```

</details>

## Span

Spans allow you to wrap code in a span to trace the behaviour.

The defaults for a spans are:

- fire count - forever
- rate limit - 100ms
- window - now - Forever
- metrics - disabled
- log - disabled
- snapshot - disabled

[see metrics](./metrics.md)
```
# Create span around function
span{file="some_file.py" method="method_name"}

# Create span around line
span{file="some_file.py" line=12}

# Create span around method with attributes
span{file="some_file.py" method="method_name" label_user="{user.id}"}
```

<details>
    <summary>Full options</summary>

### Full options

```
span{
 file="some_test.py"                           # the file name/path of the source file
 line=12                                       # the source line number
 method="some_method"                          # the method name to look for in the file (cannot define line and method)

# optional span options

name="some_test_py:12"                        # the name of the span (default to location e.g. class_name:method_name or file_name:line_no)
label_my_label="{expression}"                 # span labels for create span follw the same pattern as log labels
label_other_label="static"

# Optional Values

# control options

fire_count=-1                                 # the number of times to fire (-1 represents forever)
rate_limit=100                                # the minimum time in ms between fires
window_end=""                                 # the relative or absolute time to stop collecting (0 means never end)
window_start=now                              # the relative or absolute time to start collecting
condition="expression"                        # conditional statement to control fire

# log options

log="some injected log with {expression}"     # the injected log message with expressions (default null)
log_label_my_label="{expression}"             # labels to attach to the log, `label_` is remove from the key to
log_label_other_label="static value"          # form the real key, value can be expression of static value

# snapshot options

snapshot=false                                # should this log also capture a snapshot (default false)
watch="expression"                            # define watch expressions to attach to the snapshot add multiple expressions as required
watch="expression"

# metric options

metric=false                                  # should this log also capture a metric (default false)
metric_name="log_some_test_py_12"             # metric name for collected metric
metric_label_my_label="{expression}"          # metric labels for collected metric follow the same pattern as log labels
metric_label_other_label="static"
}

```

</details>

## Search

Search allows for searching snapshot attribute and resource values.

!!! Information
    You can currently not search on variable values, frame data or watches.

## Basic query statements

```
# find snapshots by trigger (tracepoint)
{ tracepoint="atracepointid" }

# find by service name
{ service.name="myApp" }

# find by resource values
{ rs.app_version="14" }

# find by attribute values
{ file="some_file.py" }

# find with wildcard
{ file=~"*.py" }
```
