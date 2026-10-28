# Deployment tooling

`apexfission init` creates a manifest template without overwriting files. Fill account IDs, backend, modules, build packages and environment settings. `build` needs only Go. `preview`, `deploy`, `status` and `teardown` require AWS CLI and Pulumi on PATH.

Use AWS SSO (`aws sso login --profile <profile>` plus `AWS_PROFILE`) or the ordinary AWS credential chain locally. GitHub Actions uses OIDC, then runs the same application deployment entrypoint. Pulumi owns provisioning, state locking, secrets and destruction.

Before the first operation, explicitly create the application-owned S3 state backend and Pulumi stack. Enable state bucket encryption/versioning and block public access. Never reuse Attestra's state bucket for an unrelated application.

```sh
pulumi login s3://YOUR_APPLICATION_STATE_BUCKET
cd infra
pulumi stack init dev
pulumi config set aws:region us-east-1
pulumi config set --path 'aws:allowedAccountIds[0]' 123456789012
```

Keep stack/environment names identical. The engine reads provider config and rejects missing/mismatched account restrictions, region or alternate profile/assume-role settings. Explicit custom provider code must impose equivalent account restrictions.

```sh
apexfission build --manifest apexfission.json
apexfission preview --env dev
apexfission deploy --env dev
apexfission status --env dev
```

For stronger executable/module consistency use the application's compiled deployment entrypoint, as in `examples/sales/cmd/deploy`, which checks manifest module IDs/versions against linked implementations before execution. Generic CLI cannot introspect arbitrary Go binaries. Build targets are ordinary local Go packages; compiled dependencies remain governed by go.mod/go.sum.

`preview` builds archives and invokes `pulumi preview --diff`; it does not call `up`. `deploy` previews then runs `pulumi up`; `status` reads outputs. No live operation has been executed during implementation. Current process errors deliberately avoid echoing provider output that may include secrets; rerun Pulumi directly for operator-controlled diagnostics.

Destruction requires an exact target, for example:

```sh
apexfission teardown --env prod --allow-destroy --allow-production \
  --confirm 'sales/prod/123456789012/us-east-1/prod/s3://sales-production-state'
```

That command is documentation only. Protected pools/tables remain protected; the tool does not remove protection or empty application data stores. Data-retention and removal policies belong to each application.

## Operator capabilities and boundaries

| Capability | New framework |
| --- | --- |
| Parallel Linux arm64 Lambda ZIPs | Implemented from application target list |
| Go/Pulumi build/preview/up/status/destroy | Implemented with target guards and receipts |
| AWS credentials/SSO/OIDC | Standard credential chain plus retained AWS helper |
| Vault, Cloudflare DNS, GitHub secrets, registry | Public extracted packages and tests; application orchestration opts in |
| SES identity/DKIM | Auth component exports verification values; publish DNS and verify before email tests |
| Before/after steps | Application-provided idempotent deployment steps |
| Recovery/checkpoints | Local stage receipts and Pulumi state/locks; inspect and rerun |
| Original complete guided wizard | Retained at `legacy/deploy`, coupled to original layout; not presented as generic wizard parity |
| Cross-process/cross-machine registry coordination | Retained legacy client/workflow; generic applications must integrate registry leases explicitly |

Do not execute the legacy wizard against arbitrary application resources. It still contains Attestra-specific names and inventories by design. This boundary preserves the working operator implementation while exposing reusable pieces for incremental migration.

## Installing before the first merge

Go's `go install package@commit` checks the nested module's default-branch metadata. While `main` contains only the initial README, that check can report `no matching versions for query latest`. Until the first merge/tag, install using an explicit dependency in a temporary ordinary Go module (no local replacements):

```sh
mkdir apexfission-install
cd apexfission-install
go mod init example.com/apexfission-install
go mod edit -require=github.com/lambdawalker/go.apexfission.aws.auth/deploy@v0.0.0-20261028150928-41156da59a1b
go install -mod=mod github.com/lambdawalker/go.apexfission.aws.auth/deploy/cmd/apexfission
```

That exact pinned installation and `apexfission init` were executed successfully. Use the reviewed release/commit when adopting later changes; no merge or release tag is implied by this workaround.
