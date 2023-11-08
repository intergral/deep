# Logging 
Deep offers the ability to inject log messages into your running application to augment existing logging to add additional context when it is needed.

To add a log message simply attach the argument 'log_msg' to the tracepoint.

```json
{
  "path": "some/file.py",
  "line": 22,
  "args": {
    "log_msg": "This log message will be injected"
  }
}
```

The log message can also include expressions that will be interpolated and attached as watchers to the snapshot. To include an expression use curly bracers `{}`, e.g. `This log message will get local 'name' {name}`.

{!_sections/expression.md!}
