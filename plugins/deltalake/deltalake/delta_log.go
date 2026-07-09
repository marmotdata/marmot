package deltalake

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

type logEntry struct {
	MetaData   *metaDataAction   `json:"metaData"`
	Protocol   *protocolAction   `json:"protocol"`
	Add        *addAction        `json:"add"`
	Remove     *removeAction     `json:"remove"`
	CommitInfo *commitInfoAction `json:"commitInfo"`
}

type metaDataAction struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Format           formatSpec        `json:"format"`
	SchemaString     string            `json:"schemaString"`
	PartitionColumns []string          `json:"partitionColumns"`
	Configuration    map[string]string `json:"configuration"`
	CreatedTime      int64             `json:"createdTime"`
}

type formatSpec struct {
	Provider string            `json:"provider"`
	Options  map[string]string `json:"options"`
}

type protocolAction struct {
	MinReaderVersion int `json:"minReaderVersion"`
	MinWriterVersion int `json:"minWriterVersion"`
}

type addAction struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type removeAction struct {
	Path string `json:"path"`
}

type commitInfoAction struct {
	Version   int64  `json:"version"`
	Timestamp int64  `json:"timestamp"`
	Operation string `json:"operation"`
}

type deltaSchema struct {
	Type   string       `json:"type"`
	Fields []deltaField `json:"fields"`
}

type deltaField struct {
	Name     string      `json:"name"`
	Type     interface{} `json:"type"`
	Nullable bool        `json:"nullable"`
	Metadata interface{} `json:"metadata"`
}

// deltaSnapshot holds the reconstructed state from the transaction log.
type deltaSnapshot struct {
	MetaData       *metaDataAction
	Protocol       *protocolAction
	NumFiles       int
	TotalSize      int64
	CurrentVersion int64
	ActiveFiles    map[string]int64 // path → size
}

type lastCheckpointInfo struct {
	Version int64 `json:"version"`
	Size    int64 `json:"size"`
}

// readDeltaLog reads the transaction log for a Delta table and returns its snapshot.
func readDeltaLog(tableDir string) (*deltaSnapshot, error) {
	deltaLogDir := filepath.Join(tableDir, "_delta_log")

	snapshot := &deltaSnapshot{
		ActiveFiles: make(map[string]int64),
	}

	var startVersion int64

	checkpointVersion, err := readLastCheckpoint(deltaLogDir)
	if err == nil && checkpointVersion >= 0 {
		checkpointPath := filepath.Join(deltaLogDir, fmt.Sprintf("%020d.checkpoint.parquet", checkpointVersion))
		cpSnapshot, cpErr := readCheckpoint(checkpointPath)
		if cpErr == nil {
			snapshot = cpSnapshot
			startVersion = checkpointVersion + 1
		}
	}

	jsonFiles, err := listLogFiles(deltaLogDir, startVersion)
	if err != nil {
		return nil, fmt.Errorf("listing log files: %w", err)
	}

	for _, logFile := range jsonFiles {
		if err := readLogFile(logFile, snapshot); err != nil {
			return nil, fmt.Errorf("reading log file %s: %w", filepath.Base(logFile), err)
		}
	}

	snapshot.NumFiles = len(snapshot.ActiveFiles)
	snapshot.TotalSize = 0
	for _, size := range snapshot.ActiveFiles {
		snapshot.TotalSize += size
	}

	return snapshot, nil
}

// readLastCheckpoint reads _last_checkpoint and returns the checkpoint version.
func readLastCheckpoint(deltaLogDir string) (int64, error) {
	data, err := os.ReadFile(filepath.Join(deltaLogDir, "_last_checkpoint"))
	if err != nil {
		return -1, err
	}

	var info lastCheckpointInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return -1, fmt.Errorf("parsing _last_checkpoint: %w", err)
	}

	return info.Version, nil
}

// readCheckpoint reads a Parquet checkpoint file and extracts the snapshot state.
func readCheckpoint(checkpointPath string) (*deltaSnapshot, error) {
	f, err := os.Open(checkpointPath)
	if err != nil {
		return nil, fmt.Errorf("opening checkpoint: %w", err)
	}
	defer f.Close()

	rdr, err := file.NewParquetReader(f)
	if err != nil {
		return nil, fmt.Errorf("creating parquet reader: %w", err)
	}
	defer rdr.Close()

	arrowRdr, err := pqarrow.NewFileReader(rdr, pqarrow.ArrowReadProperties{}, nil)
	if err != nil {
		return nil, fmt.Errorf("creating arrow reader: %w", err)
	}

	tbl, err := arrowRdr.ReadTable(nil)
	if err != nil {
		return nil, fmt.Errorf("reading parquet table: %w", err)
	}
	defer tbl.Release()

	snapshot := &deltaSnapshot{
		ActiveFiles: make(map[string]int64),
	}

	schema := tbl.Schema()

	metaDataIdx := -1
	protocolIdx := -1
	addIdx := -1

	for i, f := range schema.Fields() {
		switch f.Name {
		case "metaData":
			metaDataIdx = i
		case "protocol":
			protocolIdx = i
		case "add":
			addIdx = i
		}
	}

	numRows := int(tbl.NumRows())

	if metaDataIdx >= 0 {
		col := tbl.Column(metaDataIdx)
		for _, chunk := range col.Data().Chunks() {
			for row := 0; row < chunk.Len(); row++ {
				if chunk.IsNull(row) {
					continue
				}
				jsonStr := chunk.ValueStr(row)
				var md metaDataAction
				if err := json.Unmarshal([]byte(jsonStr), &md); err == nil {
					snapshot.MetaData = &md
				}
			}
		}
	}

	if protocolIdx >= 0 {
		col := tbl.Column(protocolIdx)
		for _, chunk := range col.Data().Chunks() {
			for row := 0; row < chunk.Len(); row++ {
				if chunk.IsNull(row) {
					continue
				}
				jsonStr := chunk.ValueStr(row)
				var p protocolAction
				if err := json.Unmarshal([]byte(jsonStr), &p); err == nil {
					snapshot.Protocol = &p
				}
			}
		}
	}

	if addIdx >= 0 {
		col := tbl.Column(addIdx)
		for _, chunk := range col.Data().Chunks() {
			for row := 0; row < chunk.Len(); row++ {
				if chunk.IsNull(row) {
					continue
				}
				jsonStr := chunk.ValueStr(row)
				var a addAction
				if err := json.Unmarshal([]byte(jsonStr), &a); err == nil {
					snapshot.ActiveFiles[a.Path] = a.Size
				}
			}
		}
	}

	_ = numRows
	snapshot.CurrentVersion = versionFromPath(checkpointPath)

	return snapshot, nil
}

// readLogFile parses a single JSON log file (newline-delimited) and updates the snapshot.
func readLogFile(path string, snapshot *deltaSnapshot) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	version := versionFromPath(path)
	if version > snapshot.CurrentVersion {
		snapshot.CurrentVersion = version
	}

	scanner := bufio.NewScanner(f)
	// Increase buffer size for large log entries.
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var entry logEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if entry.MetaData != nil {
			snapshot.MetaData = entry.MetaData
		}
		if entry.Protocol != nil {
			snapshot.Protocol = entry.Protocol
		}
		if entry.Add != nil {
			snapshot.ActiveFiles[entry.Add.Path] = entry.Add.Size
		}
		if entry.Remove != nil {
			delete(snapshot.ActiveFiles, entry.Remove.Path)
		}
	}

	return scanner.Err()
}

// listLogFiles returns sorted JSON log file paths with version >= startVersion.
func listLogFiles(deltaLogDir string, startVersion int64) ([]string, error) {
	entries, err := os.ReadDir(deltaLogDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		version := versionFromPath(name)
		if version >= startVersion {
			files = append(files, filepath.Join(deltaLogDir, name))
		}
	}

	sort.Strings(files)
	return files, nil
}

// versionFromPath extracts the version number from a Delta log filename.
func versionFromPath(path string) int64 {
	base := filepath.Base(path)
	// Strip all suffixes (.json, .checkpoint.parquet, etc.)
	numStr := strings.SplitN(base, ".", 2)[0]
	v, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return -1
	}
	return v
}

// parseSchemaString parses a Delta Lake JSON schema string into a deltaSchema.
func parseSchemaString(schemaString string) (*deltaSchema, error) {
	var schema deltaSchema
	if err := json.Unmarshal([]byte(schemaString), &schema); err != nil {
		return nil, fmt.Errorf("parsing schema string: %w", err)
	}
	return &schema, nil
}
