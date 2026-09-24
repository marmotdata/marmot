# Domains (fork-only)

Domains and subdomains, with domain-scoped roles. Fork-only work on `dgu`; see the distribution's ADR-006 for the design and its implementation plan for the delivery order. This file is the maintenance contract: every change to a file that exists upstream is listed under **Seams**, and every write channel is classified under **Write inventory**. Review both on every `main → dgu` sync.

## Seams

Each row is a change to a file that also exists upstream. A PR that adds, moves or removes a seam updates this table in the same commit.

| File | Function / location | Why |
| --- | --- | --- |
| `internal/store/postgres/setup.go` | `Setup.Initialize`, last statement | Runs the fork migration track (`dgumigrations`, table `public.dgu_schema_version`) after the core one, so fork tables never take an upstream migration number |
| `internal/api/v1/server.go` | `New`, the `config.Domains.Enabled` block after the `server.handlers` list; two imports | Only with `domains.enabled`: registers `/api/v1/domains`, adds the ingestion observer to `asset.Service`, sets the search domain resolver, and refuses to start with the Elasticsearch search backend |
| `internal/api/v1/server.go` | `New`: the block after `asset.NewService` that builds the domain service and guard, and one `if domainGuard != nil` after each of `lineageService.NewService`, `assetdocs.NewService`, `glossaryService.NewService`, `dataProductSvc.SetMetamodel` and `assetruleService.NewService` | Wraps each service in its write decorator before any consumer receives it, so REST, OpenLineage, ingestion, agents and MCP all go through the guard |
| `internal/api/v1/server.go` | the `config.Domains.Enabled` block: replaces the `*docsAPI.Handler` in `server.handlers` | Documentation pages are scoped by route middleware (`domainsAPI.GuardDocs`); the handler list itself is untouched |
| `internal/api/v1/assets/operations.go`, `assets/terms.go`, `assets/documentation.go`, `dataproducts/operations.go`, `glossary/operations.go`, `assetrules/operations.go`, `lineage/operations.go` | the error `switch` after each decorated write, and `respondAssetWriteError`; one import each | `domain.ErrForbidden` → 403 instead of 500 |
| `internal/core/runs/service.go` | `ProcessEntities`, first statement; one import | Puts the pipeline name in the context so new assets inherit the domain of the schedule with that name |
| `internal/core/search/store.go` | `PostgresRepository.domainResolver`; `Search` resolves `@domain` first; `buildFilterClauses` and `buildListingFacetWhereClause` call `appendDomainClauses`; `buildFacetsParallel` skips cached facets when `filter.Domain` is set | `@domain:<id>` filter over a subtree. Cached facets are global counts and would ignore it |
| `internal/core/search/service.go` | `Filter.Domain` | Carries the resolved domain filter; never read from JSON |
| `pkg/config/config.go` | `Config.Domains`; `BindEnv("domains.enabled")`; `SetDefault("domains.enabled", false)` | The feature flag (`MARMOT_DOMAINS_ENABLED`) |
| `pkg/config/config_test.go` | `TestLoad_DCRAllowedRedirectHostsFromEnv` | Asserts the flag is read from the environment; `Load` runs once per process, so it cannot live in its own test |
| `permissions`, `role_permissions` (data, fork migration `002`) | rows `dgu_view_domains`, `dgu_manage_domains` | `domains:view` for `admin` and `user`, `domains:manage` for `admin`. Names are `dgu_`-prefixed so an upstream permission with the same name cannot collide |
| `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` | generated | Include the domain endpoints. On a sync conflict, regenerate with `make swagger` |

## Write inventory

Every operation that creates, changes, moves or deletes catalog content, and how domain-scoped write enforcement (delivery 2) covers it. `decorator` means the call goes through a service interface that `internal/core/domain` wraps in `server.go` (`decorators.go`).

### How enforcement decides

- The switch is the `write_enforcement` row in `domain_settings`, read on every write. Off, the decorators delegate without checking. The flag `domains.enabled` only turns the feature on; it never enforces anything by itself.
- On, a write needs `write` (steward or domain admin) on the entity's current domain; a new entity needs it on its destination: the pipeline's domain during ingestion, Unassigned otherwise.
- The actor is the principal in the context. A scheduled run has none and writes with the scope of its schedule's domain (Unassigned if the schedule has none). A caller that names a pipeline over HTTP needs both its own scope and the pipeline's, so a pipeline name never grants anything.
- No principal and no pipeline: denied.
- Moving entities between domains (`PUT /domains/{id}/members`, pipeline reassignment) needs `write` on both ends. Every change of domain goes to `domain_audit_log`, enforced or not, and so do domain moves.

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
| OpenLineage | `internal/core/lineage/openlineage.go` | edges written inside `ProcessOpenLineageEvent` | not checked per edge; the assets the event creates or updates go through the asset decorator, so an emitter without a role on the output's domain fails there |
| REST | `internal/api/v1/assets/documentation.go` | `assetdocs.Service` Create, CreateGlobal | decorator (`GuardAssetDocs`): the asset's domain; global documentation needs global scope |
| REST | `internal/api/v1/docs` (pages and images) | `docs.Service`, a concrete type | `domainsAPI.GuardDocs` adds a check as the innermost middleware of every write route: the page's owner (asset by MRN, or data product) decides |
| Asset rules | `internal/api/v1/assetrules` | `assetrule.Service` Create, Update, Delete | decorator (`GuardAssetRules`): global scope while enforcement is on. Their evaluation writes derived links and is not scoped |

Out of scope: subscriptions and notifications (per-user), users, roles, teams, SSO and service accounts (global administration).

## Known gaps

- Elasticsearch: `@domain` is only applied by the Postgres search backend, so the server refuses to start with `domains.enabled` and Elasticsearch together.
- Helm: `charts/marmot/values.schema.json` rejects unknown `config` keys, so `config.domains.enabled` cannot be set through the chart yet. Set `MARMOT_DOMAINS_ENABLED` through the chart's `env` value instead.
