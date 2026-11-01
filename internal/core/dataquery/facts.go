package dataquery

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

// TableFacts is what the catalog holds about a table outside the asset record: its owners' names and the columns people marked as personal data.
type TableFacts struct {
	Owners       []string
	PersonalData []string
}

// rowCountKeys are the metadata keys plugins report a table's size under, most specific first.
var rowCountKeys = []string{"row_count", "num_rows", "estimated_row_count", "record_count"}

// rowCountOf reads a table's row count from its metadata, which plugins report as a number or, after a round trip through text, a numeric string.
func rowCountOf(meta map[string]any) *int64 {
	for _, key := range rowCountKeys {
		var n int64
		switch v := meta[key].(type) {
		case float64:
			n = int64(v)
		case int64:
			n = v
		case int:
			n = int64(v)
		case json.Number:
			parsed, err := v.Int64()
			if err != nil {
				continue
			}
			n = parsed
		case string:
			parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err != nil {
				continue
			}
			n = parsed
		default:
			continue
		}
		if n >= 0 {
			return &n
		}
	}
	return nil
}

// catalogColumn is one column of a table's catalogued schema.
type catalogColumn struct {
	name string
	pii  bool
}

// catalogColumns reads a table's columns from its schema documents. Database plugins store a list of column objects; JSON Schema documents list properties. A column is personal data when its tags say so, such as a PII tag synced from another catalog.
func catalogColumns(schema map[string]string) []catalogColumn {
	var out []catalogColumn
	for _, doc := range schema {
		var list []map[string]any
		if err := json.Unmarshal([]byte(doc), &list); err == nil {
			for _, c := range list {
				name, _ := c["column_name"].(string)
				if name == "" {
					name, _ = c["name"].(string)
				}
				if name != "" {
					out = append(out, catalogColumn{name: name, pii: taggedPersonal(c["tags"]) || c["pii"] == true})
				}
			}
			continue
		}
		var obj struct {
			Properties map[string]map[string]any `json:"properties"`
		}
		if err := json.Unmarshal([]byte(doc), &obj); err == nil {
			for name, p := range obj.Properties {
				out = append(out, catalogColumn{name: name, pii: taggedPersonal(p["tags"]) || p["pii"] == true})
			}
		}
	}
	return out
}

func taggedPersonal(tags any) bool {
	list, _ := tags.([]any)
	for _, t := range list {
		s, _ := t.(string)
		s = strings.ToLower(s)
		if strings.Contains(s, "pii") || strings.Contains(s, "personal") {
			return true
		}
	}
	return false
}

// columnFacts counts the table's catalogued columns the result returned and lists its personal data columns, each marked read when the result returned it. Columns are matched by name, ignoring case, since the plan does not name columns.
func columnFacts(columns []catalogColumn, marked []string, result []QueryColumn) (*int, []PersonalDataColumn) {
	returned := map[string]bool{}
	for _, c := range result {
		returned[strings.ToLower(c.Name)] = true
	}
	var read *int
	if len(columns) > 0 {
		n := 0
		for _, c := range columns {
			if returned[strings.ToLower(c.name)] {
				n++
			}
		}
		read = &n
	}

	var personal []PersonalDataColumn
	seen := map[string]bool{}
	add := func(name string) {
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		personal = append(personal, PersonalDataColumn{Name: name, Read: returned[key]})
	}
	for _, c := range columns {
		if c.pii {
			add(c.name)
		}
	}
	for _, name := range marked {
		// A marker holds the field's dotted path; the top-level column is what a result returns.
		top, _, _ := strings.Cut(name, ".")
		add(top)
	}
	slices.SortStableFunc(personal, func(a, b PersonalDataColumn) int {
		if a.Read != b.Read {
			if a.Read {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return read, personal
}

func (r *PostgresRepository) TableFacts(ctx context.Context, assetIDs []string) (map[string]TableFacts, error) {
	out := map[string]TableFacts{}
	if len(assetIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT ao.asset_id, COALESCE(NULLIF(u.name, ''), u.username, t.name)
		FROM asset_owners ao
		LEFT JOIN users u ON u.id = ao.user_id
		LEFT JOIN teams t ON t.id = ao.team_id
		WHERE ao.asset_id = ANY($1)
		ORDER BY ao.asset_id, (ao.team_id IS NULL), 2`, assetIDs)
	if err != nil {
		return nil, fmt.Errorf("loading table owners: %w", err)
	}
	for rows.Next() {
		var id string
		var name *string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		if name != nil && *name != "" {
			f := out[id]
			f.Owners = append(f.Owners, *name)
			out[id] = f
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.db.Query(ctx, `
		SELECT asset_id, field_path FROM asset_field_annotations
		WHERE asset_id = ANY($1) AND pii
		ORDER BY asset_id, field_path`, assetIDs)
	if err != nil {
		// Owners are still worth returning when the markers cannot be read.
		log.Warn().Err(err).Msg("Failed to load personal data columns for query context")
		return out, nil
	}
	defer rows.Close()
	for rows.Next() {
		var id, path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		f := out[id]
		f.PersonalData = append(f.PersonalData, path)
		out[id] = f
	}
	return out, rows.Err()
}
