# Native authentication infrastructure

This optional Go module preserves the native AWS backend from source commit
4607807ab0346f4a38532982889fe2e56954412e. The runtime root module does not depend on
Pulumi. No application code or stack credentials are imported here.

Use `Deploy(ctx, cfg, Options)` with a Pulumi `*config.Config` or an implementation
of `Config`. The supported configuration mechanism is this explicit reader; the
package does not discover remote configuration services or load config at import
time. `Validate(cfg, opts)` checks configuration and all 20 Lambda archives before
any native resource constructors. `Deploy` calls it automatically.

```go
backend, err := infrastructure.Deploy(ctx, config.New(ctx, ""), infrastructure.Options{
    ArtifactDir: artifactsDir,
    RuntimeTableName: sharedRuntime.Name,
    RuntimeTableARN: sharedRuntime.Arn,
    Environment: "dev",
    ResetFunctionName: "my-application-reset-dev",
})
if err != nil { return err }
ctx.Export("apiUrl", backend.APIURL)
```

`RuntimeTableName` and `RuntimeTableARN` are `pulumi.StringInput`: pass native
outputs directly so Pulumi retains resource dependencies. `Deployment` exposes
`APIURL`, `PoolID`, `ClientID`, `Issuer`, and `ResetFunctionName` as
`pulumi.StringOutput`, plus the native `API`, `Stage`, and `ResetFunction` handles.
An empty environment defaults to the stack in `Deploy`; standalone calls to
`Validate` must supply one. ResetFunctionName is required and must be a distinct
application-owned name; no legacy reset name is forced.

For independent deployment, point a new Pulumi Go project at `cmd/standalone`
and supply `artifactDir`, `appOrigin`, `senderDomain`, `senderAddress`, and secret
`proofKey`. The command calls `DeployStandalone(ctx, cfg, opts)` and does not need
an application admin/index deployment. When both runtime bindings are omitted,
it validates every backend setting and all archives, then creates an isolated
DynamoDB table with `id` partition key, `sk` sort key, on-demand billing,
encryption, point-in-time recovery and deletion protection. Supplying both
`runtimeTableName` and `runtimeTableArn` reuses an explicitly selected table;
supplying only one fails preflight. No resources are registered on preflight
failure, including the isolated table.

`DeployStandalone` returns the same `*Deployment`, adding `RuntimeTableName` and
`RuntimeTableARN` outputs and a native `RuntimeTable` handle when it creates the
table. It defaults Environment to the stack and ResetFunctionName to a sanitized,
length-bounded `apexfission-auth-reset-<project>-<stack>`. Explicit reset names are
still accepted. These standalone defaults do not change `Deploy`'s explicit reset
name contract. The command exports runtimeTableName, runtimeTableArn and
environment alongside backend outputs.

After `pulumi up` completes, seed the runtime policy record in the new table using
the root module's `cmd/seed-runtime` conditional seed command with the exported
table and environment. Review that command's flags before running it. This is a
separate, explicit AWS write after deployment: Pulumi preview and deployment never
invoke the AWS SDK to seed records. Existing revisioned policy must not be
overwritten. Until seeded, runtime policy readers fail closed. An external table
must already contain the policy or be seeded by its owner.

Optional legacy settings for API domain, Route53 zone, Android associations,
capture, concurrency and reset policy retain their behavior. Production capture
requires the quota-checked concurrency override used by the deployment tool.
Archives are obtained directly from the root `artifacts.Lambdas()` manifest.
No stack configs or deployment credentials are distributed in this module.

The existing supplied proof key is stored in Secrets Manager as a Pulumi secret
version. Only signup, resend and confirm receive `PROOF_KEY_SECRET_ARN` and
`PROOF_KEY_SECRET_VERSION` and exact-secret `GetSecretValue` permissions. The raw
key is never placed in Lambda environment variables. This does not generate or
rotate keys; outstanding proof validity and migration require separate review.

One authentication backend per Pulumi stack is supported because private logical
resource names remain stable. Composer validates the registered backend group and
its injected shared-runtime contract. It does not model every private AWS resource
inside this native backend; Pulumi preserves and validates those dependencies.
There are no global resource transformations or global mutable option hooks.

After deployment, export `pulumi stack output runtimeDefaults --json > runtime-defaults.json`. From the feature root use `go run ./cmd/seed-runtime -table TABLE -environment ENV -defaults-file runtime-defaults.json`. The seed consumes the exported policy, including disabled capture; it never invents defaults or overwrites an existing policy.

## Verification split

This branch deploys authentication only (17 Lambda archives). Proof-key and signup/resend/confirm resources are supplied by the separate verification feature. Integration outputs are exposed in Deployment.Integration; application composition defers reset, appends verification resources and calls FinalizeReset. DeployStandalone does not include email verification.
