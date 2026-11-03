# go.apexfission.aws.auth

Reusable AWS authentication feature for applications composed with [go.infra.composer](https://github.com/lambdawalker/go.infra.composer).

Extracted from [go.attestra.aws.auth at 4607807](https://github.com/lambdawalker/go.attestra.aws.auth/tree/4607807ab0346f4a38532982889fe2e56954412e). Passkey/password sign-in, runtime policy and auth-only reset behavior are preserved. ID capture/extraction is composed as an independent feature. The application [go.attestra.aws](https://github.com/lambdawalker/go.attestra.aws) owns admin/index, environments, credentials and deployment orchestration.

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

The root module contains reusable runtime packages and 14 feature Lambda artifacts. It imports no application package and deploys no admin UI.

The optional [infrastructure module](infrastructure/README.md) exports native `Deploy`, `Validate` and `DeployStandalone`. Pass a configuration reader, artifact directory, logical environment, application-owned reset function name and deferred Pulumi runtime-table bindings. It returns typed API/pool/client/issuer/reset outputs. No unresolved Output is flattened into a deployment string.

One native authentication backend per stack is supported. Composer validates the registered backend group and its external contracts; native Pulumi retains dependency validation for private AWS resources. It does not claim every private resource is expanded into the composer graph yet.

## Build and test independently

```sh
go test -race ./...
go vet ./...
go run ./cmd/build-lambdas -out ./dist
```

The [artifact manifest](artifacts/manifest.go) is authoritative: 14 archives, 6 unique command packages, nine sign-in routes sharing one binary. Archives contain Linux/ARM64 executable `bootstrap`. Applications can build the manifest's pinned Go package paths without cloning this repository beside the application.

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

## Email verification extraction

Email proof verification has moved to [go.apexfission.aws.verifyemail](https://github.com/lambdawalker/go.apexfission.aws.verifyemail). This branch owns 14 authentication archives (including the reset worker); verification owns three and card capture owns three. Authentication's runtime module does not depend on verification. `email_integration` exports the structural `identity.email-enrollment/v1` contract: pool/client, grants table, host API, sender identity, region/account. The verification adapter uses those injected native resources and owns its proof table and secret delivery.

`DeployStandalone` now means authentication-only: it does not register /signup, /resend or /confirm. Applications that need email enrollment must compose the verification feature as demonstrated by go.attestra.aws. `Options.DeferReset` and `Deployment.ResetInventory` let an application add other feature resources and invoke `FinalizeReset` once after composition. An auth-only caller can retain the default immediate reset deployment.

This changes resource ownership and table boundaries. It does not migrate live resources, users, grants or pending proofs, and it does not rotate any keys. Use the application migration guide before adopting an existing stack.

## Card capture boundary

Card capture implementation and its three archives belong to [go.appexfission.aws.card-capture](https://github.com/lambdawalker/go.appexfission.aws.card-capture). Authentication imports only its standard-library `contracts` package for policy wire types and consent hashing. It owns no capture upload, extraction, worker, dispatch, queue, bucket or mutation code.

Use `feature.DeclareWithCapture(id)` when composing card capture. Inject `CaptureFactsRequirement` (`capture.facts/v1`, string `table_name`/`table_arn`) from the card feature's storage resource, which is provisioned before auth. The card backend consumes `capture_integration` (`identity.card-capture/v1`) after auth; that offering includes API, pool/client/issuer, region and reset-control table outputs. This ordering avoids a resource cycle. Ordinary `Declare` remains independent of capture.

Native `Options.CaptureTableName` and `CaptureTableARN` are optional together. Status receives only `GetItem` access to `ACCOUNT#*` and `CAP#*` in that table; it consistently reads hashed-owner keys and checks record ownership, then persists only the enrollment completion fact in auth's own table. An absent capture table leaves capture facts false and creates no completion evidence. Deployment defaults disable capture when no table is bound. Published policy remains revisioned and unchanged. Auth reset inventory contains only auth-owned resources; the application explicitly combines feature inventories before `FinalizeReset`.

## Offline artifact preview

From the repository root run `./preview.sh` or `.\preview.bat`. No AWS credentials, built archives or deployment are needed. Outputs default to repository-root `deployment-reports/preview/report.{md,json,html}`. Direct `go run ./cmd/preview` from infrastructure uses that process working directory instead.

Flags: `-environment preview`, `-config nonsecret.json` (JSON string map), `-output-dir DIR`. Native optional settings use their existing config names; auth additionally accepts `"suppliedRuntime":"true"`. Defaults are illustrative `https://app.example.test`, `example.test`, and `verify@example.test`. Host bindings are external fixtures.

To read an explicitly selected cached app report use `-cache FILE -project PROJECT -account ACCOUNT -backend BACKEND` with the matching environment; optional `-application`, `-region`, `-stack` further constrain identity. Unique exact feature/type/logical-name matches preserve physical identity and original evidence timestamps. Unknown and ambiguous evidence stays unknown. Cache does not perform a live check. The full selected source scope appears in the report; account identity is caller supplied.

Checked-in `docs/artifacts/` samples are deterministic illustrative defaults, with unknown presence and no deployed IDs.
