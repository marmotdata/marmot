## Summary

<!--
ES: Una o dos frases. El título del PR sigue Conventional Commits (feat:, fix:, docs:, chore:, ci:, …).
EN: One or two sentences. The PR title follows Conventional Commits (feat:, fix:, docs:, chore:, ci:, …).
-->



## Why

<!--
ES: Qué invariante o fallo cubre. No describas el diff.
EN: Which invariant or bug this covers. Do not narrate the diff.
-->



## Changes

<!--
ES: Lista corta. Separa kernel genérico (registro, API, UI nativa) de perfil/locales corporativos: esos no van aquí.
EN: Short list. Keep generic kernel work (registry, API, native UI) separate from corporate profile/locales: those do not land here.
-->

-

## Checklist

- [ ] `make test` (or `go test` of the packages touched)
- [ ] `make server-lint` if Go changed
- [ ] `make frontend-lint` if `web/marmot` changed
- [ ] `make swagger` if a public HTTP contract changed; do not hand-edit `docs/swagger.*`
- [ ] Native identity is intact: MRNs, ownership relations, tags, `description` / `user_description`
- [ ] Unknown metadata is preserved; YAML is trusted config, not executable code
- [ ] Go returns field IDs and error codes; translations stay in the consuming distribution
- [ ] Area isolation is **not** claimed unless the acceptance matrix for scoped reads/writes/indirect access passed
- [ ] No secrets, `.env`, or tenant data
- [ ] Unrelated upstream `svelte-check` failures were not “fixed” in this PR

<!--
ES: `pnpm check` / `svelte-check` no es gate de este repo. No uses checks de la distribución (`pnpm check:upstream`, `webOverlay`).
EN: `pnpm check` / `svelte-check` is not a gate here. Do not use distribution checks (`pnpm check:upstream`, `webOverlay`).
-->

## API

<!--
ES: Rutas nuevas o cambiadas, precondiciones (If-Match), códigos (400/412/428). Vacío si no aplica.
EN: New or changed routes, preconditions (If-Match), status codes (400/412/428). Leave empty if N/A.
-->



## Persistence

<!--
ES: Migración goose (`---- create above / drop below ----`). Expansiva primero; sin NOT NULL indiscriminado ni ON DELETE CASCADE en áreas.
EN: Goose migration (`---- create above / drop below ----`). Expansive first; no blanket NOT NULL and no ON DELETE CASCADE on areas.
-->



## Test plan

<!--
ES: Qué corriste y qué no (p. ej. sin Postgres de integración). Incluye un caso de rechazo, no solo el feliz.
EN: What you ran and what you skipped (e.g. no integration Postgres). Include a rejection case, not only the happy path.
-->

- [ ]

## Breaking / removed behaviour

<!--
ES: Cliente antiguo, perfil ausente, reingesta. Vacío si no aplica.
EN: Legacy clients, missing profile, re-ingestion. Leave empty if N/A.
-->

N/A
