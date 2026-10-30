# Apexfission

A statically composed Go framework for AWS Lambda applications, with authentication, protected HTTP APIs, an extensible administration shell, Pulumi resources and Go deployment tooling.

This is an initial implementation for review, **not a production-certified release**. Local tests, actual Lambda builds and Pulumi mocks are distinct from a real preview or deployment. See [verification](docs/verification.md) and [migration](docs/migration.md).

## Packages

| Go module | Purpose |
| --- | --- |
| `github.com/lambdawalker/go.apexfission.aws.auth` | Runtime module composition, authentication, authorization, routes, admin, configuration and optional compatibility packages |
| `…/pulumi` | Composable AWS resources in the consuming application's stack |
| `…/deploy` | Manifest validation, parallel Lambda builds and guarded Pulumi orchestration; reusable operator clients |
| `…/legacy/deploy` | Original Attestra operator workflow retained for compatibility, not generic deployment |

Runtime imports never require Pulumi. Go dependencies provide executable implementations; JSON manifests only identify already linked modules and build/stack settings.

## Complete Attestra-compatible deployments

The optional pulumi/backend and pulumi/shared packages declare the full backend
and shared administrator/index infrastructure. The deploy/wizard package provides
scoped first deployment, updates and recovery from consumer-owned entrypoints.
These preserve the original Attestra contracts and use new isolated projects;
they do not adopt existing state. Generic components remain available for smaller
applications. See [migration](docs/migration.md).

## Try the sales consumer

Requires Go 1.26.6. From `examples/sales`:

```sh
go test ./...
go run ./cmd/deploy -op build
```

This builds `dist/application.zip` and `dist/challenge.zip` for Linux arm64. No AWS credentials are needed to build. The separate `examples/sales/infra` module composes customer authentication, an invitation-only administrator pool, application-owned orders, scoped Lambda roles, two HTTP APIs and protected config/proof tables.

See [integration](docs/integration.md), [deployment](docs/deployment.md), [security](docs/security.md), and [sales setup](examples/sales/README.md).

## Installation

Until reviewed releases are tagged, use the reviewed commit SHA. Before the first merge, nested CLI installation needs the [pinned-module workaround](docs/deployment.md#installing-before-the-first-merge); the ordinary commands below apply once the module exists on the default branch:

```sh
go get github.com/lambdawalker/go.apexfission.aws.auth@<reviewed-commit>
# In your separate infrastructure module:
go get github.com/lambdawalker/go.apexfission.aws.auth/pulumi@<reviewed-commit>
# Operator CLI:
go install github.com/lambdawalker/go.apexfission.aws.auth/deploy/cmd/apexfission@<reviewed-commit>
```

Nested modules are released with their own tags (`pulumi/v0.1.0`, `deploy/v0.1.0`); the root uses `v0.1.0`. No tags or package publication are performed by this PR. The repository uses `go.work` for development. Distributable nested modules pin a real remote root commit and contain no local replaces. The example additionally uses local `replace` directives for standalone development; production consumers pin released modules or reviewed commits and remove those replaces.

## Verify

Run `go test -race ./...` and `go vet ./...` in each of: root, `pulumi`, `deploy`, `legacy/deploy`, `examples/sales`, and `examples/sales/infra`. CI runs the same module matrix and builds the sales Lambda archives.

Lambda cache builds explicitly disable Go PGO (`-pgo=off`) for both dependency inspection and compilation. The fingerprint includes that build recipe. Automatic `default.pgo` and explicit `GOFLAGS=-pgo=...` profiles do not affect these Lambda builds; profile bytes are therefore not cache inputs.

The wizard lists saved and imported logical environments in sorted order, with numbered selection or a custom name. Shared GitHub snapshots retain their reviewed environment when a workflow selects a different backend environment. Post-apply recovery compares resource and prerequisite settings while allowing build controls, GitHub setup settings and local profiles to change. Pulumi previews refresh provider state before reviewing changes.
