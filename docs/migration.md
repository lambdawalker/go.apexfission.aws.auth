# Attestra migration and compatibility

No original repository, stack or AWS resource was modified. Reference commit: `cf74956eea69f04b7f96d6835462c3f2bd61c342`.

| Boundary | Current compatibility | Migration action |
| --- | --- | --- |
| Signup/resend/confirm APIs | Original handlers/services/tests retained; raw provider log removed | Replace imports first, keep existing Lambda entrypoints/routes |
| Sign-in, sessions, passkeys | Original Cognito adapter behavior retained | Preserve issuer/client IDs and existing client configuration |
| ID capture | Optional retained packages, not enabled by core | Explicitly register capture routes/builds/resources and review policy |
| Cognito users/pools | No live migration; new components create new pools | Keep existing resources until an exact URN/property migration is reviewed |
| Proof DynamoDB | Existing service schema and atomic transitions retained | Preserve table identity and IAM policies; do not move data automatically |
| Admin identity/MFA | Original adminsetup/adminapi retained; new baseline admin shell supports separate pool + TOTP/PKCE | Keep original admin pool and MFA reconciliation until full parity verified |
| Admin config/grants/audit | Original runtimeconfig schema retained; new module config is separate namespace/schema | Add module configuration alongside original config; use an explicit exporter/validator before any migration |
| Environment registry | Original model/client retained | Keep registry ownership and coordination with original shared stack during transition |
| Deployment state | Original workflow retained under legacy/deploy; new engine targets explicit app stacks | Do not point new example components at existing Attestra state |
| GitHub Actions | New example uses same engine as local CLI | Retain original workflow until credentials, OIDC trust, locks and target checks are staged |
| Credentials/vault | Existing encryption and scope retained | Never rename/re-encrypt automatically; test with a nonproduction copy |
| Reset/teardown | Original implementation retained only as compatibility code | Keep application data out of reset; require explicit deletion ownership/policy |

## Incremental stages

1. Pin the framework to a reviewed commit in an Attestra feature branch. Switch authentication imports only; run retained tests and compare HTTP responses. No infrastructure changes.
2. Introduce composition for new functionality while existing handlers, admin store and deployment workflows remain authoritative.
3. Add module configuration in new partitions/table. Preserve old revisions and audit rows. Any migration must validate schemas, preserve actors/timestamps, and be replay-safe.
4. Extract infrastructure one resource at a time. Back up state securely, inventory URNs/physical IDs, and compare desired input properties. Keep old stack/project/resource names initially.
5. Run an actual refresh/preview in an isolated nonproduction stack. Reject unplanned deletes/replaces. Only after review consider adopting existing production resources.
6. Migrate operator workflows separately, including shared-stack locks, DNS/SES checks, OIDC roles, vault recovery and teardown receipts. The presence of extracted helpers alone is not full wizard parity.

## Pulumi identity strategy

The new components use different names/parents from Attestra. **Do not replace the old infrastructure program with them in-place.** A component-level alias does not automatically prove every child resource is correctly mapped.

For a resource whose logical name or parent changes, supply an exact old URN alias or old name/parent/stack/project tuple to the exposed constructor. For example:

```go
orders, err := infrastructure.NewTable(ctx, "sales-orders", "id",
    pulumi.Aliases([]pulumi.Alias{{
        URN: pulumi.URN("urn:pulumi:dev::existing-project::aws:dynamodb/table:Table::old-orders"),
    }}))
```

For a currently unmanaged resource, use `pulumi.Import(pulumi.ID(physicalID))` at the precise resource constructor, retain protect, and compare every immutable argument. If an existing component does not expose the child migration options needed, retain direct resource declarations rather than forcing a replacement. Exported component outputs can compose with those retained resources.

`TestProtectedTableAndAlias` verifies aliases/protection reach the Pulumi engine request. This is **not** a verified alias map for existing Attestra resources, and no import/preview against Attestra state was performed. Broad child-resource migration support remains a deliberate follow-up rather than an unsafe guess about state.

## Fresh parallel deployments
The new pulumi/backend, pulumi/shared and deploy/wizard packages provide an
alternative to adopting live resources: deploy an isolated complete
Attestra-compatible backend and shared index/admin. They deliberately require
new project names (attestra-backend, attestra-shared) and explicit deployment
identity. The original state, users and data are not imported.

These packages preserve the existing runtime configuration schema and index
publication protocol; they are distinct from the smaller generic NewAuth and
NewAdmin components. A fresh backend references an external verified SES identity
and never owns its DKIM or verification records. Preserve that identity separately
before destroying its legacy owner.

A prefixed deployment and the later unprefixed final deployment have different
Cognito pools, passkeys, runtime tables and registry entries. The Android client
must use the corresponding exported environment configuration. Testing the
prefixed stack does not certify final domain bindings or live email/passkey
flows; verify them after the final deployment too.
