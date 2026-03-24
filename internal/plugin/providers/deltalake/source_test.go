package deltalake

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTempDeltaTable(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "_delta_log"), 0o755))
	return dir
}

func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) plugin.RawPluginConfig
		expectErr bool
	}{
		{
			name: "valid config with single path",
			setup: func(t *testing.T) plugin.RawPluginConfig {
				dir := createTempDeltaTable(t)
				return plugin.RawPluginConfig{
					"table_paths": []interface{}{dir},
				}
			},
			expectErr: false,
		},
		{
			name: "valid config with multiple paths",
			setup: func(t *testing.T) plugin.RawPluginConfig {
				dir1 := createTempDeltaTable(t)
				dir2 := createTempDeltaTable(t)
				return plugin.RawPluginConfig{
					"table_paths": []interface{}{dir1, dir2},
				}
			},
			expectErr: false,
		},
		{
			name: "missing table_paths",
			setup: func(t *testing.T) plugin.RawPluginConfig {
				return plugin.RawPluginConfig{}
			},
			expectErr: true,
		},
		{
			name: "empty table_paths",
			setup: func(t *testing.T) plugin.RawPluginConfig {
				return plugin.RawPluginConfig{
					"table_paths": []interface{}{},
				}
			},
			expectErr: true,
		},
		{
			name: "path without _delta_log",
			setup: func(t *testing.T) plugin.RawPluginConfig {
				dir := t.TempDir()
				return plugin.RawPluginConfig{
					"table_paths": []interface{}{dir},
				}
			},
			expectErr: true,
		},
		{
			name: "config with tags",
			setup: func(t *testing.T) plugin.RawPluginConfig {
				dir := createTempDeltaTable(t)
				return plugin.RawPluginConfig{
					"table_paths": []interface{}{dir},
					"tags":        []interface{}{"delta", "data-lake"},
				}
			},
			expectErr: false,
		},
		{
			name: "config with filter",
			setup: func(t *testing.T) plugin.RawPluginConfig {
				dir := createTempDeltaTable(t)
				return plugin.RawPluginConfig{
					"table_paths": []interface{}{dir},
					"filter": map[string]interface{}{
						"include": []interface{}{"^events.*"},
					},
				}
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{}
			config := tt.setup(t)
			_, err := s.Validate(config)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSource_ValidateStoresConfig(t *testing.T) {
	dir := createTempDeltaTable(t)

	s := &Source{}
	_, err := s.Validate(plugin.RawPluginConfig{
		"table_paths": []interface{}{dir},
	})
	require.NoError(t, err)
	require.NotNil(t, s.config)
	require.Len(t, s.config.TablePaths, 1)
	assert.Equal(t, dir, s.config.TablePaths[0])
}

func TestReadDeltaLog(t *testing.T) {
	t.Run("basic metadata and protocol", func(t *testing.T) {
		dir := createTempDeltaTable(t)
		logDir := filepath.Join(dir, "_delta_log")

		writeLogFile(t, logDir, 0, []logEntry{
			{Protocol: &protocolAction{MinReaderVersion: 1, MinWriterVersion: 2}},
			{MetaData: &metaDataAction{
				ID:               "test-table-id",
				Description:      "A test table",
				Format:           formatSpec{Provider: "parquet"},
				SchemaString:     `{"type":"struct","fields":[{"name":"id","type":"integer","nullable":false,"metadata":{}},{"name":"name","type":"string","nullable":true,"metadata":{}}]}`,
				PartitionColumns: []string{"date"},
				CreatedTime:      1700000000000,
			}},
			{Add: &addAction{Path: "part-00000.parquet", Size: 1024}},
			{Add: &addAction{Path: "part-00001.parquet", Size: 2048}},
		})

		snapshot, err := readDeltaLog(dir)
		require.NoError(t, err)

		require.NotNil(t, snapshot.Protocol)
		assert.Equal(t, 1, snapshot.Protocol.MinReaderVersion)
		assert.Equal(t, 2, snapshot.Protocol.MinWriterVersion)

		require.NotNil(t, snapshot.MetaData)
		assert.Equal(t, "test-table-id", snapshot.MetaData.ID)
		assert.Equal(t, "A test table", snapshot.MetaData.Description)
		assert.Equal(t, "parquet", snapshot.MetaData.Format.Provider)
		assert.Equal(t, []string{"date"}, snapshot.MetaData.PartitionColumns)
		assert.Equal(t, int64(1700000000000), snapshot.MetaData.CreatedTime)

		assert.Equal(t, 2, snapshot.NumFiles)
		assert.Equal(t, int64(3072), snapshot.TotalSize)
		assert.Equal(t, int64(0), snapshot.CurrentVersion)
	})

	t.Run("add and remove tracking", func(t *testing.T) {
		dir := createTempDeltaTable(t)
		logDir := filepath.Join(dir, "_delta_log")

		writeLogFile(t, logDir, 0, []logEntry{
			{MetaData: &metaDataAction{ID: "t1", Format: formatSpec{Provider: "parquet"}}},
			{Add: &addAction{Path: "file1.parquet", Size: 100}},
			{Add: &addAction{Path: "file2.parquet", Size: 200}},
		})

		writeLogFile(t, logDir, 1, []logEntry{
			{Remove: &removeAction{Path: "file1.parquet"}},
			{Add: &addAction{Path: "file3.parquet", Size: 300}},
		})

		snapshot, err := readDeltaLog(dir)
		require.NoError(t, err)

		assert.Equal(t, 2, snapshot.NumFiles)
		assert.Equal(t, int64(500), snapshot.TotalSize)
		assert.Equal(t, int64(1), snapshot.CurrentVersion)

		_, hasFile1 := snapshot.ActiveFiles["file1.parquet"]
		assert.False(t, hasFile1, "file1 should have been removed")
		_, hasFile2 := snapshot.ActiveFiles["file2.parquet"]
		assert.True(t, hasFile2)
		_, hasFile3 := snapshot.ActiveFiles["file3.parquet"]
		assert.True(t, hasFile3)
	})

	t.Run("schema evolution", func(t *testing.T) {
		dir := createTempDeltaTable(t)
		logDir := filepath.Join(dir, "_delta_log")

		writeLogFile(t, logDir, 0, []logEntry{
			{MetaData: &metaDataAction{
				ID:           "t1",
				SchemaString: `{"type":"struct","fields":[{"name":"id","type":"integer","nullable":false,"metadata":{}}]}`,
			}},
		})

		writeLogFile(t, logDir, 1, []logEntry{
			{MetaData: &metaDataAction{
				ID:           "t1",
				SchemaString: `{"type":"struct","fields":[{"name":"id","type":"integer","nullable":false,"metadata":{}},{"name":"email","type":"string","nullable":true,"metadata":{}}]}`,
			}},
		})

		snapshot, err := readDeltaLog(dir)
		require.NoError(t, err)

		require.NotNil(t, snapshot.MetaData)
		schema, err := parseSchemaString(snapshot.MetaData.SchemaString)
		require.NoError(t, err)
		assert.Len(t, schema.Fields, 2, "later metaData should win")
		assert.Equal(t, "email", schema.Fields[1].Name)
	})
}

func TestParseSchemaString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectErr   bool
		expectCount int
	}{
		{
			name:        "simple fields",
			input:       `{"type":"struct","fields":[{"name":"id","type":"long","nullable":false,"metadata":{}},{"name":"value","type":"string","nullable":true,"metadata":{}}]}`,
			expectErr:   false,
			expectCount: 2,
		},
		{
			name:        "nested type",
			input:       `{"type":"struct","fields":[{"name":"data","type":{"type":"map","keyType":"string","valueType":"integer","valueContainsNull":true},"nullable":true,"metadata":{}}]}`,
			expectErr:   false,
			expectCount: 1,
		},
		{
			name:      "invalid JSON",
			input:     `not json`,
			expectErr: true,
		},
		{
			name:        "empty fields",
			input:       `{"type":"struct","fields":[]}`,
			expectErr:   false,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parseSchemaString(tt.input)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectCount, len(schema.Fields))
		})
	}
}

func TestCreateTableAsset(t *testing.T) {
	snapshot := &deltaSnapshot{
		MetaData: &metaDataAction{
			ID:               "table-uuid-123",
			Description:      "Test table description",
			Format:           formatSpec{Provider: "parquet"},
			SchemaString:     `{"type":"struct","fields":[{"name":"id","type":"long","nullable":false,"metadata":{}},{"name":"name","type":"string","nullable":true,"metadata":{}}]}`,
			PartitionColumns: []string{"date", "region"},
			CreatedTime:      1700000000000,
			Configuration:    map[string]string{"delta.autoOptimize.optimizeWrite": "true"},
		},
		Protocol: &protocolAction{
			MinReaderVersion: 1,
			MinWriterVersion: 2,
		},
		NumFiles:       5,
		TotalSize:      10240,
		CurrentVersion: 3,
		ActiveFiles:    map[string]int64{"a": 1},
	}

	config := &Config{
		BaseConfig: plugin.BaseConfig{
			Tags: []string{"test-tag"},
		},
	}

	a := createTableAsset(snapshot, "/data/delta/events", config)

	assert.Equal(t, "events", *a.Name)
	assert.Equal(t, "mrn://table/deltalake/events", *a.MRN)
	assert.Equal(t, "Table", a.Type)
	assert.Equal(t, []string{"Delta Lake"}, a.Providers)
	assert.Equal(t, "Test table description", *a.Description)

	assert.Equal(t, "table-uuid-123", a.Metadata["table_id"])
	assert.Equal(t, "/data/delta/events", a.Metadata["location"])
	assert.Equal(t, "parquet", a.Metadata["format"])
	assert.Equal(t, 1, a.Metadata["min_reader_version"])
	assert.Equal(t, 2, a.Metadata["min_writer_version"])
	assert.Equal(t, "date, region", a.Metadata["partition_columns"])
	assert.Equal(t, 2, a.Metadata["schema_field_count"])
	assert.Equal(t, int64(1700000000000), a.Metadata["created_time"])
	assert.Equal(t, 5, a.Metadata["num_files"])
	assert.Equal(t, int64(10240), a.Metadata["total_size"])
	assert.Equal(t, int64(3), a.Metadata["current_version"])
	assert.Equal(t, "true", a.Metadata["property.delta.autoOptimize.optimizeWrite"])

	require.NotNil(t, a.Schema)
	assert.Contains(t, a.Schema["columns"], `"name":"id"`)
	assert.Contains(t, a.Schema["columns"], `"name":"name"`)

	require.Len(t, a.Sources, 1)
	assert.Equal(t, "Delta Lake", a.Sources[0].Name)
	assert.Equal(t, 1, a.Sources[0].Priority)
}

// writeLogFile writes a list of log entries as a newline-delimited JSON file.
func writeLogFile(t *testing.T, logDir string, version int64, entries []logEntry) {
	t.Helper()
	filename := filepath.Join(logDir, fmt.Sprintf("%020d.json", version))
	var data []byte
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		require.NoError(t, err)
		data = append(data, line...)
		data = append(data, '\n')
	}
	require.NoError(t, os.WriteFile(filename, data, 0o644))
}
