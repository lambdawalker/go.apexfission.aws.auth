# Source provenance

Source reference: `lambdawalker/go.attestra.aws.auth` commit `cf74956eea69f04b7f96d6835462c3f2bd61c342` (2026-10-28 audit). No changes were made to that repository.

- `email`, `api`, `passkey`, `signin`, `challenge`, `awsemail`, `awsidentity`, `awsstore`: authentication extraction with package imports changed. Removed raw provider-error logging in `email` and `passkey`, and disabled credential-bearing passkey redirects and added regression coverage. Existing wire behavior and test suites retained.
- `capture`, `captureapi`, `awscapture`, `assetlinks`, `registry`, `runtimeconfig`, `envreset`, `adminapi`, `adminsetup`, `internal/awshttp`: optional compatibility packages, imports changed. Core authentication does not import capture or reset. Original configuration/grant transactions and admin MFA reconciliation remain available.
- `deploy/aws`, `dns`, `github`, `vault`, `registry`, `ui`: reusable operator subsystems and tests extracted from `tools/deploy/internal`. Vault version, cryptography and scope retained deliberately so existing encrypted vaults remain readable.
- `deploy/build/package.go`: existing executable-bootstrap ZIP writer retained. New build driver takes application-owned targets.
- `legacy/deploy`: original operator workflow retained, with Go imports/module path updated. Launcher tests use copied source fixtures. This is compatibility tooling for the **original Attestra directory layout**, not the new application's deployment engine. Never point it at an arbitrary application's resources.

The upstream source does not provide a license file. This extraction was requested by the repository owner; no new redistribution license has been invented. Before publishing a public version, choose a license and review all dependency licenses.

## Parallel deployment extraction
The shared/backend deployment packages and process entrypoints additionally use
Attestra source commit `9ee9affd51578b873586b538a8e602fb30c1fdbf`.
The newer enrollment policy, password flows and capture completion facts are
retained with their upstream tests. Existing generic status callers without an
enrollment service retain their basic passkey response. Email/passkey redaction
fixes from the initial extraction remain in place.

The new `entrypoints/<name>.Run()` packages are process entrypoints: they read
environment configuration and start Lambda (or run the explicitly selected admin
setup command). They are intended for one-line consumer main packages, not
embedding inside an already running HTTP server.
