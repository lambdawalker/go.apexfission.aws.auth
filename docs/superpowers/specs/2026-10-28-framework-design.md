# Apexfission modular AWS framework

The supplied Option C specification is the execution authority. Implement a static Go composition layer, preserve audited Attestra authentication code, and isolate provisioning and operator dependencies in nested modules. No Attestra deployment or mutation is authorized.

## Audit
Reference: Attestra cf74956eea69f04b7f96d6835462c3f2bd61c342. Target began with README only. Inspected root/module manifests, AGENTS, consumer docs, auth services/handlers/bootstrap, infra, runtime config/admin contracts, and deployment subsystems.

Email uses an atomic Store contract, A+B or B+C proofs, bounded attempts, confirmation claims, and OTP recovery; Cognito supplies sessions and WebAuthn. Preserve those contracts and regression tests. The raw provider error log contradicts the stated redaction guarantee and will be removed in the extraction. ID capture is independent and must remain optional.

Infrastructure hardcodes archives, routes and reset inventories. Deployment packages contain useful independent AWS environment, DNS, GitHub, vault and registry clients; the workflow layer couples them to Attestra names and shared stacks. Extract clients and packaging; keep the original workflow as an explicitly named compatibility tool until all operator scenarios can be migrated safely.

## Approaches
Selected by the supplied prompt: composable framework plus application-owned stacks. A fork per application defeats reusable versioned consumption. A wholesale workflow rewrite risks credential/recovery behavior. Extraction with compatibility boundaries retains existing behavior while new applications use declarative registration.

## Contracts
Root Go module: core optional contributor interfaces, API runtime, auth adapters, authorization, admin page manifests and namespaced revisioned config. Root packages never import Pulumi. Nested pulumi module owns resources and supports app callbacks; nested deploy module owns build/preview/lifecycle orchestration. Consumers own executable composition, DynamoDB data, permission grants, stack state and deletion policy. Manifest is data only, not executable plugins.

Authenticated routes are the zero-value default; public routes require explicit declaration. Admin tokens have a distinct verifier/audience. Permissions are server enforced, not inferred from UI visibility. Config publication uses optimistic concurrency and immutable audit history. Production destruction requires explicit target confirmation, and no environment reset automatically enrolls application resources.

## Validation
Preserve extracted tests. Add module graph/conflict tests, adversarial JWT/authz tests, revision and page tests, mock Pulumi resource tests, command-recording deployment tests and a separate sales consumer. Build real Linux Lambda archives. Report Pulumi mock evidence separately from real preview or AWS deployment.
