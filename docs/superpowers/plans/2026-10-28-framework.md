# Apexfission Implementation Plan

> Implement inline with superpowers:executing-plans; review the whole branch before PR.

**Goal:** An importable framework and working sales consumer.
**Architecture:** Root runtime module; isolated Pulumi and deploy modules; explicit static application composition.
**Tech Stack:** Go 1.26.6, AWS SDK v2, Pulumi Go.
**Spec:** ../specs/2026-10-28-framework-design.md and ../../implementation-request.md

## Global constraints
Preserve Attestra source unchanged. No live AWS changes. No dynamic executable plugins. Keep Pulumi out of runtime imports. Use feature branch and submit PR.

## Review focus
- Public/default-auth route ambiguity and parameter conflicts.
- Forged/wrong-use/wrong-audience/expired JWTs and admin separation.
- Cross-environment configuration and stale publication.
- Untrusted build paths, credentials inherited by builds, target mismatch.
- Destructive commands without explicit exact-target confirmation.

### Task 1: Runtime composition and authentication
- [x] Extract audited email, api, passkey, signin, challenge, AWS adapters and tests with import rewrites and provenance.
- [x] Implement core.Module, Descriptor, dependency ordering and optional contributors; validate cycles, incompatible versions and duplicate contributions.
- [x] Implement api route descriptors/runtime and authorization Principal/Authorizer; test default denial and conflicts.
- [x] Implement auth JWT validation and adapters; test adversarial tokens and provider-log redaction.
- [x] Run root go test ./... and commit.

### Task 2: Admin and configuration
- [x] Implement page manifest, assets, navigation, separate admin verifier and permission-enforced APIs.
- [x] Implement namespaced schema validation/migration and CAS revision store, immutable history with environment isolation; test conflicts and authorization.
- [x] Run root go test ./... and commit.

### Task 3: Infrastructure composition
- [x] Implement Pulumi API, Lambda, authentication and admin components with explicit archive/resource inputs, scoped IAM, protected stateful resources and exported outputs.
- [x] Add application extension callbacks and mock tests for resources/routes/aliases.
- [x] Run pulumi module go test ./... and commit.

### Task 4: Deployment and consumer
- [x] Extract operator clients and legacy workflow with provenance; generalize Lambda packaging.
- [x] Implement strict manifest and init/build/preview/deploy/status/teardown using injected commands, AWS account checks, Pulumi state and explicit destruction guards.
- [x] Add sales runtime, assets, infrastructure, manifest and tests as separate Go modules using local development replaces.
- [x] Run all module tests, build archives, add CI, installation and migration compatibility matrix; commit.

### Task 5: Review and delivery
- [x] Run tests, race/static checks and cross-platform CLI builds. Inspect runtime dependency graph.
- [x] Request independent branch review, fix critical/important findings, rerun affected tests.
- [x] Push feature branch and open PR with exact validation evidence and limitations.

## Interface ledger
core descriptors consumed by API/admin/deploy; api descriptors consumed by Pulumi. No Pulumi types in runtime. Application manifest selects already linked module IDs and versions only. Runtime auth consumes verified Principal; infrastructure does not grant application permissions implicitly.

## Execution record
The supplied prompt authorized execution without additional design approval. Native implementation was used, followed by the skill-required independent reviewer. See docs/review.md for the full decision ledger and docs/verification.md for observed results. The generic engine retains reusable operator clients and the original workflow separately; it does not claim complete wizard parity. Existing-resource imports and live previews remain unexecuted. PR creation is the final delivery action.
