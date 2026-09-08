package sftp

import (
	"context"
	"fmt"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// sampleDataRows is how many records a preview shows.
const sampleDataRows = 20

// sampleTimeout bounds a preview. It is much shorter than a discovery
// run because someone is waiting on the answer.
const sampleTimeout = 2 * time.Minute

// FetchSampleData reads the first rows of a file so the UI can preview
// it. Only the kinds this plugin can parse are previewable.
func (s *Source) FetchSampleData(ctx context.Context, rawConfig pluginsdk.RawConfig, asset *pluginsdk.Asset) ([]string, [][]any, error) {
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	if asset == nil {
		return nil, nil, fmt.Errorf("no asset to sample")
	}
	if asset.Type != typeFile {
		return nil, nil, fmt.Errorf("sample data is only available for files, not %s assets", asset.Type)
	}

	remotePath, _ := asset.Metadata["path"].(string)
	if remotePath == "" {
		return nil, nil, fmt.Errorf("asset has no path metadata, run discovery again to record it")
	}

	kind := fileKind(remotePath)
	if !hasColumns(kind) {
		return nil, nil, fmt.Errorf("sample data is not available for %s files, only csv, tsv, json and jsonl", kind)
	}

	ctx, cancel := context.WithTimeout(ctx, sampleTimeout)
	defer cancel()

	conn, err := connect(ctx, s.config)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to %s: %w", s.config.address(), err)
	}
	defer conn.Close()

	return fetchSample(conn, remotePath, kind, s.config.MaxReadBytes)
}

// fetchSample reads the head of one file with the same readers
// discovery uses, so a preview never disagrees with the schema shown
// beside it.
func fetchSample(fsys fileSystem, remotePath, kind string, maxBytes int64) ([]string, [][]any, error) {
	file, err := fsys.Open(remotePath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening %s: %w", remotePath, err)
	}
	defer file.Close()

	if kind == kindCSV || kind == kindTSV {
		data, err := readDelimited(file, separator(kind), maxBytes, sampleDataRows)
		if err != nil {
			return nil, nil, fmt.Errorf("reading %s: %w", remotePath, err)
		}

		rows := make([][]any, 0, len(data.Rows))
		for _, record := range data.Rows {
			row := make([]any, len(data.Header))
			for i := range data.Header {
				if i < len(record) {
					row[i] = record[i]
				}
			}
			rows = append(rows, row)
		}
		return data.Header, rows, nil
	}

	data, err := readJSON(file, maxBytes, sampleDataRows)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", remotePath, err)
	}

	columns := inferJSONColumns(data.Records)
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		names = append(names, column.Name)
	}

	rows := make([][]any, 0, len(data.Records))
	for _, record := range data.Records {
		row := make([]any, 0, len(names))
		for _, name := range names {
			row = append(row, jsonValue(record, name))
		}
		rows = append(rows, row)
	}

	return names, rows, nil
}
