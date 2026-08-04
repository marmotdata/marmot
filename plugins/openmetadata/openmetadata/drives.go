package openmetadata

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
)

// Drive entities: the documents and sheets people keep in Google Drive
// or SharePoint.
//
// A drive is catalogued with the words someone looking at one would
// use: a Folder holds Files and Spreadsheets, and a Spreadsheet holds a
// Table per sheet. A Spreadsheet is kept apart from a File because it is
// the one kind of document that has something inside it.
//
// Everything is placed by its path rather than by its OpenMetadata
// name. OpenMetadata files some documents under the service instead of
// the folder they live in, and does not always hold a directory entity
// for every folder on those paths, so trusting the names alone leaves
// documents stranded at the top of the tree.

const (
	driveServiceFields = "owners,tags,domains"
	directoryFields    = "owners,tags,domains,dataProducts,parent"
	fileFields         = "owners,tags,domains,dataProducts,directory"
	spreadsheetFields  = "owners,tags,domains,dataProducts,directory"
	worksheetFields    = "owners,tags,domains,dataProducts,columns,spreadsheet"
)

func (c *collector) discoverDrives(ctx context.Context, client *client) error {
	directories, supported, err := listOptional[directory](ctx, client, "/v1/drives/directories", directoryFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing directories: %w", err)
	}
	if !supported {
		log.Debug().Msg("OpenMetadata does not expose drives, skipping")
		return nil
	}

	files, _, err := listOptional[driveFile](ctx, client, "/v1/drives/files", fileFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing files: %w", err)
	}
	spreadsheets, _, err := listOptional[spreadsheet](ctx, client, "/v1/drives/spreadsheets", spreadsheetFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing spreadsheets: %w", err)
	}
	worksheets, _, err := listOptional[worksheet](ctx, client, "/v1/drives/worksheets", worksheetFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing worksheets: %w", err)
	}

	drives, _, err := listOptional[driveService](ctx, client, "/v1/services/driveServices", driveServiceFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing drives: %w", err)
	}

	driveMRNs := c.addDrives(drives)
	folders := c.buildFolders(directories, files, spreadsheets)

	// The folders at the top of a drive belong to the drive itself.
	for path, folderMRN := range folders {
		if parentPath(path) != "" {
			continue
		}
		if driveMRN, ok := driveMRNs[serviceOf(path, directories, files, spreadsheets)]; ok {
			c.link(driveMRN, folderMRN, "CONTAINS")
		} else if len(driveMRNs) == 1 {
			for _, only := range driveMRNs {
				c.link(only, folderMRN, "CONTAINS")
			}
		}
	}

	// A Google Sheet is catalogued by OpenMetadata twice: once as the
	// file sitting in a folder, and once as the spreadsheet holding the
	// worksheets. They are one document, so the file is skipped in
	// favour of the richer spreadsheet.
	spreadsheetPaths := make(map[string]bool, len(spreadsheets))
	for _, s := range spreadsheets {
		if path := drivePath(s.Path); path != "" {
			spreadsheetPaths[path] = true
		}
	}

	fileCount := 0
	for _, f := range files {
		if !c.wanted(f.entityBase) {
			continue
		}

		path := drivePath(f.Path)
		if path != "" && spreadsheetPaths[path] {
			c.skipped["backed by a spreadsheet"]++
			continue
		}

		name := path
		if name == "" {
			name = driveName(f.FullyQualifiedName)
		}
		if name == "" {
			continue
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "path", f.Path)
		putIf(metadata, "file_type", f.FileType)
		putIf(metadata, "file_extension", f.FileExtension)
		putIf(metadata, "mime_type", f.MimeType)
		putIf(metadata, "size", int64(f.Size))
		putIf(metadata, "file_version", f.FileVersion)
		putIf(metadata, "checksum", f.Checksum)
		putIf(metadata, "shared", f.IsShared)

		p := projectionFor(f.ServiceType)
		asset := c.newAsset(f.entityBase, "file", "File", p, c.mrnName(name, f.FullyQualifiedName), metadata)
		c.add(f.ID, asset)
		fileCount++

		c.linkToFolder(name, *asset.MRN, folders)
	}

	sheetNames := make(map[string]string, len(spreadsheets))
	sheetMRNs := make(map[string]string, len(spreadsheets))
	for _, s := range spreadsheets {
		if !c.wanted(s.entityBase) {
			continue
		}

		// A spreadsheet keeps the extension of the file behind it out of
		// its name: the sheet is the document, not the .xlsx it happens
		// to be stored as.
		name := withoutExtension(drivePath(s.Path))
		if name == "" {
			name = driveName(s.FullyQualifiedName)
		}
		if name == "" {
			continue
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "path", s.Path)
		putIf(metadata, "size", int64(s.Size))

		p := projectionFor(s.ServiceType)
		asset := c.newAsset(s.entityBase, "spreadsheet", "Spreadsheet", p, c.mrnName(name, s.FullyQualifiedName), metadata)
		c.add(s.ID, asset)

		sheetNames[s.FullyQualifiedName] = name
		sheetMRNs[s.FullyQualifiedName] = *asset.MRN

		c.linkToFolder(name, *asset.MRN, folders)
	}

	sheetCount := 0
	for _, w := range worksheets {
		if !c.wanted(w.entityBase) {
			continue
		}

		name := driveName(w.FullyQualifiedName)
		if w.Spreadsheet != nil {
			if parentName, ok := sheetNames[w.Spreadsheet.FullyQualifiedName]; ok {
				name = parentName + "/" + w.Name
			}
		}
		if name == "" {
			continue
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "column_count", len(w.Columns))
		putIf(metadata, "row_count", int64(w.RowCount))
		putIf(metadata, "hidden", w.IsHidden)
		if w.Spreadsheet != nil {
			putIf(metadata, "spreadsheet", w.Spreadsheet.Name)
		}

		// A worksheet is a sheet of columns, so it is catalogued the way
		// every other columnar thing is.
		p := projectionFor(w.ServiceType)
		asset := c.newAsset(w.entityBase, "worksheet", "Table", p, c.mrnName(name, w.FullyQualifiedName), metadata)
		if c.config.IncludeColumns {
			setColumns(&asset, w.Columns)
		}
		c.add(w.ID, asset)
		sheetCount++

		if w.Spreadsheet != nil {
			if parent, ok := sheetMRNs[w.Spreadsheet.FullyQualifiedName]; ok {
				c.link(parent, *asset.MRN, "CONTAINS")
			}
		}
	}

	log.Debug().
		Int("folders", len(folders)).
		Int("files", fileCount).
		Int("spreadsheets", len(sheetMRNs)).
		Int("worksheets", sheetCount).
		Msg("Discovered drive entities")
	return nil
}

// addDrives catalogues each drive, the container everything else in it
// hangs from. A drive is the root someone browsing actually starts at,
// the way a bucket is for object storage.
func (c *collector) addDrives(drives []driveService) map[string]string {
	mrns := make(map[string]string, len(drives))

	for _, d := range drives {
		if !c.wanted(d.entityBase) {
			continue
		}
		name := d.Name
		if name == "" {
			name = d.FullyQualifiedName
		}
		if name == "" {
			continue
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "drive", name)

		p := projectionFor(d.ServiceType)
		asset := c.newAsset(d.entityBase, "", "Drive", p, name, metadata)
		c.add(d.ID, asset)
		mrns[name] = *asset.MRN
	}

	return mrns
}

// serviceOf names the drive a path came from, so a top level folder can
// be linked to the right one when several drives are catalogued.
func serviceOf(path string, directories []directory, files []driveFile, spreadsheets []spreadsheet) string {
	for _, d := range directories {
		if drivePath(d.Path) == path || driveName(d.FullyQualifiedName) == path {
			return d.Service.Name
		}
	}
	for _, f := range files {
		if strings.HasPrefix(drivePath(f.Path), path+"/") {
			return f.Service.Name
		}
	}
	for _, s := range spreadsheets {
		if strings.HasPrefix(drivePath(s.Path), path+"/") {
			return s.Service.Name
		}
	}
	return ""
}

// buildFolders creates the folder tree and returns each folder's MRN by
// path. OpenMetadata does not always hold a directory entity for every
// folder a document sits in, so folders named by a path but missing from
// OpenMetadata are created too; without them a document would be filed
// under a folder that is not in the catalog.
func (c *collector) buildFolders(directories []directory, files []driveFile, spreadsheets []spreadsheet) map[string]string {
	described := make(map[string]directory, len(directories))
	serviceType := ""

	for _, dir := range directories {
		if !c.wanted(dir.entityBase) {
			continue
		}
		path := drivePath(dir.Path)
		if path == "" {
			path = driveName(dir.FullyQualifiedName)
		}
		if path == "" {
			continue
		}
		described[path] = dir
		serviceType = dir.ServiceType
	}

	// An inferred folder has no entity of its own to take a service type
	// from, so it borrows one from whatever the drive did report.
	if serviceType == "" {
		for _, f := range files {
			if f.ServiceType != "" {
				serviceType = f.ServiceType
				break
			}
		}
	}
	if serviceType == "" {
		for _, s := range spreadsheets {
			if s.ServiceType != "" {
				serviceType = s.ServiceType
				break
			}
		}
	}

	// Every folder on the way down to a document has to exist.
	wanted := make(map[string]bool, len(described))
	for path := range described {
		for _, ancestor := range ancestors(path) {
			wanted[ancestor] = true
		}
	}
	for _, path := range append(filePaths(files), spreadsheetPathsOf(spreadsheets)...) {
		for _, ancestor := range ancestors(parentPath(path)) {
			wanted[ancestor] = true
		}
	}

	// Shallowest first, so a folder's parent already has an MRN.
	paths := make([]string, 0, len(wanted))
	for path := range wanted {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		di, dj := strings.Count(paths[i], "/"), strings.Count(paths[j], "/")
		if di != dj {
			return di < dj
		}
		return paths[i] < paths[j]
	})

	folders := make(map[string]string, len(paths))
	for _, path := range paths {
		dir, isDescribed := described[path]
		if !isDescribed {
			// A folder OpenMetadata does not describe still exists in the
			// drive; the paths of the documents inside it say so.
			dir = directory{}
			dir.entityBase = entityBase{FullyQualifiedName: path, ServiceType: serviceType}
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "path", "/"+path)
		putIf(metadata, "directory_type", dir.DirectoryType)
		putIf(metadata, "shared", dir.IsShared)
		if !isDescribed {
			metadata["inferred_from_path"] = true
			c.skipped["folders inferred from a path"]++
		}

		p := projectionFor(dir.ServiceType)
		asset := c.newAsset(dir.entityBase, "directory", "Folder", p, c.mrnName(path, dir.FullyQualifiedName), metadata)
		c.add(dir.ID, asset)

		folders[path] = *asset.MRN
		c.linkToFolder(path, *asset.MRN, folders)
	}

	return folders
}

// linkToFolder links an asset to the folder its path sits in.
func (c *collector) linkToFolder(path, assetMRN string, folders map[string]string) {
	parent := parentPath(path)
	if parent == "" {
		return
	}
	if parentMRN, ok := folders[parent]; ok && parentMRN != assetMRN {
		c.link(parentMRN, assetMRN, "CONTAINS")
	}
}

// drivePath normalises a drive path into the slash separated location
// used as an asset name.
func drivePath(path string) string {
	return strings.Trim(strings.TrimSpace(path), "/")
}

// parentPath is the folder a path sits in, or "" at the top of a drive.
func parentPath(path string) string {
	if cut := strings.LastIndex(path, "/"); cut > 0 {
		return path[:cut]
	}
	return ""
}

// ancestors lists a path and every folder above it, shallowest first.
func ancestors(path string) []string {
	if path == "" {
		return nil
	}

	parts := strings.Split(path, "/")
	out := make([]string, 0, len(parts))
	for i := range parts {
		out = append(out, strings.Join(parts[:i+1], "/"))
	}
	return out
}

// withoutExtension drops a trailing file extension from a name.
func withoutExtension(name string) string {
	if dot := strings.LastIndex(name, "."); dot > strings.LastIndex(name, "/") {
		return name[:dot]
	}
	return name
}

func filePaths(files []driveFile) []string {
	paths := make([]string, 0, len(files))
	for _, f := range files {
		if path := drivePath(f.Path); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func spreadsheetPathsOf(spreadsheets []spreadsheet) []string {
	paths := make([]string, 0, len(spreadsheets))
	for _, s := range spreadsheets {
		if path := drivePath(s.Path); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

// driveName is the path below the service, joined with slashes. It is
// the fallback when OpenMetadata records no path for an entity.
func driveName(fqn string) string {
	return strings.Join(fqnBelowService(fqn), "/")
}
