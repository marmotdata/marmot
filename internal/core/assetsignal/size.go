package assetsignal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/marmotdata/marmot/internal/core/assetdetail"
)

// Size is how big an asset is, where its source says. Either part is nil when unknown.
type Size struct {
	Rows    *int64
	Columns *int
}

// SizeOf returns the size an asset's metadata and schema describe, read the same way as the asset page's header.
func SizeOf(metadata map[string]interface{}, schema map[string]string) Size {
	var size Size
	if n, ok := assetdetail.RowCount(metadata); ok {
		size.Rows = &n
	}
	if n, ok := assetdetail.ColumnCount(schema); ok {
		size.Columns = &n
	}
	return size
}

// Sizes returns the size of each asset in ids; assets with no known size are left out.
func (s *Service) Sizes(ctx context.Context, ids []string) (map[string]Size, error) {
	out := make(map[string]Size, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `SELECT id, metadata, schema FROM assets WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("reading asset sizes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var metadata map[string]interface{}
		var rawSchema map[string]json.RawMessage
		if err := rows.Scan(&id, &metadata, &rawSchema); err != nil {
			return nil, fmt.Errorf("scanning asset size: %w", err)
		}
		if size := SizeOf(metadata, schemaText(rawSchema)); size.Rows != nil || size.Columns != nil {
			out[id] = size
		}
	}
	return out, rows.Err()
}

// schemaText turns stored schema entries into the text form assets carry, keeping entries that were stored as JSON objects rather than strings.
func schemaText(raw map[string]json.RawMessage) map[string]string {
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		var text string
		if err := json.Unmarshal(v, &text); err == nil {
			out[k] = text
			continue
		}
		out[k] = string(v)
	}
	return out
}
