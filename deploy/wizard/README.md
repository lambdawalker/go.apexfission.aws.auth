# Reusable deployment wizard

Consumer entrypoint:

```go
err := wizard.Run(ctx, wizard.Options{
    Root: root, Input: os.Stdin, Output: os.Stdout, Args: os.Args[1:],
    SharedBuilds: sharedTargets, BackendBuilds: backendTargets,
})
```

Import `github.com/lambdawalker/go.apexfission.aws.auth/deploy/wizard`. The consumer supplies `infra-index/Pulumi.yaml` declaring `attestra-shared`, `infra/Pulumi.yaml` declaring `attestra-backend`, thin Go infrastructure mains, runtime build targets, and `cmd/admin-setup`. Build target ZIPs go into `dist/`; the builder makes ARM64 provided.al2023 archives before Pulumi preview/update. `sourceHash` hashes shared build artifacts.

`Options.Runner` accepts the existing `deploy.Runner`. A custom runner must additionally implement `wizard.StdinRunner` for new proof keys. `Build`, `HTTP`, `DNS`, and `Registry` are optional test/integration seams. Production uses local AWS CLI/Pulumi, the existing Cloudflare client, and the existing SigV4 registry client. No additional executable is downloaded by this package.

## Inputs

With no arguments, a line-oriented wizard asks for the target, scope, operation, account, region, existing S3 state base, domains/sender, optional administrator, AWS profile, Route53 zone, Android associations, and environment class. Production also asks capacity values. Prefix defaults to `next`, logical environment to `dev`, region to `us-east-2`, base domain to `attestra.com`, and operation to `preview`. Enter `-` at the prefix prompt to choose the separate empty-prefix identity. Terminal-only secret prompts create/unlock an encrypted local credential vault and resolve the Pulumi passphrase and Cloudflare token. AWS SSO credentials remain managed by the AWS CLI.

Flags override saved nonsecret choices:

| Flags | Meaning |
| --- | --- |
| `--prefix`, `--environment` | Prefix (explicit `--prefix=` selects final identity); logical environment |
| `--scope` | `both`, `shared`, or `backend`; shared runs first |
| `--operation` | `preview`, `deploy`, `status`, `health`, or `publish` (resume saved post-apply prerequisites/publication) |
| `--yes` | Required explicit deployment approval for noninteractive deploy; never saved |
| `--account`, `--region` | Required 12-digit account; AWS region |
| `--state-backend` | Existing dedicated `s3://bucket/base`; identity suffix appended automatically |
| `--base-domain` | Base hostname, e.g. `attestra.com` |
| `--sender-domain`, `--sender-address` | External verified SES domain and sender; backend deploy prerequisites |
| `--admin-email` | Explicit bootstrap email for selected backend environment; omitted means no invitation/grant |
| `--profile` | Existing AWS CLI profile/session; removes ambient static/web identity credential overrides |
| `--route53-zone-id` | Pulumi-managed ACM DNS validation and hostname records |
| `--cloudflare-zone-id` | Optional zone restriction; otherwise authorized longest matching zone selected |
| `--index-domain`, `--admin-domain`, `--api-domain`, `--app-origin` | Explicit overrides; distinct hostnames, selected prefix label required when prefix nonempty |
| `--index-certificate-arn`, `--admin-certificate-arn`, `--api-certificate-arn`, `--app-certificate-arn` | Issued external certificates for external DNS mode; app certificate is in us-east-1 |
| `--android-associations` | Existing infrastructure JSON association schema; omitted preserves config; `[]` explicitly hosts empty/revoked associations |
| `--capture-enabled` | Initial server runtime default, false by default |
| `--capture-reserved-concurrency` | Per-function reservation, default -1 (shared test capacity) |
| `--capture-worker-max-concurrency` | Worker maximum, default 2 |
| `--environment-class` | Optional `test` or `production` |

Interactive AWS profiles use IAM Identity Center SSO; expired sessions stream device-code/no-browser login instructions and are reverified. CI uses OIDC/explicit environment credentials and never initiates interactive SSO. No AWS access keys or SSO sessions are copied into the new operator vault. CI supplies `PULUMI_CONFIG_PASSPHRASE` or `PULUMI_CONFIG_PASSPHRASE_FILE`; Cloudflare DNS uses `CLOUDFLARE_API_TOKEN`. Nonterminal execution never prompts for secrets. Custom `AWS_ENDPOINT_URL*` variables are rejected; configured AWS endpoint overrides are disabled. Commercial AWS ARN/endpoint forms are supported.

Choices and immutable per-identity account/region/backend bindings are saved atomically as private `.attestra/wizard.json`. Existing bindings are checked early; a new binding is committed only after target verification and explicit authorization to mutate that identity. Failed first verification does not reserve a prefix. A failed preview never saves its requested choices. Secret values never appear in this JSON. `proofKey` is cryptographically generated once, sent to `pulumi config set --secret` over stdin, and preserved on subsequent runs; only Pulumi's encrypted stack config stores it. Protect your Pulumi passphrase separately. Existing plaintext proof keys are rejected. Selecting an existing deployed stack first invokes `pulumi config refresh --force` to recover the encrypted configuration and secrets-provider metadata from the latest deployment. This may write the local ignored Pulumi YAML (and CLI backup), including during preview/status/health, but does not change cloud configuration or issue `config set`. Optional settings absent from wizard flags are preserved. A never-deployed/partially initialized stack can retain valid target-bound local encrypted config; an existing empty stack without authoritative remote config or valid local YAML fails closed, rather than generating a replacement proof key. Recover the original config/checkout or inspect and explicitly remove an unused stack before retrying.

## Isolated targets

`next` selects projects/stacks `attestra-shared/next-shared` and `attestra-backend/next-dev`; empty prefix selects `final-shared` and `final-dev` in those new projects. Literal prefix `final` is rejected. Prefixes are lowercase alphanumeric, start with a letter, and are at most 20 characters; logical environments are DNS-safe, cannot be `shared`, and complete backend stack names are at most 32 characters. Neither selection adopts legacy projects/state.

An entered `s3://dedicated/base` becomes `s3://dedicated/base/attestra/next` or `/final`. The bucket must already exist in the selected account, have versioning enabled, and block all public access. The wizard never creates a bucket. STS identity and existing Pulumi provider account/region pins are verified. Shared output identity/stack/account/region and exact runtime table ARN/name agreement are checked before backend wiring; no table override flags exist.

Default domains are `next.index.attestra.com`, `next.admin.attestra.com`, `next.api.dev.attestra.com`, and app origin `https://next.dev.attestra.com`. The environment index is `/v1/environments`. SES sender domains are not prefixed. Backend links/WebAuthn origin and Android host use the selected app origin. App association hosting requires `--android-associations`; absence is reported explicitly. Associations are optional, and omission does not promise working Android app links or native passkey association.

## First deployment and DNS

First preview cannot create/select a nonexistent stack and gives the explicit deploy prerequisite. Preview selects existing stacks, restores local encrypted configuration, validates pins/outputs, builds, and checks requested resource-driving configuration against the authoritative stack config. Pending flag/saved-choice differences (capture, sender, domains, associations, capacity, TLS, etc.) are explicitly rejected with the differing keys before that stack preview; choices are not saved. Restore matching choices or choose an explicit approved deploy, which previews the desired config before applying. Code-only changes remain previewable; shared `sourceHash` is export-only provenance and is excluded from this rejection. Preview never initializes stacks, runs `config set`, changes cloud configuration, requests certificates, publishes DNS/registry, or runs admin setup. Status/health only read verified outputs and perform HTTPS probes.

An explicit approved deploy initializes missing stacks, configures new-project namespaced settings and account/region provider pins, previews, then applies. There is no teardown path. Production (`prod`, `production`, or class `production`) requires positive explicitly chosen capture reservation/worker capacity. Existing reservations and account headroom are inspected before a both-scope shared update, then checked again close to backend update; production never falls back to shared capacity.

For Cloudflare, provide a zone-scoped token with Zone Read and DNS Edit. The wizard requests external ACM DNS certificates with deterministic idempotency tokens, writes ownership/pending receipts under the identity's S3 state prefix, publishes only validation CNAMEs, and waits at most 15 minutes for issuance. API/index/admin certificates are regional; app association certificates use us-east-1. Issued ARNs are supplied to Pulumi, so these certificates have one external owner and are not adopted by two stacks. The wizard never deletes certificates. After the update it publishes DNS-only, unproxied hostname CNAMEs to stack output targets. Conflicting existing records are not overwritten. No SES/DKIM/email DNS is created, changed, or removed.

Pending ACM requests retain their receipts on failure. Retries reuse recorded ARNs. A pending request without an ARN older than 50 minutes stops for manual ACM/tag/receipt reconciliation, avoiding creation after AWS's request-token idempotency window. Error output identifies the certificate/receipt to inspect. For Route53, Pulumi owns certificate validation and hostname records. For externally preissued certificates without a Cloudflare token, the wizard verifies ARN account/region, hostname coverage, and ISSUED status, then prints required hostname CNAMEs after apply and returns an actionable incomplete-DNS error. Manual publication alone does not mark wizard completion: this mode continues to return incomplete on retry without provider credentials. Supply Cloudflare credentials (or a consumer DNS callback) to reconcile DNS and complete the wizard; infrastructure remains in place.

Shared admin MFA/passkey authentication is reconciled with `go run ./cmd/admin-setup -auth-only`. No shared-phase invitation occurs. After successful backend setup, an explicit administrator email invokes the same consumer command for the full backend stack's grant. Existing bootstrap guards preserve prior grants.

## Locks, publication, outputs, and recovery

A conditional S3 identity-wide lock covers shared/backend config and registry coordination across all environments and checkouts; Pulumi's update lock alone is insufficient. Lock creation uses `If-None-Match: *`, release uses its returned ETag, and stale locks fail closed. Inspect the S3 lock, active deployments, and pending receipts before manually removing a stale lock. The wizard does not steal locks.

The registry's raw regional API Gateway endpoint is validated before any credential export or request. Selected AWS CLI session credentials sign requests through `deploy/registry`. A durable S3 receipt records the token before registry acquisition, records its revision before backend update, then records a versioned post-apply recovery journal containing the verified shared/backend outputs and original nonsecret choices. Backend hostname DNS, the requested administrator bootstrap, and the current checkout Android export must complete before runtime defaults/public configuration are advertised. DNS/admin completion is durably checkpointed; retries reuse the same acquired token/revision. A failed DNS/bootstrap/export/publication step is resumed with `--operation publish` and the same saved choices, or by retrying deploy with the saved prerequisites. Post-apply/ready/defaults recovery does not repeat Pulumi updates; it completes only unfinished approved prerequisites and recreates the public Android export in the current checkout before publication/success. Recovery rejects changed deployment choices. Old receipts without the journal fail closed for manual inspection. Receipts are private and versioned in the existing state bucket; no plain credentials/proof keys are included.

Public Android properties are exported to `.attestra/<full-backend-stack>-android.properties`, retaining original `attestraEnvironment`, `attestraApiBaseUrl`, `attestraLinkHost`, `attestraAwsRegion`, `attestraCognitoUserPoolId`, `attestraCognitoClientId`, `attestraCaptureEnabled`, and `attestraCaptureDocumentType` keys, plus the index URL and uppercase aliases. Use that file as the Android build's public configuration input.

Health checks verify the auth-status sign-in-required response, public index schema and backend entry/config hash, association JSON/content when present, and the admin page/expected login redirect. They do not create users, send mail, upload capture documents, or prove complete end-user sign-in. Missing optional app association hosting is reported as a warning; configured-but-missing hosting fails health. No live AWS/Pulumi deployment is performed by tests.

### Import existing operator values

Run `./export-wizard.sh --environment dev` (Windows: `export-wizard.bat --environment dev`)
in the original `go.attestra.aws.auth` checkout first. It writes nonsecret portable
preferences to `~/.attestra/legacy-wizard.json`, grouped by logical environment.
The new wizard reads that file automatically after choosing the environment.
Imported questions are skipped. Explicit flags take priority, followed by this
checkout's saved choices, the matching exported environment, and built-in defaults.
Prefix, scope and operation remain explicit wizard questions; legacy hostnames,
certificate ARNs, proof keys, state bindings and vault credentials are never imported.
Missing values still prompt. Corrupt/unsupported exports fail with instructions to
re-export. AWS account/region/state verification still applies to imported values.

Operator settings are saved per prefix, logical environment and scope, while the
Version 1 `Choices` field remains available to existing CI settings consumers.
Choose scope and environment first in interactive mode. `admin-index` and `all`
are aliases for `shared` and `both`. Saved explicit empty strings are retained.
Use `--reconfigure` to ask again, or `--operation reconfigure` to save choices
without cloud access. Flags override saved choices; absent local fields can use
`ATTESTRA_<FLAG_NAME>` variables (hyphens become underscores), then legacy imports
and defaults. Select `--profile ""` in CI to clear a saved interactive profile.
A nonempty profile deliberately excludes ambient AWS credentials; terminal users
can renew an expired SSO session. CI never runs `aws sso login`.

`--operation build` builds selected targets without AWS or Pulumi. The options
`--build-jobs`, `--force-rebuild`, and `--clean-cache` control verified build reuse.

Terminal secrets use the existing encrypted credential vault, scoped to the
consumer root and prefix. The Pulumi passphrase is shared across environments of
a prefix because they share the shared stack. Existing provider credentials are
preserved; AWS credentials are never copied into the vault. Environment Pulumi
and Cloudflare credentials take precedence. Vault creation and Pulumi passphrase
creation require hidden confirmation; corrupt or wrong-password vaults fail
without replacement. `--operation rotate-secrets` explicitly rotates only the
vault encryption passphrase, retaining provider credentials and Pulumi encryption.
Use `--replace-cloudflare-token` to replace the encrypted DNS token through a
hidden prompt. An ambient Cloudflare token still takes precedence for this run.

After each successful deploy preview, the saved Pulumi plan is applied with
`--plan`. A real stdin terminal proceeds after a visible five-second countdown; Yes/Enter proceeds immediately; No/Escape/Ctrl-C, EOF or
context cancellation aborts. Unattended deployment requires `--yes`. Previews
containing delete, replace or destroy require a separate terminal phrase even
with `--yes`. Embedders can provide `Options.ConfirmPreview` to own approval.
The wizard verifies state bucket owner, selected region, versioning, default
AES256/KMS encryption and all public access blocks before using it.

Reconfiguration preserves explicit flags, including explicit empty strings and
production capacity flags. Shared scope never asks backend capacity questions;
valid saved backend production capacity is reused unless reconfigured.

Interactive SSO renewal streams device authorization instructions to the terminal
using `InteractiveRunner`, with browser-independent device-code login. Only an
expired-token diagnostic (`ErrSSOSessionExpired` for custom runners) on a
configured SSO profile triggers renewal; other STS failures stop. STS is verified
again after login. The subprocess honours context cancellation. Countdown and
explicit destructive-phrase reads both use cancellable readers and join their
read worker before returning. `Options.ApprovalReader` and `ApprovalTicker` allow
embedders/tests to exercise the actual approval logic with controlled input/time.

## GitHub setup and deployment efficiency

`--operation setup-github --scope all --github-repository owner/repo --yes`
configures selected GitHub environments without deploying. Supply
`--github-policy /path/to/reviewed-policy.json` for new OIDC deployment roles,
or compatible `--github-shared-role-arn` / `--github-backend-role-arn` references.
Interactive deploy offers setup once; `--github-setup` enables it explicitly.
Authenticate `gh` first. The consumer must include `.github/workflows/deploy.yml`
and `deploy-target.yml`; setup fingerprints both, verifies the exact repository
and operator identity, preserves unrelated settings/protections, and uses the
existing S3 identity lock. Incompatible branch restrictions or trust fail with
manual recovery guidance. No Action is dispatched automatically.

See [GitHub setup](../githubsetup/README.md) for managed settings, permissions,
encrypted secret-delivery metadata, force/rotation flags and ownership boundaries.
The five-second local confirmation never runs in GitHub Actions. Actions must use
explicit noninteractive approval and OIDC, with scope/environment settings restored
from the matching protected environment rather than invented defaults.

The default builder uses content-addressed verified artifacts from `.lambda-cache`.
Use `--build-jobs`, `--force-rebuild` and `--clean-cache` for bounded parallelism
and explicit cache controls. Force/clean flags are invocation-only. Independent
packages build concurrently; duplicate entrypoints share compiled archives.
Dependency, asset, module/workspace, toolchain and build-option changes invalidate
the cache; missing/corrupt entries rebuild. Unsupported `-toolexec` wrappers fail
rather than silently accepting an incomplete fingerprint. Local cache locks fail
closed and never steal another build's lock. Pulumi still plans infrastructure
normally and uses actual base64 ZIP hashes for Lambda code changes.
