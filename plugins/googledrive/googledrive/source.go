// Package googledrive discovers folders, files and spreadsheets from
// Google Drive.
//
// A drive is catalogued the way someone looking at one would describe
// it: a Folder holds Files and Spreadsheets, and a Spreadsheet holds a
// Table per sheet. Those are the same asset types and names the
// OpenMetadata plugin produces for a Google Drive service, so a catalog
// imported from OpenMetadata and a catalog read straight from Drive
// land on the same assets.
package googledrive

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// provider is the Marmot provider for everything this plugin discovers.
// It matches the OpenMetadata plugin's projection for a GoogleDrive
// service, which is what lets the two merge.
const provider = "GoogleDrive"

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "googledrive",
		Name:        "Google Drive",
		Description: "Discover folders, files and spreadsheets from Google Drive",
		Icon:        "googledrive",
		Category:    "storage",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the Google Drive plugin
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`
	pluginsdk.GCPConfig  `json:",inline"`

	// Access
	ImpersonateUser string `json:"impersonate_user,omitempty" description:"Workspace user to act as. Needed to read a whole organisation's Drive, and requires domain-wide delegation on the service account. Without it, only files shared with the service account are visible"`
	DriveID         string `json:"drive_id,omitempty" label:"Drive ID" description:"Shared drive to read. Empty means the user's My Drive"`
	FolderID        string `json:"folder_id,omitempty" label:"Folder ID" description:"Only discover this folder and everything under it"`

	// What to discover
	IncludeFiles        bool     `json:"include_files" description:"Discover files, not just folders" default:"true"`
	IncludeSpreadsheets bool     `json:"include_spreadsheets" description:"Discover Google Sheets, including each of their sheets" default:"true"`
	IncludeWorksheets   bool     `json:"include_worksheets" description:"Discover each sheet of a spreadsheet as a table, with its columns. Needs the Sheets read scope" default:"true"`
	IncludeTrashed      bool     `json:"include_trashed" description:"Discover files in the trash" default:"false"`
	ExcludeMimeTypes    []string `json:"exclude_mime_types,omitempty" description:"MIME types to skip, for example application/vnd.google-apps.shortcut"`

	// Limits
	MaxFiles    int `json:"max_files" description:"Stop after this many files. 0 means no limit" default:"0" validate:"omitempty,min=0"`
	PageSize    int `json:"page_size" description:"Files per API request" default:"1000" validate:"omitempty,min=1,max=1000"`
	HeaderRow   int `json:"header_row" description:"Row of a sheet that holds the column names" default:"1" validate:"omitempty,min=1"`
	Concurrency int `json:"concurrency" description:"Parallel requests for the sheets of a spreadsheet" default:"8" validate:"omitempty,min=1,max=64"`
}

// Example configuration for the plugin
var _ = `
credentials:
  credentials_file: "/etc/marmot/gcp-service-account.json"
impersonate_user: "data-platform@company.com"
tags:
  - "google-drive"
`

// Source represents the Google Drive plugin
type Source struct {
	config *Config
	// drive and sheets are injected by tests; a real run builds them
	// from the configured credentials.
	drive  driveAPI
	sheets sheetsAPI
}

func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	// ApplyDefaults leaves a key alone when the user wrote it, so an
	// explicit zero reaches here. Concurrency especially: zero would
	// make an unbuffered semaphore and hang the sheets pass forever.
	applyFloor(&config.PageSize, 1000)
	applyFloor(&config.HeaderRow, 1)
	applyFloor(&config.Concurrency, 8)

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover
	// cannot rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, err
	}

	if s.drive == nil {
		clients, err := newClients(ctx, s.config)
		if err != nil {
			return nil, err
		}
		s.drive, s.sheets = clients.drive, clients.sheets
	}

	files, err := s.drive.list(ctx, listOptions{
		DriveID:        s.config.DriveID,
		FolderID:       s.config.FolderID,
		PageSize:       s.config.PageSize,
		MaxFiles:       s.config.MaxFiles,
		IncludeTrashed: s.config.IncludeTrashed,
	})
	if err != nil {
		return nil, fmt.Errorf("listing drive: %w", err)
	}

	c := newCollector(s.config)
	c.driveMRN = c.addDrive(s.config.driveName())
	c.build(ctx, files, s.sheets)

	log.Info().
		Int("assets", len(c.assets)).
		Int("lineage", len(c.lineage)).
		Msg("Discovered Google Drive")

	return &pluginsdk.DiscoveryResult{Assets: c.assets, Lineage: c.lineage}, nil
}

// collector turns a flat file listing into the folder tree Marmot
// stores: a Folder per folder, a File or Spreadsheet per document, and
// one Table per sheet of a spreadsheet.
type collector struct {
	config *Config
	now    time.Time

	assets  []pluginsdk.Asset
	lineage []pluginsdk.LineageEdge

	// mrnByID resolves a Drive file id to the MRN of the asset it
	// produced, so a child can be linked to its parent.
	mrnByID map[string]string
	skipped map[string]int
	// itemByMRN holds the first item to claim each MRN, so a second one
	// claiming it can be reported rather than silently merged.
	itemByMRN  map[string]map[string]interface{}
	collisions []collision
	// driveMRN is the drive everything in this run belongs to.
	driveMRN string
}

// collision is two Drive items that resolved to one Marmot asset.
type collision struct {
	MRN    string
	First  map[string]interface{}
	Second map[string]interface{}
}

// maxReportedCollisions caps how many merged items a run lists
// individually; the total is always reported.
const maxReportedCollisions = 20

// driveName is what the drive is called in the catalog. A shared drive
// is named by its id until the Drive API tells us otherwise; My Drive
// has no id of its own.
func (c *Config) driveName() string {
	if c.DriveID != "" {
		return c.DriveID
	}
	return "My Drive"
}

func newCollector(config *Config) *collector {
	return &collector{
		config:    config,
		now:       time.Now(),
		mrnByID:   make(map[string]string),
		skipped:   make(map[string]int),
		itemByMRN: make(map[string]map[string]interface{}),
	}
}

func (c *collector) build(ctx context.Context, files []driveItem, sheets sheetsAPI) {
	index := make(map[string]driveItem, len(files))
	for _, f := range files {
		index[f.ID] = f
	}

	// Folders first: a file cannot be linked to its folder until the
	// folder has an MRN.
	for _, f := range files {
		if f.isFolder() && c.wanted(f) {
			c.addFolder(f, index)
		}
	}

	var spreadsheets []spreadsheetRef
	for _, f := range files {
		if f.isFolder() || !c.wanted(f) {
			continue
		}

		switch {
		case f.isSpreadsheet():
			if c.config.IncludeSpreadsheets {
				if s := c.addSpreadsheet(f, index); s != nil {
					spreadsheets = append(spreadsheets, *s)
				}
			}
		case c.config.IncludeFiles:
			c.addFile(f, index)
		}
	}

	c.addWorksheets(ctx, spreadsheets, sheets)
	c.report()
}

// report logs what the run produced, so a large drive is auditable
// without reading the catalog.
func (c *collector) report() {
	byType := make(map[string]int, len(c.assets))
	for _, asset := range c.assets {
		byType[asset.Type]++
	}

	for assetType, count := range byType {
		log.Info().Str("type", assetType).Int("assets", count).Msg("Discovered in Google Drive")
	}
	for reason, count := range c.skipped {
		log.Info().Str("reason", reason).Int("files", count).Msg("Skipped by configuration")
	}

	if len(c.collisions) == 0 {
		return
	}

	// Merging is unavoidable once two paths slugify the same way, but a
	// run that folds a hundred documents into fifty assets should say so.
	log.Warn().
		Int("items", len(c.collisions)).
		Msg("Drive items share a Marmot asset because their paths resolve to the same name")

	for i, merged := range c.collisions {
		if i >= maxReportedCollisions {
			log.Warn().Int("remaining", len(c.collisions)-maxReportedCollisions).Msg("Further merged items not listed")
			break
		}
		log.Warn().
			Str("mrn", merged.MRN).
			Str("first", metadataPath(merged.First)).
			Str("second", metadataPath(merged.Second)).
			Msg("Merged into one asset")
	}
}

// applyFloor replaces a non-positive value with its default.
func applyFloor(value *int, fallback int) {
	if *value <= 0 {
		*value = fallback
	}
}

// metadataPath is where an item sat in the drive, for reporting.
func metadataPath(metadata map[string]interface{}) string {
	if path, ok := metadata["path"].(string); ok {
		return path
	}
	return ""
}

// wanted reports whether a Drive item is in scope for this run.
func (c *collector) wanted(f driveItem) bool {
	if f.Trashed && !c.config.IncludeTrashed {
		c.skipped["trashed"]++
		return false
	}
	for _, excluded := range c.config.ExcludeMimeTypes {
		if strings.EqualFold(strings.TrimSpace(excluded), f.MimeType) {
			c.skipped["excluded mime type"]++
			return false
		}
	}
	return true
}
