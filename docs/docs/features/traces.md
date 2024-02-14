# Traces / Spans

Deep offers the ability to dynamically create Spans at arbitrary points in the code. This allows you to augment your
exising monitoring with specific Spans when you need additional data while diagnosing an issue.

To create a Span the `span` argument should be attached to the tracepoint. This argument can have the values of:

- `method` - This will create a span around the method the tracepoint is located in
- `line` - This will create a span around the line the tracepoint is on

Additionally, the argument `method_name` can be set (to the name of the method) to crate a span around any method in the
file with that name. If `method_name` is set then the `line_number` property is ignored for the span. This method also
allows for '[method entry tracepoints](./method_entry.md)'  

```json
{
  "path": "some/file.py",
  "line_number": 22,
  "args": {
    "span": "method"
  }
}
```

# Supported providers

The specific supported providers is dependent on the client that is being used. In all cases custom providers can be set
using the plugin architecture of the client. As a generalisation the following providers will be detected and used automatically:

- OpenTelemarty (OTel)

For more info on what providers are supported by which clients see the client docs.

{!_sections/client_docs.md!}
