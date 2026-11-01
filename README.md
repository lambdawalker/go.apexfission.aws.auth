# go.apexfission.aws.auth

Reusable AWS authentication feature for applications composed with [go.infra.composer](https://github.com/lambdawalker/go.infra.composer).

Extracted from [go.attestra.aws.auth at 4607807](https://github.com/lambdawalker/go.attestra.aws.auth/tree/4607807ab0346f4a38532982889fe2e56954412e). Existing email proof, passkey/password sign-in, ID capture/extraction, runtime policy and auth-only reset behavior are preserved. The application [go.attestra.aws](https://github.com/lambdawalker/go.attestra.aws) owns admin/index, environments, credentials and deployment orchestration.

**Status:** source implementation with unit/race/native mock coverage; no cloud deployment or live state migration performed. Go 1.27.3+. No release tag is implied by source availability.

## Feature contract

```go
auth := feature.Declare("authentication")
app := composer.NewApplication("my-application", runtimeProvider, auth)
err := app.Inject("authentication", feature.RuntimeRequirement, runtimeProvider.ID, "policy")
if err != nil { return err }
plan, err := app.Plan(composer.Environment{Name: "dev"})
```

Import `feature` from this module and `composer` from go.infra.composer. The application-provided offering implements `feature.RuntimeContract()`: `auth.runtime-policy/v1` with string `table_name` and `table_arn` fields. Declarations read no configuration/credentials and register no AWS resources. Missing bindings fail planning.

Offerings `identity` and `api` expose pool/client/issuer and API contracts. Other features depend on those contracts, not this feature's implementation packages. `feature.ConfigurationContract` exposes supported bootstrap/delivery properties with reader/category/sensitivity metadata; risk descriptions are separate. The full mutable onboarding schema remains in `runtimeconfig`.

## Runtime and infrastructure modules

The root module contains reusable runtime packages and 20 feature Lambda artifacts. It imports no application package and deploys no admin UI.

The optional [infrastructure module](infrastructure/README.md) exports native `Deploy`, `Validate` and `DeployStandalone`. Pass a configuration reader, artifact directory, logical environment, application-owned reset function name and deferred Pulumi runtime-table bindings. It returns typed API/pool/client/issuer/reset outputs. No unresolved Output is flattened into a deployment string.

One native authentication backend per stack is supported. Composer validates the registered backend group and its external contracts; native Pulumi retains dependency validation for private AWS resources. It does not claim every private resource is expanded into the composer graph yet.

## Build and test independently

```sh
go test -race ./...
go vet ./...
go run ./cmd/build-lambdas -out ./dist
```

The [artifact manifest](artifacts/manifest.go) is authoritative: 20 archives, 12 unique command packages, nine sign-in routes sharing one binary. Archives contain Linux/ARM64 executable `bootstrap`. Applications can build the manifest's pinned Go package paths without cloning this repository beside the application.

From `infrastructure`, run `go test -race ./...` and `go vet ./...` for native mock tests. There is no cloud account or Pulumi CLI requirement for those tests.

## Standalone deployment

Create a deliberately isolated Pulumi project using the program in `infrastructure/cmd/standalone`. Supply `appOrigin`, `senderDomain`, `senderAddress`, `artifactDir` and secret `proofKey`. Use Pulumi's interactive secret input; never put raw key values in command arguments, Git or documentation.

When runtime table bindings are omitted together, the standalone program creates its own protected, encrypted policy table; supplying both reuses an explicitly selected table. It never deploys the application dashboard/index. Complete configuration/archive preflight precedes resource registration.

After an explicitly authorized deployment, export its exact defaults and initialize that table separately:

```sh
pulumi stack output runtimeDefaults --json > runtime-defaults.json
```

From this repository's root, pass the exported table, environment and file to:

```sh
go run ./cmd/seed-runtime -table TABLE -environment ENV -defaults-file runtime-defaults.json
```

The seed command checks environment/schema, uses conditional transactional initialization and preserves existing overrides. It does not substitute freshly generated defaults: a disabled capture policy stays disabled. It makes AWS calls only when explicitly invoked; never run it during preview or documentation validation.

## Secret delivery and compatibility

The caller's existing base64 proof key is persisted as a secret version in AWS Secrets Manager. Email Lambdas receive only `PROOF_KEY_SECRET_ARN` and `PROOF_KEY_SECRET_VERSION`, plus exact-secret read permission. They retrieve the pinned key once at cold start; there is no per-invocation remote read and no plaintext `PROOF_KEY` fallback. Key bytes are not generated or rotated by the infrastructure library.

Pinned-version reading is not automatic rotation. A future key change needs an explicit Lambda rollout and an outstanding-proof/old-client compatibility plan. Existing clients, data, domains and sessions are not migrated merely by merging this code.

See [extraction design](docs/composition-design.md) and the application's [migration guide](https://github.com/lambdawalker/go.attestra.aws/blob/main/docs/migration.md). Production promotion, data transfer, aliases and live-state adoption remain separate reviewed operations.
