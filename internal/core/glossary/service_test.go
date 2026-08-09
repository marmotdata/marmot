package glossary

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A term keeps two definitions: the one the last run brought in and the
// one a person wrote. These tests pin which of them a reader gets, and
// that writing a term back never confuses the two.

func seedTerm(t *testing.T, repo *fakeRepo, term *GlossaryTerm) *GlossaryTerm {
	t.Helper()
	if term.Metadata == nil {
		term.Metadata = map[string]interface{}{}
	}
	require.NoError(t, repo.Create(context.Background(), term, nil))
	return term
}

func TestGet_PrefersTheDefinitionAPersonWrote(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	theirs := "What the business actually means."
	term := seedTerm(t, repo, &GlossaryTerm{
		Name:           "BusinessTerms.Customer",
		Definition:     "What the source system says.",
		UserDefinition: &theirs,
	})

	served, err := svc.Get(context.Background(), term.ID)
	require.NoError(t, err)

	assert.Equal(t, theirs, served.Definition)
	require.NotNil(t, served.UserDefinition)
	assert.Equal(t, theirs, *served.UserDefinition, "the reader can still tell the two apart")
}

func TestGet_FallsBackToTheDefinitionTheRunWrote(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	term := seedTerm(t, repo, &GlossaryTerm{
		Name:       "BusinessTerms.Customer",
		Definition: "What the source system says.",
	})

	served, err := svc.Get(context.Background(), term.ID)
	require.NoError(t, err)

	assert.Equal(t, "What the source system says.", served.Definition)
}

func TestGet_IgnoresABlankDefinitionAPersonLeftBehind(t *testing.T) {
	// Clearing the box in the UI must not blank the term.
	repo := newFakeRepo()
	svc := NewService(repo)

	blank := "   "
	term := seedTerm(t, repo, &GlossaryTerm{
		Name:           "BusinessTerms.Customer",
		Definition:     "What the source system says.",
		UserDefinition: &blank,
	})

	served, err := svc.Get(context.Background(), term.ID)
	require.NoError(t, err)

	assert.Equal(t, "What the source system says.", served.Definition)
}

func TestGet_DoesNotChangeTheStoredRow(t *testing.T) {
	// Reading resolves onto a copy. If it resolved in place, the next
	// write of that term would put a person's wording in the run's
	// column, where the next run would erase it.
	repo := newFakeRepo()
	svc := NewService(repo)

	theirs := "What the business actually means."
	term := seedTerm(t, repo, &GlossaryTerm{
		Name:           "BusinessTerms.Customer",
		Definition:     "What the source system says.",
		UserDefinition: &theirs,
	})

	_, err := svc.Get(context.Background(), term.ID)
	require.NoError(t, err)

	assert.Equal(t, "What the source system says.", repo.terms[term.ID].Definition)
}

func TestUpdate_LeavesTheRunsDefinitionAloneWhenOnlyTheNameChanges(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	theirs := "What the business actually means."
	term := seedTerm(t, repo, &GlossaryTerm{
		Name:           "BusinessTerms.Customer",
		Definition:     "What the source system says.",
		UserDefinition: &theirs,
	})

	renamed := "BusinessTerms.Buyer"
	_, err := svc.Update(context.Background(), term.ID, UpdateTermInput{Name: &renamed})
	require.NoError(t, err)

	assert.Equal(t, "What the source system says.", repo.terms[term.ID].Definition,
		"an edit that says nothing about the definition must not move one column into the other")
}

func TestSearch_PrefersTheDefinitionAPersonWrote(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	theirs := "What the business actually means."
	seedTerm(t, repo, &GlossaryTerm{
		Name:           "BusinessTerms.Customer",
		Definition:     "What the source system says.",
		UserDefinition: &theirs,
	})

	result, err := svc.Search(context.Background(), SearchFilter{})
	require.NoError(t, err)

	require.Len(t, result.Terms, 1)
	assert.Equal(t, theirs, result.Terms[0].Definition)
}

func TestGetChildren_PrefersTheDefinitionAPersonWrote(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	parent := seedTerm(t, repo, &GlossaryTerm{Name: "BusinessTerms.Customer", Definition: "Someone who buys."})

	theirs := "Value over the whole relationship."
	child := seedTerm(t, repo, &GlossaryTerm{
		Name:           "BusinessTerms.Customer.LifetimeValue",
		Definition:     "Value over time.",
		UserDefinition: &theirs,
		ParentTermID:   &parent.ID,
	})

	children, err := svc.GetChildren(context.Background(), parent.ID)
	require.NoError(t, err)

	require.Len(t, children, 1)
	assert.Equal(t, child.ID, children[0].ID)
	assert.Equal(t, theirs, children[0].Definition)
}
