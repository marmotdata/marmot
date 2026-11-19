# OpenLineage specification (vendored for tests)

These files are copied unchanged from the OpenLineage project so the tests can validate every event fixture against the official JSON schemas without network access.

| | |
| --- | --- |
| Source | https://github.com/OpenLineage/OpenLineage |
| Release | 1.55.0 |
| Commit | 5ec09f180d0857327a58b85531b7447d069abf07 (2026-10-29) |
| Core schema | `https://openlineage.io/spec/2-0-2/OpenLineage.json` |
| Licence | Apache License 2.0, copyright The Linux Foundation and OpenLineage contributors |

- `OpenLineage.json` is `spec/OpenLineage.json`.
- `facets/` is `spec/facets/*.json`.
- `facet-examples/` is `spec/tests/`, the example facets the project validates its own schemas with.

To move to a newer release, replace the three from the same tag and update the table above and `../events/README.md`.
