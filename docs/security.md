# Security and trust boundaries

The framework's trust boundary is its executable Go application and reviewed module dependencies. Manifests select no arbitrary code. Frontend modules are trusted same-origin code, not an isolation sandbox for untrusted tenants.

## Tokens

`auth.Verifier` permits only RS256 with RSA keys of at least 2048 bits, exact configured issuer, Cognito access-token `client_id`, `token_use=access`, nonempty subject, future expiration, and nonfuture `nbf`/`iat`. Cognito access tokens bind the client through `client_id`; ID-token `aud` is intentionally not accepted as a substitute. Tokens are bounded to 16 KiB.

`NewJWKS` pins the HTTPS discovery endpoint to operator configuration, rejects redirects, bounds response size and request time, caches keys for an hour and rate-limits refresh to once per minute. Unknown keys fail closed; brief rotation outages are possible during the refresh throttle. No token-supplied URL or algorithm controls key lookup. Clocks must be synchronized; there is no implicit expiry grace.

The admin verifier must use the dedicated admin issuer/client, never the customer pool. Example administrators require invitation and MFA. `NewAdminWithLogin` implements a baseline password/TOTP pool and OAuth PKCE login. Existing Attestra's additional passkey-MFA reconciliation is retained in `adminsetup`/`adminapi`; it is not automatically run by the new generic component. Do not claim equivalent administrator passkey-MFA setup until that adapter is explicitly integrated and verified.

## IAM and state

Lambda logs are restricted to a provisioned log group. Business order roles can query/write only their orders table; they receive no proof key or identity-management permissions. Admin grants are read/condition-check only; the admin runtime cannot grant itself access. Configuration writes are scoped to the selected environment partition. Authentication policy combines the permissions needed by the authentication Lambda; its actions are scoped to proof table, user pool and SES identities/from address. More granular applications may split authentication endpoints into separate functions using the retained handlers.

Tables and identity pools are protected and encrypted, with table point-in-time recovery enabled. New application resources never enter the legacy reset inventory automatically. No blanket resource deletion API exists in the generic engine. Pulumi protection must be deliberately reviewed and changed before deleting protected resources; the CLI never silently unprotects them.

The deployment engine checks AWS CLI identity and requires the Pulumi default provider's region/account restriction to match the manifest. Provider override configuration is rejected. Explicit providers in application Go code remain trusted executable code and must impose the same `allowedAccountIds` restriction. The engine cannot sandbox arbitrary Pulumi programs.

## Secrets and operations

Proof keys are Pulumi secrets; credentials are not stored in manifests or receipts. Extracted vaults retain AES-GCM/Argon2id and their original scope format. Local build processes strip AWS/Pulumi environment credentials, but compilation is not a security sandbox: trusted dependencies/build tooling can access the developer account's files or other environment variables.

Receipts contain operation stage and target, not secrets. Pulumi locks protect state operations; a local lock rejects simultaneous engine processes. Interrupted operations retain a stage receipt and can be rerun after inspecting the actual stack. External before/after steps must be idempotent; the engine does not pretend it can roll back arbitrary external actions.

Production destruction requires `--allow-destroy`, `--allow-production`, and the exact application/environment/account/region/stack/backend confirmation. Environment and stack names must match. Keep production in separate accounts and protected GitHub environments. User-supplied manifests are reviewed operational configuration, not proof that a target is nonproduction.

Tests and mocks do not certify real-device WebAuthn, email delivery, AWS IAM effectiveness, browser OAuth, provider configuration, or recovery from all cloud failures. Those require a new isolated environment and an explicitly authorized validation plan.
