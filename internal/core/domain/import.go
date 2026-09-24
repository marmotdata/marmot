package domain

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ImportInput maps the values found at a metadata path to domains, for
// migrating a free-text domain field (such as metadata.dgu.domain) to
// memberships. Values are matched exactly: homonyms and spelling variants are
// resolved in the mapping, never guessed.
type ImportInput struct {
	Source  string            `json:"source"`
	Mapping map[string]string `json:"mapping"`
	Apply   bool              `json:"apply"`
}

type ImportValue struct {
	Kind     Kind   `json:"kind"`
	Value    string `json:"value"`
	Count    int    `json:"count"`
	DomainID string `json:"domain_id,omitempty"`
}

// ImportReport describes what an import would do, or did when Applied. Only
// entities still in Unassigned are candidates, so an import never overrides an
// explicit assignment.
type ImportReport struct {
	Mapped          []ImportValue `json:"mapped"`
	Unmapped        []ImportValue `json:"unmapped"`
	AlreadyAssigned int           `json:"already_assigned"`
	Applied         bool          `json:"applied"`
}

// ImportCandidate is an entity whose metadata holds a value at the source path.
type ImportCandidate struct {
	ID       string
	Value    string
	Assigned bool
}

var importableKinds = []Kind{KindAsset, KindDataProduct, KindGlossaryTerm}

var metadataPathRegex = regexp.MustCompile(`^metadata(\.[A-Za-z0-9_-]+){1,8}$`)

func (s *service) Import(ctx context.Context, in ImportInput) (*ImportReport, error) {
	if !metadataPathRegex.MatchString(in.Source) {
		return nil, fmt.Errorf("%w: source must be a metadata path such as metadata.dgu.domain", ErrInvalidInput)
	}
	if len(in.Mapping) == 0 {
		return nil, fmt.Errorf("%w: mapping is empty", ErrInvalidInput)
	}
	for _, domainID := range in.Mapping {
		if domainID == UnassignedID {
			continue
		}
		if _, err := s.repo.Get(ctx, domainID); err != nil {
			return nil, fmt.Errorf("mapping target %s: %w", domainID, err)
		}
	}
	path := strings.Split(strings.TrimPrefix(in.Source, "metadata."), ".")

	report := &ImportReport{Mapped: []ImportValue{}, Unmapped: []ImportValue{}}
	plan := map[Kind]map[string][]string{}
	for _, kind := range importableKinds {
		candidates, err := s.repo.ImportCandidates(ctx, kind, path)
		if err != nil {
			return nil, err
		}
		counts := map[string]int{}
		for _, c := range candidates {
			if c.Assigned {
				report.AlreadyAssigned++
				continue
			}
			counts[c.Value]++
			if domainID, ok := in.Mapping[c.Value]; ok && domainID != UnassignedID {
				if plan[kind] == nil {
					plan[kind] = map[string][]string{}
				}
				plan[kind][domainID] = append(plan[kind][domainID], c.ID)
			}
		}
		for value, n := range counts {
			if domainID, ok := in.Mapping[value]; ok {
				report.Mapped = append(report.Mapped, ImportValue{Kind: kind, Value: value, Count: n, DomainID: domainID})
			} else {
				report.Unmapped = append(report.Unmapped, ImportValue{Kind: kind, Value: value, Count: n})
			}
		}
	}
	sortValues(report.Mapped)
	sortValues(report.Unmapped)

	if in.Apply && len(plan) > 0 {
		if err := s.repo.ApplyImport(ctx, plan); err != nil {
			return nil, err
		}
		report.Applied = true
	}
	return report, nil
}

func sortValues(values []ImportValue) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Kind != values[j].Kind {
			return values[i].Kind < values[j].Kind
		}
		return values[i].Value < values[j].Value
	})
}
