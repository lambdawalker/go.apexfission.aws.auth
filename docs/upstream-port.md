# Original backend synchronization

Ported runtime and worker IAM changes from go.attestra.aws.auth 9ee9aff through 4607807ab0346f4a38532982889fe2e56954412e.

Includes failed-extraction manual review, abandoned extraction recovery with revision fencing, safe inference/store/dispatcher diagnostics, and worker UpdateItem permissions restricted to ACCOUNT# records. The extracted entrypoint and Pulumi backend preserve their modular packages and deployment isolation. See the [upstream capture contract](https://github.com/lambdawalker/go.attestra.aws.auth/blob/4607807ab0346f4a38532982889fe2e56954412e/docs/id-capture.md).

Legacy export scripts and site publication metadata remain in the original repository; the modular wizard already consumes their export format. No legacy infrastructure or deployment settings were adopted.

Ready captures with complete or failed extraction accept reviewed card data and origin_country. Pending extraction remains blocked. Manual review preserves the failed extraction result, enforces ownership/expiry/revision checks, and supports identical retries. Clients must wait for persisted reviewed data before showing Details saved; id_captured and onboarding_complete are not save acknowledgements.

GET recovers missing/abandoned extraction into manual-review state after five minutes past the scheduled attempt, without interrupting a live lease. The revision fence prevents late workers overwriting recovery. Diagnostics contain identifiers, states, timing and error codes, never card fields or provider response bodies. The capture worker requires scoped DynamoDB UpdateItem on ACCOUNT# records for the readiness transaction. Redeploy infrastructure as well as code to apply that grant.
