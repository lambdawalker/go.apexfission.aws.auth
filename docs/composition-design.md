# Authentication feature extraction

Source baseline: go.attestra.aws.auth at 4607807ab0346f4a38532982889fe2e56954412e. Application owner: go.attestra.aws. Composer: go.infra.composer at 2628bc245bc6cbbef00f9fdbfcdd8c0ffab637e1.

The authentication repository owns existing email proof, passkey/sign-in, capture/extraction, onboarding-policy, auth-only reset runtime packages and their Lambda entry points. The application owns admin/index UI, shared registry/runtime infrastructure, setup/deployment credentials, environment policy, CI and discovery publication. Runtime config schema/store stays with the auth feature; application-owned administration consumes that contract. No feature imports the application.

Approach: extract existing runtime behavior and native Pulumi infrastructure into reusable Go packages, then compose the feature through the new composer. Avoid reimplementing AWS behavior from scratch or keeping an application-local copy of authentication runtime. The legacy repository remains operational; new deployments use distinct application project/state identities and reviewed targets. No live state transfer, resource deletion, credential migration or cloud deployment is authorized here.

## Module boundaries

- Root feature module: github.com/lambdawalker/go.apexfission.aws.auth, Go 1.27.3. Runtime packages/commands plus `feature` declarations, `artifacts` manifest and static configuration inventory.
- Optional `infrastructure` module: native Pulumi AWS provisioning and standalone command, exported Deploy API. No private application config, domains or credentials are checked in.
- Application root module: github.com/lambdawalker/go.attestra.aws. Application/admin/index packages, composer registration/injection and a local plan command.
- Application `infra`: Pulumi program, bounded factories resolving the feature and shared-runtime reference contracts; calls reusable feature infrastructure.
- Application `infra-index`: existing shared admin/index native program, now application-owned.
- Application `tools/deploy`: preserved operator workflow, building pinned external feature commands via Go module imports and local app commands. Hashes include module dependency identity through committed source/dependency files.

## Contracts and execution

`feature.Declare(id)` returns a side-effect-free composer feature. It requires `runtime_configuration` with contract `auth.runtime-policy`, major 1, fields `table_name` and `table_arn` (strings). Its `backend` resource kind is `apexfission:auth:Backend`, requiring those fields through RequirementRef. Outputs are `api_url`, `pool_id`, `client_id`, `issuer`, and `reset_function_name` (strings). Offerings `identity` and `api` expose typed named capabilities. Stable constants name these IDs/kinds/fields.

The application declares a registered reference to its already-deployed shared runtime table and injects that offering. Missing bindings fail logical planning before native constructors. Native factory receives Pulumi deferred values for the table; never stringify unresolved Outputs. The feature's native deployment preserves internal provider dependencies. Composer initially validates the feature/resource-group boundary; it does not claim to model every private AWS resource inside the existing native deployer. Pulumi remains the second dependency-validation layer.

The integrated application does not deploy a feature-owned panel. The original admin UI moves to the application and retains existing runtime authorization, consent and audit behavior. Standalone feature use may create an isolated runtime table; it does not require deploying application admin/index. There is no invented universal configuration reader: expose supported source mechanisms and reject unsupported bindings.

## Deployment configuration and artifacts

Native infrastructure takes caller-supplied artifact directory, configuration reader, runtime table StringInputs and optional reset-function name/environment options; it returns typed API/pool/client/issuer/reset outputs. It must validate all required settings and artifact existence before registering resources. Standalone and integrated paths call the same implementation. Do not load configuration or register resources at package import/declaration time.

Artifact manifest lists all 20 feature Lambda archive names and Go command package paths (nine auth routes share cmd/signin). Application builds these pinned module commands plus its own admin/index binaries. No command may require a sibling clone. New app project names are `attestra-application` and `attestra-control-plane`; do not automatically select old projects/state. A new reset-function prefix must avoid legacy collisions and be shared by admin invocation and native deployment.

Proof-key material remains unchanged during extraction. New infrastructure stores the supplied Pulumi secret in AWS Secrets Manager and passes only its ARN/version to email Lambdas. A feature cold-start reader retrieves and validates the key once per execution environment; no per-invocation network read. Exact secret-scoped IAM is provisioned. Legacy plaintext PROOF_KEY is not a silent fallback. Rotation, outstanding proof validity and old/new consumers require a separate reviewed migration; this refactor does not generate or rotate keys implicitly.

## Verification and rollout

Preserve existing runtime tests. Add failing tests for import separation, manifest completeness, composer binding gaps/cycles, strict configuration sources, secret retrieval/error redaction, missing artifact preflight, native deferred runtime bindings and no PROOF_KEY environment values. Use Pulumi mocks only; no deployment for validation. Test/build application root, shared/backend infra and operator modules, plus Go/Node/Python helper checks as applicable. Pin published feature/core commits, eliminate external sibling replaces from application manifests, and open feature/application PRs. A legacy README notice may point to the extraction; no runtime deletion or redirect of live traffic.
