package mysql

import (
	"regexp"
	"strings"
)

// callPattern finds a function call in a statement: a name, optionally
// schema-qualified or backtick-quoted, immediately followed by an opening
// parenthesis. A stored routine call requires no space before the
// parenthesis unless the IGNORE_SPACE SQL mode is set, so a call written
// with a space is missed here the same way it is by MySQL's own default
// parsing.
var callPattern = regexp.MustCompile("((?:`(?:[^`]|``)+`|[A-Za-z_][A-Za-z0-9_$]*)(?:\\.(?:`(?:[^`]|``)+`|[A-Za-z_][A-Za-z0-9_$]*))*)\\(")

// safeFunctions are built-ins that compute from their arguments alone. None
// of them reads a table or a file, so a statement calling only these reads
// nothing beyond the tables in its plan. Anything not listed, including
// every stored function, is treated as able to read data the plan cannot
// see, because a stored function's body can run a query of its own.
var safeFunctions = toSet(
	// aggregates and window functions
	"count", "sum", "avg", "min", "max", "group_concat", "std", "stddev", "stddev_pop", "stddev_samp",
	"variance", "var_pop", "var_samp", "bit_and", "bit_or", "bit_xor", "json_arrayagg", "json_objectagg",
	"row_number", "rank", "dense_rank", "percent_rank", "cume_dist", "ntile", "lag", "lead",
	"first_value", "last_value", "nth_value", "grouping",
	// strings
	"lower", "upper", "length", "char_length", "character_length", "octet_length", "bit_length",
	"substring", "substr", "mid", "trim", "ltrim", "rtrim", "replace", "concat", "concat_ws",
	"left", "right", "lpad", "rpad", "locate", "position", "instr", "strcmp", "format", "reverse",
	"repeat", "space", "ascii", "ord", "char", "hex", "unhex", "md5", "sha1", "sha2", "soundex",
	"elt", "field", "insert", "quote", "to_base64", "from_base64",
	// numbers
	"abs", "ceil", "ceiling", "floor", "round", "truncate", "mod", "pow", "power", "sqrt", "exp",
	"ln", "log", "log2", "log10", "sign", "pi", "degrees", "radians", "rand", "greatest", "least", "conv",
	// dates and times
	"now", "curdate", "current_date", "curtime", "current_time", "current_timestamp", "sysdate",
	"date", "time", "year", "month", "day", "dayofweek", "dayofyear", "dayofmonth", "dayname",
	"monthname", "week", "weekday", "weekofyear", "quarter", "hour", "minute", "second", "microsecond",
	"date_add", "date_sub", "adddate", "subdate", "datediff", "timediff", "timestampdiff", "timestampadd",
	"date_format", "time_format", "str_to_date", "from_unixtime", "unix_timestamp", "last_day", "extract",
	"period_add", "period_diff", "to_days", "from_days", "makedate", "maketime", "sec_to_time", "time_to_sec",
	"utc_date", "utc_time", "utc_timestamp",
	// json
	"json_extract", "json_unquote", "json_object", "json_array", "json_array_append", "json_array_insert",
	"json_insert", "json_replace", "json_set", "json_remove", "json_merge", "json_merge_patch",
	"json_merge_preserve", "json_type", "json_valid", "json_depth", "json_length", "json_keys",
	"json_contains", "json_contains_path", "json_search", "json_quote", "json_pretty", "json_overlaps",
	"json_schema_valid",
	// keywords and type conversions that deparse like calls
	"if", "ifnull", "nullif", "coalesce", "isnull", "cast", "convert", "binary", "row", "uuid",
	"crc32", "compress", "uncompress", "inet_aton", "inet_ntoa", "inet6_aton", "inet6_ntoa", "is_ipv4", "is_ipv6",
)

func toSet(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

// opaqueCalls returns the statement's calls to functions that are not known
// to be safe, scanning the statement text itself rather than the plan,
// because MySQL's EXPLAIN FORMAT=JSON does not surface a function used only
// in the select list the way it surfaces one used in a condition.
func opaqueCalls(statement string) []string {
	masked := maskLiteralsAndComments(statement)
	var calls []string
	seen := map[string]bool{}
	for _, m := range callPattern.FindAllStringSubmatch(masked, -1) {
		if isSafeFunction(m[1]) || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		calls = append(calls, m[1])
	}
	return calls
}

// isSafeFunction reports whether a called name is an allowlisted built-in.
// A schema-qualified call is always someone's stored function, since no
// built-in is called that way, and a stored routine name is not case
// sensitive, unlike a MySQL table name can be, so both quoted and unquoted
// forms are compared in lower case.
func isSafeFunction(name string) bool {
	if strings.Contains(name, ".") {
		return false
	}
	name = strings.ReplaceAll(strings.Trim(name, "`"), "``", "`")
	return safeFunctions[strings.ToLower(name)]
}
