package glossary

import (
	"context"
	"errors"
	"fmt"
)

// ImportTerm is one validated row of a bulk import: a term to create, or an
// existing one (ExistingID) to update. ParentName names the parent term,
// existing or another row of the same import, and is resolved when writing.
type ImportTerm struct {
	// Name is the term's name as the catalog will hold it, which parents in
	// the same batch refer to.
	Name       string
	ExistingID string
	Create     CreateTermInput
	Update     UpdateTermInput
	ParentName string
}

func (t ImportTerm) name() string {
	if t.Name != "" {
		return t.Name
	}
	if t.ExistingID != "" && t.Update.Name != nil {
		return *t.Update.Name
	}
	return t.Create.Name
}

func (s *service) GetByName(ctx context.Context, name string) (*GlossaryTerm, error) {
	term, err := s.repo.GetByName(ctx, name)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrTermNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, term.ID)
}

func (s *service) ByNames(ctx context.Context, names []string) (map[string][]*GlossaryTerm, error) {
	terms, err := s.repo.ByNames(ctx, names)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]*GlossaryTerm, len(terms))
	for _, t := range terms {
		out[t.Name] = append(out[t.Name], t)
	}
	return out, nil
}

// ErrImportCycle rejects an import whose parents form a loop.
var ErrImportCycle = fmt.Errorf("%w: parent terms form a cycle", ErrInvalidInput)

type txRunner interface {
	InTx(ctx context.Context, fn func(Repository) error) error
}

// Import writes every term or none. Each row goes through the same Create or
// Update as the API, inside one transaction, parents before children, so a
// row may name as parent a term created by another row.
func (s *service) Import(ctx context.Context, terms []ImportTerm) ([]*GlossaryTerm, error) {
	var written []*GlossaryTerm
	err := s.inTx(ctx, func(tx *service) error {
		var err error
		written, err = tx.importOrdered(ctx, terms)
		return err
	})
	if err != nil {
		return nil, err
	}
	if s.searchObserver != nil {
		for _, term := range written {
			s.searchObserver.OnEntityChanged(ctx, "glossary", term.ID)
		}
	}
	return written, nil
}

// inTx runs fn on a copy of the service bound to one transaction. The search
// observer is left out so nothing is indexed before the commit.
func (s *service) inTx(ctx context.Context, fn func(*service) error) error {
	runner, ok := s.repo.(txRunner)
	if !ok {
		tx := *s
		tx.searchObserver = nil
		return fn(&tx)
	}
	return runner.InTx(ctx, func(repo Repository) error {
		tx := *s
		tx.repo = repo
		tx.searchObserver = nil
		return fn(&tx)
	})
}

func (s *service) importOrdered(ctx context.Context, terms []ImportTerm) ([]*GlossaryTerm, error) {
	inImport := make(map[string]bool, len(terms))
	for _, t := range terms {
		inImport[t.name()] = true
	}
	ids := make(map[string]string, len(terms))
	pending := terms
	var written []*GlossaryTerm
	for len(pending) > 0 {
		var next []ImportTerm
		for _, t := range pending {
			parentID, ready, err := s.importParent(ctx, t.ParentName, inImport, ids)
			if err != nil {
				return nil, err
			}
			if !ready {
				next = append(next, t)
				continue
			}
			term, err := s.importOne(ctx, t, parentID)
			if err != nil {
				return nil, fmt.Errorf("term %q: %w", t.name(), err)
			}
			ids[term.Name] = term.ID
			written = append(written, term)
		}
		if len(next) == len(pending) {
			return nil, ErrImportCycle
		}
		pending = next
	}
	return written, nil
}

// importParent resolves a parent name to an ID. ready is false while the
// parent is a row of this import that has not been written yet.
func (s *service) importParent(ctx context.Context, name string, inImport map[string]bool, ids map[string]string) (id *string, ready bool, err error) {
	if name == "" {
		return nil, true, nil
	}
	if written, ok := ids[name]; ok {
		return &written, true, nil
	}
	if inImport[name] {
		return nil, false, nil
	}
	parent, err := s.repo.GetByName(ctx, name)
	if errors.Is(err, ErrNotFound) {
		return nil, false, fmt.Errorf("%w: parent term %q not found", ErrInvalidInput, name)
	}
	if err != nil {
		return nil, false, err
	}
	return &parent.ID, true, nil
}

func (s *service) importOne(ctx context.Context, t ImportTerm, parentID *string) (*GlossaryTerm, error) {
	if t.ExistingID == "" {
		in := t.Create
		in.ParentTermID = parentID
		return s.Create(ctx, in)
	}
	in := t.Update
	if parentID != nil {
		in.ParentTermID = parentID
	}
	return s.Update(ctx, t.ExistingID, in)
}
