# Automatic GitHub Setup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** One guided authorization step configures GitHub deployment without a hand-written policy.
**Architecture:** Versioned Attestra profile generates canonical per-target deployment and runtime boundary policies; wizard owns approval and persists automatic mode; existing reconciler owns drift checks and mutation. Pulumi role helper receives exact boundary/prefix config.
**Tech Stack:** Go, AWS CLI/SSO/IAM/OIDC, Pulumi Go, GitHub CLI.
**Spec:** ../specs/2026-10-30-automatic-github-setup-design.md

## Global Constraints

- No live cloud mutation while implementing; push paired reviewable PRs.
- Existing manual mode remains compatible; automatic mode explicitly enabled by consumer.
- No AdministratorAccess, generated role/boundary self-administration, wildcard KMS access, or implicit approval of changed permissions.
- State inputs and AWS account verified before setup; unchanged desired state still reads remote policy/trust for drift.

## Review Focus

- Generated inline policy <=10240 chars and boundary <=6144 chars.
- Partial failure/retry and old saved GitHubAsked settings must reach auto mode without repeating known input.
- Shared snapshot must not depend on selected backend env; backend isolation must not reuse another env's digest.
- Region/account/repository/zone/KMS changes invalidate approval; CI never enrolls roles.
- Existing runtime IAM role replacement visible in preview; manual resources remain unchanged.

### Task 1: Policy generation and reconciliation

Files: deploy/githubsetup/generated.go, generated_test.go, setup.go, iam.go.
Interface: Generate(PolicyInput) (GeneratedPolicy,error); Target.Generated *GeneratedPolicy.
- [x] Write failing policy tests: shared excludes capture buckets; backend includes state, capture, bounded PassRole; invalid account/region/prefix/key rejected; canonical bytes stable and quota respected.
- [x] Generate enumerated per-target policies and immutable content-addressed boundary.
- [x] Add boundary create/reuse validation and per-target inline policy reconciliation; reject drift in immutable boundary; do not mutate foreign roles.
- [x] Run deployment module tests and commit.

### Task 2: Bounded Pulumi runtime roles

Files: pulumi/internal/guard/role.go, role_test.go and active backend/shared role declarations.
Interface: guard.Role(ctx,name,args,opts...) applies deploymentRolePrefix and deploymentBoundaryArn when both configured.
- [x] Write failing mock tests for bounded naming, correct boundary and unchanged manual mode; reject incomplete/malformed boundary pins.
- [x] Route all active runtime role declarations through helper; wizard desiredConfig writes corresponding exact pins.
- [x] Run Pulumi module tests and commit.

### Task 3: Wizard approval and saved automatic mode

Files: deploy/wizard/github_auto.go, github_auto_test.go, github.go, config.go, wizard.go.
Interface: Options.GitHubPolicyProfile string; choices.GitHubAutomatic bool and per-target approval digests; --github-approve-policy invocation-only.
- [x] Write failing tests for explicit approval, reject/EOF no setup, exact hash requirement unattended, no re-prompt unchanged, changed target re-review.
- [x] Build generated targets before snapshots; discover KMS key ARN from verified state bucket; write nonsecret policy JSON for review; explicitly review all selected permission documents.
- [x] Persist approved mode/digest before remote changes. Carry role pins into CI snapshots and resource recovery projection.
- [x] Update docs and run full deployment tests/race/vet and Windows compile; commit.

### Task 4: Consumer integration and delivery

Files: cmd/deploy/main.go, go.mod/go.sum, infra/go.mod/go.sum, infra-index/go.mod/go.sum, docs/deployment.md.
- [ ] Enable profile, update all dependency pins to published library commit, document automatic setup and advanced overrides.
- [ ] Verify root/infra tests and vet, workflow tests, Windows CLI compilation with -buildvcs=false; no local replaces.
- [ ] Review final changes and open paired PRs. No merge or deployment.

## Implementation findings and review

- Runtime Lambda code needs the same target isolation as execution roles; both are prefixed. Exact reset routing is retained and its boundary checked before enrollment.
- Optional Android/KMS policies exceed the aggregate inline-role quota, so deployment grants use immutable managed parts below 6144 characters each.
- Final review's fresh-role routing and application/state bucket wildcard collisions were reproduced by failing tests and fixed. Backend-only shared snapshot churn was also reproduced and fixed.
- Native author verification: full deploy and Pulumi race tests/vet, root tests, Windows wizard cross-compilation. Audited 236 exact action occurrences against current AWS service reference JSON; no unknown actions. No live cloud calls.
