# Metrics

Deep offers the ability to dynamically create metrics at arbitrary points in the code. This allows you to create metrics
about specific actions within your applications without having to change the code.

To attach a metric
a '[Metric](https://github.com/intergral/deep-proto/blob/master/deepproto/proto/tracepoint/v1/tracepoint.proto#L45)'
must be attached to the tracepoint config.

```json
{
  "path": "some/file.py",
  "line_number": 22,
  "metrics": [
    {
      "name": "session_created",
      "labelExpressions": [
        {
          "key": "user_id",
          "value": {
            "expression": "user.id"
          }
        }
      ]
    }
  ]
}
```

The above example would create a metric called `session_created` with the labels `user_ud: 167252` (the value of the
label evaluated at runtime). The value of the labels can be either expressions (as above) or static values. The value of
the metric is optional, if not set then the value will be `1`, if set the value should be a valid expression for the
point it is executed.

# Supported providers

The specific supported providers is dependent on the client that is being used. In all cases custom providers can be set
using the plugin architecture of the client. As a generalisation the following providers will be detected and used automatically:

 - Prometheus
- OpenTelemarty (OTel)

For more info on what providers are supported by which clients see the client docs.

{!_sections/client_docs.md!}

# Example (User Session Duration)

Let us imagine a scenario where we want to track when a user session is destroyed. Let us imagine the follow code is
called when a user session is destroyed.

```py title="users/session.py" linenums="22"
def delete_session(self, user, session):
    self.process_session_end(session)
    session = self.db.delete_session(user.id)
    return True
```

In this example we can create the tracepoint as follows:

```json
{
  "path": "users/sessions.py",
  "line_number": 24,
  "metrics": [
    {
      "name": "session_duration",
      "type": "HISTOGRAM",
      "labelExpressions": [
        {
          "key": "user_id",
          "value": {
            "expression": "user.id"
          }
        }
      ],
      "expression": "time.time() - session.start_ts"
    }
  ]
}
```

This tracepoint would result in the Deep intercepting line 24 and creating a new histogram metric using the value of the
expression `time.time() - session.start_ts` as the value. This metric will then be pushed to metric provider and
captured by your existing metric services (e.g. Prometheus).

{!_sections/expression.md!}