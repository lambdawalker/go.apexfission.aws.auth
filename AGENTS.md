# Feature ownership

Authentication is an independent feature. Never import go.attestra.aws or add application admin/index/deployment credentials here. The runtime module and optional infrastructure module have separate dependency manifests.

Preserve source behavior and test contracts; docs/composition-design.md records the extraction boundary. Run Go tests/vet in root and infrastructure. Do not run AWS deployment, teardown, seed-runtime, or package publication as validation. Never commit generated dist, stack settings, credentials or live outputs.
