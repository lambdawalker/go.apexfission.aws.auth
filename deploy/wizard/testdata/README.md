# Real Pulumi plan fixtures

Captured using the official Pulumi CLI v3.273.0 and Go SDK v3.264.0, with a local
file backend and a Go component resource. No cloud provider or AWS account was
used. The update plan was successfully applied with `pulumi up --plan` before the
delete preview was captured. The component's policy property intentionally names
`lambda:DeleteFunctionConcurrency` to exercise false-positive detection.

These files contain only sample config, component state, random plan seeds and
build metadata. `manifest.version` is `v3.273.0`: it is a CLI build label, not the
plan schema version. Compatibility is checked from the resource-plan structure
and known operation names.
