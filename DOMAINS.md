# Domains (fork-only)

Domains and subdomains, with domain-scoped roles. Fork-only work on `dgu`; see the distribution's ADR-006 for the design and its implementation plan for the delivery order. This file is the maintenance contract: every change to a file that exists upstream is listed under **Seams**, and every write channel is classified under **Write inventory**. Review both on every `main → dgu` sync.

## Seams

Each row is a change to a file that also exists upstream. A PR that adds, moves or removes a seam updates this table in the same commit.

| File | Function / location | Why |
| --- | --- | --- |
| `internal/store/postgres/setup.go` | `Setup.Initialize`, last statement | Runs the fork migration track (`dgumigrations`, table `public.dgu_schema_version`) after the core one, so fork tables never take an upstream migration number |
| `internal/api/v1/server.go` | `New`, the `config.Domains.Enabled` block after the `server.handlers` list; two imports | Only with `domains.enabled`: registers `/api/v1/domains`, adds the ingestion observer to `asset.Service`, sets the search domain resolver, and refuses to start with the Elasticsearch search backend |
| `internal/api/v1/server.go` | `New`: the block after `asset.NewService` that builds the domain service and guard, and one `if domainGuard != nil` after each of `lineageService.NewService`, `assetdocs.NewService`, `glossaryService.NewService`, `dataProductSvc.SetMetamodel` and `assetruleService.NewService` | Wraps each service in its write decorator before any consumer receives it, so REST, OpenLineage, ingestion, agents and MCP all go through the guard |
| `internal/api/v1/server.go` | the `config.Domains.Enabled` block: replaces the docs, assets, data products and glossary handlers in `server.handlers` | Route middleware: `domainsAPI.GuardDocs` scopes documentation pages and `domainsAPI.WithCreateTargets` reads `?domain_id=` on the create routes. The handler list itself is untouched |
| `internal/api/v1/assets/operations.go`, `assets/terms.go`, `assets/documentation.go`, `dataproducts/operations.go`, `glossary/operations.go`, `assetrules/operations.go`, `lineage/operations.go` | the error `switch` after each decorated write, and `respondAssetWriteError`; one import each | `domain.ErrForbidden` → 403 instead of 500 |
| `internal/core/runs/service.go` | `ProcessEntities`, first statement; one import | Puts the pipeline name in the context so new assets inherit the domain of the schedule with that name |
| `internal/core/lineage/service.go` | `service.edgeGuard` and the check at the top of `CreateDirectLineage`; the option lives in the new file `edge_guard.go` | Vets every edge, including those an OpenLineage event writes internally, which never go through a decorator |
| `internal/core/search/store.go` | `PostgresRepository.domainResolver`; `Search` resolves `@domain` first; `buildFilterClauses` and `buildListingFacetWhereClause` call `appendDomainClauses`; `buildFacetsParallel` skips cached facets when `filter.Domain` is set | `@domain` filter over subtrees, by id, name or name path, and `NOT @domain` exclusion. Cached facets are global counts and would ignore it |
| `internal/core/search/service.go` | `Filter.Domain` | Carries the resolved domain filter; never read from JSON |
| `pkg/config/config.go` | `Config.Domains`; `BindEnv("domains.enabled")`; `SetDefault("domains.enabled", false)` | The feature flag (`MARMOT_DOMAINS_ENABLED`) |
| `pkg/config/config_test.go` | `TestLoad_DCRAllowedRedirectHostsFromEnv` | Asserts the flag is read from the environment; `Load` runs once per process, so it cannot live in its own test |
| `permissions`, `role_permissions` (data, fork migration `002`) | rows `dgu_view_domains`, `dgu_manage_domains` | `domains:view` for `admin` and `user`, `domains:manage` for `admin`. Names are `dgu_`-prefixed so an upstream permission with the same name cannot collide |
| `charts/marmot/values.yaml`, `values.schema.json`, `templates/validation.yaml`, `tests/{configmap,validation}_test.yaml` | `config.domains`; the Elasticsearch check | The flag through the chart; refuses to render domains with Elasticsearch, as the server refuses to start |
| `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` | generated | Include the domain endpoints. On a sync conflict, regenerate with `make swagger` |

### Web seams

Fork-only UI lives in `web/marmot/src/lib/domains/`, `components/domain/` and `routes/domains/`. These are the upstream files it touches:

| File | Location | Why |
| --- | --- | --- |
| `web/marmot/messages/{en,es}.json` | `domains_*` keys | Paraglide catalogue; keys sorted, so a sync conflict is a line merge |
| `routes/+layout.svelte` | Governance menu entry and `pagetitle_domains` | Navigation |
| `routes/discover/+page.svelte` | `DomainFilter` in the filter column | `@domain` filter |
| `routes/discover/[type]/[service]/[name]/+page.svelte` | `DomainChip`; `domainWrite` joins `canManageAssets` | Shows the domain; hides edits the asset's domain does not allow |
| `routes/products/[id]/+page.svelte` | `DomainChip`; `domainWrite` joins `canManage` | Same for products |
| `routes/glossary/[[id]]/+page.svelte` | `DomainChip`, `DomainSelect` in the create modal with `?domain_id=` on the create request; `canEditTerm` for the selected term's edits | Same for terms; creating stays on `glossary:manage` |
| `components/asset/AssetBlade.svelte`, `components/product/ProductBlade.svelte` | read-only `DomainChip` | Domain in the Discover side panels |
| `components/product/DataProductForm.svelte`, `routes/assets/new/+page.svelte` | `DomainSelect`; `?domain_id=` on the create request | Domain on creation |
| `routes/pipelines/new/+page.svelte`, `routes/pipelines/[id]/edit/+page.svelte` | `PipelineDomain` | Pipeline domain, with the explicit asset move |
| `components/query/QueryBuilder.svelte`, `components/query/QueryInput.svelte` | `@domain` field, its values (shown by label, inserted by value) and its `=` operator | Query builder and `@` autocomplete |

## Write inventory

Every operation that creates, changes, moves or deletes catalog content, and how domain-scoped write enforcement (delivery 2) covers it. `decorator` means the call goes through a service interface that `internal/core/domain` wraps in `server.go` (`decorators.go`).

### How enforcement decides

- The switch is the `write_enforcement` row in `domain_settings`, read on every write. Off, the decorators delegate without checking. The flag `domains.enabled` only turns the feature on; it never enforces anything by itself.
- On, a write needs `write` (steward or domain admin) on the entity's current domain; a new entity needs it on its destination: the `?domain_id=` target of the create request, else the pipeline's domain during ingestion, else Unassigned.
- `POST /api/v1/assets/`, `/products/` and `/glossary/` accept an optional `?domain_id=`, enforced or not. The entity is created and placed there, and the placement is audited (`create`); if it cannot be placed, it is deleted rather than left in Unassigned.
- The actor is the principal in the context. A scheduled run has none and writes with the scope of its schedule's domain (Unassigned if the schedule has none). A caller that names a pipeline over HTTP needs both its own scope and the pipeline's, so a pipeline name never grants anything.
- No principal and no pipeline: denied.
- Moving entities between domains (`PUT /domains/{id}/members`, pipeline reassignment) needs `write` on both ends. Every change of domain goes to `domain_audit_log`, enforced or not, and so do domain moves.

### Turning it on

- `GET /api/v1/domains/enforcement/plan` (global scope only) lists the identities with `assets:manage` or `glossary:manage`, outside the admin role, that would lose write access, with the topmost domains they keep and lose. It also lists the pipelines whose ingested assets sit outside their domain: their runs could no longer update or remove them.
- `POST /api/v1/domains/enforcement` with `{"write": true, "confirm": "<plan hash>"}` turns it on. A plan that changed since it was reviewed is refused with `plan_changed`. `{"write": false}` turns it off. Both need global scope and both are audited (`entity_kind = setting`).
- `GET /api/v1/domains/enforcement` returns the state to anyone with `domains:view`.
- CLI: `marmot domains enforcement status | plan | enable --confirm <hash> | disable` (`internal/cmd/domains.go`, fork-only; it calls the API directly because domains are not in the generated SDK).

### Assets (`asset.Service`)

| Channel | Call site | Methods | Coverage |
| --- | --- | --- | --- |
| REST | `internal/api/v1/assets/operations.go`, `tags.go`, `terms.go` | Create, Update, PatchFields, Delete, AddTag, RemoveTag, AddTerms, RemoveTerm | decorator; linking a term changes the asset, so only the asset's domain counts |
| OpenLineage | `internal/core/lineage/openlineage.go` | Create, Update | decorator; the service account needs a role on the target domain (`unassigned` for new stubs) |
| Ingestion | `internal/core/runs/service.go` | Create, Update, DeleteByMRN, AddTerms | decorator with an explicit principal scoped to the run's schedule domain |
| MCP | `internal/mcp/write_tools.go` | Update (user description), AddTag, RemoveTag | decorator; MCP requests carry the caller's principal |

### Data products (`dataproduct.Service`)

| Channel | Call site | Methods | Coverage |
| --- | --- | --- | --- |
| REST | `internal/api/v1/dataproducts/operations.go` | Create, Update, Delete, AddAssets, RemoveAsset, UploadImage, DeleteImage | decorator; membership changes are checked against the **product's** domain |
| REST | same | CreateRule, UpdateRule, DeleteRule | global scope only while enforcement is on |
| Rule engine | `NewMembershipService` / `NewReconciler` (wired in `server.go`) | derived memberships | not scoped: derived from rules that only global scope can author |

### Glossary terms (`glossary.Service`)

| Channel | Call site | Methods | Coverage |
| --- | --- | --- | --- |
| REST | `internal/api/v1/glossary/operations.go` | Create, Update, Delete | decorator |
| Ingestion | `internal/core/runs/service.go` | SyncTerms | decorator; all or nothing: every existing term in the batch must be writable, and new terms land in the pipeline's domain |

### Lineage and documentation

| Channel | Call site | Methods | Coverage |
| --- | --- | --- | --- |
| REST, ingestion | `internal/api/v1/lineage` (`/lineage/direct`, `/lineage/batch`), `internal/core/runs/service.go` | `lineage.Service` CreateDirectLineage, DeleteDirectLineage | decorator (`GuardLineage`): the **target** asset's domain decides; the downstream side declares what it reads. An MRN with no asset yet counts as Unassigned |
| Agents | `internal/core/agent/service.go` | `lineage.Service` BatchObservedLineage | decorator; all edges or none. The run record is kept and the agent gets the error text, as for any lineage failure |
| OpenLineage | `internal/core/lineage/openlineage.go` | edges written inside `ProcessOpenLineageEvent` | `lineage.WithEdgeGuard`: each edge by its target, like any other; a refused edge is skipped with a warning and the rest of the event is processed |
| REST | `internal/api/v1/assets/documentation.go` | `assetdocs.Service` Create, CreateGlobal | decorator (`GuardAssetDocs`): the asset's domain; global documentation needs global scope |
| REST | `internal/api/v1/docs` (pages and images) | `docs.Service`, a concrete type | `domainsAPI.GuardDocs` adds a check as the innermost middleware of every write route: the page's owner (asset by MRN, or data product) decides |
| Asset rules | `internal/api/v1/assetrules` | `assetrule.Service` Create, Update, Delete | decorator (`GuardAssetRules`): global scope while enforcement is on. Their evaluation writes derived links and is not scoped |

Out of scope: subscriptions and notifications (per-user), users, roles, teams, SSO and service accounts (global administration).

## Known gaps

- Elasticsearch: `@domain` is only applied by the Postgres search backend, so the server refuses to start with `domains.enabled` and Elasticsearch together.
