package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/marmotdata/marmot/internal/core/auth"
)

const unassignedPath = "/" + UnassignedID + "/"

// Placement is the domain an entity belongs to.
type Placement struct {
	DomainID string
	Path     string
}

type AuditEntry struct {
	Actor      string    `json:"actor"`
	Action     string    `json:"action"`
	EntityKind string    `json:"entity_kind"`
	EntityID   string    `json:"entity_id"`
	FromDomain *string   `json:"from_domain,omitempty"`
	ToDomain   *string   `json:"to_domain,omitempty"`
	At         time.Time `json:"at"`
}

const (
	AuditAssign         = "assign"
	AuditAssignPipeline = "assign_pipeline"
	AuditMove           = "move"
)

// PrincipalFunc reads the acting principal from a context. The HTTP layer owns
// the context key, so server.go injects it.
type PrincipalFunc func(ctx context.Context) (auth.Principal, bool)

// Guard enforces domain-scoped writes once write enforcement is on. It is
// shared by the service decorators and the domain handlers.
type Guard struct {
	svc       Service
	repo      Repository
	principal PrincipalFunc
}

func NewGuard(svc Service, repo Repository, principal PrincipalFunc) *Guard {
	return &Guard{svc: svc, repo: repo, principal: principal}
}

// writer is who a write runs as. Every scope must allow it, so a caller that
// names a pipeline never gains what the pipeline's domain would add.
type writer struct {
	actor  string
	scopes []*Scope
	target string
}

func (w *writer) can(path string) bool {
	if len(w.scopes) == 0 {
		return false
	}
	for _, sc := range w.scopes {
		if !sc.Can(ActionWrite, path) {
			return false
		}
	}
	return true
}

func (w *writer) global() bool {
	if len(w.scopes) == 0 {
		return false
	}
	for _, sc := range w.scopes {
		if !sc.Global {
			return false
		}
	}
	return true
}

// writerFor resolves the acting principal and, during ingestion, the scope of
// the pipeline's domain. The scheduler runs without a principal, so there the
// pipeline alone decides. Without either, nothing may be written.
func (g *Guard) writerFor(ctx context.Context) (*writer, error) {
	w := &writer{actor: "anonymous", target: unassignedPath}
	if p, ok := g.principal(ctx); ok && p != nil {
		sc, err := g.svc.Scope(ctx, p)
		if err != nil {
			return nil, err
		}
		w.scopes = append(w.scopes, sc)
		w.actor = string(p.Type()) + ":" + p.ID()
	}
	if name, ok := PipelineFrom(ctx); ok {
		path, err := g.pipelinePath(ctx, name)
		if err != nil {
			return nil, err
		}
		w.scopes = append(w.scopes, &Scope{Grants: []Grant{{Path: path, Role: RoleSteward}}})
		w.target = path
		if len(w.scopes) == 1 {
			w.actor = "pipeline:" + name
		}
	}
	return w, nil
}

func (g *Guard) pipelinePath(ctx context.Context, name string) (string, error) {
	id, found, err := g.repo.PipelineDomain(ctx, name)
	if err != nil || !found {
		return unassignedPath, err
	}
	d, err := g.repo.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("resolving pipeline domain: %w", err)
	}
	return d.Path, nil
}

// Enforced reports whether domain-scoped writes are enforced.
func (g *Guard) Enforced(ctx context.Context) (bool, error) {
	return g.repo.WriteEnforced(ctx)
}

func (g *Guard) authorize(ctx context.Context, paths func(w *writer) ([]string, error)) error {
	on, err := g.repo.WriteEnforced(ctx)
	if err != nil || !on {
		return err
	}
	w, err := g.writerFor(ctx)
	if err != nil {
		return err
	}
	list, err := paths(w)
	if err != nil {
		return err
	}
	for _, p := range list {
		if !w.can(p) {
			return ErrForbidden
		}
	}
	return nil
}

// AuthorizeCreate checks a new entity's destination: the pipeline's domain
// during ingestion, Unassigned otherwise.
func (g *Guard) AuthorizeCreate(ctx context.Context) error {
	return g.authorize(ctx, func(w *writer) ([]string, error) { return []string{w.target}, nil })
}

// AuthorizeEntities checks the domains the entities are in now.
func (g *Guard) AuthorizeEntities(ctx context.Context, kind Kind, ids ...string) error {
	return g.authorize(ctx, func(*writer) ([]string, error) {
		placed, err := g.repo.Placements(ctx, kind, ids)
		if err != nil {
			return nil, err
		}
		paths := make([]string, 0, len(placed))
		for _, p := range placed {
			paths = append(paths, p.Path)
		}
		return paths, nil
	})
}

// AuthorizeAssetMRNs checks assets named by MRN. An MRN with no asset yet
// is a stub that would be created in Unassigned.
func (g *Guard) AuthorizeAssetMRNs(ctx context.Context, mrns ...string) error {
	return g.authorize(ctx, func(*writer) ([]string, error) {
		ids, err := g.repo.AssetIDsByMRN(ctx, mrns)
		if err != nil {
			return nil, err
		}
		var known []string
		var paths []string
		for _, m := range mrns {
			if id, ok := ids[m]; ok {
				known = append(known, id)
			} else {
				paths = append(paths, unassignedPath)
			}
		}
		placed, err := g.repo.Placements(ctx, KindAsset, known)
		if err != nil {
			return nil, err
		}
		for _, p := range placed {
			paths = append(paths, p.Path)
		}
		return paths, nil
	})
}

// AuthorizeDoc checks a documentation write against the entity that owns the
// page: the entity named in the path, or the owner of pageID or imageID. A page
// or image that does not exist is left for the handler to report.
func (g *Guard) AuthorizeDoc(ctx context.Context, entityType, entityID, pageID, imageID string) error {
	if entityType == "" {
		var found bool
		var err error
		entityType, entityID, found, err = g.repo.DocOwner(ctx, pageID, imageID)
		if err != nil || !found {
			return err
		}
	}
	switch entityType {
	case "asset":
		return g.AuthorizeAssetMRNs(ctx, entityID)
	case "data_product":
		return g.AuthorizeEntities(ctx, KindDataProduct, entityID)
	}
	return nil
}

// AuthorizeTerms checks a glossary sync: every existing term it rewrites, and
// the destination of the terms it creates.
func (g *Guard) AuthorizeTerms(ctx context.Context, names []string) error {
	return g.authorize(ctx, func(w *writer) ([]string, error) {
		placed, err := g.repo.TermPlacements(ctx, names)
		if err != nil {
			return nil, err
		}
		paths := make([]string, 0, len(names))
		for _, name := range names {
			if p, ok := placed[name]; ok {
				paths = append(paths, p.Path)
			} else {
				paths = append(paths, w.target)
			}
		}
		return paths, nil
	})
}

// AuthorizeGlobal is for writes that no domain owns, such as rules that match
// assets anywhere in the catalog.
func (g *Guard) AuthorizeGlobal(ctx context.Context) error {
	on, err := g.repo.WriteEnforced(ctx)
	if err != nil || !on {
		return err
	}
	w, err := g.writerFor(ctx)
	if err != nil {
		return err
	}
	if !w.global() {
		return ErrForbidden
	}
	return nil
}

func (g *Guard) destination(ctx context.Context, domainID string) (string, error) {
	if domainID == UnassignedID {
		return unassignedPath, nil
	}
	d, err := g.repo.Get(ctx, domainID)
	if err != nil {
		return "", err
	}
	return d.Path, nil
}

// Transfer assigns entities to a domain. With enforcement on, the actor must
// be able to write both where each entity is and where it goes. Every entity
// that changes domain is audited, enforced or not.
func (g *Guard) Transfer(ctx context.Context, kind Kind, ids []string, domainID string) error {
	to, err := g.destination(ctx, domainID)
	if err != nil {
		return err
	}
	placed, err := g.repo.Placements(ctx, kind, ids)
	if err != nil {
		return err
	}
	err = g.authorize(ctx, func(*writer) ([]string, error) {
		paths := []string{to}
		for _, p := range placed {
			paths = append(paths, p.Path)
		}
		return paths, nil
	})
	if err != nil {
		return err
	}
	if err := g.svc.Assign(ctx, kind, ids, domainID); err != nil {
		return err
	}
	actor, err := g.actor(ctx)
	if err != nil {
		return err
	}
	var entries []AuditEntry
	for _, id := range unique(ids) {
		from := placed[id].DomainID
		if from != domainID {
			entries = append(entries, auditEntry(actor, AuditAssign, string(kind), id, &from, &domainID))
		}
	}
	return g.repo.Audit(ctx, entries)
}

// AssignPipeline changes a pipeline's domain, and optionally moves the assets
// it ingested that are still in its previous domain. The actor must be able to
// write in both domains.
func (g *Guard) AssignPipeline(ctx context.Context, scheduleID, domainID string, moveAssets bool) (*PipelineMoveResult, error) {
	to, err := g.destination(ctx, domainID)
	if err != nil {
		return nil, err
	}
	exists, err := g.repo.EntityExists(ctx, KindIngestionSchedule, scheduleID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrEntityNotFound
	}
	placed, err := g.repo.Placements(ctx, KindIngestionSchedule, []string{scheduleID})
	if err != nil {
		return nil, err
	}
	from := placed[scheduleID]
	if err := g.authorize(ctx, func(*writer) ([]string, error) { return []string{from.Path, to}, nil }); err != nil {
		return nil, err
	}
	var moving []string
	if moveAssets {
		if moving, err = g.repo.PipelineAssetsIn(ctx, scheduleID, from.DomainID); err != nil {
			return nil, err
		}
	}
	result, err := g.svc.AssignPipeline(ctx, scheduleID, domainID, moveAssets)
	if err != nil {
		return nil, err
	}
	actor, err := g.actor(ctx)
	if err != nil {
		return nil, err
	}
	if from.DomainID == domainID {
		return result, nil
	}
	entries := []AuditEntry{auditEntry(actor, AuditAssignPipeline, string(KindIngestionSchedule), scheduleID, &from.DomainID, &domainID)}
	for _, id := range moving {
		entries = append(entries, auditEntry(actor, AuditAssignPipeline, string(KindAsset), id, &from.DomainID, &domainID))
	}
	return result, g.repo.Audit(ctx, entries)
}

// AuditMove records a domain moving under a new parent; nil is the root.
func (g *Guard) AuditMove(ctx context.Context, domainID string, from, to *string) error {
	actor, err := g.actor(ctx)
	if err != nil {
		return err
	}
	return g.repo.Audit(ctx, []AuditEntry{auditEntry(actor, AuditMove, "domain", domainID, from, to)})
}

func (g *Guard) AuditLog(ctx context.Context, entityKind, entityID string) ([]AuditEntry, error) {
	return g.repo.AuditLog(ctx, entityKind, entityID)
}

func (g *Guard) actor(ctx context.Context) (string, error) {
	w, err := g.writerFor(ctx)
	if err != nil {
		return "", err
	}
	return w.actor, nil
}

func auditEntry(actor, action, kind, id string, from, to *string) AuditEntry {
	return AuditEntry{Actor: actor, Action: action, EntityKind: kind, EntityID: id, FromDomain: from, ToDomain: to}
}
