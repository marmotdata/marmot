package postgresql

import (
	"regexp"
	"strings"
)

// callPattern finds a function call in a deparsed expression: a name, optionally schema-qualified or quoted, immediately followed by an opening parenthesis. PostgreSQL deparses calls with no space before the parenthesis, while keywords such as ANY and IN are followed by one.
var callPattern = regexp.MustCompile(`((?:"(?:[^"]|"")+"|[A-Za-z_][A-Za-z0-9_$]*)(?:\.(?:"(?:[^"]|"")+"|[A-Za-z_][A-Za-z0-9_$]*))*)\(`)

// safeFunctions are built-ins that compute from their arguments alone. None of them reads a table, a file or another session, so a statement calling only these reads nothing beyond the relations in its plan. Anything not listed, including every user-defined function, is treated as able to read data the plan cannot see. A few entries are keywords or type names that deparse like calls, such as COALESCE( and numeric(10,2).
var safeFunctions = toSet(
	// aggregates and window functions
	"count", "sum", "avg", "min", "max", "array_agg", "string_agg", "bool_and", "bool_or", "every",
	"bit_and", "bit_or", "stddev", "stddev_pop", "stddev_samp", "variance", "var_pop", "var_samp",
	"percentile_cont", "percentile_disc", "mode", "corr", "covar_pop", "covar_samp",
	"json_agg", "jsonb_agg", "json_object_agg", "jsonb_object_agg",
	"row_number", "rank", "dense_rank", "percent_rank", "cume_dist", "ntile", "lag", "lead",
	"first_value", "last_value", "nth_value",
	// strings
	"lower", "upper", "length", "char_length", "character_length", "octet_length", "bit_length",
	"substring", "substr", "trim", "btrim", "ltrim", "rtrim", "replace", "concat", "concat_ws",
	"left", "right", "lpad", "rpad", "position", "strpos", "split_part", "initcap", "md5", "repeat",
	"reverse", "translate", "starts_with", "regexp_replace", "regexp_match", "regexp_matches",
	"regexp_split_to_array", "regexp_split_to_table", "regexp_like", "regexp_count", "ascii", "chr", "to_hex",
	// numbers
	"abs", "ceil", "ceiling", "floor", "round", "trunc", "mod", "power", "pow", "sqrt", "cbrt", "exp",
	"ln", "log", "log10", "sign", "div", "pi", "degrees", "radians", "width_bucket", "random",
	// dates and times
	"now", "clock_timestamp", "statement_timestamp", "transaction_timestamp", "date_trunc", "date_part",
	"date_bin", "age", "to_char", "to_date", "to_timestamp", "to_number", "make_date", "make_time",
	"make_timestamp", "make_timestamptz", "make_interval", "justify_days", "justify_hours",
	"justify_interval", "isfinite", "timezone",
	// json
	"to_json", "to_jsonb", "row_to_json", "array_to_json", "json_build_object", "jsonb_build_object",
	"json_build_array", "jsonb_build_array", "json_extract_path", "json_extract_path_text",
	"jsonb_extract_path", "jsonb_extract_path_text", "json_array_length", "jsonb_array_length",
	"json_array_elements", "jsonb_array_elements", "json_array_elements_text", "jsonb_array_elements_text",
	"json_each", "jsonb_each", "json_each_text", "jsonb_each_text", "json_object_keys", "jsonb_object_keys",
	"json_typeof", "jsonb_typeof", "jsonb_set", "jsonb_insert", "jsonb_strip_nulls", "jsonb_pretty",
	"jsonb_path_exists", "jsonb_path_match", "jsonb_path_query", "jsonb_path_query_array", "jsonb_path_query_first",
	// arrays and series
	"array_length", "array_upper", "array_lower", "array_ndims", "cardinality", "unnest", "array_position",
	"array_positions", "array_remove", "array_replace", "array_append", "array_prepend", "array_cat",
	"array_to_string", "string_to_array", "generate_series", "generate_subscripts",
	// keywords and type names that deparse like calls
	"coalesce", "nullif", "greatest", "least", "row", "exists", "array", "extract", "cast",
	"character", "varying", "varchar", "char", "numeric", "decimal", "bit", "time", "timestamp", "interval", "float",
)

func toSet(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

// opaqueCalls returns the functions a deparsed plan expression calls that are not known to be safe. An expression it cannot read, such as one with an unterminated string literal, is reported as an opaque call itself, so anything unexpected fails closed.
func opaqueCalls(expr string) []string {
	stripped, ok := stripLiterals(expr)
	if !ok {
		return []string{"<unreadable expression>"}
	}
	var calls []string
	for _, m := range callPattern.FindAllStringSubmatch(stripped, -1) {
		if !isSafeFunction(m[1]) {
			calls = append(calls, m[1])
		}
	}
	return calls
}

// isSafeFunction reports whether a called name is an allowlisted built-in. Unquoted names are case-insensitive and may be qualified with pg_catalog; quoted names must match exactly, and any other schema means a function someone created.
func isSafeFunction(name string) bool {
	if rest, ok := strings.CutPrefix(name, "pg_catalog."); ok {
		name = rest
	}
	if strings.Contains(name, ".") {
		return false
	}
	if unquoted, ok := strings.CutPrefix(name, `"`); ok {
		return safeFunctions[strings.TrimSuffix(unquoted, `"`)]
	}
	return safeFunctions[strings.ToLower(name)]
}

// stripLiterals removes the contents of single-quoted string literals, which may contain anything including text that looks like a call, and reports false if a literal is not terminated. E'...' literals honour backslash escapes.
func stripLiterals(expr string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		if c != '\'' {
			b.WriteByte(c)
			continue
		}
		escapes := i > 0 && (expr[i-1] == 'E' || expr[i-1] == 'e') && (i < 2 || !isIdentByte(expr[i-2]))
		b.WriteString("''")
		closed := false
		for i++; i < len(expr); i++ {
			if escapes && expr[i] == '\\' {
				i++
				continue
			}
			if expr[i] == '\'' {
				if i+1 < len(expr) && expr[i+1] == '\'' {
					i++
					continue
				}
				closed = true
				break
			}
		}
		if !closed {
			return "", false
		}
	}
	return b.String(), true
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
