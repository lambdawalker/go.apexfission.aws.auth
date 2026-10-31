# Automatic GitHub deployment setup

The operator approved replacing manual IAM preparation with a wizard that generates
project-specific policies, shows them for approval, and reconciles AWS OIDC roles
and GitHub environments. Reuse saved answers; no live deployment is part of this
implementation. Keep custom policies/existing roles as advanced overrides.

## Approach

Use a versioned policy profile for the shipped Attestra shared/backend Pulumi
programs, explicitly enabled by the consumer. Generic consumers without that
profile keep manual authorization. Inferring arbitrary IAM from Go source would
be unreliable; an administrator grant would be unnecessarily broad.

Generate deployment managed-policy parts and one runtime boundary per target.
Each part stays below the 6144-character IAM quota; no generated inline policy
uses the aggregate 10240-character role limit.
Derive collision-resistant runtime IAM role and Lambda function prefixes from repository, account,
region, stack. The boundary is an immutable content-addressed managed policy:
changes create another policy rather than expanding existing runtime roles.
Only the authenticated local operator manages boundaries/OIDC/deployment roles;
the GitHub role cannot mutate those identities or the boundary. Permit role
creation only with the expected boundary; PassRole only for the generated runtime
prefix to Lambda. The Pulumi helpers apply role/function prefixes and the boundary in automatic
mode; manual mode preserves existing naming/permissions. Switching an existing
stack to automatic mode can replace runtime IAM roles and functions: preview/explicit destructive
approval must expose that; no automatic state migration or teardown. The exact reset function name is kept
for admin routing; setup refuses to attach GitHub permissions to an existing reset
function until a local SSO deployment has applied the approved prefix/boundary.

Deployment policies enumerate service operations. Scope account/region/resource
families where current physical naming supports it, state bucket/prefix, selected
SES identity, domains and optional Route53 zone. Current Pulumi auto-generated
names and ID-addressed AWS APIs prevent perfect environment isolation across all
services. Review text must disclose regional API Gateway/Cognito/ACM and global
CloudFront creation/access and resource-family overlap, not claim exact isolation.
No wildcard IAM/STS administrator permissions. Runtime boundary includes the
existing application/reset/log/SES/Bedrock operations but no IAM or state bucket
access. State KMS encryption gets exact key ARNs discovered from bucket defaults,
restricted through S3; no wildcard KMS grants or automatic KMS key-policy edits.

## Approval and persistence

The ordinary question becomes 'Set up deployment from GitHub? [Y/n]'. With the
profile and no advanced overrides, generate canonical JSON and SHA256. Show target,
policy file paths, boundaries, notable scope limitations and the complete JSON
before explicit approval (no five-second timer for permission grants). Save the
approved digest and auto mode; reruns with unchanged desired inputs skip review
but verify remote managed policies/trust/settings for drift. A policy change
requires fresh approval. Unattended setup requires both --yes and the exact
--github-approve-policy digest; --yes alone never approves a new policy. Settings
saved for CI contain auto mode/boundary configuration but no setup approval token.

## Verification

Tests cover scope changes, bad identifiers, no secret material in fingerprints,
policy limits, forbidden IAM actions, mandatory boundaries, constrained PassRole,
no-write on rejected/stale approvals, drift reconciliation, and saved answer reuse.
Pulumi mocks verify auto roles have bounded names/boundaries while manual roles
remain unchanged. Test native/race/vet and Windows build; verify actual consumer
pins in all three Go modules and workflow snapshot behavior. No live IAM changes.
