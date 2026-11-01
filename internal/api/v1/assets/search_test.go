package assets

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The provider filter has two names. The API documents it as services, and the
// generated clients send that; the UI has always sent providers. A caller who
// reads the docs has to get a filtered result, not the whole catalog.

func filterFor(t *testing.T, rawQuery string) []string {
	t.Helper()

	filter, err := parseFilter(httptest.NewRequest("GET", "/api/v1/assets/search?"+rawQuery, nil))
	require.NoError(t, err)

	return filter.Providers
}

func TestParseFilter_ReadsTheDocumentedServicesName(t *testing.T) {
	assert.Equal(t, []string{"PostgreSQL"}, filterFor(t, "services=PostgreSQL"))
}

func TestParseFilter_StillReadsProvidersForTheUI(t *testing.T) {
	assert.Equal(t, []string{"PostgreSQL"}, filterFor(t, "providers=PostgreSQL"))
}

func TestParseFilter_SplitsSeveralServices(t *testing.T) {
	assert.Equal(t, []string{"PostgreSQL", "MySQL"}, filterFor(t, "services=PostgreSQL,MySQL"))
}

func TestParseFilter_KeepsAServiceNameWithASpace(t *testing.T) {
	// Providers like SQL Server and Unity Catalog carry a space, and the
	// filter has to match the provider exactly.
	assert.Equal(t, []string{"SQL Server"}, filterFor(t, "services=SQL+Server"))
}

func TestParseFilter_NoServiceFilterLeavesTheSearchUnfiltered(t *testing.T) {
	assert.Empty(t, filterFor(t, "q=orders"))
}
