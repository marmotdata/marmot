package superset

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validConfig() pluginsdk.RawConfig {
	return pluginsdk.RawConfig{"host": "https://superset.example.com", "username": "marmot", "password": "secret"}
}

func TestValidate_ValidConfig(t *testing.T) {
	_, err := (&Source{}).Validate(validConfig())
	require.NoError(t, err)
}

func TestValidate_RequiresHost(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"username": "marmot", "password": "secret"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_RequiresUsername(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "https://superset.example.com", "password": "secret"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "username")
}

func TestValidate_RequiresPassword(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "https://superset.example.com", "username": "marmot"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "password")
}

func TestValidate_RejectsAHostThatIsNotAURL(t *testing.T) {
	config := validConfig()
	config["host"] = "superset.example.com"

	_, err := (&Source{}).Validate(config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_RejectsAnUnknownProvider(t *testing.T) {
	config := validConfig()
	config["provider"] = "oauth"

	_, err := (&Source{}).Validate(config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider")
}

func TestValidate_AcceptsLDAP(t *testing.T) {
	config := validConfig()
	config["provider"] = "ldap"

	s := &Source{}
	_, err := s.Validate(config)
	require.NoError(t, err)
	assert.Equal(t, "ldap", s.config.Provider)
}

func TestValidate_AppliesDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)

	assert.Equal(t, "db", s.config.Provider)
	assert.True(t, s.config.VerifySSL)
	assert.True(t, s.config.IncludeCharts)
	assert.True(t, s.config.IncludeDatasets)
	assert.True(t, s.config.IncludeDatabases)
	assert.True(t, s.config.IncludeDraft)
	assert.True(t, s.config.DiscoverLineage)
	assert.Equal(t, 100, s.config.PageSize)
}

func TestValidate_KeepsExplicitFalse(t *testing.T) {
	config := validConfig()
	config["include_draft"] = false
	config["verify_ssl"] = false
	config["page_size"] = 25

	s := &Source{}
	_, err := s.Validate(config)
	require.NoError(t, err)

	assert.False(t, s.config.IncludeDraft)
	assert.False(t, s.config.VerifySSL)
	assert.Equal(t, 25, s.config.PageSize)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeCharts)
}

func TestValidate_RejectsAZeroPageSizeRatherThanRewritingIt(t *testing.T) {
	config := validConfig()
	config["page_size"] = 0

	_, err := (&Source{}).Validate(config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "page_size")
}

func TestValidate_TrimsATrailingSlashOffTheHost(t *testing.T) {
	config := validConfig()
	config["host"] = "https://superset.example.com/"

	s := &Source{}
	_, err := s.Validate(config)
	require.NoError(t, err)

	assert.Equal(t, "https://superset.example.com", s.config.Host)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	config := validConfig()
	config["filter"] = map[string]interface{}{
		"include": []interface{}{"^Sales.*"},
		"exclude": []interface{}{".*Draft$"},
	}

	_, err := (&Source{}).Validate(config)
	require.NoError(t, err)
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "superset", meta.ID)
	assert.Equal(t, "Superset", meta.Name)
	assert.Equal(t, "superset", meta.Icon)
	assert.Equal(t, "dashboard", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestMeta_MarksThePasswordSensitive(t *testing.T) {
	for _, field := range Meta().ConfigSpec {
		if field.Name == "password" {
			assert.True(t, field.Sensitive)
			return
		}
	}
	t.Fatal("password field missing from the config spec")
}

func TestChartType_MapsSupersetVizTypes(t *testing.T) {
	assert.Equal(t, "Table", chartType("table"))
	assert.Equal(t, "Table", chartType("pivot_table_v2"))
	assert.Equal(t, "Line", chartType("big_number_total"))
	assert.Equal(t, "Line", chartType("echarts_timeseries_line"))
	assert.Equal(t, "Bar", chartType("dist_bar"))
	assert.Equal(t, "Pie", chartType("pie"))
	assert.Equal(t, "Area", chartType("treemap_v2"))
	assert.Equal(t, "BoxPlot", chartType("box_plot"))
	assert.Equal(t, "Histogram", chartType("histogram"))
	assert.Equal(t, "Scatter", chartType("scatter"))
	assert.Equal(t, "Gauge", chartType("gauge_chart"))
	assert.Equal(t, "Map", chartType("world_map"))
	assert.Equal(t, "Map", chartType("country_map"))
	assert.Equal(t, "Map", chartType("deck_polygon"))
	assert.Equal(t, "Graph", chartType("graph_chart"))
	assert.Equal(t, "Heatmap", chartType("heatmap_v2"))
	assert.Equal(t, "SanKey", chartType("sankey_v2"))
}

func TestChartType_UnknownIsOther(t *testing.T) {
	assert.Equal(t, "Other", chartType("word_cloud"))
	assert.Equal(t, "Other", chartType(""))
}

func TestOwnerNames_JoinsFirstAndLastName(t *testing.T) {
	names := ownerNames([]owner{{FirstName: "A", LastName: "B"}, {FirstName: "Solo"}, {}})
	assert.Equal(t, []string{"A B", "Solo"}, names)
}

func TestUserTags_KeepsOnlyTypeOne(t *testing.T) {
	names := userTags([]tag{{Name: "finance", Type: 1}, {Name: "owner:1", Type: 2}, {Name: "favorited_by:1", Type: 3}, {Name: "type:dashboard", Type: 4}})
	assert.Equal(t, []string{"finance"}, names)
}

func TestRedactURI_MasksThePassword(t *testing.T) {
	assert.Equal(t, "postgresql://marmot:xxx@db:5432/shop", redactURI("postgresql://marmot:s3cret@db:5432/shop"))
}

func TestRedactURI_LeavesAURIWithoutAPasswordAlone(t *testing.T) {
	assert.Equal(t, "sqlite:////app/superset.db", redactURI("sqlite:////app/superset.db"))
}

func TestURIDatabase_IsTheFirstPathSegment(t *testing.T) {
	assert.Equal(t, "shop", uriDatabase("postgresql://marmot:xxx@db:5432/shop"))
	assert.Equal(t, "hive", uriDatabase("trino://user@trino:8080/hive/default"))
	assert.Equal(t, "SALES", uriDatabase("snowflake://user:xxx@acct/SALES/PUBLIC?warehouse=wh"))
	assert.Equal(t, "", uriDatabase("bigquery://"))
}
