# Writing an application module

Implement `core.Module.Describe() core.Descriptor`. IDs are lowercase identifiers; versions are stable `major.minor.patch`. Dependencies declare required major versions, missing dependencies and cycles fail, duplicates fail, and initialization is deterministic by dependency then lexical ID. For pre-1.0 APIs, pin exact Go versions because major-zero compatibility is not a stability promise.

Implement only the optional contracts you use:

| Contract | Contribution |
| --- | --- |
| `router.Module.Routes` | HTTP routes, handler, Lambda name, permissions, validation and module dependencies |
| `core.PermissionModule.Permissions` | Namespaced permission names, e.g. `sales:read` |
| `admin.Module.AdminPages` | Page ID/title/permission, `fs.FS` and entry asset |
| `config.Module.ConfigSchema` | Current schema version, validator and optional forward migration |
| `core.BuildModule.BuildTargets` | Application-local Lambda entry packages |
| `core.RuntimeModule.Start` | Ordered runtime startup |
| `core.HealthModule.Health` | Application-invoked health check |
| `core.LifecycleModule` | Application-invoked before/after deployment hooks |
| `pulumi.Extension.Provision` | Application-stack resource contribution and outputs |

`application.Compose` validates declarations before runtime construction. Call `core.Start` when your application needs startup hooks. Create `Application.Handler` with a verifier and authorizer, and `Application.Admin` with a **different administrator verifier** and durable grant/config store. Provisioning remains an explicit Go composition root; it does not run runtime startup methods.

## Routes and identity

`router.Route.Access` defaults to authenticated. Set `router.Public` only for intentional public endpoints. `Permission` must be declared by a module. Exact duplicate routes and overlapping literal/parameter templates are rejected so runtime and API Gateway cannot select different policies. Simple single-segment parameters are supported; broad catchalls belong to dedicated runtime-authenticated services such as the admin shell.

Use `authorization.FromContext` to obtain the verified subject. The verifier does not grant application permissions. `ScopeAuthorizer` uses explicit namespaced scopes; `DynamoGrants` uses server-owned environment grants; applications may implement `Authorizer` for ownership/role policy. The sales example explicitly allows signed-in customers to read/write only their own orders; it does not assume Cognito emits custom sales scopes.

`router.Lambda` adapts HTTP handlers to API Gateway v2. Existing authentication handlers remain available in `api`, `signin` and `passkey`; `auth.Module` registers them. Auth flows retain A+B/B+C confirmation, resend limits, interruption recovery, Cognito sessions and WebAuthn. `capture` and related adapters remain optional; they are not enrolled in runtime, builds, infrastructure or reset by default.

## Infrastructure

Use the separate `/pulumi` module. `NewAuth`, `NewAdminWithLogin`, `NewFunction`, `NewAPI`, `NewAdminAPI` and `NewTable` return resources/outputs. Consumers may freely add AWS resources to their own stack. Archive paths and Lambda maps are explicit. Resource options support providers, aliases and imports at exposed resource boundaries.

For `NewAPI`, every protected Lambda must be listed in `RuntimeEnforced`. This is an explicit attestation by the application author that the handler independently verifies access-token use and enforces permission and request-validation declarations. API Gateway JWT authentication is defense in depth, not a replacement for runtime policy. The framework rejects unacknowledged protected integrations. Do not attest arbitrary handlers. `NewAdminAPI` similarly requires the runtime-authenticated admin shell and routes all asset depths through it.

## Frontend assets and configuration

Page assets can come from any build tool: embed its output with `embed.FS` or provide another `fs.FS`. Build relative asset URLs under the page directory. Pages run as trusted application code on the admin origin, load assets through server permission checks, and can call `/admin/api/extensions/...` with the HttpOnly session cookie. Avoid secrets in HTML/JS. The shell's CSP permits same-origin scripts/assets; avoid inline scripts and external CDNs.

Cognito authorization-code + PKCE sign-in is available through `admin.Login`; manual access-token connection is also available for development. The token is moved into a Secure, HttpOnly, SameSite=Strict, host-only cookie and verified on every request. Cookie-authorized mutations check the same-origin `Origin` header. Environment changes clear the displayed page/config before loading another environment.

Configuration uses `ENV#<environment>#MODULE#<namespace>` partitions, optimistic revision checks, immutable revision rows, schema validation and optional forward migration. The shell supports publication and the latest 100 audit revisions. Restoring a value means publishing its content as a new revision, never overwriting history. When `DynamoGrants` is used, publication checks active permission in the same transaction as the revision write; a concurrent revocation fails the transaction. Custom authorizers/stores must supply equivalent consistency for their own grant model.
