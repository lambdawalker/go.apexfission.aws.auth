# GitHub deployment setup

The deployment wizard invokes this package for explicit setup requests. It uses
an existing `gh auth login` session; it does not save GitHub access tokens.
Run `gh auth status` and ensure your selected account can administer repository
environments, variables and secrets. Missing permissions produce an incomplete
setup error with rerun guidance. Setup never dispatches a deployment.

Environment names match the deployment identity: `next-shared` for admin/index,
`next-dev` for a dev backend, and `final-*` for an explicit empty prefix. Admin/index
and backend have separate scoped OIDC roles and environment settings. The manual
Action routes All through admin/index once, then the selected backend. It reads
reviewed settings rather than silently replacing options with defaults.

New environments receive a main-only branch restriction. Existing reviewers and
other protections remain unchanged. Existing environments that are unrestricted
or permit additional branches/tags stop for manual review in GitHub Settings >
Environments; the setup does not weaken or overwrite their protections.

Setup reuses the GitHub OIDC provider and a role whose trust is exactly scoped to
this repository and environment. For new roles, supply a reviewed least-privilege
policy JSON with `--github-policy`; no AdministratorAccess policy is generated.
Review the Pulumi preview/service inventory when preparing permissions, including
state S3 access, narrowly scoped IAM/PassRole, the managed AWS services and registry
publication. A deployment policy is account-specific and is not inferred from a
wildcard administrator grant. Existing compatible role permissions are preserved;
an optional policy file reconciles only the `attestra-deployment` inline policy.

`DEPLOY_CONFIG_SHA256` is a SHA-256 canonical manifest of managed public settings,
workflow content, exact role reference, main-branch restriction, required secret
names and a nonsecret secret-version identifier. The marker excludes itself and
secret plaintext. Every run checks readable fields, branch restrictions, OIDC
trust and secret-name inventory; matching hashes never prove secret values.
Local delivery receipts are encrypted with a domain-separated key derived from
the unlocked vault. Keyed secret fingerprints remain inside that encrypted receipt;
only an opaque random rotation identifier contributes to the public marker.
Changing a local token automatically invalidates its receipt and redelivers it.
The receipt commits only after verified remote completion. Missing receipts or a
vault-passphrase rotation cause safe redelivery. Setup without an unlocked vault
key conservatively redelivers secrets each run rather than claiming equality.
`--github-secret-version` and explicit secret-redelivery flags remain available
for deliberate rotations and recovery.
Only encrypted sealed-box values are sent to GitHub's secret API. The marker is
written last after rereading managed values and required inventory. No unrelated
variable/secret/policy is deleted. A distributed S3 identity lock serializes setup
across checkouts; external manual administrators remain outside that lock.

The checked-out manual and reusable workflow files are fingerprinted. Merge the
workflow changes before running them from Actions, since GitHub reads manual
workflows from the default branch. Local setup can be run from the reviewed task
branch before merge. Changing GitHub setup is distinct from deploying AWS.
