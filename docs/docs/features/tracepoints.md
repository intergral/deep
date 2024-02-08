# Tracepoints

Tracepoints are the way Deep defines the actions it will take. A tracepoint is primarily a code location (file name and
line number), however, there can also be other configurations that control how the data is collected and any
restrictions the client should make.

For details of the structure of a tracepoint see
the [proto definition](https://github.com/intergral/deep-proto/blob/master/deepproto/proto/tracepoint/v1/tracepoint.proto).

## Tracepoint Types

There are a few different types of tracepoints supported by Deep, each with slightly different config options and use
cases.

### Line Tracepoints

A line tracepoint is the basic form of a tracepoint and allows tracepoints to be installed on a line of code. There are
a couple of options that change the behaviour of line tracepoints.

- **Line Start**: This is the default behaviour for line tracepoints and will trigger the tracepoint at the start of the
  line.
- **Line End**: This will tell Deep to collect data at the end of the line, and capture the time spent on the line as
  well as any exceptions from the execution of the line.
- **Line Capture**: This will combine the two, collecting the data at the start of the line, but also capturing the line
  execution time and exception.

### Method Tracepoints

A method tracepoint is a form of tracepoint that will wrap a method (or function) and capture the start, or end of a
method call. There are a few options for this tracepoint type:

- **Method Start**: This is the default behaviour of a method tracepoint, and will capture a snapshot at the start of a
  method. Capturing only the method parameters (or arguments) and the declaring object (this/self)
- **Method End**: This type allows the end of a method call to be captured, capturing the result of the call or
  exception to be captured
- **Method Capture**: This will combine the two, capturing the data at the start of the method, and capturing the
  returned data or exception.

## Dynamic Monitoring

Deep provides the ability to create Spans, Metrics and Logs dynamically by configuring the tracepoint appropriately.

- [Logging](./logging.md)
- [Spans](./traces.md)
- [Metrics](./metrics.md)

It is also possible to set the tracepoint to not capture any data and only create Spans, Logs or Metrics.

### Logging

All tracepoints can define a log message that will be evaluated at the point the tracepoint is executed and result in a
log message in the application logs.

### Metrics

All tracepoints can define metrics that will be evaluated at the point the tracepoint is executed and the result will be
processed by the configured metric processor.

## Span Options

It is also possible to start a span when a tracepoint is captured. The Span will have the name of the method, or
file/line number it was created from.
