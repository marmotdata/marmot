package amundsen

import (
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// sourceName is what Amundsen-sourced properties are recorded under on
// an asset. It stays distinct from the provider so that an asset
// discovered by both this plugin and the technology's native plugin
// shows both contributions.
const sourceName = "Amundsen"

// Statistics Amundsen can supply. Amundsen counts how often a table or
// dashboard was read rather than how large it is, so these are not the
// asset.row_count family the database plugins emit.
const (
	metricReadCount     = "asset.read_count"
	metricUniqueReaders = "asset.unique_readers"
)

// collector accumulates everything one discovery run produces, and
// remembers the MRN behind every Amundsen key so the lineage passes can
// resolve their endpoints.
type collector struct {
	config     *Config
	now        time.Time
	assets     []pluginsdk.Asset
	lineage    []pluginsdk.LineageEdge
	statistics []pluginsdk.Statistic

	// mrnByKey resolves an Amundsen node key to the MRN of the asset it
	// produced. Amundsen's lineage relationships reference nodes by key.
	mrnByKey map[string]string
	// ownersByTable holds the owners read before the tables, keyed by
	// Amundsen table key.
	ownersByTable map[string][]owner
	// seenMRN keeps two Amundsen entries that project onto one MRN from
	// becoming two assets, and seenEdge does the same for lineage.
	seenMRN  map[string]struct{}
	seenEdge map[string]struct{}
}

func newCollector(config *Config) *collector {
	return &collector{
		config:        config,
		now:           time.Now(),
		mrnByKey:      make(map[string]string),
		ownersByTable: make(map[string][]owner),
		seenMRN:       make(map[string]struct{}),
		seenEdge:      make(map[string]struct{}),
	}
}

// newAsset builds the parts every Amundsen entry shares: identity,
// description, tags, the Amundsen source record and any links the run
// was configured with.
//
// Name and MRN are the same string on purpose: both have to match what
// the technology's own plugin produces, or the two runs stop merging
// onto one asset. provider is used unslugged for the same reason, so a
// provider with a space, such as "Delta Lake", lands that space in the
// MRN, which is where the technology's own plugin lands too.
func (c *collector) newAsset(assetType, provider, name, description string, tags []string, metadata map[string]any) pluginsdk.Asset {
	mrnValue := assetMRN(assetType, provider, name)

	if metadata == nil {
		metadata = make(map[string]any)
	}

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      append(tags, pluginsdk.InterpolateTags(c.config.Tags, metadata)...),
		Sources: []pluginsdk.AssetSource{{
			Name:       sourceName,
			LastSyncAt: c.now,
			Properties: metadata,
			Priority:   1,
		}},
	}

	if c.config.IncludeDescriptions && description != "" {
		asset.Description = &description
	}

	for _, link := range c.config.ExternalLinks {
		asset.ExternalLinks = append(asset.ExternalLinks, pluginsdk.AssetExternalLink{Name: link.Name, Icon: link.Icon, URL: link.URL})
	}

	return asset
}

// add records an asset and the Amundsen key it came from, so the lineage
// passes can find it.
//
// Two Amundsen entries can land on one MRN: naming an asset the way a
// native Marmot plugin would means dropping the levels that plugin does
// not use, so a Postgres public.orders and staging.orders both become
// orders. Keeping the first and reporting the rest is the same outcome
// the native plugin reaches, and is what the OpenMetadata import does.
func (c *collector) add(key string, asset pluginsdk.Asset) bool {
	if _, seen := c.seenMRN[*asset.MRN]; seen {
		log.Debug().Str("mrn", *asset.MRN).Str("key", key).Msg("Skipping an Amundsen entry that projects onto an MRN already discovered")
		return false
	}
	c.seenMRN[*asset.MRN] = struct{}{}

	c.assets = append(c.assets, asset)
	if key != "" {
		c.mrnByKey[key] = *asset.MRN
	}
	return true
}

// link records a lineage edge, skipping one already recorded. Amundsen
// stores its table lineage twice, once as HAS_UPSTREAM and once as
// HAS_DOWNSTREAM, so duplicates are normal rather than a mistake.
func (c *collector) link(source, target, edgeType string) {
	if source == "" || target == "" || source == target {
		return
	}

	key := source + "|" + target + "|" + edgeType
	if _, seen := c.seenEdge[key]; seen {
		return
	}
	c.seenEdge[key] = struct{}{}

	c.lineage = append(c.lineage, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
}

// stat records one usage metric, leaving out the zero that Amundsen
// reports for anything nobody has read.
func (c *collector) stat(assetMRN, metric string, value float64) {
	if !c.config.IncludeUsage || value <= 0 {
		return
	}
	c.statistics = append(c.statistics, pluginsdk.Statistic{AssetMRN: assetMRN, MetricName: metric, Value: value})
}

// assetMRN is the single place an Amundsen MRN is built. Every pass goes
// through it, including the lineage ones, so they can never drift into
// addressing the same table differently.
func assetMRN(assetType, provider, name string) string {
	return mrn.New(assetType, provider, name)
}

// putIf writes a metadata key only when the value carries information,
// keeping empty strings and empty lists out of the catalog.
func putIf(metadata map[string]any, key string, value any) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return
		}
	case []string:
		if len(v) == 0 {
			return
		}
	case int64:
		if v == 0 {
			return
		}
	case nil:
		return
	}
	metadata[key] = value
}
