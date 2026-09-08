package hive

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DESCRIBE FORMATTED is the only way HiveServer2 exposes a table's storage,
// ownership, statistics, constraints and view text in one call. It comes
// back as rows of (col_name, data_type, comment) that read as a report:
// the column list first, then "# ..." section headers with "Key:" rows,
// nested key/value blocks and multi-line query text. parseDescribeFormatted
// turns that report into a tableInfo.

type tableInfo struct {
	Columns          []columnInfo
	PartitionColumns []columnInfo
	// Details holds the "# Detailed Table Information" rows by key:
	// Database, Owner, OwnerType, CreateTime, LastAccessTime, Location,
	// Table Type. View sections add Rewrite Enabled and the like.
	Details map[string]string
	// Parameters is the Table Parameters block: numRows, totalSize,
	// comment, transactional, EXTERNAL and any user property.
	Parameters map[string]string
	// Storage holds the "# Storage Information" rows by key: SerDe Library,
	// InputFormat, OutputFormat, Compressed, Num Buckets, Bucket Columns,
	// Sort Columns.
	Storage       map[string]string
	StorageParams map[string]string
	OriginalQuery string
	ExpandedQuery string
	PrimaryKey    []string
	NotNull       []string
	Defaults      map[string]string
	ForeignKeys   []foreignKey
	// SourceTables lists the db.table names a materialized view is built
	// from, as Hive records them.
	SourceTables []string
}

type columnInfo struct {
	Name     string
	DataType string
	Comment  string
}

type foreignKey struct {
	Name         string
	Column       string
	ParentTable  string // db.table
	ParentColumn string
}

func parseDescribeFormatted(rows [][]string) *tableInfo {
	info := &tableInfo{
		Details:       make(map[string]string),
		Parameters:    make(map[string]string),
		Storage:       make(map[string]string),
		StorageParams: make(map[string]string),
		Defaults:      make(map[string]string),
	}

	section := ""
	// The key/value block (Table Parameters, Storage Desc Params) rows are
	// currently being added to, if any.
	var params map[string]string
	// The query text (Original Query, Expanded Query) continuation rows are
	// currently being appended to, if any.
	var query *string
	var fk foreignKey

	for _, row := range rows {
		name, typ, comment := cell(row, 0), cell(row, 1), cell(row, 2)

		if strings.HasPrefix(name, "# ") {
			// "# col_name" is the header row inside the partition block,
			// not a section of its own.
			if name == "# col_name" {
				continue
			}
			section = name
			params = nil
			query = nil
			continue
		}

		blank := name == "" && typ == "" && comment == ""

		switch section {
		case "":
			if !blank {
				info.Columns = append(info.Columns, columnInfo{Name: name, DataType: typ, Comment: comment})
			}

		case "# Partition Information":
			if !blank {
				info.PartitionColumns = append(info.PartitionColumns, columnInfo{Name: name, DataType: typ, Comment: comment})
			}

		case "# Detailed Table Information", "# Storage Information":
			if blank {
				params = nil
				continue
			}
			if name == "" {
				if params != nil {
					params[typ] = comment
				}
				continue
			}
			key, value, ok := keyValue(name, typ)
			if !ok {
				continue
			}
			switch key {
			case "Table Parameters":
				params = info.Parameters
			case "Storage Desc Params":
				params = info.StorageParams
			default:
				if section == "# Storage Information" {
					info.Storage[key] = value
				} else {
					info.Details[key] = value
				}
			}

		case "# View Information", "# Materialized View Information":
			if blank {
				query = nil
				continue
			}
			if name == "" {
				// Continuation lines of a multi-line query arrive with the
				// text in the comment column and padding in data_type.
				line := typ
				if line == "" {
					line = comment
				}
				if query != nil && line != "" {
					*query += "\n" + line
				}
				continue
			}
			key, value, ok := keyValue(name, typ)
			if !ok {
				continue
			}
			switch key {
			case "Original Query":
				info.OriginalQuery = value
				query = &info.OriginalQuery
			case "Expanded Query":
				info.ExpandedQuery = value
				query = &info.ExpandedQuery
			default:
				query = nil
				info.Details[key] = value
			}

		case "# Primary Key":
			if key, value, ok := keyValue(name, typ); ok && key == "Column Name" {
				info.PrimaryKey = append(info.PrimaryKey, value)
			}

		case "# Not Null Constraints":
			if key, value, ok := keyValue(name, typ); ok && key == "Column Name" {
				info.NotNull = append(info.NotNull, value)
			}

		case "# Default Constraints":
			// The column and its default share one row: "Column Name:qty",
			// "Default Value:0".
			key, column, ok := keyValue(name, "")
			if !ok || key != "Column Name" {
				continue
			}
			if dkey, value, ok := keyValue(typ, ""); ok && dkey == "Default Value" {
				info.Defaults[column] = value
			}

		case "# Foreign Keys":
			key, value, ok := keyValue(name, typ)
			if !ok {
				continue
			}
			switch key {
			case "Constraint Name":
				fk = foreignKey{Name: value}
			case "Parent Column Name":
				// One row per key column: "Parent Column Name:db.table.col",
				// "Column Name:col", "Key Sequence:n".
				_, column, _ := keyValue(typ, "")
				parentTable, parentColumn := splitParentColumn(value)
				info.ForeignKeys = append(info.ForeignKeys, foreignKey{
					Name:         fk.Name,
					Column:       column,
					ParentTable:  parentTable,
					ParentColumn: parentColumn,
				})
			}

		case "# Materialized View Source table information":
			if blank || name == "Table name" {
				continue
			}
			info.SourceTables = append(info.SourceTables, name)
		}
	}

	return info
}

func cell(row []string, i int) string {
	if i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// keyValue splits a "Key:" cell from its value. Hive writes the value in
// the next column for most rows ("Owner:", "hive") but inline for the
// constraint rows ("Column Name:qty"), so both shapes are accepted.
func keyValue(name, next string) (key, value string, ok bool) {
	i := strings.Index(name, ":")
	if i < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(name[:i])
	value = strings.TrimSpace(name[i+1:])
	if value == "" {
		value = strings.TrimSpace(next)
	}
	return key, value, true
}

// splitParentColumn splits the db.table.column a foreign key points at
// into the table and the column.
func splitParentColumn(ref string) (table, column string) {
	i := strings.LastIndex(ref, ".")
	if i < 0 {
		return "", ref
	}
	return ref[:i], ref[i+1:]
}

// objectType names the kind of object in the words the rest of Marmot's
// SQL plugins use.
func (t *tableInfo) objectType() string {
	switch t.Details["Table Type"] {
	case "MANAGED_TABLE":
		return "managed"
	case "EXTERNAL_TABLE":
		return "external"
	case "VIRTUAL_VIEW":
		return "view"
	case "MATERIALIZED_VIEW":
		return "materialized_view"
	}
	return strings.ToLower(t.Details["Table Type"])
}

func (t *tableInfo) isView() bool {
	switch t.objectType() {
	case "view", "materialized_view":
		return true
	}
	return false
}

// paramInt reads a numeric table parameter such as numRows or totalSize.
func (t *tableInfo) paramInt(key string) (int64, bool) {
	raw, ok := t.Parameters[key]
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func (t *tableInfo) paramBool(key string) (bool, bool) {
	raw, ok := t.Parameters[key]
	if !ok {
		return false, false
	}
	return strings.EqualFold(strings.TrimSpace(raw), "true"), true
}

// bucketColumns and sortColumns arrive as a Java list: "[customer_id]".
func (t *tableInfo) bucketColumns() []string {
	return parseList(t.Storage["Bucket Columns"])
}

func (t *tableInfo) sortColumns() []string {
	return parseList(t.Storage["Sort Columns"])
}

func (t *tableInfo) numBuckets() int {
	n, err := strconv.Atoi(strings.TrimSpace(t.Storage["Num Buckets"]))
	if err != nil {
		return 0
	}
	return n
}

// sortOrderPattern matches one entry of a Sort Columns list, which Hive
// prints as "[Order(col:ts, order:1), Order(col:b, order:0)]".
var sortOrderPattern = regexp.MustCompile(`Order\(col:([^,)]+), order:-?\d+\)`)

func parseList(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	if raw == "" {
		return nil
	}
	var items []string
	if strings.Contains(raw, "Order(") {
		for _, match := range sortOrderPattern.FindAllStringSubmatch(raw, -1) {
			items = append(items, strings.TrimSpace(match[1]))
		}
		return items
	}
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// hiveTimeLayout is how DESCRIBE FORMATTED prints CreateTime and
// LastAccessTime: Java's Date.toString.
const hiveTimeLayout = "Mon Jan 02 15:04:05 MST 2006"

// hiveTime converts a DESCRIBE FORMATTED timestamp to RFC 3339, keeping the
// raw text when it is not a timestamp. "UNKNOWN" (never accessed) becomes "".
func hiveTime(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "UNKNOWN" {
		return ""
	}
	t, err := time.Parse(hiveTimeLayout, raw)
	if err != nil {
		return raw
	}
	return t.UTC().Format(time.RFC3339)
}
