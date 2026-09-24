package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/auth"
)

type Role string

const (
	RoleDomainAdmin Role = "domain_admin"
	RoleSteward     Role = "steward"
	RoleReader      Role = "reader"
)

type SubjectType string

const (
	SubjectUser           SubjectType = "user"
	SubjectTeam           SubjectType = "team"
	SubjectServiceAccount SubjectType = "service_account"
)

type Action string

const (
	// ActionWrite edits entities in the subtree.
	ActionWrite Action = "write"
	// ActionAdmin manages subdomains and role assignments in the subtree.
	ActionAdmin Action = "admin"
)

// reader grants nothing here: it only matters for restricted domains, which
// are not enforced yet (delivery 3).
var roleActions = map[Role][]Action{
	RoleDomainAdmin: {ActionWrite, ActionAdmin},
	RoleSteward:     {ActionWrite},
	RoleReader:      {},
}

var (
	ErrForbidden = errors.New("not allowed in this domain")
	ErrDuplicate = errors.New("the subject already has this role in this domain")
)

type RoleAssignment struct {
	ID          string      `json:"id"`
	DomainID    string      `json:"domain_id"`
	DomainName  string      `json:"domain_name"`
	SubjectType SubjectType `json:"subject_type"`
	SubjectID   string      `json:"subject_id"`
	SubjectName string      `json:"subject_name,omitempty"`
	// SubjectMissing marks a row whose user, team or service account was deleted.
	SubjectMissing bool      `json:"subject_missing"`
	Role           Role      `json:"role"`
	Inherited      bool      `json:"inherited"`
	CreatedBy      *string   `json:"created_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type GrantInput struct {
	SubjectType SubjectType `json:"subject_type"`
	SubjectID   string      `json:"subject_id"`
	Role        Role        `json:"role"`
}

// Grant is a role held over a domain subtree, identified by its path.
type Grant struct {
	Path string
	Role Role
}

// Scope is what one principal may do, by domain subtree.
type Scope struct {
	Global bool
	Grants []Grant
}

func (sc *Scope) Can(action Action, domainPath string) bool {
	if sc == nil {
		return false
	}
	if sc.Global {
		return true
	}
	for _, g := range sc.Grants {
		if !strings.HasPrefix(domainPath, g.Path) {
			continue
		}
		for _, a := range roleActions[g.Role] {
			if a == action {
				return true
			}
		}
	}
	return false
}

// Scope resolves a principal's grants: its own, and for a user those of its
// teams. The operator and native admins act globally. It reads the database on
// every call so a revocation applies to the next request.
func (s *service) Scope(ctx context.Context, p auth.Principal) (*Scope, error) {
	if p == nil {
		return &Scope{}, nil
	}
	if p.Type() == auth.PrincipalTypeOperator || p.IsAdmin() {
		return &Scope{Global: true}, nil
	}
	var subject SubjectType
	switch p.Type() {
	case auth.PrincipalTypeUser:
		subject = SubjectUser
	case auth.PrincipalTypeServiceAccount:
		subject = SubjectServiceAccount
	default:
		return &Scope{}, nil
	}
	grants, err := s.repo.Grants(ctx, subject, p.ID())
	if err != nil {
		return nil, fmt.Errorf("resolving domain grants: %w", err)
	}
	return &Scope{Grants: grants}, nil
}

func validGrant(in GrantInput) error {
	switch in.SubjectType {
	case SubjectUser, SubjectTeam, SubjectServiceAccount:
	default:
		return fmt.Errorf("%w: unknown subject type %q", ErrInvalidInput, in.SubjectType)
	}
	if _, ok := roleActions[in.Role]; !ok {
		return fmt.Errorf("%w: unknown role %q", ErrInvalidInput, in.Role)
	}
	if strings.TrimSpace(in.SubjectID) == "" {
		return fmt.Errorf("%w: subject id is required", ErrInvalidInput)
	}
	return nil
}

func (s *service) authorizeAdmin(ctx context.Context, p auth.Principal, domainID string) (*Domain, error) {
	d, err := s.repo.Get(ctx, domainID)
	if err != nil {
		return nil, err
	}
	scope, err := s.Scope(ctx, p)
	if err != nil {
		return nil, err
	}
	if !scope.Can(ActionAdmin, d.Path) {
		return nil, ErrForbidden
	}
	return d, nil
}

// Roles lists the assignments that apply to a domain: its own and those
// inherited from its ancestors.
func (s *service) Roles(ctx context.Context, domainID string) ([]RoleAssignment, error) {
	d, err := s.repo.Get(ctx, domainID)
	if err != nil {
		return nil, err
	}
	return s.repo.Roles(ctx, d)
}

// GrantRole needs domain_admin on the domain or an ancestor, or global scope;
// a domain admin can therefore never grant beyond its own subtree.
func (s *service) GrantRole(ctx context.Context, p auth.Principal, domainID string, in GrantInput) (*RoleAssignment, error) {
	if err := validGrant(in); err != nil {
		return nil, err
	}
	d, err := s.authorizeAdmin(ctx, p, domainID)
	if err != nil {
		return nil, err
	}
	exists, err := s.repo.SubjectExists(ctx, in.SubjectType, in.SubjectID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrEntityNotFound
	}
	createdBy := ""
	if p != nil {
		createdBy = p.ID()
	}
	return s.repo.GrantRole(ctx, d, in, createdBy)
}

func (s *service) RevokeRole(ctx context.Context, p auth.Principal, domainID, assignmentID string) error {
	if _, err := s.authorizeAdmin(ctx, p, domainID); err != nil {
		return err
	}
	return s.repo.RevokeRole(ctx, domainID, assignmentID)
}
