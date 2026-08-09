package glossary

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A run brings the business terms a source system curates. These tests
// pin the rules that let it be re-run: terms are keyed by name, the
// hierarchy resolves whatever order it arrives in, and a definition a
// person wrote is never written over.

const testSource = "openmetadata"

// fakeRepo is an in-memory glossary store. Only the calls a sync makes
// are implemented; the rest exist to satisfy the interface.
type fakeRepo struct {
	terms      map[string]*GlossaryTerm // by id
	nextID     int
	createCall int
	updateCall int
	parentCall int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{terms: map[string]*GlossaryTerm{}}
}

func (r *fakeRepo) Create(ctx context.Context, term *GlossaryTerm, owners []OwnerInput) error {
	r.createCall++
	r.nextID++
	term.ID = fmt.Sprintf("term-%d", r.nextID)
	stored := *term
	r.terms[term.ID] = &stored
	return nil
}

func (r *fakeRepo) Get(ctx context.Context, id string) (*GlossaryTerm, error) {
	term, ok := r.terms[id]
	if !ok {
		return nil, ErrNotFound
	}
	copied := *term
	return &copied, nil
}

func (r *fakeRepo) GetByName(ctx context.Context, name string) (*GlossaryTerm, error) {
	for _, term := range r.terms {
		if term.Name == name && term.DeletedAt == nil {
			copied := *term
			return &copied, nil
		}
	}
	return nil, ErrNotFound
}

// Update mirrors the postgres statement, which leaves user_definition out
// of its column list, so a caller cannot overwrite it however it fills
// the struct.
func (r *fakeRepo) Update(ctx context.Context, term *GlossaryTerm, owners []OwnerInput) error {
	r.updateCall++
	existing, ok := r.terms[term.ID]
	if !ok {
		return ErrNotFound
	}
	stored := *term
	stored.UserDefinition = existing.UserDefinition
	r.terms[term.ID] = &stored
	return nil
}

func (r *fakeRepo) SetParent(ctx context.Context, termID string, parentTermID *string) error {
	r.parentCall++
	term, ok := r.terms[termID]
	if !ok {
		return ErrNotFound
	}
	term.ParentTermID = parentTermID
	return nil
}

func (r *fakeRepo) List(ctx context.Context, offset, limit int) (*ListResult, error) {
	terms := r.live()
	return &ListResult{Terms: terms, Total: len(terms)}, nil
}

// Search ignores the filter: what the tests here care about is the shape
// of what comes back, not which rows the database picked.
func (r *fakeRepo) Search(ctx context.Context, filter SearchFilter) (*ListResult, error) {
	terms := r.live()
	return &ListResult{Terms: terms, Total: len(terms)}, nil
}

func (r *fakeRepo) GetChildren(ctx context.Context, parentID string) ([]*GlossaryTerm, error) {
	children := []*GlossaryTerm{}
	for _, term := range r.live() {
		if term.ParentTermID != nil && *term.ParentTermID == parentID {
			children = append(children, term)
		}
	}
	return children, nil
}

// live returns copies of every term still in the glossary, so a caller
// cannot reach into the store through what it was handed.
func (r *fakeRepo) live() []*GlossaryTerm {
	terms := []*GlossaryTerm{}
	for _, term := range r.terms {
		if term.DeletedAt != nil {
			continue
		}
		copied := *term
		terms = append(terms, &copied)
	}
	return terms
}

// byName is a test lookup that fails loudly, so an assertion can never
// silently pass against a term that was never written.
func (r *fakeRepo) byName(t *testing.T, name string) *GlossaryTerm {
	t.Helper()
	term, err := r.GetByName(context.Background(), name)
	require.NoError(t, err, "expected a term named %q", name)
	return term
}

func TestSyncTerms_CreatesTermsKeyedByName(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	result, err := svc.SyncTerms(context.Background(), testSource, []TermInput{
		{Name: "BusinessTerms.Customer", Definition: "Someone who buys."},
		{Name: "BusinessTerms.Order", Definition: "A purchase."},
	})
	require.NoError(t, err)

	assert.Equal(t, 2, result.Created)
	assert.Len(t, result.IDsByName, 2, "the caller needs an id for every name it sent")
	assert.Equal(t, "Someone who buys.", repo.byName(t, "BusinessTerms.Customer").Definition)
}

func TestSyncTerms_ResolvesAParentThatArrivesLater(t *testing.T) {
	// The child comes first, so the hierarchy can only resolve if parents
	// are settled after every term exists.
	repo := newFakeRepo()
	svc := NewService(repo)

	_, err := svc.SyncTerms(context.Background(), testSource, []TermInput{
		{Name: "BusinessTerms.Customer.LifetimeValue", Definition: "Value over time.", Parent: "BusinessTerms.Customer"},
		{Name: "BusinessTerms.Customer", Definition: "Someone who buys."},
	})
	require.NoError(t, err)

	child := repo.byName(t, "BusinessTerms.Customer.LifetimeValue")
	parent := repo.byName(t, "BusinessTerms.Customer")
	require.NotNil(t, child.ParentTermID)
	assert.Equal(t, parent.ID, *child.ParentTermID)
}

func TestSyncTerms_ResolvesAParentStoredByAnEarlierRun(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	_, err := svc.SyncTerms(ctx, testSource, []TermInput{
		{Name: "BusinessTerms.Customer", Definition: "Someone who buys."},
	})
	require.NoError(t, err)

	_, err = svc.SyncTerms(ctx, testSource, []TermInput{
		{Name: "BusinessTerms.Customer.LifetimeValue", Definition: "Value over time.", Parent: "BusinessTerms.Customer"},
	})
	require.NoError(t, err)

	child := repo.byName(t, "BusinessTerms.Customer.LifetimeValue")
	require.NotNil(t, child.ParentTermID)
	assert.Equal(t, repo.byName(t, "BusinessTerms.Customer").ID, *child.ParentTermID)
}

func TestSyncTerms_KeepsATermWhoseParentIsMissing(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	result, err := svc.SyncTerms(context.Background(), testSource, []TermInput{
		{Name: "BusinessTerms.Customer.LifetimeValue", Definition: "Value over time.", Parent: "BusinessTerms.Nowhere"},
	})
	require.NoError(t, err, "a dangling parent must not fail the run")

	assert.Equal(t, 1, result.Created)
	assert.Nil(t, repo.byName(t, "BusinessTerms.Customer.LifetimeValue").ParentTermID,
		"an unresolvable parent leaves the term at the root")
}

func TestSyncTerms_FallsBackToTheNameWhenThereIsNoDefinition(t *testing.T) {
	// The column is NOT NULL and a source system will hand over terms
	// nobody has written up yet.
	repo := newFakeRepo()
	svc := NewService(repo)

	_, err := svc.SyncTerms(context.Background(), testSource, []TermInput{
		{Name: "BusinessTerms.Revenue", Definition: "   "},
	})
	require.NoError(t, err)

	assert.Equal(t, "BusinessTerms.Revenue", repo.byName(t, "BusinessTerms.Revenue").Definition)
}

func TestSyncTerms_SecondRunOfTheSameInputWritesNothing(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	inputs := []TermInput{
		{Name: "BusinessTerms.Customer", Definition: "Someone who buys.", Synonyms: []string{"Client"}},
		{Name: "BusinessTerms.Customer.LifetimeValue", Definition: "Value over time.", Parent: "BusinessTerms.Customer"},
	}

	first, err := svc.SyncTerms(ctx, testSource, inputs)
	require.NoError(t, err)
	require.Equal(t, 2, first.Created)

	second, err := svc.SyncTerms(ctx, testSource, inputs)
	require.NoError(t, err)

	assert.Equal(t, 0, second.Created, "a re-run must not duplicate terms")
	assert.Equal(t, 0, second.Updated)
	assert.Equal(t, 2, second.Unchanged)
	assert.Len(t, repo.terms, 2)
	assert.Equal(t, 2, repo.createCall, "no term is written twice")
	assert.Equal(t, 0, repo.updateCall, "an unchanged term costs no write")
}

func TestSyncTerms_SecondRunUpdatesAChangedDefinition(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	_, err := svc.SyncTerms(ctx, testSource, []TermInput{
		{Name: "BusinessTerms.Customer", Definition: "Someone who buys."},
	})
	require.NoError(t, err)

	result, err := svc.SyncTerms(ctx, testSource, []TermInput{
		{Name: "BusinessTerms.Customer", Definition: "A party that has placed an order."},
	})
	require.NoError(t, err)

	assert.Equal(t, 1, result.Updated)
	assert.Equal(t, "A party that has placed an order.", repo.byName(t, "BusinessTerms.Customer").Definition)
}

func TestSyncTerms_TakesOverATermItDidNotCreate(t *testing.T) {
	// A term that arrived some other way is still this source's term to
	// keep current. Nothing on the row marks who wrote it.
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	existing := &GlossaryTerm{
		Name:       "BusinessTerms.Customer",
		Definition: "Written elsewhere.",
		Metadata:   map[string]interface{}{},
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, existing, nil))

	result, err := svc.SyncTerms(ctx, testSource, []TermInput{
		{Name: "BusinessTerms.Customer", Definition: "Whatever the source system says."},
	})
	require.NoError(t, err)

	assert.Equal(t, 1, result.Updated)
	assert.Equal(t, "Whatever the source system says.", repo.byName(t, "BusinessTerms.Customer").Definition)
	assert.Equal(t, existing.ID, result.IDsByName["BusinessTerms.Customer"],
		"the run still needs the id so it can link assets to it")
}

func TestSyncTerms_KeepsTheDefinitionAPersonWrote(t *testing.T) {
	// The point of the column: a run rewrites its own definition and
	// cannot reach the one a person put in user_definition.
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	theirs := "Whatever the data team agreed on."
	require.NoError(t, repo.Create(ctx, &GlossaryTerm{
		Name:           "BusinessTerms.Customer",
		Definition:     "Source wording, first run.",
		UserDefinition: &theirs,
		Metadata:       map[string]interface{}{},
	}, nil))

	_, err := svc.SyncTerms(ctx, testSource, []TermInput{
		{Name: "BusinessTerms.Customer", Definition: "Source wording, second run."},
	})
	require.NoError(t, err)

	stored := repo.byName(t, "BusinessTerms.Customer")
	assert.Equal(t, "Source wording, second run.", stored.Definition, "the run owns the source wording")
	require.NotNil(t, stored.UserDefinition)
	assert.Equal(t, theirs, *stored.UserDefinition, "a person's wording survives re-ingestion")
}

func TestSyncTerms_ReadsBackTheDefinitionAPersonWrote(t *testing.T) {
	// Re-ingesting must not change what a reader is served either.
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	theirs := "Whatever the data team agreed on."
	term := &GlossaryTerm{
		Name:           "BusinessTerms.Customer",
		Definition:     "Source wording, first run.",
		UserDefinition: &theirs,
		Metadata:       map[string]interface{}{},
	}
	require.NoError(t, repo.Create(ctx, term, nil))

	_, err := svc.SyncTerms(ctx, testSource, []TermInput{
		{Name: "BusinessTerms.Customer", Definition: "Source wording, second run."},
	})
	require.NoError(t, err)

	served, err := svc.Get(ctx, term.ID)
	require.NoError(t, err)
	assert.Equal(t, theirs, served.Definition)
}

func TestSyncTerms_ReparentsATermItDidNotCreate(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &GlossaryTerm{
		Name:       "BusinessTerms.Customer.LifetimeValue",
		Definition: "Written elsewhere.",
		Metadata:   map[string]interface{}{},
	}, nil))

	_, err := svc.SyncTerms(ctx, testSource, []TermInput{
		{Name: "BusinessTerms.Customer", Definition: "Someone who buys."},
		{Name: "BusinessTerms.Customer.LifetimeValue", Definition: "Value over time.", Parent: "BusinessTerms.Customer"},
	})
	require.NoError(t, err)

	child := repo.byName(t, "BusinessTerms.Customer.LifetimeValue")
	require.NotNil(t, child.ParentTermID, "the hierarchy comes from the source system")
	assert.Equal(t, repo.byName(t, "BusinessTerms.Customer").ID, *child.ParentTermID)
}

func TestSyncTerms_KeepsSynonymsInMetadata(t *testing.T) {
	// The table has no column for synonyms, and losing them would lose
	// the words people actually search for.
	repo := newFakeRepo()
	svc := NewService(repo)

	_, err := svc.SyncTerms(context.Background(), testSource, []TermInput{
		{Name: "BusinessTerms.Customer", Definition: "Someone who buys.", Synonyms: []string{"Client", "Buyer"}},
	})
	require.NoError(t, err)

	metadata := repo.byName(t, "BusinessTerms.Customer").Metadata
	assert.Equal(t, []interface{}{"Client", "Buyer"}, metadata[synonymsKey])
}

func TestSyncTerms_IgnoresARepeatedNameInOneBatch(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	result, err := svc.SyncTerms(context.Background(), testSource, []TermInput{
		{Name: "BusinessTerms.Customer", Definition: "Someone who buys."},
		{Name: "BusinessTerms.Customer", Definition: "Someone who buys, again."},
	})
	require.NoError(t, err)

	assert.Equal(t, 1, result.Created)
	assert.Len(t, repo.terms, 1)
}

func TestSyncTerms_SkipsATermWithNoName(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	result, err := svc.SyncTerms(context.Background(), testSource, []TermInput{
		{Name: "  ", Definition: "Nameless."},
	})
	require.NoError(t, err)

	assert.Equal(t, 0, result.Created)
	assert.Empty(t, repo.terms)
}

func TestSyncTerms_RequiresASource(t *testing.T) {
	// A batch that cannot say which run it came from cannot be traced
	// back to one when a term turns up wrong.
	_, err := NewService(newFakeRepo()).SyncTerms(context.Background(), "", []TermInput{
		{Name: "BusinessTerms.Customer", Definition: "Someone who buys."},
	})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestParentAssignments_DropsATermThatIsItsOwnParent(t *testing.T) {
	assignments := parentAssignments([]TermInput{
		{Name: "BusinessTerms.Customer", Parent: "BusinessTerms.Customer"},
	})

	assert.Empty(t, assignments)
}

func TestParentAssignments_DropsACycle(t *testing.T) {
	// A hierarchy that loops is one the term tree can never finish
	// walking, so none of the loop is written.
	assignments := parentAssignments([]TermInput{
		{Name: "A", Parent: "B"},
		{Name: "B", Parent: "C"},
		{Name: "C", Parent: "A"},
	})

	assert.Empty(t, assignments)
}

func TestParentAssignments_KeepsAChain(t *testing.T) {
	assignments := parentAssignments([]TermInput{
		{Name: "A", Parent: "B"},
		{Name: "B", Parent: "C"},
	})

	assert.Equal(t, map[string]string{"A": "B", "B": "C"}, assignments)
}
