package openmetadata

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
)

// sourceName is what OpenMetadata-sourced properties are recorded under
// on an asset. It stays distinct from the provider so that an asset
// discovered by both this plugin and the technology's native plugin
// shows both contributions.
const sourceName = "OpenMetadata"

// collector accumulates everything one discovery run produces. It also
// remembers the MRN of every asset by its OpenMetadata id, which is how
// lineage edges (which reference entities by id) are resolved.
type collector struct {
	config     *Config
	now        time.Time
	assets     []pluginsdk.Asset
	lineage    []pluginsdk.LineageEdge
	runHistory []pluginsdk.AssetRunHistory
	terms      []pluginsdk.GlossaryTerm

	// mrnByID resolves an OpenMetadata entity id to the MRN of the asset
	// it produced. Lineage edges reference entities by id.
	mrnByID map[string]string
	// lineageKinds holds the OpenMetadata entity kind of every entity
	// worth asking for lineage.
	lineageKinds map[string]string
	// deferred holds work that can only run once every asset exists,
	// such as linking a model to the tables its features came from.
	deferred []func()
	// entityByMRN and collisions track OpenMetadata entities that share
	// one Marmot MRN, so a run can report what it merged.
	entityByMRN map[string]map[string]interface{}
	collisions  []collision
	// skipped counts entities left out by configuration, per service type.
	skipped map[string]int
}

// collision is two OpenMetadata entities that produced the same MRN.
type collision struct {
	MRN    string
	First  map[string]interface{}
	Second map[string]interface{}
}

func newCollector(config *Config) *collector {
	return &collector{
		config:       config,
		now:          time.Now(),
		mrnByID:      make(map[string]string),
		lineageKinds: make(map[string]string),
		entityByMRN:  make(map[string]map[string]interface{}),
		skipped:      make(map[string]int),
	}
}

// add records an asset and the id it came from, so lineage can find it.
//
// Two OpenMetadata entities can land on one MRN: naming an asset the way
// a native Marmot plugin would means dropping the levels that plugin
// does not use, and two services of the same technology can hold the
// same table name. Marmot merges them into one asset, which is usually
// what a reader wants and is the same thing the native plugin does, so
// the collision is counted and reported rather than treated as an error.
func (c *collector) add(id string, asset pluginsdk.Asset) {
	if asset.MRN != nil {
		if first, seen := c.entityByMRN[*asset.MRN]; seen {
			c.collisions = append(c.collisions, collision{MRN: *asset.MRN, First: first, Second: asset.Metadata})
		} else {
			c.entityByMRN[*asset.MRN] = asset.Metadata
		}
	}

	c.assets = append(c.assets, asset)
	if id != "" && asset.MRN != nil {
		c.mrnByID[id] = *asset.MRN
	}
}

// link records a lineage edge between two MRNs.
func (c *collector) link(source, target, edgeType string) {
	c.lineage = append(c.lineage, pluginsdk.LineageEdge{
		Source: source,
		Target: target,
		Type:   edgeType,
	})
}

// mrnName picks the name component of an asset's MRN. In native mode it
// is the name the technology's own Marmot plugin would produce, so the
// two runs land on the same asset. In qualified mode it is the full
// OpenMetadata path below the service, which keeps two services of the
// same technology apart at the cost of merging.
func (c *collector) mrnName(native, fqn string) string {
	if c.config.Naming == namingQualified {
		// The whole path, service included: without the service two
		// instances of the same technology holding the same table name
		// would still collapse, which is the thing this mode exists to
		// prevent.
		if qualified := strings.Join(splitFQN(fqn), "."); qualified != "" {
			return qualified
		}
	}
	return native
}

// newAsset builds the asset shared by every entity kind: identity,
// description, the OpenMetadata provenance metadata, tags, and links
// back to OpenMetadata and to the system the entity lives in.
func (c *collector) newAsset(base entityBase, kind, assetType string, p projection, name string, metadata map[string]interface{}) pluginsdk.Asset {
	// name is the qualified path that identifies the asset, so it is what
	// the MRN is built from. What people read is the object's own name:
	// a table is "orders", not "sales.public.orders". Marmot keeps the
	// two apart, and every link is built from the MRN rather than from
	// the name, the same way the ClickHouse, Iceberg and dbt plugins do.
	mrnValue := mrn.New(assetType, mrnService(p.Provider), name)

	displayName := strings.TrimSpace(base.Name)
	if displayName == "" {
		displayName = name
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	c.stampProvenance(base, metadata)
	c.trackForLineage(kind, base.ID)

	asset := pluginsdk.Asset{
		Name:          &displayName,
		MRN:           &mrnValue,
		Type:          assetType,
		Providers:     []string{p.Provider},
		Metadata:      metadata,
		Schema:        make(map[string]string),
		Tags:          c.tagsFor(base, metadata),
		Terms:         c.termsFor(base),
		ExternalLinks: c.linksFor(base, kind),
		Sources: []pluginsdk.AssetSource{{
			Name:       sourceName,
			LastSyncAt: c.now,
			Properties: metadata,
			Priority:   c.config.SourcePriority,
		}},
	}

	if description := strings.TrimSpace(base.Description); description != "" {
		asset.Description = &description
	}

	return asset
}

// stampProvenance records where in OpenMetadata an asset came from, plus
// the curation OpenMetadata holds that Marmot cannot model as first
// class objects yet: owners, domains and data products.
func (c *collector) stampProvenance(base entityBase, metadata map[string]interface{}) {
	om := map[string]interface{}{}
	putIf(om, "id", base.ID)
	putIf(om, "fqn", base.FullyQualifiedName)
	putIf(om, "service", base.Service.Name)
	putIf(om, "service_type", base.ServiceType)
	if base.UpdatedAt > 0 {
		om["updated_at"] = time.UnixMilli(base.UpdatedAt).UTC().Format(time.RFC3339)
	}
	if url := c.entityURL(base, ""); url != "" {
		om["url"] = url
	}
	metadata["openmetadata"] = om

	if owners := refNames(base.Owners); len(owners) > 0 {
		metadata["owners"] = owners
	}
	if domains := refNames(base.Domains); len(domains) > 0 {
		metadata["domains"] = domains
	}
	if products := refNames(base.DataProducts); len(products) > 0 {
		metadata["data_products"] = products
	}
	if terms := glossaryTerms(base.Tags); len(terms) > 0 {
		metadata["glossary_terms"] = terms
	}
}

// tagsFor turns OpenMetadata's classification tags into Marmot tags,
// then appends the run's configured tags. An assigned glossary term is
// a term rather than a tag, and only appears here when the run asks for
// a copy of it on the tags as well.
func (c *collector) tagsFor(base entityBase, metadata map[string]interface{}) []string {
	var tags []string

	for _, tag := range base.Tags {
		if tag.State == "Suggested" {
			continue
		}
		if tag.Source == "Glossary" && !c.config.GlossaryTermsAsTags {
			continue
		}
		if tag.Source != "Glossary" && !c.config.TagsFromOpenMetadata {
			continue
		}
		tags = append(tags, tag.TagFQN)
	}

	return append(tags, pluginsdk.InterpolateTags(c.config.Tags, metadata)...)
}

// linksFor builds the external links on an asset: one back to the
// entity in OpenMetadata, and one to the entity in the system it
// actually lives in when OpenMetadata knows that URL.
func (c *collector) linksFor(base entityBase, kind string) []pluginsdk.AssetExternalLink {
	var links []pluginsdk.AssetExternalLink

	if url := c.entityURL(base, kind); url != "" {
		links = append(links, pluginsdk.AssetExternalLink{
			Name: "OpenMetadata",
			Icon: "mdi:database-search",
			URL:  url,
		})
	}

	if base.SourceURL != "" {
		links = append(links, pluginsdk.AssetExternalLink{
			Name: "Open in " + projectionFor(base.ServiceType).Provider,
			Icon: "mdi:open-in-new",
			URL:  base.SourceURL,
		})
	}

	for _, link := range c.config.ExternalLinks {
		links = append(links, pluginsdk.AssetExternalLink{Name: link.Name, Icon: link.Icon, URL: link.URL})
	}

	return links
}

// entityURL is the OpenMetadata UI address of an entity. OpenMetadata
// routes entities by kind and fully qualified name.
func (c *collector) entityURL(base entityBase, kind string) string {
	if kind == "" || base.FullyQualifiedName == "" || !c.config.LinkToOpenMetadata {
		return ""
	}

	// The host may have been given as the API root; the UI lives above it.
	host := strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(c.config.Host), "/"), "/api")

	return fmt.Sprintf("%s/%s/%s", host, kind, base.FullyQualifiedName)
}

// setColumns attaches column information in the shape Marmot's database
// plugins use: a JSON array under the schema's "columns" key.
func setColumns(asset *pluginsdk.Asset, columns []column) {
	if len(columns) == 0 {
		return
	}

	flat := make([]map[string]interface{}, 0, len(columns))
	for _, col := range columns {
		flat = append(flat, columnFields(col))
	}

	encoded, err := json.Marshal(flat)
	if err != nil {
		return
	}
	asset.Schema["columns"] = string(encoded)
}

// columnFields flattens one column, and any nested struct or array
// children, into the field shape Marmot renders.
func columnFields(col column) map[string]interface{} {
	fields := map[string]interface{}{
		"column_name": col.Name,
		"data_type":   col.DataType,
	}
	if col.DataTypeDisplay != "" {
		fields["data_type_display"] = col.DataTypeDisplay
	}
	if col.Description != "" {
		fields["comment"] = col.Description
	}
	if col.Constraint != "" {
		fields["constraint"] = col.Constraint
		fields["is_nullable"] = col.Constraint != "NOT_NULL" && col.Constraint != "PRIMARY_KEY"
		fields["is_primary_key"] = col.Constraint == "PRIMARY_KEY"
	}
	if col.OrdinalPosition > 0 {
		fields["ordinal_position"] = col.OrdinalPosition
	}
	if tags := tagNames(col.Tags); len(tags) > 0 {
		fields["tags"] = tags
	}
	if len(col.Children) > 0 {
		children := make([]map[string]interface{}, 0, len(col.Children))
		for _, child := range col.Children {
			children = append(children, columnFields(child))
		}
		fields["children"] = children
	}
	return fields
}

func refNames(refs []entityRef) []string {
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		if name := ref.DisplayName; name != "" {
			names = append(names, name)
			continue
		}
		if ref.Name != "" {
			names = append(names, ref.Name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

func tagNames(tags []tagLabel) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.TagFQN)
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

// termsFor is the glossary terms curated onto an entity. OpenMetadata
// records an assignment as a tag label sourced from the glossary; in
// Marmot the term is an object of its own, so the assignment travels as
// a term. A run that leaves the glossary behind assigns nothing, because
// the terms it would point at were never imported.
func (c *collector) termsFor(base entityBase) []string {
	if !c.config.IncludeGlossary {
		return nil
	}
	return glossaryTerms(base.Tags)
}

// glossaryTerms is the fully qualified names of the glossary terms
// assigned to an entity. A suggestion nobody accepted is not an
// assignment, the same rule tagsFor applies to tags.
func glossaryTerms(tags []tagLabel) []string {
	var terms []string
	for _, tag := range tags {
		if tag.Source != "Glossary" || tag.State == "Suggested" {
			continue
		}
		terms = append(terms, tag.TagFQN)
	}
	return terms
}

// putIf writes a metadata key only when the value carries information,
// keeping empty strings and zero counts out of the catalog.
func putIf(metadata map[string]interface{}, key string, value interface{}) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return
		}
	case int:
		if v == 0 {
			return
		}
	case float64:
		if v == 0 {
			return
		}
	case []string:
		if len(v) == 0 {
			return
		}
	case nil:
		return
	}
	metadata[key] = value
}

// mrnService is the service component of an MRN for a provider. mrn.New
// sanitizes the name it is given but not the service, so a provider with
// a space in it would put that space into the MRN and into every URL
// built from it. Spaces are dropped rather than hyphenated so that
// "Delta Lake" addresses the same assets as the Delta Lake plugin, which
// slugs itself "DeltaLake".
func mrnService(provider string) string {
	return strings.ReplaceAll(provider, " ", "")
}
