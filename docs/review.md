# Independent review and implementation decisions

Independent read-only review of commit `47a7fc2` identified five important findings. The author addressed each:

1. AWS CLI identity alone did not constrain Pulumi: require pinned default-provider account IDs/region and reject conflicting provider config. Application-owned explicit providers remain trusted code and must impose equivalent restrictions.
2. Teardown confirmation omitted actual target fields: bind confirmation to application/environment/account/region/stack/backend; reject environment/stack aliases.
3. Arbitrary protected Lambda integration could omit runtime policies: require explicit `RuntimeEnforced` attestation and document exact handler obligations. API Gateway is not permission or token-use authority.
4. Admin srcdoc prevented standard relative assets: use verified HttpOnly browser session, server-authorized asset requests at arbitrary depths, same-origin iframe and extension APIs. Added cross-origin mutation rejection.
5. Default-provider invokes could produce wrong issuer/SES ARNs: derive them from the actual provisioned pool/sender outputs.

Additional author fixes: atomic grant recheck during config publication, preserving trusted API Gateway host for cookie mutation checks, retained launcher test fixtures, and actual archive/bootstrap verification.

Decisions and costs:
- Preserve the old operator workflow rather than rewrite it wholesale. Cost: the new generic engine does not yet reproduce the entire interactive wizard/shared-registry recovery experience.
- Keep existing Attestra config/admin packages alongside the new namespaced shell. Cost: schema migration and administrator passkey-MFA adapter integration remain separate work.
- Use stable-major dependency requirements and exact manifest version matching. Cost: no semantic-version range language and no compatibility promise for major-zero APIs.
- Do not guess existing Pulumi child aliases. Cost: existing Attestra resources must remain directly declared until precise migration options/state mappings are reviewed.

Review exclusions were unfinished docs/CI, unchanged legacy behavior, live AWS validation and later author fixes. Docs/CI and local verification were completed afterward; legacy behavior is covered by retained tests, and live AWS remains explicitly unverified. No second independent review is claimed.
