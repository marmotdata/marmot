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
