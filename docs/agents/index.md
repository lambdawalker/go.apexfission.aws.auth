# Go authentication integration

Scope: current source of the independent authentication feature.

- [Feature setup, validation and composition](../../README.md)
- [Composition boundary](../composition-design.md)
- [Attestra system flow](https://raw.githubusercontent.com/lambdawalker/design.attestra/main/docs/agents/journeys.md)
- [Capture API, retake and OCR timing](https://raw.githubusercontent.com/lambdawalker/go.appexfission.aws.card-capture/main/docs/agents/index.md)
- [Application rollout and module pins](https://raw.githubusercontent.com/lambdawalker/go.attestra.aws/main/docs/card-capture-composition.md)

Authentication supplies the API and identity bindings via `identity.card-capture/v1`
and consumes the capture facts table for status. The capture feature owns the
upload, retake, OCR and review endpoints. Changes to its OCR worker are picked up
through the runtime and infrastructure Go module pins. The application selects the
resolved versions used to build and deploy the worker.
