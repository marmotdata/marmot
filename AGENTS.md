# Marmot fork

This repository is `dgu-development/marmot`. Read [FORK.md](FORK.md) before opening a PR.

- Target **`dgu`**, not `main`. `main` tracks upstream with fast-forward only.
- Preserve Marmot native identity, ownership relations and APIs. Corporate schemas and translations belong to the consuming distribution; reusable metamodel and authorization belong here.
- Keep functional changes in small, reviewable commits. No automatic merge, release, default-branch or visibility changes unless the task says so.
- Core membership and `required` are independent. Validate the final persisted state (including ingest and dedicated mutations). Preserve unknown metadata and MRNs.
- Use Go from `go.mod`, existing dependencies and Svelte 5. Do not add another backend or UI plugin framework. YAML is trusted configuration, never executable code.
- Translation keys belong in schemas; catalogues live in the distribution. Go returns codes and field IDs.
- Area authorization must be enforced by the server on every exposed channel. Ownership or a UI filter does not grant access. Do not announce isolation until the acceptance matrix passes.
- Test changed Go packages. Frontend uses this repo's pnpm lock and CI. `svelte-check` has an upstream baseline; do not repair unrelated upstream errors.
