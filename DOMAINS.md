# Domains (fork-only)

Domains and subdomains, with domain-scoped roles. Fork-only work on `dgu`; see the distribution's ADR-006 for the design and its implementation plan for the delivery order. This file is the maintenance contract: every change to a file that exists upstream is listed under **Seams**, and every write channel is classified under **Write inventory**. Review both on every `main → dgu` sync.

## Seams

Each row is a change to a file that also exists upstream. A PR that adds, moves or removes a seam updates this table in the same commit.

| File | Function / location | Why |
| --- | --- | --- |

## Write inventory

Every operation that creates, changes, moves or deletes catalog content, and how domain-scoped write enforcement (delivery 2) covers it. `decorator` means the call goes through a service interface that `internal/core/domain` wraps in `server.go`.

### Assets (`asset.Service`)

| Channel | Call site | Methods | Coverage |
| --- | --- | --- | --- |
| REST | `internal/api/v1/assets/operations.go`, `tags.go`, `terms.go` | Create, Update, PatchFields, Delete, AddTag, RemoveTag, AddTerms, RemoveTerm | decorator |
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
| Ingestion | `internal/core/runs/service.go` | SyncTerms | decorator; requires global scope while any affected term sits outside `unassigned` |

### Gaps (not behind a decorated service yet)

| Channel | Call site | Permission today | Decision |
| --- | --- | --- | --- |
| Asset rules | `internal/api/v1/assetrules` and their evaluation | `assets:manage` | global scope only while enforcement is on |
| Documentation pages | `internal/api/v1/docs` (`/docs/entity/{entityType}/{entityId}/pages`, `/docs/pages/{pageId}`…) | `assets:manage` | open: scope by the owning entity's domain (needs a decorator or seam on the docs service) |
| Manual lineage edges | `internal/api/v1/lineage` (`/lineage/direct`, `/lineage/batch`) | `assets:manage` | open: which endpoint's domain governs a cross-domain edge |
| Observed lineage from agents | `internal/core/agent/service.go` → `lineage.Service.BatchObservedLineage` | `agents:emit` | open: same rule as manual edges |
| Asset docs | `assetdocs.Service` (wired in `server.go`) | to verify | verify what it writes before delivery 2 |

Out of scope: subscriptions and notifications (per-user), users, roles, teams, SSO and service accounts (global administration).
