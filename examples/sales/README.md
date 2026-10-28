# Sales reference application

This separate module imports the framework and adds two customer order routes, a Sales admin page/status API, a configuration schema, application-owned DynamoDB orders, scoped IAM and build targets. No framework internals are modified to add the module.

1. Run `go test ./...` and `go run ./cmd/deploy -op build` here.
2. Replace placeholder account/backend settings in `apexfission.json`.
3. Initialize `infra` as described in ../../docs/deployment.md, including `aws:allowedAccountIds`.
4. Configure `appOrigin`, `adminOrigin`, `senderDomain`, `senderAddress`, and secret `proofKey` (base64 of at least 32 random bytes) in Pulumi. `adminOrigin` is the HTTPS custom origin you will map to the admin API; its `/admin` path is the OAuth callback.
5. Run `go run ./cmd/deploy -op preview -env dev`. Review all resources and IAM. This example is for a new stack, never existing Attestra state.
6. After an explicitly authorized deployment, publish SES verification/DKIM DNS records and map the admin custom domain/certificate to the returned admin API. DNS/custom-domain mapping is operator-owned; the example does not automatically change Cloudflare or DNS.
7. Invite an administrator into the separate admin pool, complete password/TOTP enrollment, and explicitly create a grant row in the config table using an operator role. No initial administrator is granted implicitly.

A grant uses string keys `pk=GRANT#dev`, `sk=<admin Cognito subject>`, Boolean `active=true`, and string-set `permissions` including `admin:access`, `sales:admin`, `sales:config.read`, `sales:config.write`. The admin Lambda cannot write grant rows. Use audited operator change management for grant creation/revocation; a generic grant-management UI is not implemented.

Customer order APIs use verified subject ownership: each query specifies the caller's subject as partition key. A customer cannot query another customer's orders through these endpoints. Amounts are integer cents, bounded to 1..100000000. The example schema `max_order_cents` demonstrates revisioned configuration storage/publication; applying it as a business limit is an application integration exercise, not an implemented dynamic override.

The admin shell supports OAuth PKCE, environment selection, module navigation, standard relative frontend assets, configuration publication and audit history. The Sales page is a small bundled HTML page. The authenticated extension status endpoint is `/admin/api/extensions/sales/status`.

Copy `deploy-workflow.yml` into an application's workflows after provisioning an OIDC role whose trust is restricted to that repository and protected environment. Store the Pulumi passphrase in the environment secret; no static AWS keys are required.

Local tests and mocked resource registration are not real AWS, email, browser-login or passkey verification. No cloud resources were created for this example during implementation.
