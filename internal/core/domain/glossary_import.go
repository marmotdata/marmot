package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/glossary/importer"
)

// ImportColumn is the glossary import column that places each term.
const ImportColumn = "domain"

var errDomainNotFound = errors.New("domain not found")

// resolveOne turns a domain reference (name or name path, ignoring case) into
// the single domain it names.
func resolveOne(ctx context.Context, svc Service, ref string) (*Domain, error) {
	found, err := svc.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	switch len(found) {
	case 0:
		return nil, errDomainNotFound
	case 1:
		return found[0], nil
	}
	return nil, fmt.Errorf("%w: %d domains are named %q; write the path from the root", ErrInvalidInput, len(found), ref)
}

type glossaryDomains struct {
	svc   Service
	guard *Guard
}

// GlossaryImportColumns adds a domain column to glossary imports, exports and
// templates.
func GlossaryImportColumns(svc Service, guard *Guard) importer.ColumnProvider {
	return &glossaryDomains{svc: svc, guard: guard}
}

func (p *glossaryDomains) Columns() []importer.Column {
	return []importer.Column{{
		ID:       ImportColumn,
		Type:     "string",
		Format:   "domain",
		LabelKey: "glossary.import.domain",
		Label:    "Domain",
		Help:     "The domain the term belongs to, by name or path of names (Finance/Payments), ignoring case. Empty keeps an existing term where it is and puts a new one in the default domain.",
	}}
}

func (p *glossaryDomains) Check(ctx context.Context, row importer.RowContext) (errs, warnings []importer.Problem) {
	fail := func(code, message string) {
		errs = append(errs, importer.Problem{Column: ImportColumn, Code: code, Message: message})
	}
	targetID := ""
	if ref := strings.TrimSpace(row.Cells[ImportColumn]); ref != "" {
		d, err := resolveOne(ctx, p.svc, ref)
		switch {
		case errors.Is(err, errDomainNotFound):
			fail("domain_not_found", fmt.Sprintf("no domain is named %q", ref))
			return errs, nil
		case errors.Is(err, ErrInvalidInput):
			fail("ambiguous_domain", err.Error())
			return errs, nil
		case err != nil:
			fail("domain_error", err.Error())
			return errs, nil
		}
		targetID = d.ID
	}
	existingID := ""
	if row.Current != nil {
		existingID = row.Current.ID
	}
	if err := p.guard.AuthorizeImportRow(ctx, KindGlossaryTerm, existingID, targetID); errors.Is(err, ErrForbidden) {
		fail("domain_forbidden", "you cannot write in this term's domain or its destination")
	} else if err != nil {
		fail("domain_error", err.Error())
	}
	return errs, nil
}

func (p *glossaryDomains) Value(ctx context.Context, term *glossary.GlossaryTerm, _ string) (string, error) {
	id, err := p.svc.DomainOf(ctx, KindGlossaryTerm, term.ID)
	if err != nil || id == UnassignedID {
		return "", err
	}
	var names []string
	for id != "" {
		d, err := p.svc.Get(ctx, id)
		if err != nil {
			return "", err
		}
		names = append([]string{d.Name}, names...)
		id = ""
		if d.ParentID != nil {
			id = *d.ParentID
		}
	}
	return strings.Join(names, "/"), nil
}

// Import authorizes every row by domain before the batch is written, then
// places each term in its domain column's domain, or new terms in the
// context's default. The batch itself stays all or nothing; placing runs
// after it commits, over domains already checked, and is audited.
func (s *guardedGlossary) Import(ctx context.Context, terms []glossary.ImportTerm) ([]*glossary.GlossaryTerm, error) {
	targets := make([]string, len(terms))
	for i, t := range terms {
		if ref := strings.TrimSpace(t.Extra[ImportColumn]); ref != "" {
			d, err := resolveOne(ctx, s.g.svc, ref)
			if err != nil {
				return nil, fmt.Errorf("term %q: domain %q: %w", t.Name, ref, err)
			}
			targets[i] = d.ID
		}
		if err := s.g.AuthorizeImportRow(ctx, KindGlossaryTerm, t.ExistingID, targets[i]); err != nil {
			return nil, fmt.Errorf("term %q: %w", t.Name, err)
		}
	}
	written, err := s.Service.Import(ctx, terms)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]*glossary.GlossaryTerm, len(written))
	for _, w := range written {
		byName[w.Name] = w
	}
	for i, t := range terms {
		term := byName[t.Name]
		if term == nil {
			continue
		}
		switch {
		case t.ExistingID != "" && targets[i] != "":
			err = s.g.Transfer(ctx, KindGlossaryTerm, []string{term.ID}, targets[i])
		case t.ExistingID == "" && targets[i] != "":
			err = s.g.PlaceCreatedIn(ctx, KindGlossaryTerm, term.ID, targets[i])
		case t.ExistingID == "":
			err = s.g.PlaceCreated(ctx, KindGlossaryTerm, term.ID)
		}
		if err != nil {
			return written, fmt.Errorf("term %q was imported but not placed in its domain: %w", t.Name, err)
		}
	}
	return written, nil
}
