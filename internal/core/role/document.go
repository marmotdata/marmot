package role

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// A role can be written as a document, by hand or in an editor, naming its permissions by key ("assets:view") rather than by the ids the permissions table assigns, which differ between instances.

// ErrUnknownPermission means a document names a permission key that no permission has.
var ErrUnknownPermission = errors.New("unknown permission")

// PermissionKey is a permission's "resource_type:action" form.
func PermissionKey(p Permission) string { return p.ResourceType + ":" + p.Action }

// withPermissionKeys fills in the keys of the permissions a role already carries, so a role read back can be written straight back as a document.
func withPermissionKeys(r *Role) *Role {
	if r == nil {
		return nil
	}
	keys := make([]string, 0, len(r.Permissions))
	for _, p := range r.Permissions {
		keys = append(keys, PermissionKey(p))
	}
	slices.Sort(keys)
	r.PermissionKeys = keys
	return r
}

// DocumentProblem locates one thing wrong with a role document.
type DocumentProblem struct {
	Path    string `json:"path"`
	Message string `json:"message"`
} // @name RoleDocumentProblem

// ResolvePermissionKeys turns permission keys into ids, refusing any key that names no permission.
func (s *service) ResolvePermissionKeys(ctx context.Context, keys []string) ([]string, error) {
	ids, problems, err := s.resolveKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrUnknownPermission, problems[0].Message)
	}
	return ids, nil
}

func (s *service) resolveKeys(ctx context.Context, keys []string) ([]string, []DocumentProblem, error) {
	if len(keys) == 0 {
		return nil, nil, nil
	}
	perms, err := s.store.ListPermissions(ctx)
	if err != nil {
		return nil, nil, err
	}
	byKey := make(map[string]string, len(perms))
	for _, p := range perms {
		byKey[PermissionKey(p)] = p.ID
	}
	var ids []string
	var problems []DocumentProblem
	for i, key := range keys {
		id, ok := byKey[strings.TrimSpace(key)]
		if !ok {
			problems = append(problems, DocumentProblem{
				Path:    fmt.Sprintf("permission_keys[%d]", i),
				Message: fmt.Sprintf("no permission is called %q", key),
			})
			continue
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids, problems, nil
}

// ValidateDocument checks a role document without saving it, returning every problem found. existingID is the role being edited, or empty for a new one, so a role keeping its own name is not reported as a clash.
func (s *service) ValidateDocument(ctx context.Context, input CreateInput, existingID string) ([]DocumentProblem, error) {
	var problems []DocumentProblem
	name := strings.TrimSpace(input.Name)
	if name == "" {
		problems = append(problems, DocumentProblem{Path: "name", Message: "a role needs a name"})
	} else {
		roles, err := s.store.List(ctx, false)
		if err != nil {
			return nil, err
		}
		for _, r := range roles {
			if strings.EqualFold(r.Name, name) && r.ID != existingID {
				problems = append(problems, DocumentProblem{Path: "name", Message: fmt.Sprintf("a role called %q already exists", r.Name)})
				break
			}
		}
	}
	_, keyProblems, err := s.resolveKeys(ctx, input.PermissionKeys)
	if err != nil {
		return nil, err
	}
	return append(problems, keyProblems...), nil
}
