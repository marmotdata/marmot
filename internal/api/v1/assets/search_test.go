package assets

import (
	"net/http"
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

func TestLookupMRN_RawAndSluggedNamesMeetAtOneMRN(t *testing.T) {
	want := "mrn://asset/repository/github/marmotdata-marmot"

	for _, name := range []string{"marmotdata-marmot", "marmotdata/marmot", "MarmotData/Marmot"} {
		got, err := lookupMRN("Repository", "GitHub", name)
		require.NoError(t, err, name)
		assert.Equal(t, want, got, name)
	}
}

func TestLookupMRN_KeepsTheZoneAndDotsInTheName(t *testing.T) {
	got, err := lookupMRN("Topic", "Kafka@Prod", "orders.v1")
	require.NoError(t, err)
	assert.Equal(t, "mrn://asset/topic/kafka@prod/orders.v1", got)
}

func TestLookupMRN_RejectsEmptySegments(t *testing.T) {
	_, err := lookupMRN("Table", "", "orders")
	assert.Error(t, err)
}

// An encoded slash stays inside one path segment, so the router hands the handler the whole raw name.
func TestLookupRoute_EncodedSlashReachesTheNameSegment(t *testing.T) {
	var gotType, gotService, gotName string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/assets/lookup/{type}/{service}/{name}", func(w http.ResponseWriter, r *http.Request) {
		gotType, gotService, gotName = r.PathValue("type"), r.PathValue("service"), r.PathValue("name")
	})

	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/assets/lookup/Repository/GitHub/marmotdata%2Fmarmot", nil))

	assert.Equal(t, "Repository", gotType)
	assert.Equal(t, "GitHub", gotService)
	assert.Equal(t, "marmotdata/marmot", gotName)
}
