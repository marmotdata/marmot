package query

import (
	"fmt"
	"strings"
)

// Helper functions for parsing and validation
func isBooleanOperator(token string) bool {
	token = strings.ToUpper(token)
	return token == "AND" || token == "OR" || token == "NOT"
}

func parseOperator(op string) (Operator, error) {
	switch strings.ToLower(op) {
	case ":", "=", "==":
		return OpEquals, nil
	case "contains":
		return OpContains, nil
	case "!=", "<>":
		return OpNotEquals, nil
	case ">":
		return OpGreater, nil
	case "<":
		return OpLess, nil
	case ">=":
		return OpGreaterEqual, nil
	case "<=":
		return OpLessEqual, nil
	case "in":
		return OpIn, nil
	case "not", "not in":
		return OpNotIn, nil
	case "range":
		return OpRange, nil
	case "~", "like":
		return OpWildcard, nil
	default:
		return "", fmt.Errorf("unknown operator: %q", op)
	}
}

// MetadataColumnRef builds a JSONB accessor on the metadata column for a dot-separated field path, escaping each key as a SQL string literal so user-supplied paths cannot break out of the expression.
func MetadataColumnRef(field string) string {
	path := strings.Split(field, ".")
	keys := make([]string, len(path))
	for i, key := range path {
		keys[i] = QuoteLiteral(key)
	}
	if len(keys) == 1 {
		return "metadata->>" + keys[0]
	}
	return "metadata->" + strings.Join(keys[:len(keys)-1], "->") + "->>" + keys[len(keys)-1]
}

// QuoteLiteral quotes s as a PostgreSQL string literal by doubling embedded single quotes, which is safe because pgx requires standard_conforming_strings to be on.
func QuoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
