package googledrive

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDrive stands in for the Drive API so discovery can be exercised
// without a Google account.
type fakeDrive struct {
	items []driveItem
	opts  listOptions
}

func (f *fakeDrive) list(_ context.Context, opts listOptions) ([]driveItem, error) {
	f.opts = opts
	return f.items, nil
}

type fakeSheets struct {
	tabs map[string][]sheetTab
	err  error
}

func (f *fakeSheets) sheets(_ context.Context, spreadsheetID string, _ int) ([]sheetTab, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tabs[spreadsheetID], nil
}

// countingSheets records how many requests were in flight at once, so
// a test can tell parallel fetching from serial fetching.
type countingSheets struct {
	tabs map[string][]sheetTab

	mu      sync.Mutex
	total   int
	inFlue  int
	highest int
}

func (f *countingSheets) sheets(_ context.Context, spreadsheetID string, _ int) ([]sheetTab, error) {
	f.mu.Lock()
	f.total++
	f.inFlue++
	if f.inFlue > f.highest {
		f.highest = f.inFlue
	}
	f.mu.Unlock()

	// Hold the slot long enough that a serial implementation could not
	// overlap two requests.
	time.Sleep(20 * time.Millisecond)

	f.mu.Lock()
	f.inFlue--
	f.mu.Unlock()
	return f.tabs[spreadsheetID], nil
}

func (f *countingSheets) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.total
}

func (f *countingSheets) highWater() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.highest
}

// staggeredSheets replies out of listing order, so a test can prove the
// assets are still built in listing order.
type staggeredSheets struct {
	tabs map[string][]sheetTab
}

func (f *staggeredSheets) sheets(_ context.Context, spreadsheetID string, _ int) ([]sheetTab, error) {
	if spreadsheetID == "s1" {
		time.Sleep(15 * time.Millisecond)
	}
	return f.tabs[spreadsheetID], nil
}

// collectorFor runs discovery and hands back the collector, for the
// bookkeeping a DiscoveryResult does not carry.
func collectorFor(t *testing.T, drive *fakeDrive, sheets sheetsAPI, overrides pluginsdk.RawConfig) *collector {
	t.Helper()

	source := &Source{}
	config := pluginsdk.RawConfig{}
	for k, v := range overrides {
		config[k] = v
	}
	_, err := source.Validate(config)
	require.NoError(t, err)

	files, err := drive.list(context.Background(), listOptions{})
	require.NoError(t, err)

	c := newCollector(source.config)
	c.driveMRN = c.addDrive(source.config.driveName())
	c.build(context.Background(), files, sheets)
	return c
}

func folder(id, name string, parents ...string) driveItem {
	return driveItem{ID: id, Name: name, MimeType: folderMimeType, Parents: parents}
}

func file(id, name, mimeType string, parents ...string) driveItem {
	return driveItem{ID: id, Name: name, MimeType: mimeType, Parents: parents}
}

func discover(t *testing.T, drive *fakeDrive, sheets sheetsAPI, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := pluginsdk.RawConfig{}
	for k, v := range overrides {
		config[k] = v
	}

	source := &Source{drive: drive, sheets: sheets}
	result, err := source.Discover(t.Context(), config)
	require.NoError(t, err)
	return result
}

func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i, a := range result.Assets {
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == "CONTAINS" {
			return true
		}
	}
	return false
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "googledrive", meta.ID)
	assert.Equal(t, "Google Drive", meta.Name)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestValidate_AppliesDefaults(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{})
	require.NoError(t, err)

	assert.True(t, source.config.IncludeFiles)
	assert.True(t, source.config.IncludeSpreadsheets)
	assert.True(t, source.config.IncludeWorksheets)
	assert.False(t, source.config.IncludeTrashed)
	assert.Equal(t, 1000, source.config.PageSize)
	assert.Equal(t, 1, source.config.HeaderRow)
}

func TestValidate_ReplacesExplicitZerosWithDefaults(t *testing.T) {
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{"page_size": 0, "header_row": 0})
	require.NoError(t, err)

	assert.Equal(t, 1000, source.config.PageSize)
	assert.Equal(t, 1, source.config.HeaderRow)
}

func TestDiscover_CataloguesFolders(t *testing.T) {
	result := discover(t, &fakeDrive{items: []driveItem{
		folder("f1", "Marketing"),
		folder("f2", "Campaigns_2024", "f1"),
	}}, nil, nil)

	assert.NotNil(t, findAsset(result, "Folder", "Marketing"))
	assert.NotNil(t, findAsset(result, "Folder", "Marketing/Campaigns_2024"))
}

func TestDiscover_NestsFoldersAndFiles(t *testing.T) {
	result := discover(t, &fakeDrive{items: []driveItem{
		folder("f1", "Marketing"),
		folder("f2", "Campaigns_2024", "f1"),
		file("d1", "brand_guidelines.pdf", "application/pdf", "f1"),
	}}, nil, nil)

	assert.True(t, hasEdge(result,
		"mrn://folder/googledrive/marketing", "mrn://folder/googledrive/marketing-campaigns_2024"))
	assert.True(t, hasEdge(result,
		"mrn://folder/googledrive/marketing", "mrn://file/googledrive/marketing-brand_guidelines.pdf"))
}

// The whole point of matching the OpenMetadata plugin's projection is
// that a drive imported from OpenMetadata and the same drive read
// directly land on one asset rather than two.
func TestDiscover_MatchesTheOpenMetadataProjection(t *testing.T) {
	result := discover(t, &fakeDrive{items: []driveItem{
		folder("f1", "Marketing"),
		file("d1", "plan.pdf", "application/pdf", "f1"),
	}}, nil, nil)

	folderAsset := findAsset(result, "Folder", "Marketing")
	require.NotNil(t, folderAsset)
	assert.Equal(t, "mrn://folder/googledrive/marketing", *folderAsset.MRN)

	fileAsset := findAsset(result, "File", "Marketing/plan.pdf")
	require.NotNil(t, fileAsset)
	assert.Equal(t, "mrn://file/googledrive/marketing-plan.pdf", *fileAsset.MRN)
}

// Marmot rebuilds an asset's MRN from its type, first provider and name
// when a run reaches the server, so all three have to agree.
func TestDiscover_MRNsSurviveTheServer(t *testing.T) {
	result := discover(t, &fakeDrive{items: []driveItem{
		folder("f1", "Finance"),
		file("d1", "budget.xlsx", "application/vnd.ms-excel", "f1"),
		file("s1", "annual_budget", spreadsheetMimeType, "f1"),
	}}, &fakeSheets{tabs: map[string][]sheetTab{
		"s1": {{Title: "Q1", Columns: []string{"cost_centre", "amount"}}},
	}}, nil)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN,
			"asset %q must be addressable by the MRN the server rebuilds", *a.Name)
	}
}

func TestDiscover_LineagePointsAtRealAssets(t *testing.T) {
	result := discover(t, &fakeDrive{items: []driveItem{
		folder("f1", "Finance"),
		file("s1", "annual_budget", spreadsheetMimeType, "f1"),
	}}, &fakeSheets{tabs: map[string][]sheetTab{
		"s1": {{Title: "Q1", Columns: []string{"amount"}}},
	}}, nil)

	known := map[string]bool{}
	for _, a := range result.Assets {
		known[*a.MRN] = true
	}
	require.NotEmpty(t, result.Lineage)
	for _, e := range result.Lineage {
		assert.True(t, known[e.Source], "edge source %q is not an emitted asset", e.Source)
		assert.True(t, known[e.Target], "edge target %q is not an emitted asset", e.Target)
	}
}

func TestDiscover_CataloguesSpreadsheetsAndTheirSheets(t *testing.T) {
	result := discover(t, &fakeDrive{items: []driveItem{
		folder("f1", "Finance"),
		file("s1", "annual_budget", spreadsheetMimeType, "f1"),
	}}, &fakeSheets{tabs: map[string][]sheetTab{
		"s1": {{Title: "Q1", Columns: []string{"cost_centre", "amount"}, RowCount: 120}},
	}}, nil)

	sheet := findAsset(result, "Spreadsheet", "Finance/annual_budget")
	require.NotNil(t, sheet)

	tab := findAsset(result, "Table", "Finance/annual_budget/Q1")
	require.NotNil(t, tab)
	assert.True(t, hasEdge(result, *sheet.MRN, *tab.MRN))

	var columns []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(tab.Schema["columns"]), &columns))
	require.Len(t, columns, 2)
	assert.Equal(t, "cost_centre", columns[0]["column_name"])
}

func TestDiscover_SurvivesASpreadsheetItCannotOpen(t *testing.T) {
	result := discover(t, &fakeDrive{items: []driveItem{
		file("s1", "locked", spreadsheetMimeType),
	}}, &fakeSheets{err: assert.AnError}, nil)

	// The file is still worth cataloguing even when its sheets are not readable.
	assert.NotNil(t, findAsset(result, "Spreadsheet", "locked"))
	assert.Nil(t, findAsset(result, "Table", "locked/Q1"))
}

func TestDiscover_CanLeaveOutWorksheets(t *testing.T) {
	result := discover(t, &fakeDrive{items: []driveItem{
		file("s1", "annual_budget", spreadsheetMimeType),
	}}, &fakeSheets{tabs: map[string][]sheetTab{
		"s1": {{Title: "Q1", Columns: []string{"amount"}}},
	}}, pluginsdk.RawConfig{"include_worksheets": false})

	assert.NotNil(t, findAsset(result, "Spreadsheet", "annual_budget"))
	assert.Nil(t, findAsset(result, "Table", "annual_budget/Q1"))
}

func TestDiscover_CanLeaveOutFiles(t *testing.T) {
	result := discover(t, &fakeDrive{items: []driveItem{
		folder("f1", "Marketing"),
		file("d1", "plan.pdf", "application/pdf", "f1"),
	}}, nil, pluginsdk.RawConfig{"include_files": false})

	assert.NotNil(t, findAsset(result, "Folder", "Marketing"))
	assert.Nil(t, findAsset(result, "File", "Marketing/plan.pdf"))
}

func TestDiscover_SkipsTrashedFiles(t *testing.T) {
	trashed := file("d2", "old.pdf", "application/pdf")
	trashed.Trashed = true

	result := discover(t, &fakeDrive{items: []driveItem{
		file("d1", "current.pdf", "application/pdf"), trashed,
	}}, nil, nil)

	assert.NotNil(t, findAsset(result, "File", "current.pdf"))
	assert.Nil(t, findAsset(result, "File", "old.pdf"))
}

func TestDiscover_SkipsExcludedMimeTypes(t *testing.T) {
	result := discover(t, &fakeDrive{items: []driveItem{
		file("d1", "plan.pdf", "application/pdf"),
		file("d2", "link", "application/vnd.google-apps.shortcut"),
	}}, nil, pluginsdk.RawConfig{"exclude_mime_types": []string{"application/vnd.google-apps.shortcut"}})

	assert.NotNil(t, findAsset(result, "File", "plan.pdf"))
	assert.Nil(t, findAsset(result, "File", "link"))
}

func TestDiscover_RecordsFileMetadata(t *testing.T) {
	f := file("d1", "plan.pdf", "application/pdf")
	f.Size = 2097152
	f.FileExtension = "pdf"
	f.Description = "The plan"
	f.Owners = []string{"Ada Lovelace"}
	f.WebViewLink = "https://drive.google.com/file/d/1ABC"

	result := discover(t, &fakeDrive{items: []driveItem{f}}, nil, nil)

	asset := findAsset(result, "File", "plan.pdf")
	require.NotNil(t, asset)
	assert.Equal(t, int64(2097152), asset.Metadata["size"])
	assert.Equal(t, "pdf", asset.Metadata["file_extension"])
	assert.Equal(t, "Document", asset.Metadata["file_type"])
	assert.Equal(t, []string{"Ada Lovelace"}, asset.Metadata["owners"])
	require.NotNil(t, asset.Description)
	assert.Equal(t, "The plan", *asset.Description)
	require.Len(t, asset.ExternalLinks, 1)
	assert.Equal(t, "https://drive.google.com/file/d/1ABC", asset.ExternalLinks[0].URL)
}

func TestDiscover_PassesScopeToTheDriveAPI(t *testing.T) {
	drive := &fakeDrive{}
	discover(t, drive, nil, pluginsdk.RawConfig{
		"drive_id": "shared-1", "folder_id": "folder-9", "max_files": 50,
	})

	assert.Equal(t, "shared-1", drive.opts.DriveID)
	assert.Equal(t, "folder-9", drive.opts.FolderID)
	assert.Equal(t, 50, drive.opts.MaxFiles)
}

// Drive is a graph, not a tree: a folder cycle must not hang discovery.
func TestDrivePath_TerminatesOnACycle(t *testing.T) {
	a := folder("a", "A", "b")
	b := folder("b", "B", "a")
	index := map[string]driveItem{"a": a, "b": b}

	assert.NotEmpty(t, drivePath(a, index))
}

func TestDrivePath_StopsAtAMissingParent(t *testing.T) {
	orphan := file("d1", "plan.pdf", "application/pdf", "gone")

	assert.Equal(t, "plan.pdf", drivePath(orphan, map[string]driveItem{"d1": orphan}))
}

func TestFileType_GroupsByKind(t *testing.T) {
	assert.Equal(t, "Document", fileType("application/pdf"))
	assert.Equal(t, "Spreadsheet", fileType(spreadsheetMimeType))
	assert.Equal(t, "Image", fileType("image/png"))
	assert.Equal(t, "Video", fileType("video/mp4"))
	assert.Equal(t, "Other", fileType("application/octet-stream"))
}

func TestDiscover_CataloguesTheDriveItself(t *testing.T) {
	// A drive is the container everything else hangs from, the way a
	// bucket is for object storage, so it is an asset of its own.
	result := discover(t, &fakeDrive{items: []driveItem{
		folder("f1", "Marketing"),
	}}, nil, nil)

	assert.NotNil(t, findAsset(result, "Drive", "My Drive"))
	assert.True(t, hasEdge(result,
		"mrn://drive/googledrive/my-drive", "mrn://folder/googledrive/marketing"),
		"a folder with nothing above it sits at the top of the drive")
}

func TestDiscover_NamesTheDriveAfterASharedDrive(t *testing.T) {
	result := discover(t, &fakeDrive{items: []driveItem{
		folder("f1", "Marketing"),
	}}, nil, pluginsdk.RawConfig{"drive_id": "0AItAbCdEfGh"})

	assert.NotNil(t, findAsset(result, "Drive", "0AItAbCdEfGh"))
	assert.True(t, hasEdge(result,
		"mrn://drive/googledrive/0aitabcdefgh", "mrn://folder/googledrive/marketing"))
}

func TestDiscover_OnlyTheTopOfTheTreeBelongsToTheDrive(t *testing.T) {
	result := discover(t, &fakeDrive{items: []driveItem{
		folder("f1", "Marketing"),
		folder("f2", "Campaigns_2024", "f1"),
		file("d1", "brief.pdf", "application/pdf", "f2"),
	}}, nil, nil)

	assert.True(t, hasEdge(result,
		"mrn://drive/googledrive/my-drive", "mrn://folder/googledrive/marketing"))
	assert.False(t, hasEdge(result,
		"mrn://drive/googledrive/my-drive", "mrn://folder/googledrive/marketing-campaigns_2024"),
		"a nested folder belongs to its parent, not to the drive")
	assert.False(t, hasEdge(result,
		"mrn://drive/googledrive/my-drive", "mrn://file/googledrive/marketing-campaigns_2024-brief.pdf"),
		"a file inside a folder belongs to that folder")
}

// spreadsheet builds a Google Sheet item, which the plugin recognises
// by MIME type rather than by extension.
func spreadsheetItem(id, name string, parents ...string) driveItem {
	return driveItem{ID: id, Name: name, MimeType: spreadsheetMimeType, Parents: parents}
}

func TestDiscover_ReportsItemsThatMergeIntoOneAsset(t *testing.T) {
	// Two names that slugify the same way land on one MRN. Marmot can
	// only merge them, but the run has to say so rather than quietly
	// losing a document.
	c := collectorFor(t, &fakeDrive{items: []driveItem{
		folder("f1", "Q4 Reports"),
		folder("f2", "Q4-Reports"),
	}}, nil, nil)

	require.Len(t, c.collisions, 1)
	assert.Equal(t, "mrn://folder/googledrive/q4-reports", c.collisions[0].MRN)
	assert.Equal(t, "/Q4 Reports", metadataPath(c.collisions[0].First))
	assert.Equal(t, "/Q4-Reports", metadataPath(c.collisions[0].Second))
}

func TestDiscover_DoesNotReportACollisionWhenNamesAreDistinct(t *testing.T) {
	c := collectorFor(t, &fakeDrive{items: []driveItem{
		folder("f1", "Marketing"),
		folder("f2", "Finance"),
	}}, nil, nil)

	assert.Empty(t, c.collisions)
}

func TestDiscover_CountsSpreadsheetsItCannotOpen(t *testing.T) {
	c := collectorFor(t, &fakeDrive{items: []driveItem{
		spreadsheetItem("s1", "Budget"),
	}}, &fakeSheets{err: assert.AnError}, nil)

	assert.Equal(t, 1, c.skipped["sheets could not be read"])
	assert.NotNil(t, findAsset(&pluginsdk.DiscoveryResult{Assets: c.assets}, "Spreadsheet", "Budget"),
		"the spreadsheet is still catalogued when its sheets cannot be read")
}

func TestDiscover_ReadsSheetsConcurrently(t *testing.T) {
	drive := &fakeDrive{items: []driveItem{
		spreadsheetItem("s1", "A"),
		spreadsheetItem("s2", "B"),
		spreadsheetItem("s3", "C"),
	}}
	sheets := &countingSheets{tabs: map[string][]sheetTab{
		"s1": {{Title: "One"}}, "s2": {{Title: "Two"}}, "s3": {{Title: "Three"}},
	}}

	result := discover(t, drive, sheets, pluginsdk.RawConfig{"concurrency": 3})

	assert.Equal(t, 3, sheets.calls())
	assert.Equal(t, 3, sheets.highWater(), "all three requests were in flight together")
	assert.NotNil(t, findAsset(result, "Table", "A/One"))
	assert.NotNil(t, findAsset(result, "Table", "C/Three"))
}

func TestDiscover_OrdersSheetsTheSameWayEveryRun(t *testing.T) {
	// The sheets are fetched in parallel, so the assets must be built
	// from the listing rather than from whichever reply arrived first.
	drive := &fakeDrive{items: []driveItem{
		spreadsheetItem("s1", "A"),
		spreadsheetItem("s2", "B"),
		spreadsheetItem("s3", "C"),
	}}
	tabs := map[string][]sheetTab{
		"s1": {{Title: "One"}}, "s2": {{Title: "Two"}}, "s3": {{Title: "Three"}},
	}

	// s1 replies last, so completion order would put A/One at the end.
	want := []string{"A/One", "B/Two", "C/Three"}

	for run := 0; run < 8; run++ {
		result := discover(t, drive, &staggeredSheets{tabs: tabs}, nil)

		var names []string
		for _, a := range result.Assets {
			if a.Type == "Table" {
				names = append(names, *a.Name)
			}
		}
		assert.Equal(t, want, names, "tables must follow the listing, not the order replies arrived")
	}
}

func TestValidate_ReplacesAnExplicitZeroConcurrency(t *testing.T) {
	// An unbuffered semaphore would hang the sheets pass forever.
	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{"concurrency": 0})

	require.NoError(t, err)
	assert.Equal(t, 8, source.config.Concurrency)
}
