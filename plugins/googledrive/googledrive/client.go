package googledrive

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// driveAPI and sheetsAPI are the slices of Google's API this plugin
// uses. They are interfaces so tests can drive discovery without a
// Google account.
type driveAPI interface {
	list(ctx context.Context, opts listOptions) ([]driveItem, error)
}

type sheetsAPI interface {
	sheets(ctx context.Context, spreadsheetID string, headerRow int) ([]sheetTab, error)
}

type listOptions struct {
	DriveID        string
	FolderID       string
	PageSize       int
	MaxFiles       int
	IncludeTrashed bool
}

// driveItem is one entry in a Drive listing.
type driveItem struct {
	ID            string
	Name          string
	MimeType      string
	Description   string
	Parents       []string
	Size          int64
	FileExtension string
	MD5Checksum   string
	Version       string
	Shared        bool
	Trashed       bool
	WebViewLink   string
	ModifiedTime  time.Time
	Owners        []string
}

const folderMimeType = "application/vnd.google-apps.folder"
const spreadsheetMimeType = "application/vnd.google-apps.spreadsheet"

func (f driveItem) isFolder() bool      { return f.MimeType == folderMimeType }
func (f driveItem) isSpreadsheet() bool { return f.MimeType == spreadsheetMimeType }

func (f driveItem) ownerNames() []string {
	if len(f.Owners) == 0 {
		return nil
	}
	return f.Owners
}

// sheetTab is one sheet of a spreadsheet.
type sheetTab struct {
	Title    string
	Columns  []string
	RowCount int64
	Hidden   bool
}

type clients struct {
	drive  driveAPI
	sheets sheetsAPI
}

// newClients builds the Google API clients from the configured
// credentials. Reading a whole organisation's Drive needs the service
// account to act as a user, which is why impersonation is separate from
// the shared GCP credentials handling.
func newClients(ctx context.Context, config *Config) (*clients, error) {
	scopes := []string{drive.DriveMetadataReadonlyScope}
	if config.IncludeWorksheets {
		scopes = append(scopes, sheets.SpreadsheetsReadonlyScope)
	}

	tokenSource, err := tokenSource(ctx, config, scopes)
	if err != nil {
		return nil, err
	}

	driveService, err := drive.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("creating Drive client: %w", err)
	}

	built := &clients{drive: &driveClient{service: driveService}}

	if config.IncludeWorksheets {
		sheetsService, err := sheets.NewService(ctx, option.WithTokenSource(tokenSource))
		if err != nil {
			return nil, fmt.Errorf("creating Sheets client: %w", err)
		}
		built.sheets = &sheetsClient{service: sheetsService}
	}

	return built, nil
}

// tokenSource builds credentials for the run. With impersonate_user set
// the service account acts as that person, which is what domain-wide
// delegation is for; without it the account sees only what has been
// shared with it directly.
func tokenSource(ctx context.Context, config *Config, scopes []string) (oauth2.TokenSource, error) {
	if config.ImpersonateUser == "" {
		return config.GCPConfig.TokenSource(ctx, scopes...)
	}

	keyJSON := []byte(config.Credentials.CredentialsJSON)
	if len(keyJSON) == 0 && config.Credentials.CredentialsFile != "" {
		data, err := os.ReadFile(config.Credentials.CredentialsFile)
		if err != nil {
			return nil, fmt.Errorf("reading GCP credentials file: %w", err)
		}
		keyJSON = data
	}
	if len(keyJSON) == 0 {
		return nil, fmt.Errorf("impersonate_user needs a service account key: set credentials_json or credentials_file")
	}

	jwtConfig, err := google.JWTConfigFromJSON(keyJSON, scopes...)
	if err != nil {
		return nil, fmt.Errorf("parsing GCP credentials: %w", err)
	}
	jwtConfig.Subject = config.ImpersonateUser

	return jwtConfig.TokenSource(ctx), nil
}

type driveClient struct {
	service *drive.Service
}

const driveFields = "nextPageToken, files(id, name, mimeType, description, parents, size, fileExtension, md5Checksum, version, shared, trashed, webViewLink, modifiedTime, owners(displayName, emailAddress))"

func (d *driveClient) list(ctx context.Context, opts listOptions) ([]driveItem, error) {
	call := d.service.Files.List().
		Fields(driveFields).
		PageSize(int64(opts.PageSize)).
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true)

	if opts.DriveID != "" {
		call = call.DriveId(opts.DriveID).Corpora("drive")
	}

	query := []string{}
	if !opts.IncludeTrashed {
		query = append(query, "trashed = false")
	}
	if opts.FolderID != "" {
		query = append(query, fmt.Sprintf("'%s' in parents", opts.FolderID))
	}
	if len(query) > 0 {
		call = call.Q(strings.Join(query, " and "))
	}

	var items []driveItem
	err := call.Pages(ctx, func(page *drive.FileList) error {
		for _, f := range page.Files {
			items = append(items, toDriveItem(f))
			if opts.MaxFiles > 0 && len(items) >= opts.MaxFiles {
				return fmt.Errorf("%w", errMaxFiles)
			}
		}
		return nil
	})
	if err != nil && !strings.Contains(err.Error(), errMaxFiles.Error()) {
		return items, err
	}

	return items, nil
}

// errMaxFiles stops paging once the configured limit is reached; the
// Google client has no other way to break out of Pages.
var errMaxFiles = fmt.Errorf("reached max_files")

func toDriveItem(f *drive.File) driveItem {
	item := driveItem{
		ID:            f.Id,
		Name:          f.Name,
		MimeType:      f.MimeType,
		Description:   f.Description,
		Parents:       f.Parents,
		Size:          f.Size,
		FileExtension: f.FileExtension,
		MD5Checksum:   f.Md5Checksum,
		Version:       fmt.Sprintf("%d", f.Version),
		Shared:        f.Shared,
		Trashed:       f.Trashed,
		WebViewLink:   f.WebViewLink,
	}
	if f.Version == 0 {
		item.Version = ""
	}
	if f.ModifiedTime != "" {
		if parsed, err := time.Parse(time.RFC3339, f.ModifiedTime); err == nil {
			item.ModifiedTime = parsed
		}
	}
	for _, owner := range f.Owners {
		if owner.DisplayName != "" {
			item.Owners = append(item.Owners, owner.DisplayName)
		} else if owner.EmailAddress != "" {
			item.Owners = append(item.Owners, owner.EmailAddress)
		}
	}
	return item
}

type sheetsClient struct {
	service *sheets.Service
}

func (s *sheetsClient) sheets(ctx context.Context, spreadsheetID string, headerRow int) ([]sheetTab, error) {
	// IncludeGridData with a header-row range returns the sheet titles
	// and just that row, rather than every cell in the document.
	resp, err := s.service.Spreadsheets.Get(spreadsheetID).
		IncludeGridData(true).
		Ranges(fmt.Sprintf("%d:%d", headerRow, headerRow)).
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}

	tabs := make([]sheetTab, 0, len(resp.Sheets))
	for _, sheet := range resp.Sheets {
		if sheet.Properties == nil {
			continue
		}

		tab := sheetTab{Title: sheet.Properties.Title, Hidden: sheet.Properties.Hidden}
		if grid := sheet.Properties.GridProperties; grid != nil {
			tab.RowCount = grid.RowCount
		}

		for _, data := range sheet.Data {
			for _, row := range data.RowData {
				for _, cell := range row.Values {
					if cell.FormattedValue != "" {
						tab.Columns = append(tab.Columns, cell.FormattedValue)
					}
				}
			}
		}

		tabs = append(tabs, tab)
	}

	return tabs, nil
}

var _ pluginsdk.Source = (*Source)(nil)
