// Package metabase discovers dashboards, charts and models from Metabase,
// with lineage from the tables they read.
package metabase

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact provider string on every asset this plugin
// emits and the service part of every MRN it builds.
const provider = "Metabase"

// Config for the Metabase plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host     string `json:"host" description:"Metabase URL, for example https://metabase.example.com" validate:"required,url"`
	APIKey   string `json:"api_key,omitempty" label:"API Key" description:"API key (Metabase 0.49 and newer)" sensitive:"true"`
	Username string `json:"username,omitempty" description:"Username for session login, used when no API key is set"`
	Password string `json:"password,omitempty" description:"Password for session login" sensitive:"true"`

	IncludeCharts   bool `json:"include_charts" description:"Discover questions and metrics as Chart assets" default:"true"`
	IncludeModels   bool `json:"include_models" description:"Discover models as Data Model Object assets" default:"true"`
	DiscoverLineage bool `json:"discover_lineage" description:"Link cards to the tables and models they read" default:"true"`
	IncludeArchived bool `json:"include_archived" description:"Include archived dashboards and cards" default:"false"`
	VerifySSL       bool `json:"verify_ssl" label:"Verify SSL" description:"Verify the server's TLS certificate" default:"true"`
}

// Example configuration for the plugin
var _ = `
host: "https://metabase.example.com"
api_key: "${METABASE_API_KEY}"
include_charts: true
include_models: true
discover_lineage: true
include_archived: false
tags:
  - "metabase"
  - "bi"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "metabase",
		Name:        "Metabase",
		Description: "Discover dashboards, charts and models from Metabase, with lineage from the tables they read",
		Icon:        "metabase",
		Category:    "dashboard",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the Metabase plugin.
type Source struct {
	config *Config
	client *client
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)

	config.Host = strings.TrimSuffix(strings.TrimSpace(config.Host), "/")

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	if config.APIKey == "" && (config.Username == "" || config.Password == "") {
		return nil, fmt.Errorf("authentication required: set api_key, or username and password")
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Metabase dashboards, charts and models, and the
// lineage between them and the tables they read.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	s.client = newClient(s.config.Host, s.config.APIKey, 30*time.Second, s.config.VerifySSL)
	if s.config.APIKey == "" {
		if err := s.client.login(ctx, s.config.Username, s.config.Password); err != nil {
			return nil, fmt.Errorf("logging in to Metabase: %w", err)
		}
	}

	log.Debug().Str("host", s.config.Host).Msg("Starting Metabase discovery")

	c := newCollector(s.config, s.client)
	if err := c.run(ctx); err != nil {
		return nil, err
	}

	log.Info().
		Int("assets", len(c.assets)).
		Int("lineages", len(c.edges)).
		Msg("Metabase discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:  c.assets,
		Lineage: c.edges,
	}, nil
}

// collector holds one run's state: what Metabase returned, the assets
// built from it, and the edges between them.
type collector struct {
	config *Config
	client *client

	collections map[collectionID]collection
	databases   map[int]database
	// tableIndexes caches /database/{id}/metadata per database, fetched
	// the first time a card on that database needs its tables.
	tableIndexes map[int]*tableIndex

	// cards is every card fetched, in API order, whether or not it
	// became an asset. cardMRNs holds only the ones that did.
	cards    []card
	cardMRNs map[int]string
	// dashboardsByCard maps a card id to the MRNs of the dashboards
	// showing it.
	dashboardsByCard map[int][]string

	assets []pluginsdk.Asset
	edges  []pluginsdk.LineageEdge
	seen   map[string]struct{}
}

func newCollector(config *Config, client *client) *collector {
	return &collector{
		config:           config,
		client:           client,
		collections:      make(map[collectionID]collection),
		databases:        make(map[int]database),
		tableIndexes:     make(map[int]*tableIndex),
		cardMRNs:         make(map[int]string),
		dashboardsByCard: make(map[int][]string),
		seen:             make(map[string]struct{}),
	}
}

func (c *collector) run(ctx context.Context) error {
	collections, err := c.client.listCollections(ctx)
	if err != nil {
		return fmt.Errorf("listing collections: %w", err)
	}
	for _, col := range collections {
		c.collections[col.ID] = col
	}
	log.Debug().Int("count", len(collections)).Msg("Found collections")

	databases, err := c.client.listDatabases(ctx)
	if err != nil {
		return fmt.Errorf("listing databases: %w", err)
	}
	for _, db := range databases {
		c.databases[db.ID] = db
	}
	log.Debug().Int("count", len(databases)).Msg("Found databases")

	c.cards, err = c.client.listCards(ctx, false)
	if err != nil {
		return fmt.Errorf("listing cards: %w", err)
	}
	if c.config.IncludeArchived {
		archived, err := c.client.listCards(ctx, true)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to list archived cards")
		} else {
			c.cards = append(c.cards, archived...)
		}
	}
	log.Debug().Int("count", len(c.cards)).Msg("Found cards")

	dashboards, err := c.client.listDashboards(ctx, false)
	if err != nil {
		return fmt.Errorf("listing dashboards: %w", err)
	}
	if c.config.IncludeArchived {
		archived, err := c.client.listDashboards(ctx, true)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to list archived dashboards")
		} else {
			dashboards = append(dashboards, archived...)
		}
	}
	log.Debug().Int("count", len(dashboards)).Msg("Found dashboards")

	// Cards first: the dashboard pass links to their MRNs.
	var cardAssets []pluginsdk.Asset
	for _, card := range c.cards {
		if card.Archived && !c.config.IncludeArchived {
			continue
		}
		if card.Type == "model" && !c.config.IncludeModels {
			continue
		}
		if card.Type != "model" && !c.config.IncludeCharts {
			continue
		}
		asset := c.cardAsset(card)
		c.cardMRNs[card.ID] = *asset.MRN
		cardAssets = append(cardAssets, asset)
	}

	for _, d := range dashboards {
		if d.Archived && !c.config.IncludeArchived {
			continue
		}
		c.assets = append(c.assets, c.dashboardAsset(ctx, d))
	}
	c.assets = append(c.assets, cardAssets...)

	if c.config.DiscoverLineage {
		c.discoverLineage(ctx)
	}
	return nil
}

// dashboardAsset builds the Dashboard asset and links it to the cards
// placed on it. The placements only come with the dashboard's detail
// call; when that fails the dashboard is still catalogued from the
// list entry, without its card count or CONTAINS edges. Only the
// placements are taken from the detail: for an archived dashboard it
// reports the Trash as the collection, while the list keeps the
// collection the dashboard was archived from, which is the name people
// know it by.
func (c *collector) dashboardAsset(ctx context.Context, d dashboard) pluginsdk.Asset {
	detail, err := c.client.getDashboard(ctx, d.ID)
	if err != nil {
		log.Warn().Err(err).Int("dashboard_id", d.ID).Str("name", d.Name).Msg("Failed to fetch dashboard detail")
	} else {
		d.Dashcards = detail.Dashcards
		d.OrderedCards = detail.OrderedCards
	}

	name := c.assetName(d.CollectionID, d.Name)
	mrnValue := assetMRN("Dashboard", name)
	url := fmt.Sprintf("%s/dashboard/%d-%s", c.config.Host, d.ID, slug(d.Name))

	metadata := map[string]any{
		"id":       d.ID,
		"archived": d.Archived,
		"url":      url,
	}
	c.putCollection(metadata, d.CollectionID)
	putIf(metadata, "creator_id", d.CreatorID)
	putIf(metadata, "created_at", d.CreatedAt)
	putIf(metadata, "updated_at", d.UpdatedAt)

	cardIDs := d.cardIDs()
	if detail != nil {
		metadata["card_count"] = len(cardIDs)
	}
	for _, cardID := range cardIDs {
		c.dashboardsByCard[cardID] = append(c.dashboardsByCard[cardID], mrnValue)
		if cardMRN, ok := c.cardMRNs[cardID]; ok {
			c.link(mrnValue, cardMRN, "CONTAINS")
		}
	}

	asset := c.newAsset("Dashboard", name, mrnValue, url, metadata)
	if desc := strings.TrimSpace(d.Description); desc != "" {
		asset.Description = &desc
	}
	return asset
}

// cardAsset builds the asset for one card: a Chart for a question or
// metric, a Data Model Object for a model.
func (c *collector) cardAsset(card card) pluginsdk.Asset {
	assetType := "Chart"
	page := "question"
	if card.Type == "model" {
		assetType = "Data Model Object"
		page = "model"
	}

	query, err := parseDatasetQuery(card.DatasetQuery)
	if err != nil {
		log.Warn().Err(err).Int("card_id", card.ID).Str("name", card.Name).Msg("Failed to parse card query")
	}
	queryType := card.QueryType
	if queryType == "" {
		queryType = "query"
		if query.isNative() {
			queryType = "native"
		}
	}

	name := c.assetName(card.CollectionID, card.Name)
	mrnValue := assetMRN(assetType, name)
	url := fmt.Sprintf("%s/%s/%d-%s", c.config.Host, page, card.ID, slug(card.Name))

	metadata := map[string]any{
		"id":         card.ID,
		"display":    card.Display,
		"chart_type": chartType(card.Display),
		"card_type":  card.Type,
		"query_type": queryType,
		"archived":   card.Archived,
		"url":        url,
	}
	c.putCollection(metadata, card.CollectionID)
	if db, ok := c.databases[card.DatabaseID]; ok {
		metadata["database"] = db.Name
	}
	putIf(metadata, "database_id", card.DatabaseID)
	putIf(metadata, "table_id", card.TableID)
	putIf(metadata, "creator_id", card.CreatorID)
	putIf(metadata, "created_at", card.CreatedAt)
	putIf(metadata, "updated_at", card.UpdatedAt)

	asset := c.newAsset(assetType, name, mrnValue, url, metadata)
	if desc := strings.TrimSpace(card.Description); desc != "" {
		asset.Description = &desc
	}
	if sql := query.sql(); sql != "" {
		language := "SQL"
		asset.Query = &sql
		asset.QueryLanguage = &language
	}
	return asset
}

func (c *collector) newAsset(assetType, name, mrnValue, url string, metadata map[string]any) pluginsdk.Asset {
	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    map[string]string{},
		Tags:      pluginsdk.InterpolateTags(c.config.Tags, metadata),
		ExternalLinks: []pluginsdk.AssetExternalLink{{
			Name: "Open in Metabase",
			URL:  url,
		}},
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// discoverLineage links every card to the tables and cards it reads,
// and those tables to every dashboard showing the card. Cards that
// were not emitted (an archived card on a dashboard, or charts turned
// off) still carry their tables through to their dashboards.
func (c *collector) discoverLineage(ctx context.Context) {
	for _, card := range c.cards {
		cardMRN, emitted := c.cardMRNs[card.ID]
		dashboards := c.dashboardsByCard[card.ID]
		if !emitted && len(dashboards) == 0 {
			continue
		}

		tables, sourceCards := c.cardSources(ctx, card)
		for _, tableMRN := range tables {
			if emitted {
				c.link(tableMRN, cardMRN, "FEEDS")
			}
			for _, dashboardMRN := range dashboards {
				c.link(tableMRN, dashboardMRN, "FEEDS")
			}
		}
		if !emitted {
			continue
		}
		for _, sourceID := range sourceCards {
			if sourceMRN, ok := c.cardMRNs[sourceID]; ok && sourceMRN != cardMRN {
				c.link(sourceMRN, cardMRN, "FEEDS")
			}
		}
	}
}

// cardSources resolves a card's inputs to table MRNs, named as the
// warehouse's own plugin names them, and to the ids of the cards it
// builds on. An MBQL card names its sources by id; a native card is
// scanned for the tables after FROM and JOIN, which are then matched
// against the tables Metabase has synced for the card's database.
func (c *collector) cardSources(ctx context.Context, card card) ([]string, []int) {
	query, err := parseDatasetQuery(card.DatasetQuery)
	if err != nil {
		return nil, nil
	}

	db, ok := c.databases[card.DatabaseID]
	if !ok {
		log.Debug().Int("card_id", card.ID).Int("database_id", card.DatabaseID).Msg("Card refers to an unknown database")
		return nil, nil
	}

	var tables []table
	var sourceCards []int

	if query.isNative() {
		refs, cardIDs := sqlTableRefs(query.sql())
		sourceCards = cardIDs
		if len(refs) > 0 {
			index := c.tables(ctx, db.ID)
			for _, ref := range refs {
				if t, ok := index.resolve(ref); ok {
					tables = append(tables, t)
				} else {
					log.Debug().Int("card_id", card.ID).Str("table", join(ref.Schema, ref.Name)).Msg("Table in native query not found in Metabase metadata")
				}
			}
		}
	} else {
		sources := query.sources()
		if len(sources) == 0 {
			// Older card shapes may carry the source only at the top level.
			sources = append(sources, querySource{TableID: card.TableID, CardID: card.SourceCardID})
		}
		for _, src := range sources {
			if src.CardID != 0 {
				sourceCards = append(sourceCards, src.CardID)
			}
			if src.TableID == 0 {
				continue
			}
			if t, ok := c.tables(ctx, db.ID).byID[src.TableID]; ok {
				tables = append(tables, t)
			} else {
				log.Debug().Int("card_id", card.ID).Int("table_id", src.TableID).Msg("Card source table not found in Metabase metadata")
			}
		}
	}

	mrns := make([]string, 0, len(tables))
	for _, t := range tables {
		mrns = append(mrns, tableMRN(db, t))
	}
	return mrns, sourceCards
}

// tableIndex is the tables of one database, by id and by name.
type tableIndex struct {
	byID   map[int]table
	byName map[string][]table
}

// tables returns the table index for a database, fetching it on first
// use. A database whose metadata cannot be read yields an empty index,
// so its cards simply get no table lineage.
func (c *collector) tables(ctx context.Context, databaseID int) *tableIndex {
	if index, ok := c.tableIndexes[databaseID]; ok {
		return index
	}

	index := &tableIndex{byID: make(map[int]table), byName: make(map[string][]table)}
	c.tableIndexes[databaseID] = index

	tables, err := c.client.listTables(ctx, databaseID)
	if err != nil {
		log.Warn().Err(err).Int("database_id", databaseID).Msg("Failed to fetch database metadata")
		return index
	}
	for _, t := range tables {
		index.byID[t.ID] = t
		key := strings.ToLower(t.Name)
		index.byName[key] = append(index.byName[key], t)
	}
	return index
}

// resolve finds the synced table a SQL reference points at. A
// qualified name must match schema and table; a bare name matches
// when only one schema has it, or else the default schema does.
func (idx *tableIndex) resolve(ref tableRef) (table, bool) {
	candidates := idx.byName[strings.ToLower(ref.Name)]
	if ref.Schema != "" {
		for _, t := range candidates {
			if strings.EqualFold(t.Schema, ref.Schema) {
				return t, true
			}
		}
		return table{}, false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	for _, t := range candidates {
		switch strings.ToLower(t.Schema) {
		case "", "public", "dbo":
			return t, true
		}
	}
	return table{}, false
}

// assetName qualifies a dashboard or card name with the path of the
// collection holding it, so two same-named cards in different
// collections stay apart. Items in the root collection keep their
// bare name.
func (c *collector) assetName(id collectionID, name string) string {
	if path := c.collectionPath(id); path != "" {
		return path + "/" + name
	}
	return name
}

// collectionPath is the names of a collection and its ancestors from
// the root down, joined with "/". The root collection itself ("Our
// analytics") is left out.
func (c *collector) collectionPath(id collectionID) string {
	if id == "" || id == rootCollectionID {
		return ""
	}
	col, ok := c.collections[id]
	if !ok {
		log.Debug().Str("collection_id", string(id)).Msg("Collection not in the listing, naming by bare name")
		return ""
	}

	var names []string
	// location is the ancestor chain as "/<id>/<id>/", "/" at top level.
	for _, ancestor := range strings.Split(strings.Trim(col.Location, "/"), "/") {
		if ancestor == "" {
			continue
		}
		if parent, ok := c.collections[collectionID(ancestor)]; ok {
			names = append(names, parent.Name)
		}
	}
	names = append(names, col.Name)
	return strings.Join(names, "/")
}

// putCollection records the collection an item lives in.
func (c *collector) putCollection(metadata map[string]any, id collectionID) {
	if path := c.collectionPath(id); path != "" {
		metadata["collection"] = path
	}
	if n, err := strconv.Atoi(string(id)); err == nil {
		metadata["collection_id"] = n
	}
}

func (c *collector) link(source, target, edgeType string) {
	key := source + "|" + target + "|" + edgeType
	if _, dup := c.seen[key]; dup {
		return
	}
	c.seen[key] = struct{}{}
	c.edges = append(c.edges, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
}

// putIf stores a value unless it is empty.
func putIf(metadata map[string]any, key string, value any) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return
		}
	case int:
		if v == 0 {
			return
		}
	}
	metadata[key] = value
}

// chartType normalises Metabase's display value to the chart types
// Marmot's other dashboard sources use.
func chartType(display string) string {
	switch display {
	case "table":
		return "Table"
	case "bar", "row", "dist_bar":
		return "Bar"
	case "line", "dual_line":
		return "Line"
	case "pie":
		return "Pie"
	case "area", "treemap":
		return "Area"
	case "scatter":
		return "Scatter"
	case "map":
		return "Map"
	case "gauge":
		return "Gauge"
	case "scalar", "smartscalar", "progress":
		return "Text"
	default:
		return "Other"
	}
}

// slug builds the readable tail of a Metabase URL: the lowercased name
// with every character outside a-z and 0-9 replaced by a hyphen.
// Metabase routes on the numeric id and ignores the tail.
func slug(name string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, strings.ToLower(name))
}

// assetMRN is the single place a Metabase MRN is built, so assets and
// the edges between them can never drift apart.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
