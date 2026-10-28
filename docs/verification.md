# Verification evidence — 2026-10-28

Reference audited: Attestra `cf74956eea69f04b7f96d6835462c3f2bd61c342`. Attestra's original root `go test ./...` passed before extraction. No original source changes were made.

## Executed locally

| Check | Result |
| --- | --- |
| Root `go test -race ./...` and `go vet ./...` | Passed, including extracted authentication/capture/admin/config tests and new contracts |
| `deploy` test/race/vet | Passed, including retained vault/DNS/GitHub/registry tests, manifest guards and archive validation |
| `pulumi` test/race/vet | Passed with mocks; integration enforcement, protection/alias forwarding and resource-derived issuer tests |
| `legacy/deploy` test/race/vet | Passed; retained operator tests and launcher fixtures |
| `examples/sales` test/race/vet | Passed; consumer composition and authenticated order isolation |
| `examples/sales/infra` test/race/vet | Passed with mocks; complete stack resource composition |
| Sales `go run ./cmd/deploy -op build` | Produced real application/challenge Lambda ZIPs |
| ZIP verification | Each contains executable `bootstrap`, ELF Linux arm64 |
| CLI Windows amd64 / macOS arm64 cross-compilation | Passed; binaries were not executed on those operating systems |
| Admin shell JavaScript `node --check` | Passed; not a real-browser OAuth smoke test |
| Clean external Go consumer without local replaces | `go get` and composition test passed against root commit `6e6bb155c044` |
| CLI installation from explicitly pinned remote module and `init` | Passed; direct `go install …@commit` is blocked by the empty default branch until merge/tag |
| Standalone nested module tests with `GOWORK=off` | Passed against the actual pinned remote root module |
| Runtime dependency graph | `auth` and `application` have no Pulumi imports |

An independent reviewer reviewed the initial implementation; findings and fixes are recorded in [review.md](review.md). Later fixes were validated by targeted regression tests and the suite; no second independent review is claimed.

## Not executed

- Real `pulumi preview`, refresh, import, up or destroy against AWS/state.
- SES delivery, DNS propagation, managed-login/TOTP/passkey interaction, or browser frontend smoke tests.
- Real IAM enforcement, cross-account OIDC, production teardown or recovery drills.
- Attestra resource migration or physical-resource replacement checks.
- Version tagging or package publication.

Mocks demonstrate resource registration and contracts, not deployed security or operational readiness. The PR remains a reviewable initial framework, with full generic wizard parity and safe existing-resource migration explicitly separated from these passing checks.
