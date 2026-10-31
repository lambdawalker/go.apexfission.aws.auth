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

The Attestra consumer enables the versioned `attestra-v1` policy profile. Select
**Set up deployment from GitHub? [Y/n]**, review the generated documents and answer
`yes` to the separate permission approval. No policy file or role ARN is required.
The wizard creates/reuses the OIDC provider, separate `attestra-auto-<environment>`
deployment roles, content-addressed managed policy parts, and runtime permission
boundaries. Only the local SSO operator can administer those policies and roles;
GitHub cannot edit its own permissions or its runtime boundary. The role trust
names the exact repository, GitHub environment and STS audience. No permanent
AWS access keys are uploaded.

Policy JSON is saved under `.attestra/github-policies/` and displayed before any
IAM/GitHub writes. Explicit permission approval has no timeout. Saved target policy
digests skip the same review on later runs; changed permissions require approval.
Unattended enrollment requires `--yes --github-approve-policy <review-SHA256>`.
Run once without the approval digest to generate the files and see the required
digest, review them, then rerun with that exact digest. `--yes` alone cannot
approve a new permission grant. Ordinary CI deploys never enroll policies.

Each immutable managed policy fits the 6144-character AWS quota. Deployment
policies are split into managed parts, avoiding the aggregate inline-role limit.
Unchanged remote documents are reused; same-name content drift fails for manual
review. Old content-addressed policies are retained, since other runtime roles
may still use them; cleanup is a separate operator task. Automatic deployment
roles with unexpected inline or unrelated attached policies fail closed.

The review is explicit about scope: state object access is limited to the selected
bucket/identity prefix (bucket metadata/listing is readable); Lambda code and IAM
roles use a target-specific prefix and PassRole is Lambda-only. Existing table,
queue and bucket name families overlap modular environments. API Gateway,
Cognito and ACM control-plane IDs require broader selected-region permissions;
optional CloudFront operations are global within the account. These bootstrap
policies are not a claim of perfect environment isolation. State KMS keys are
resolved to exact ARNs, cryptographic access is S3-only with bucket/object context,
and no KMS key policy is changed. The existing key policy must permit IAM use.
SES sending remains restricted to the configured From address, including verified
recipient identities when the account is in the SES sandbox.

Automatic mode names and bounds runtime roles and Lambda functions. Adopting it
on an existing deployment can replace those resources. Complete a **local SSO
preview/deploy** and review replacements before dispatching Actions. The reset
Lambda retains its exact name for admin routing; enrollment refuses access to an
existing reset function until its role has the approved prefix and boundary.
If setup requests local adoption, its boundary has already been created: run
`--operation deploy --github-setup=false`, then retry `--operation setup-github`.
The wizard does not migrate state, tear down legacy infrastructure, or dispatch
an Action during setup.

Generic consumers without the supported profile retain manual setup. Advanced
Attestra overrides remain available: `--github-policy` supplies a reviewed custom
inline policy, or `--github-shared-role-arn` / `--github-backend-role-arn` reuse
compatible existing roles. Manual mode preserves unrelated role policies; its
custom file reconciles only the `attestra-deployment` inline policy. An explicit
manual override disables automatic enrollment for that invocation; previously
approved runtime naming/boundaries remain pinned for existing deployments.

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
