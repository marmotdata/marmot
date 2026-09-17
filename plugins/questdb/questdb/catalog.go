package questdb

import (
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// objectKind is what a tables() row describes. QuestDB reports it in
// table_type as T, V or M on builds that have views.
type objectKind string

const (
	kindTable            objectKind = "table"
	kindView             objectKind = "view"
	kindMaterializedView objectKind = "materialized_view"
)

// tableInfo is one row of tables(), read by column name.
type tableInfo struct {
	Name                string
	Kind                objectKind
	DesignatedTimestamp string
	PartitionBy         string
	WALEnabled          bool
	Dedup               bool
	TTLValue            int64
	TTLUnit             string
	MaxUncommittedRows  int64
	// O3MaxLag is -1 when the build does not report it or the object has
	// none (materialized views report -1 themselves).
	O3MaxLag int64
}

// ttl renders the retention as QuestDB stores it, for example "1 WEEK" for
// a table created with TTL 7 DAYS. Empty when the table keeps data forever.
func (t tableInfo) ttl() string {
	if t.TTLValue <= 0 || t.TTLUnit == "" {
		return ""
	}
	return fmt.Sprintf("%d %s", t.TTLValue, t.TTLUnit)
}

// isStored reports whether the object has partitions on disk, which is true
// for tables and materialized views but not for plain views.
func (t tableInfo) isStored() bool {
	return t.Kind != kindView
}

func parseTableRow(r row) (tableInfo, bool) {
	name := r.str("table_name")
	if name == "" {
		return tableInfo{}, false
	}

	info := tableInfo{
		Name:                name,
		Kind:                objectKindOf(r),
		DesignatedTimestamp: r.str("designatedTimestamp"),
		PartitionBy:         r.str("partitionBy"),
		WALEnabled:          r.boolean("walEnabled"),
		Dedup:               r.boolean("dedup"),
		TTLValue:            r.integer("ttlValue"),
		TTLUnit:             r.str("ttlUnit"),
		MaxUncommittedRows:  r.integer("maxUncommittedRows"),
		O3MaxLag:            -1,
	}
	if r.has("o3MaxLag") {
		info.O3MaxLag = r.integer("o3MaxLag")
	}

	return info, true
}

func objectKindOf(r row) objectKind {
	switch strings.ToUpper(strings.TrimSpace(r.str("table_type"))) {
	case "T":
		return kindTable
	case "V":
		return kindView
	case "M":
		return kindMaterializedView
	}
	// Builds before table_type only flag materialized views.
	if r.boolean("matView") {
		return kindMaterializedView
	}
	return kindTable
}

// columnInfo is the per-column shape serialised into an asset's schema:
// the SDK's standard fields plus what QuestDB adds. Symbol fields are only
// set for SYMBOL columns, so they are omitted everywhere else.
type columnInfo struct {
	pluginsdk.Column
	DesignatedTimestamp bool  `json:"designated_timestamp,omitempty"`
	Indexed             bool  `json:"indexed,omitempty"`
	SymbolCapacity      int64 `json:"symbol_capacity,omitempty"`
	SymbolCached        *bool `json:"symbol_cached,omitempty"`
	UpsertKey           bool  `json:"upsert_key,omitempty"`
}

// parseColumnRow reads one row of table_columns(). Every QuestDB column is
// nullable and there are no primary keys, so only the QuestDB-specific
// flags carry information.
func parseColumnRow(r row) (columnInfo, bool) {
	name := r.str("column")
	if name == "" {
		return columnInfo{}, false
	}

	c := columnInfo{
		Column: pluginsdk.Column{
			Name:     name,
			DataType: r.str("type"),
			Nullable: true,
		},
		DesignatedTimestamp: r.boolean("designated"),
		Indexed:             r.boolean("indexed"),
		UpsertKey:           r.boolean("upsertKey"),
	}

	if strings.EqualFold(c.DataType, "SYMBOL") {
		c.SymbolCapacity = r.integer("symbolCapacity")
		cached := r.boolean("symbolCached")
		c.SymbolCached = &cached
	}

	return c, true
}

// viewInfo is one row of views() or materialized_views().
type viewInfo struct {
	Name          string
	SQL           string
	Materialized  bool
	BaseTable     string
	RefreshType   string
	RefreshPeriod string
	LastRefresh   time.Time
}

func parseViewRow(r row) (viewInfo, bool) {
	name := r.str("view_name")
	if name == "" {
		return viewInfo{}, false
	}
	return viewInfo{Name: name, SQL: r.str("view_sql")}, true
}

func parseMaterializedViewRow(r row) (viewInfo, bool) {
	name := r.str("view_name")
	if name == "" {
		return viewInfo{}, false
	}

	v := viewInfo{
		Name:          name,
		SQL:           r.str("view_sql"),
		Materialized:  true,
		BaseTable:     r.str("base_table_name"),
		RefreshType:   r.str("refresh_type"),
		RefreshPeriod: refreshPeriodOf(r),
	}
	if ts, ok := r.timestamp("last_refresh_start_timestamp"); ok {
		v.LastRefresh = ts
	}

	return v, true
}

// refreshPeriodOf renders how often a materialized view refreshes, for
// example "1 HOUR" for REFRESH EVERY 1h. QuestDB splits it into a value and
// unit column per refresh type; immediate and manual views have none.
func refreshPeriodOf(r row) string {
	if period := r.str("refresh_period"); period != "" {
		return period
	}
	for _, pair := range [][2]string{
		{"timer_interval", "timer_interval_unit"},
		{"period_length", "period_length_unit"},
	} {
		value, unit := r.integer(pair[0]), r.str(pair[1])
		if value > 0 && unit != "" {
			return fmt.Sprintf("%d %s", value, unit)
		}
	}
	return ""
}

// isSystemTable reports whether name is one of QuestDB's own tables: the
// sys.* and underscore-prefixed internals and the telemetry pair.
func isSystemTable(name string) bool {
	return strings.HasPrefix(name, "sys.") ||
		strings.HasPrefix(name, "_") ||
		name == "telemetry" ||
		name == "telemetry_config"
}
