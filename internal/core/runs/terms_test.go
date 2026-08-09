package runs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// An asset names the glossary terms it carries; the run has to turn
// those names into the ids the link table stores.

func TestResolveTermIDs_MapsNamesToIDs(t *testing.T) {
	ids := resolveTermIDs(
		[]string{"BusinessTerms.Customer", "BusinessTerms.Order"},
		map[string]string{
			"BusinessTerms.Customer": "term-1",
			"BusinessTerms.Order":    "term-2",
		},
	)

	assert.Equal(t, []string{"term-1", "term-2"}, ids)
}

func TestResolveTermIDs_DropsANameTheGlossaryDoesNotKnow(t *testing.T) {
	// A source can tag an asset with a term it never sent. Linking the
	// terms it did send still has to work.
	ids := resolveTermIDs(
		[]string{"BusinessTerms.Customer", "BusinessTerms.Nowhere"},
		map[string]string{"BusinessTerms.Customer": "term-1"},
	)

	assert.Equal(t, []string{"term-1"}, ids)
}

func TestResolveTermIDs_DropsARepeatedTerm(t *testing.T) {
	ids := resolveTermIDs(
		[]string{"BusinessTerms.Customer", "BusinessTerms.Customer"},
		map[string]string{"BusinessTerms.Customer": "term-1"},
	)

	assert.Equal(t, []string{"term-1"}, ids)
}

func TestResolveTermIDs_IgnoresSurroundingSpace(t *testing.T) {
	ids := resolveTermIDs(
		[]string{" BusinessTerms.Customer "},
		map[string]string{"BusinessTerms.Customer": "term-1"},
	)

	assert.Equal(t, []string{"term-1"}, ids)
}

func TestResolveTermIDs_ReturnsNothingWithoutAGlossary(t *testing.T) {
	// No terms were synced, so there is nothing to link and the run must
	// not try.
	assert.Empty(t, resolveTermIDs([]string{"BusinessTerms.Customer"}, nil))
}
