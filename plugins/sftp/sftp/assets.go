package sftp

import (
	"context"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// Asset types this plugin creates. A server root is not an asset: the
// directories directly below each configured root are the top of the
// tree, so moving a root does not rename everything under it.
const (
	typeFolder = "Folder"
	typeFile   = "File"
)

// collector turns the walk's records into assets, lineage and statistics.
type collector struct {
	config *Config
	fs     fileSystem
	now    time.Time

	assets     []pluginsdk.Asset
	lineage    []pluginsdk.LineageEdge
	statistics []pluginsdk.Statistic

	// pathByMRN holds the first path to claim each MRN, so a second one
	// claiming it can be reported rather than silently merged.
	pathByMRN map[string]string
	skipped   map[string]int
}

func newCollector(config *Config, fs fileSystem) *collector {
	return &collector{
		config:    config,
		fs:        fs,
		now:       time.Now(),
		pathByMRN: make(map[string]string),
		skipped:   make(map[string]int),
	}
}

func (c *collector) build(ctx context.Context, directories []dirRecord, files []fileRecord) {
	// The set of catalogued folders is built first, so an edge is only
	// created when both ends really became assets.
	folders := make(map[string]bool, len(directories))
	for _, d := range directories {
		folders[d.Name] = true
	}

	for _, d := range directories {
		c.addFolder(d, folders)
	}

	for _, f := range files {
		if ctx.Err() != nil {
			log.Warn().Msg("Discovery cancelled, returning what was found so far")
			break
		}
		c.addFile(f, folders)
	}

	c.report()
}

func (c *collector) addFolder(d dirRecord, folders map[string]bool) {
	metadata := map[string]any{
		"path":            d.Path,
		"root":            d.Root,
		"file_count":      d.FileCount,
		"directory_count": d.DirCount,
		"size_bytes":      d.SizeBytes,
	}
	putString(metadata, "parent", d.Parent)
	putTime(metadata, "modified", d.Modified)

	asset := c.newAsset(typeFolder, d.Name, metadata)
	c.record(asset, d.Path)

	if d.Parent != "" && folders[d.Parent] {
		c.link(assetMRN(typeFolder, d.Parent), *asset.MRN)
	}
}

func (c *collector) addFile(f fileRecord, folders map[string]bool) {
	kind := fileKind(f.Name)

	metadata := map[string]any{
		"path":      f.Path,
		"size":      f.Size,
		"file_type": kind,
		"host":      c.config.Host,
		"port":      c.config.Port,
	}
	putString(metadata, "directory", f.Directory)
	putString(metadata, "extension", f.Extension)
	putString(metadata, "mime_type", mimeType(f.Name))
	putString(metadata, "mode", f.Mode)
	putTime(metadata, "modified", f.Modified)
	if f.HasOwner {
		metadata["owner_uid"] = int(f.OwnerUID)
		metadata["owner_gid"] = int(f.OwnerGID)
	}

	columns, rows, exact := c.readColumns(f, kind)
	if rows >= 0 {
		metadata["row_count_exact"] = exact
	}

	asset := c.newAsset(typeFile, f.Name, metadata)
	if columns != nil && c.config.IncludeColumns {
		if err := columns.attach(&asset); err != nil {
			log.Warn().Err(err).Str("path", f.Path).Msg("Failed to attach columns")
		}
	}
	c.record(asset, f.Path)

	if c.config.IncludeStatistics {
		c.statistics = append(c.statistics, pluginsdk.Statistic{
			AssetMRN:   *asset.MRN,
			MetricName: "asset.size_bytes",
			Value:      float64(f.Size),
		})
		if rows >= 0 {
			c.statistics = append(c.statistics,
				pluginsdk.Statistic{AssetMRN: *asset.MRN, MetricName: "asset.row_count", Value: float64(rows)},
				pluginsdk.Statistic{AssetMRN: *asset.MRN, MetricName: "asset.column_count", Value: float64(columns.count())},
			)
		}
	}

	if f.Directory != "" && folders[f.Directory] {
		c.link(assetMRN(typeFolder, f.Directory), *asset.MRN)
	}
}

// columnSet is a file's inferred columns. Delimited and JSON files carry
// different extra fields, so the two shapes are kept apart until they
// are written onto the asset.
type columnSet struct {
	delimited []pluginsdk.Column
	json      []jsonColumn
}

func (s *columnSet) attach(asset *pluginsdk.Asset) error {
	if s.json != nil {
		return pluginsdk.SetColumns(asset, s.json)
	}
	return pluginsdk.SetColumns(asset, s.delimited)
}

func (s *columnSet) count() int {
	if s == nil {
		return 0
	}
	if s.json != nil {
		return len(s.json)
	}
	return len(s.delimited)
}

// readColumns reads a file to work out its columns and, for delimited
// files, how many rows it holds. rowCount is -1 when no row count was
// produced, which is every kind but csv and tsv.
func (c *collector) readColumns(f fileRecord, kind string) (columns *columnSet, rowCount int64, exact bool) {
	if !hasColumns(kind) {
		return nil, -1, false
	}

	isDelimited := kind == kindCSV || kind == kindTSV
	// Reading a file costs a round trip per file, so it only happens when
	// something in the result actually needs it.
	if !c.config.IncludeColumns && !(c.config.IncludeStatistics && isDelimited) {
		return nil, -1, false
	}

	file, err := c.fs.Open(f.Path)
	if err != nil {
		// A file the login cannot read is normal on a shared server; it is
		// still catalogued, just without columns.
		log.Warn().Err(err).Str("path", f.Path).Msg("Failed to open file, skipping its columns")
		c.skipped["file could not be read"]++
		return nil, -1, false
	}
	defer file.Close()

	if isDelimited {
		data, err := readDelimited(file, separator(kind), c.config.MaxReadBytes, c.config.SampleRows)
		if err != nil {
			log.Warn().Err(err).Str("path", f.Path).Msg("Failed to parse delimited file")
			c.skipped["file could not be parsed"]++
			return nil, -1, false
		}
		if len(data.Header) == 0 {
			// An empty file has no header, so there is nothing to describe.
			return nil, 0, data.Exact
		}
		return &columnSet{delimited: inferDelimitedColumns(data.Header, data.Rows)}, data.RowCount, data.Exact
	}

	data, err := readJSON(file, c.config.MaxReadBytes, c.config.SampleRows)
	if err != nil {
		log.Warn().Err(err).Str("path", f.Path).Msg("Failed to parse json file")
		c.skipped["file could not be parsed"]++
		return nil, -1, false
	}
	if len(data.Records) == 0 {
		return nil, -1, false
	}
	return &columnSet{json: inferJSONColumns(data.Records)}, -1, false
}

func (c *collector) newAsset(assetType, name string, metadata map[string]any) pluginsdk.Asset {
	mrnValue := assetMRN(assetType, name)

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(c.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: c.now,
			Properties: metadata,
			Priority:   1,
		}},
	}

	// An SFTP server has no web UI to deep link into, so the only links
	// are the ones the config supplies.
	for _, link := range c.config.ExternalLinks {
		asset.ExternalLinks = append(asset.ExternalLinks, pluginsdk.AssetExternalLink{
			Name: link.Name, Icon: link.Icon, URL: link.URL,
		})
	}

	return asset
}

// record adds an asset to the run. Two paths can land on one MRN,
// because a name is sanitised: "q4 reports/x" and "q4-reports/x" both
// become "q4-reports-x". Merging them is the only thing Marmot can do,
// but it must not happen silently.
func (c *collector) record(asset pluginsdk.Asset, remotePath string) {
	if first, seen := c.pathByMRN[*asset.MRN]; seen {
		log.Warn().
			Str("mrn", *asset.MRN).
			Str("first", first).
			Str("second", remotePath).
			Msg("Two paths share one Marmot asset because their names resolve the same way")
	} else {
		c.pathByMRN[*asset.MRN] = remotePath
	}

	c.assets = append(c.assets, asset)
}

func (c *collector) link(parent, child string) {
	c.lineage = append(c.lineage, pluginsdk.LineageEdge{
		Source: parent,
		Target: child,
		Type:   "CONTAINS",
	})
}

// report logs what the run produced, so a large server is auditable
// without reading the catalog.
func (c *collector) report() {
	byType := make(map[string]int, 2)
	for _, asset := range c.assets {
		byType[asset.Type]++
	}
	for assetType, count := range byType {
		log.Info().Str("type", assetType).Int("assets", count).Msg("Discovered on the SFTP server")
	}
	for reason, count := range c.skipped {
		log.Info().Str("reason", reason).Int("files", count).Msg("Skipped")
	}
}

// putString writes a metadata key only when the value says something.
func putString(metadata map[string]any, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}

func putTime(metadata map[string]any, key string, value time.Time) {
	if !value.IsZero() {
		metadata[key] = value.UTC().Format(time.RFC3339)
	}
}
