# Method Entry

This feature is linked to the [span features](./traces.md). Essentially this allows Deep to create snapshots or spans on
methods rather than lines, by setting the argument `method_name`.

```json
{
  "path": "some/file.py",
  "line_number": -1,
  "args": {
    "method_name": "session_created"
  }
}
```

This would essentially ignore the line number and instead create a snapshot when the method/function `session_created`
in the file `some/file.py` is called. This can be useful if the line numbers of the running code is not known, but you
know the method/function name. 
