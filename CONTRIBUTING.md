# Contributing

Deep uses GitHub to manage reviews of pull requests:

- If you have a trivial fix or improvement, go ahead and create a pull request.
- If you plan to do something more involved, discuss you ideas on the relevant GitHub issue.

# Dependency Management

We use Go modules to manage dependencies on external packages. This requires a working Go environment with version 1.18
or greater and git installed.

To add or update a new dependency, use the `go get` command:

```bash
# Pick the latest tagged release.
go get example.com/some/module/pkg

# Pick a specific version
go get example.com/some/module/pkg@vX.Y.Z
```

Before submitting please run the following to verify that all dependencies and proto definitions are consistent.

```bash
make vendor-check
```

Additionally, this project uses [`gofumpt`](https://github.com/mvdan/gofumpt) to format the style of the go code. Ensure that `make fmt` has been run before
submitting.
