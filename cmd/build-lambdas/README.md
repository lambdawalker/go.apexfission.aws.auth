Build the standalone authentication feature's deployment artifacts from the feature module root:

```sh
go run ./cmd/build-lambdas -out ./dist
```

The command uses `artifacts.Lambdas()` to produce all 20 archives expected by the standalone infrastructure. Nine sign-in routes share one compilation. Each archive contains only `bootstrap`, with executable permissions, compiled for Linux/ARM64 with CGO disabled and `lambda.norpc` enabled.

Go must be on PATH. Normal Go module dependencies must be available; no application repository, sibling clone, or AWS credentials are needed. AWS and Pulumi environment variables are excluded from compiler subprocesses. Ctrl-C cancels compilation. Completed archives atomically replace existing files; on failure, previously completed archives remain in place. Pass the resulting directory as the infrastructure artifact directory.
