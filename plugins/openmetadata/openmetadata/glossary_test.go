package openmetadata

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The glossary is the vocabulary a team wrote by hand, and the reason an
// asset means something to somebody. Marmot holds terms as objects of
// their own, so an import has to keep both the tree they were arranged
// in and the assets they were put on.

func TestDiscover_ImportsGlossaryTerms(t *testing.T) {
	result := discover(t, businessGlossary(), nil)

	customer := findTerm(result, "BusinessTerms.Customer")
	require.NotNil(t, customer)
	assert.Equal(t, "Someone who has bought something", customer.Definition)
	assert.Equal(t, []string{"Client", "Buyer"}, customer.Synonyms)
}

func TestDiscover_ImportsTheGlossaryItselfAsARootTerm(t *testing.T) {
	// Marmot has one tree of terms rather than a set of named
	// glossaries, so without the glossary as a root every vocabulary
	// would land side by side at the top with nothing telling them apart.
	result := discover(t, businessGlossary(), nil)

	glossary := findTerm(result, "BusinessTerms")
	require.NotNil(t, glossary)
	assert.Empty(t, glossary.Parent)
}

func TestDiscover_NestsATermUnderItsParentTerm(t *testing.T) {
	result := discover(t, businessGlossary(), nil)

	lifetimeValue := findTerm(result, "BusinessTerms.Customer.LifetimeValue")
	require.NotNil(t, lifetimeValue)
	assert.Equal(t, "BusinessTerms.Customer", lifetimeValue.Parent)
}

func TestDiscover_NestsATermWithNoParentUnderItsGlossary(t *testing.T) {
	result := discover(t, businessGlossary(), nil)

	customer := findTerm(result, "BusinessTerms.Customer")
	require.NotNil(t, customer)
	assert.Equal(t, "BusinessTerms", customer.Parent)
}

func TestDiscover_KeepsTermsOfTheSameNameInTwoGlossariesApart(t *testing.T) {
	// Identity is the fully qualified name, so a Customer curated by the
	// sales team is not the Customer the support team defined.
	result := discover(t, newFakeOM().
		with("glossaries", glossaryEntity("Sales"), glossaryEntity("Support")).
		with("glossaryTerms",
			termEntity("Sales", "Sales.Customer"),
			termEntity("Support", "Support.Customer")),
		nil)

	assert.NotNil(t, findTerm(result, "Sales.Customer"))
	assert.NotNil(t, findTerm(result, "Support.Customer"))
}

func TestDiscover_DefinesATermByItsNameWhenNobodyWroteADefinition(t *testing.T) {
	// Marmot requires a definition and OpenMetadata does not, so a term
	// nobody described still has to survive the import.
	result := discover(t, businessGlossary(), nil)

	order := findTerm(result, "BusinessTerms.Order")
	require.NotNil(t, order)
	assert.Equal(t, "Order", order.Definition)
}

func TestDiscover_RecordsWhereATermCameFrom(t *testing.T) {
	result := discover(t, businessGlossary(), nil)

	customer := findTerm(result, "BusinessTerms.Customer")
	require.NotNil(t, customer)

	om, ok := customer.Metadata["openmetadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "id-BusinessTerms.Customer", om["id"])
	assert.Equal(t, "BusinessTerms.Customer", om["fqn"])
	assert.Equal(t, "https://om.example.com/api/v1/glossaryTerms/id-BusinessTerms.Customer", om["href"])
	assert.Equal(t, "BusinessTerms", customer.Metadata["glossary"])
}

func TestDiscover_SkipsDeletedTerms(t *testing.T) {
	retired := termEntity("BusinessTerms", "BusinessTerms.Prospect")
	retired["deleted"] = true

	result := discover(t, newFakeOM().
		with("glossaries", glossaryEntity("BusinessTerms")).
		with("glossaryTerms", termEntity("BusinessTerms", "BusinessTerms.Customer"), retired),
		nil)

	assert.NotNil(t, findTerm(result, "BusinessTerms.Customer"))
	assert.Nil(t, findTerm(result, "BusinessTerms.Prospect"))
}

func TestDiscover_ImportsTheGlossaryOfAScopedRun(t *testing.T) {
	// A glossary belongs to no service, so a run scoped to one service
	// must still bring the vocabulary its assets are described in.
	result := discover(t, businessGlossary(), pluginsdk.RawConfig{"services": []string{"pg"}})

	assert.NotNil(t, findTerm(result, "BusinessTerms.Customer"))
}

func TestDiscover_AssignsTermsToTheAssetTheyWereCuratedOnto(t *testing.T) {
	result := discover(t, businessGlossary(), nil)

	customers := findAsset(result, "Table", "customers")
	require.NotNil(t, customers)
	assert.Equal(t, []string{"BusinessTerms.Customer"}, customers.Terms)

	orders := findAsset(result, "Table", "orders")
	require.NotNil(t, orders)
	assert.Equal(t, []string{"BusinessTerms.Order"}, orders.Terms,
		"the term on one table must not spread to the other")
}

// TestDiscover_AssignmentsPointAtImportedTerms is the invariant that
// makes an assignment mean anything: Marmot resolves it by name, so a
// name the run did not also import assigns nothing.
func TestDiscover_AssignmentsPointAtImportedTerms(t *testing.T) {
	result := discover(t, businessGlossary(), nil)

	imported := make(map[string]bool, len(result.GlossaryTerms))
	for _, term := range result.GlossaryTerms {
		imported[term.Name] = true
	}

	assigned := 0
	for _, asset := range result.Assets {
		for _, term := range asset.Terms {
			assert.True(t, imported[term], "asset %q is assigned a term the run did not import: %q", *asset.MRN, term)
			assigned++
		}
	}
	assert.NotZero(t, assigned)
}

func TestDiscover_SkipsSuggestedTermAssignments(t *testing.T) {
	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["tags"] = []map[string]interface{}{
		{"tagFQN": "BusinessTerms.Order", "source": "Glossary", "state": "Suggested"},
	}

	result := discover(t, newFakeOM().
		with("glossaries", glossaryEntity("BusinessTerms")).
		with("glossaryTerms", termEntity("BusinessTerms", "BusinessTerms.Order")).
		with("tables", orders),
		nil)

	assert.Empty(t, findAsset(result, "Table", "orders").Terms,
		"a suggestion nobody accepted is not an assignment")
}

func TestDiscover_KeepsAssignedTermsOffTheTags(t *testing.T) {
	// Terms are first class now, so copying them onto the tags as well
	// is opt in rather than the default it once was.
	result := discover(t, businessGlossary(), nil)

	customers := findAsset(result, "Table", "customers")
	require.NotNil(t, customers)
	assert.Empty(t, customers.Tags)
	assert.Equal(t, []string{"BusinessTerms.Customer"}, customers.Terms)
}

func TestDiscover_LeavesTheGlossaryBehindWhenTurnedOff(t *testing.T) {
	result := discover(t, businessGlossary(), pluginsdk.RawConfig{"include_glossary": false})

	assert.Empty(t, result.GlossaryTerms)
	assert.Empty(t, findAsset(result, "Table", "customers").Terms,
		"an assignment to a term nobody imported points at nothing")
}

func TestDiscover_SurvivesAnOpenMetadataWithoutAGlossary(t *testing.T) {
	// The glossary endpoints arrived later than the entity kinds around
	// them; an older server answering 404 must not fail the import.
	result := discover(t, businessGlossary().without("glossaries", "glossaryTerms"), nil)

	assert.Empty(t, result.GlossaryTerms)
	assert.NotNil(t, findAsset(result, "Table", "customers"))
}

func TestDiscover_SurvivesAGlossaryWithoutTerms(t *testing.T) {
	result := discover(t, businessGlossary().without("glossaryTerms"), nil)

	require.Len(t, result.GlossaryTerms, 1)
	assert.Equal(t, "BusinessTerms", result.GlossaryTerms[0].Name)
}

func TestValidate_ImportsTheGlossaryButNotAsTagsByDefault(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{"host": "https://om.example.com", "jwt_token": "t"})
	require.NoError(t, err)

	assert.True(t, source.config.IncludeGlossary)
	assert.False(t, source.config.GlossaryTermsAsTags)
}

// businessGlossary is a fake OpenMetadata holding one vocabulary, a term
// nested below another, and the two tables the terms were curated onto.
func businessGlossary() *fakeOM {
	customer := termEntity("BusinessTerms", "BusinessTerms.Customer")
	customer["description"] = "Someone who has bought something"
	customer["synonyms"] = []string{"Client", "Buyer"}

	lifetimeValue := termEntity("BusinessTerms", "BusinessTerms.Customer.LifetimeValue")
	lifetimeValue["parent"] = map[string]interface{}{
		"fullyQualifiedName": "BusinessTerms.Customer",
		"name":               "Customer",
	}

	customers := tableEntity("pg", "Postgres", "pg.shop.public.customers", "Regular")
	customers["tags"] = []map[string]interface{}{
		{"tagFQN": "BusinessTerms.Customer", "source": "Glossary", "state": "Confirmed"},
	}

	orders := tableEntity("pg", "Postgres", "pg.shop.public.orders", "Regular")
	orders["tags"] = []map[string]interface{}{
		{"tagFQN": "BusinessTerms.Order", "source": "Glossary", "state": "Confirmed"},
	}

	return newFakeOM().
		with("glossaries", glossaryEntity("BusinessTerms")).
		with("glossaryTerms", customer, lifetimeValue, termEntity("BusinessTerms", "BusinessTerms.Order")).
		with("tables", customers, orders)
}
