# Parallel Deployment Wizard Implementation Plan
> **For agentic workers:** Use superpowers:subagent-driven-development to implement task-by-task.

**Goal:** Deploy isolated shared admin/index and backend infrastructure from a portable Go wizard.
**Architecture:** Reusable library infrastructure preserves existing runtime contracts; reusable wizard builds/selects scoped deployments; Attestra provides launchers and thin infrastructure/runtime entrypoints.
**Tech Stack:** Go, Pulumi AWS, existing AWS/Cloudflare/registry/vault helpers.
**Spec:** ../specs/2026-10-28-deployment-wizard.md

## Global Constraints
No AWS deployment/teardown. Prefix defaults to next; explicit empty prefix selects separate identity. External SES resources are never owned. Preserve admin MFA and authenticated index publication. No live state adoption.

## Review Focus
- Empty prefix must not target legacy project/state.
- Shared outputs cannot bind another prefix/account/region.
- Preview must not mutate cloud/DNS.
- Sender identity remains external during teardown.
- Missing prerequisites fail before cloud mutation; no silent partial success.

### Task 1: Infrastructure
Files: pulumi/shared/*, pulumi/backend/*; tests alongside.
Expose shared.Deploy(ctx) and backend.Deploy(ctx), using stack config and consumer dist artifacts, extracted from original infra-index and infra. Change imports to framework runtime. External verified sender support required. Keep original runtime/config schemas and feature coverage. Add scope/account/region guards and prefix stack isolation. Export needed outputs. Document config keys and archives.
- [x] Extract declarations and write regression tests for isolation/external sender.
- [x] Run Pulumi module tests/vet.
- [x] Commit and review.

### Task 2: Reusable wizard
Files: deploy/wizard/*.go; tests alongside.
Expose Run(ctx, root, input, output, args) error (or explicit Options API documented for consumer). Persistent versioned deployment selections; prefix/env/scope/operation; build target configuration; first-deploy stack initialization; certificate readiness and explicit DNS setup; shared output wiring; registry publication; health and Android configuration. Reuse existing helper code where appropriate. Print exact targets and require explicit deploy intent. Preserve backend protections and select separate projects/stacks. No teardown wizard required in this change.
- [x] Write tests with fake runner for scope order, no cloud mutation preview, mismatch rejection, blank prefix, config persistence.
- [x] Implement, test race/vet; document API.
- [x] Commit and review.

### Task 3: Consumer integration
Files: cmd/deploy, cmd runtime entrypoints, infra, infra-index, deploy.sh, deploy.bat, workflows/docs.
Use extracted infrastructure and runtime library; map build artifacts; configure defaults; explicit first-deploy instructions. Pin real remote framework commit before publishing consumer; no sibling replace dependencies.
- [x] Build integration and launcher tests.
- [x] Run root and infra tests, actual Lambda packaging, Windows CLI crosscompile.
- [x] Review full changes; prepare paired PRs with exact validation limits.

## Verification result

Infrastructure and wizard review findings were fixed and scoped rereviews found no remaining blockers. Standalone runtime, Pulumi and deploy module tests/race checks/vet passed. The consumer was tested with GOWORK=off against published framework commit eda2f3f2a93633e3b005f761e0d94ee24a4cf15c, with no local replacements. All 22 Linux ARM64 Lambda ZIPs were built and inspected; Windows CLI cross-compilation, POSIX launcher and workflow YAML checks passed. No live cloud deployment, DNS publication, invitation or teardown was performed. Device sign-in/capture and actual AWS readiness remain deployment acceptance checks.
