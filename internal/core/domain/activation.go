package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/auth"
)

// ErrPlanChanged rejects an activation confirmed against a plan that no
// longer matches the catalog.
var ErrPlanChanged = errors.New("the enforcement plan changed since it was reviewed")

const (
	AuditEnableEnforcement  = "enable_enforcement"
	AuditDisableEnforcement = "disable_enforcement"
	auditSettingKind        = "setting"
)

type EnforcementState struct {
	Write     bool       `json:"write"`
	UpdatedBy *string    `json:"updated_by,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type DomainRef struct {
	ID string `json:"id"`
	// Path is the chain of names from the root, joined by " / ".
	Path string `json:"path"`
}

// EditorPrincipal is an identity that edits the catalog through native RBAC:
// assets:manage or glossary:manage, without the global admin role.
type EditorPrincipal struct {
	SubjectType SubjectType `json:"subject_type"`
	SubjectID   string      `json:"subject_id"`
	Name        string      `json:"name"`
	Permissions []string    `json:"permissions"`
}

// PlanPrincipal is an editor that would lose write access somewhere. Keeps and
// Loses are the topmost domains of each side, so a whole subtree is one entry.
type PlanPrincipal struct {
	EditorPrincipal
	Keeps []DomainRef `json:"keeps"`
	Loses []DomainRef `json:"loses"`
}

// PlanPipeline is a schedule whose ingested assets sit outside its domain;
// under enforcement its runs could no longer update or remove them.
type PlanPipeline struct {
	ScheduleID    string    `json:"schedule_id"`
	Name          string    `json:"name"`
	Domain        DomainRef `json:"domain"`
	AssetsOutside int       `json:"assets_outside"`
}

type EnforcementPlan struct {
	State       EnforcementState `json:"state"`
	Principals  []PlanPrincipal  `json:"principals"`
	Pipelines   []PlanPipeline   `json:"pipelines"`
	Hash        string           `json:"hash"`
	GeneratedAt time.Time        `json:"generated_at"`
}

type WritableDomains struct {
	Enforced  bool     `json:"enforced"`
	All       bool     `json:"all"`
	DomainIDs []string `json:"domain_ids"`
}

func (s *service) WritableDomains(ctx context.Context, p auth.Principal) (*WritableDomains, error) {
	state, err := s.repo.EnforcementState(ctx)
	if err != nil {
		return nil, err
	}
	scope, err := s.Scope(ctx, p)
	if err != nil {
		return nil, err
	}
	out := &WritableDomains{Enforced: state.Write, All: !state.Write || scope.Global, DomainIDs: []string{}}
	if out.All {
		return out, nil
	}
	domains, err := s.repo.All(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range domains {
		if scope.Can(ActionWrite, d.Path) {
			out.DomainIDs = append(out.DomainIDs, d.ID)
		}
	}
	return out, nil
}

func (s *service) Enforcement(ctx context.Context) (*EnforcementState, error) {
	return s.repo.EnforcementState(ctx)
}

// EnforcementPlan lists who would lose write access, and where, once writes
// are scoped by domain. Only global scope may read it: it enumerates identities
// across the whole catalog.
func (s *service) EnforcementPlan(ctx context.Context, p auth.Principal) (*EnforcementPlan, error) {
	if err := s.requireGlobal(ctx, p); err != nil {
		return nil, err
	}
	return s.plan(ctx)
}

func (s *service) plan(ctx context.Context) (*EnforcementPlan, error) {
	state, err := s.repo.EnforcementState(ctx)
	if err != nil {
		return nil, err
	}
	domains, err := s.repo.All(ctx)
	if err != nil {
		return nil, err
	}
	names := namePaths(domains)
	byID := make(map[string]*Domain, len(domains))
	for _, d := range domains {
		byID[d.ID] = d
	}

	editors, err := s.repo.EditorPrincipals(ctx)
	if err != nil {
		return nil, err
	}
	plan := &EnforcementPlan{State: *state, Principals: []PlanPrincipal{}, Pipelines: []PlanPipeline{}, GeneratedAt: time.Now().UTC()}
	for _, e := range editors {
		grants, err := s.repo.Grants(ctx, e.SubjectType, e.SubjectID)
		if err != nil {
			return nil, err
		}
		scope := &Scope{Grants: grants}
		pp := PlanPrincipal{EditorPrincipal: e, Keeps: []DomainRef{}, Loses: []DomainRef{}}
		for _, d := range domains {
			can := scope.Can(ActionWrite, d.Path)
			parentCan := d.ParentID != nil && scope.Can(ActionWrite, byID[*d.ParentID].Path)
			switch {
			case can && !parentCan:
				pp.Keeps = append(pp.Keeps, DomainRef{ID: d.ID, Path: names[d.ID]})
			case !can && (d.ParentID == nil || parentCan):
				pp.Loses = append(pp.Loses, DomainRef{ID: d.ID, Path: names[d.ID]})
			}
		}
		if len(pp.Loses) > 0 {
			plan.Principals = append(plan.Principals, pp)
		}
	}

	pipelines, err := s.repo.PipelinesOutsideDomain(ctx)
	if err != nil {
		return nil, err
	}
	for _, pl := range pipelines {
		pl.Domain.Path = names[pl.Domain.ID]
		plan.Pipelines = append(plan.Pipelines, pl)
	}
	plan.Hash = planHash(plan)
	return plan, nil
}

// SetWriteEnforcement turns domain-scoped writes on or off. Turning them on
// needs the hash of the plan the caller reviewed, so a plan that changed in
// between is never activated blind. Both directions are audited.
func (s *service) SetWriteEnforcement(ctx context.Context, p auth.Principal, on bool, confirm string) (*EnforcementState, error) {
	if err := s.requireGlobal(ctx, p); err != nil {
		return nil, err
	}
	if on {
		plan, err := s.plan(ctx)
		if err != nil {
			return nil, err
		}
		if confirm != plan.Hash {
			return nil, ErrPlanChanged
		}
	}
	actor := string(p.Type()) + ":" + p.ID()
	if err := s.repo.SetWriteEnforced(ctx, on, actor); err != nil {
		return nil, err
	}
	action := AuditDisableEnforcement
	if on {
		action = AuditEnableEnforcement
	}
	if err := s.repo.Audit(ctx, []AuditEntry{{Actor: actor, Action: action, EntityKind: auditSettingKind, EntityID: writeEnforcementKey}}); err != nil {
		return nil, err
	}
	return s.repo.EnforcementState(ctx)
}

func (s *service) requireGlobal(ctx context.Context, p auth.Principal) error {
	scope, err := s.Scope(ctx, p)
	if err != nil {
		return err
	}
	if !scope.Global {
		return ErrForbidden
	}
	return nil
}

// planHash covers what an operator decides on: who loses what, and which
// pipelines are affected. Names and timestamps are left out so a rename does
// not invalidate a reviewed plan.
func planHash(plan *EnforcementPlan) string {
	type principal struct {
		Type  SubjectType
		ID    string
		Perms []string
		Loses []string
	}
	type pipeline struct {
		ID      string
		Domain  string
		Outside int
	}
	var ps []principal
	for _, p := range plan.Principals {
		loses := make([]string, 0, len(p.Loses))
		for _, d := range p.Loses {
			loses = append(loses, d.ID)
		}
		sort.Strings(loses)
		perms := append([]string(nil), p.Permissions...)
		sort.Strings(perms)
		ps = append(ps, principal{p.SubjectType, p.SubjectID, perms, loses})
	}
	sort.Slice(ps, func(i, j int) bool {
		return string(ps[i].Type)+ps[i].ID < string(ps[j].Type)+ps[j].ID
	})
	var pls []pipeline
	for _, pl := range plan.Pipelines {
		pls = append(pls, pipeline{pl.ScheduleID, pl.Domain.ID, pl.AssetsOutside})
	}
	sort.Slice(pls, func(i, j int) bool { return pls[i].ID < pls[j].ID })
	data, _ := json.Marshal(struct {
		Principals []principal
		Pipelines  []pipeline
	}{ps, pls})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16])
}

// namePaths maps each domain to its chain of names; domains arrive ordered by
// depth, so every parent is resolved before its children.
func namePaths(domains []*Domain) map[string]string {
	out := make(map[string]string, len(domains))
	for _, d := range domains {
		if d.ParentID != nil {
			out[d.ID] = strings.Join([]string{out[*d.ParentID], d.Name}, " / ")
		} else {
			out[d.ID] = d.Name
		}
	}
	return out
}
