package sftp

// FolderFields describes the metadata fields the SFTP plugin emits for a
// Folder asset. It is kept as a documentation-only struct so downstream
// tooling can introspect the shape of the metadata map. The counts and
// the size cover a folder's immediate contents, not its whole subtree.
type FolderFields struct {
	Path           string `json:"path" metadata:"path" description:"Absolute path of the directory on the server"`
	Parent         string `json:"parent" metadata:"parent" description:"Name of the folder above this one, absent at the top of a root"`
	Root           string `json:"root" metadata:"root" description:"Configured root directory this folder was reached from"`
	Modified       string `json:"modified" metadata:"modified" description:"Last modification time, RFC3339"`
	FileCount      int    `json:"file_count" metadata:"file_count" description:"Files directly in this directory"`
	DirectoryCount int    `json:"directory_count" metadata:"directory_count" description:"Subdirectories directly in this directory"`
	SizeBytes      int64  `json:"size_bytes" metadata:"size_bytes" description:"Total size of the files directly in this directory"`
}

// FileFields describes the metadata fields the SFTP plugin emits for a
// File asset.
type FileFields struct {
	Path          string `json:"path" metadata:"path" description:"Absolute path of the file on the server"`
	Directory     string `json:"directory" metadata:"directory" description:"Name of the folder holding the file, absent at the top of a root"`
	Size          int64  `json:"size" metadata:"size" description:"File size in bytes"`
	Modified      string `json:"modified" metadata:"modified" description:"Last modification time, RFC3339"`
	Mode          string `json:"mode" metadata:"mode" description:"Permission bits, as ls prints them"`
	OwnerUID      int    `json:"owner_uid" metadata:"owner_uid" description:"Numeric user id of the owner"`
	OwnerGID      int    `json:"owner_gid" metadata:"owner_gid" description:"Numeric group id of the owner"`
	Extension     string `json:"extension" metadata:"extension" description:"Lowercase file extension without the dot"`
	MimeType      string `json:"mime_type" metadata:"mime_type" description:"MIME type guessed from the extension"`
	FileType      string `json:"file_type" metadata:"file_type" description:"File kind: csv, tsv, json, jsonl, parquet, avro or other"`
	RowCountExact bool   `json:"row_count_exact" metadata:"row_count_exact" description:"Whether the row count covers the whole file or stopped at max_read_bytes"`
	Host          string `json:"host" metadata:"host" description:"Server the file was read from"`
	Port          int    `json:"port" metadata:"port" description:"Port the server was reached on"`
}

// DelimitedColumnFields describes the per-column fields inferred for a
// csv or tsv file.
type DelimitedColumnFields struct {
	ColumnName string `json:"column_name" metadata:"column_name" description:"Column name, taken from the header row"`
	DataType   string `json:"data_type" metadata:"data_type" description:"Inferred type: INT, FLOAT, BOOLEAN, DATETIME or STRING"`
	IsNullable bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether any sampled row left the column empty"`
}

// JSONColumnFields describes the per-column fields inferred for a json
// or jsonl file. A nested key is written parent.child.
type JSONColumnFields struct {
	ColumnName string `json:"column_name" metadata:"column_name" description:"Key name, with one level of nesting written parent.child"`
	DataType   string `json:"data_type" metadata:"data_type" description:"JSON types seen for the key, joined by | when mixed"`
	IsNullable bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether the key was null or missing in any sampled record"`
	Occurrence int    `json:"occurrence" metadata:"occurrence" description:"Sampled records that carried the key"`
}
