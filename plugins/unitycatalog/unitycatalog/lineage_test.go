package unitycatalog

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func viewWith(definition string) tableInfo {
	return tableInfo{CatalogName: "shop", SchemaName: "sales", Name: "v", TableType: "VIEW", ViewDefinition: definition}
}

func TestViewReferences_FindsFromAndJoinTargets(t *testing.T) {
	refs := viewReferences(viewWith("SELECT o.id FROM shop.sales.orders o JOIN customers c ON o.customer_id = c.id"))

	assert.Equal(t, []string{"shop.sales.orders", "customers"}, refs)
}

func TestViewReferences_StripsBackticksAndQuotes(t *testing.T) {
	refs := viewReferences(viewWith("SELECT * FROM `shop`.`sales`.`order-lines` LEFT JOIN \"customers\""))

	assert.Equal(t, []string{"shop.sales.order-lines", "customers"}, refs)
}

func TestViewReferences_IgnoresSubqueries(t *testing.T) {
	refs := viewReferences(viewWith("SELECT * FROM (SELECT id FROM orders) t"))

	assert.Equal(t, []string{"orders"}, refs)
}

func TestViewReferences_IsCaseInsensitiveOnKeywords(t *testing.T) {
	refs := viewReferences(viewWith("select * from Orders inner join Customers on 1=1"))

	assert.Equal(t, []string{"Orders", "Customers"}, refs)
}

func TestViewReferences_PrefersRecordedDependencies(t *testing.T) {
	table := viewWith("SELECT 1")
	table.ViewDependencies = &dependencyList{Dependencies: []dependency{
		{Table: &tableDependency{TableFullName: "shop.sales.orders"}},
		{Table: nil},
	}}

	assert.Equal(t, []string{"shop.sales.orders"}, viewReferences(table))
}

func TestResolveReference_ExpandsABareName(t *testing.T) {
	assert.Equal(t, "shop.sales.orders", resolveReference("orders", "shop", "sales"))
}

func TestResolveReference_ExpandsASchemaQualifiedName(t *testing.T) {
	assert.Equal(t, "shop.reporting.orders", resolveReference("reporting.orders", "shop", "sales"))
}

func TestResolveReference_KeepsAFullName(t *testing.T) {
	assert.Equal(t, "other.place.thing", resolveReference("other.place.thing", "shop", "sales"))
}

func TestStorageContainerMRN_S3(t *testing.T) {
	assert.Equal(t, "mrn://bucket/s3/marmot-lake", storageContainerMRN("s3://marmot-lake/orders"))
	assert.Equal(t, "mrn://bucket/s3/marmot-lake", storageContainerMRN("s3a://marmot-lake/"))
}

func TestStorageContainerMRN_GCS(t *testing.T) {
	assert.Equal(t, "mrn://bucket/gcs/marmot-gcs-lake", storageContainerMRN("gs://marmot-gcs-lake/payments"))
}

func TestStorageContainerMRN_AzureBlob(t *testing.T) {
	assert.Equal(t, "mrn://container/azureblob/lake",
		storageContainerMRN("abfss://lake@acct.dfs.core.windows.net/payments"))
	assert.Equal(t, "mrn://container/azureblob/lake",
		storageContainerMRN("wasbs://lake@acct.blob.core.windows.net/payments"))
}

func TestStorageContainerMRN_IgnoresLocalAndUnknownLocations(t *testing.T) {
	assert.Empty(t, storageContainerMRN("file:///home/unitycatalog/etc/data/managed/unity/default/tables/marksheet"))
	assert.Empty(t, storageContainerMRN("dbfs:/mnt/lake/orders"))
	assert.Empty(t, storageContainerMRN(""))
	assert.Empty(t, storageContainerMRN("abfss://no-container-here"))
}

// loopingServer answers every listing with one catalog and the same
// page token, forever.
type loopingServer struct {
	pages int
}

func (l *loopingServer) start(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == apiPrefix+"/catalogs" {
			l.pages++
			_, _ = w.Write([]byte(`{"catalogs":[{"name":"c"}],"next_page_token":"again"}`))
			return
		}
		_, _ = w.Write([]byte(`{"next_page_token":null}`))
	}))
	t.Cleanup(server.Close)
	return server
}
