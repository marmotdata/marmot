package googledrive

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// sourceName is what Drive-sourced properties are recorded under on an
// asset, so a merged asset shows which run contributed what.
const sourceName = "Google Drive"

// addDrive catalogues the drive itself: the container everything else
// hangs from, and the root someone browsing actually starts at.
func (c *collector) addDrive(name string) string {
	if name == "" {
		return ""
	}

	metadata := map[string]interface{}{}
	putIf(metadata, "drive", name)
	putIf(metadata, "drive_id", c.config.DriveID)

	asset := c.record(c.newAsset(driveItem{}, "Drive", name, metadata))
	return *asset.MRN
}

func (c *collector) addFolder(f driveItem, index map[string]driveItem) {
	name := drivePath(f, index)
	if name == "" {
		return
	}

	metadata := map[string]interface{}{}
	putIf(metadata, "path", "/"+name)
	putIf(metadata, "shared", f.Shared)
	putIf(metadata, "owners", f.ownerNames())

	c.add(f, "Folder", name, metadata)
}

func (c *collector) addFile(f driveItem, index map[string]driveItem) {
	name := drivePath(f, index)
	if name == "" {
		return
	}

	metadata := map[string]interface{}{}
	putIf(metadata, "path", "/"+name)
	putIf(metadata, "mime_type", f.MimeType)
	putIf(metadata, "file_extension", f.FileExtension)
	putIf(metadata, "file_type", fileType(f.MimeType))
	putIf(metadata, "size", f.Size)
	putIf(metadata, "file_version", f.Version)
	putIf(metadata, "checksum", f.MD5Checksum)
	putIf(metadata, "shared", f.Shared)
	putIf(metadata, "owners", f.ownerNames())

	c.add(f, "File", name, metadata)
}

// addSpreadsheet catalogues a Google Sheet. Its sheets are read
// separately, because that is one API call per spreadsheet and a drive
// can hold thousands.
func (c *collector) addSpreadsheet(f driveItem, index map[string]driveItem) *spreadsheetRef {
	name := drivePath(f, index)
	if name == "" {
		return nil
	}

	metadata := map[string]interface{}{}
	putIf(metadata, "path", "/"+name)
	putIf(metadata, "mime_type", f.MimeType)
	putIf(metadata, "file_type", "Spreadsheet")
	putIf(metadata, "size", f.Size)
	putIf(metadata, "shared", f.Shared)
	putIf(metadata, "owners", f.ownerNames())

	spreadsheet := c.add(f, "Spreadsheet", name, metadata)
	return &spreadsheetRef{item: f, name: name, mrn: *spreadsheet.MRN}
}

// spreadsheetRef is a catalogued spreadsheet waiting for its sheets.
type spreadsheetRef struct {
	item driveItem
	name string
	mrn  string
}

// addWorksheets catalogues each sheet of every spreadsheet as a Table
// carrying that sheet's header row as columns. The Sheets API is asked
// about one spreadsheet at a time, so the requests are made in parallel;
// the assets are appended afterwards in listing order, which keeps a
// run's output the same whatever order the replies arrived in.
func (c *collector) addWorksheets(ctx context.Context, spreadsheets []spreadsheetRef, sheets sheetsAPI) {
	if !c.config.IncludeWorksheets || sheets == nil || len(spreadsheets) == 0 {
		return
	}

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		sem    = make(chan struct{}, c.config.Concurrency)
		tabsOf = make(map[string][]sheetTab, len(spreadsheets))
	)

	for _, s := range spreadsheets {
		wg.Add(1)
		go func(s spreadsheetRef) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			tabs, err := sheets.sheets(ctx, s.item.ID, c.config.HeaderRow)
			if err != nil {
				// A spreadsheet the service account cannot open is normal
				// in a shared drive; the file itself is still catalogued.
				log.Debug().Err(err).Str("spreadsheet", s.name).Msg("Could not read sheets")
				mu.Lock()
				c.skipped["sheets could not be read"]++
				mu.Unlock()
				return
			}

			mu.Lock()
			tabsOf[s.item.ID] = tabs
			mu.Unlock()
		}(s)
	}

	wg.Wait()

	for _, s := range spreadsheets {
		for _, tab := range tabsOf[s.item.ID] {
			sheetName := s.name + "/" + tab.Title

			sheetMetadata := map[string]interface{}{}
			putIf(sheetMetadata, "spreadsheet", s.item.Name)
			putIf(sheetMetadata, "path", "/"+s.name)
			putIf(sheetMetadata, "column_count", len(tab.Columns))
			putIf(sheetMetadata, "row_count", tab.RowCount)
			putIf(sheetMetadata, "hidden", tab.Hidden)

			// The sheet is its own object, so it reads as its own tab
			// title rather than the spreadsheet's name.
			sheetItem := s.item
			sheetItem.Name = tab.Title

			asset := c.newAsset(sheetItem, "Table", sheetName, sheetMetadata)
			setColumns(&asset, tab.Columns)
			c.record(asset)

			c.link(s.mrn, *asset.MRN)
		}
	}
}

// record adds an asset to the run. Two Drive items can land on one MRN,
// because a name is slugified: a folder "Q4 Reports" and a folder
// "Q4-Reports" side by side both become "q4-reports". Merging them is
// the only thing Marmot can do, but it must not happen silently.
func (c *collector) record(asset pluginsdk.Asset) pluginsdk.Asset {
	if asset.MRN != nil {
		if first, seen := c.itemByMRN[*asset.MRN]; seen {
			c.collisions = append(c.collisions, collision{
				MRN:    *asset.MRN,
				First:  first,
				Second: asset.Metadata,
			})
		} else {
			c.itemByMRN[*asset.MRN] = asset.Metadata
		}
	}

	c.assets = append(c.assets, asset)
	return asset
}

// add builds an asset for a Drive item, records it, and links it to the
// folder it lives in.
func (c *collector) add(f driveItem, assetType, name string, metadata map[string]interface{}) pluginsdk.Asset {
	asset := c.record(c.newAsset(f, assetType, name, metadata))

	c.mrnByID[f.ID] = *asset.MRN

	linked := false
	for _, parent := range f.Parents {
		if parentMRN, ok := c.mrnByID[parent]; ok {
			c.link(parentMRN, *asset.MRN)
			linked = true
		}
	}

	// Anything with no folder above it sits at the top of the drive.
	if !linked && c.driveMRN != "" && c.driveMRN != *asset.MRN {
		c.link(c.driveMRN, *asset.MRN)
	}

	return asset
}

func (c *collector) newAsset(f driveItem, assetType, name string, metadata map[string]interface{}) pluginsdk.Asset {
	// Name and MRN are the same string on purpose. name is the item's
	// path, which is what identifies it: two folders can each hold a
	// "notes.md". The Contents tab supplies the folder it sits in.
	mrnValue := mrn.New(assetType, provider, name)

	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	putIf(metadata, "drive_id", f.ID)
	if !f.ModifiedTime.IsZero() {
		metadata["modified_at"] = f.ModifiedTime.UTC().Format(time.RFC3339)
	}

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(c.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       sourceName,
			LastSyncAt: c.now,
			Properties: metadata,
			Priority:   1,
		}},
	}

	if description := strings.TrimSpace(f.Description); description != "" {
		asset.Description = &description
	}

	if f.WebViewLink != "" {
		asset.ExternalLinks = append(asset.ExternalLinks, pluginsdk.AssetExternalLink{
			Name: "Open in Google Drive",
			Icon: "mdi:open-in-new",
			URL:  f.WebViewLink,
		})
	}
	for _, link := range c.config.ExternalLinks {
		asset.ExternalLinks = append(asset.ExternalLinks, pluginsdk.AssetExternalLink{
			Name: link.Name, Icon: link.Icon, URL: link.URL,
		})
	}

	return asset
}

func (c *collector) link(parent, child string) {
	c.lineage = append(c.lineage, pluginsdk.LineageEdge{
		Source: parent,
		Target: child,
		Type:   "CONTAINS",
	})
}

// drivePath is an item's location written the way people write it, as
// the folder names above it joined with slashes. It is also the asset
// name, so a file keeps its place in the tree even though Marmot stores
// assets flat.
func drivePath(f driveItem, index map[string]driveItem) string {
	parts := []string{f.Name}

	// Drive is a graph, not a tree: an item can sit in more than one
	// folder and, with a corrupt listing, in itself. Walking a bounded
	// number of steps and refusing to revisit keeps this terminating.
	seen := map[string]bool{f.ID: true}
	current := f

	for len(parts) < 64 {
		if len(current.Parents) == 0 {
			break
		}
		parent, ok := index[current.Parents[0]]
		if !ok || seen[parent.ID] {
			break
		}
		seen[parent.ID] = true
		parts = append(parts, parent.Name)
		current = parent
	}

	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, "/")
}

// fileType is the broad kind of a file, matching the vocabulary
// OpenMetadata reports so the two agree on a merged asset.
func fileType(mimeType string) string {
	switch {
	case mimeType == "":
		return ""
	case strings.Contains(mimeType, "spreadsheet") || strings.Contains(mimeType, "excel"):
		return "Spreadsheet"
	case strings.Contains(mimeType, "presentation") || strings.Contains(mimeType, "powerpoint"):
		return "Presentation"
	case strings.HasPrefix(mimeType, "image/"):
		return "Image"
	case strings.HasPrefix(mimeType, "video/"):
		return "Video"
	case strings.HasPrefix(mimeType, "audio/"):
		return "Audio"
	case strings.Contains(mimeType, "pdf"), strings.Contains(mimeType, "document"),
		strings.Contains(mimeType, "word"), strings.HasPrefix(mimeType, "text/"):
		return "Document"
	default:
		return "Other"
	}
}

// setColumns attaches columns in the shape Marmot's other plugins use:
// a JSON array under the schema's "columns" key.
func setColumns(asset *pluginsdk.Asset, columns []string) {
	if len(columns) == 0 {
		return
	}

	fields := make([]map[string]interface{}, 0, len(columns))
	for i, name := range columns {
		fields = append(fields, map[string]interface{}{
			"column_name":      name,
			"data_type":        "STRING",
			"ordinal_position": i + 1,
		})
	}

	encoded, err := json.Marshal(fields)
	if err != nil {
		return
	}
	asset.Schema["columns"] = string(encoded)
}

// putIf writes a metadata key only when the value carries information.
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
	case int64:
		if v == 0 {
			return
		}
	case bool:
		if !v {
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
