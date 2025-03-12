package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
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

func parseRelativeTime(value string) (time.Time, error) {
	if value == "now" {
		return time.Now(), nil
	}

	re := regexp.MustCompile(`now([+-])(\d+)([hdwmy])`)
	matches := re.FindStringSubmatch(value)
	if matches == nil {
		return time.Time{}, fmt.Errorf("invalid relative time format: %s", value)
	}

	operator := matches[1]
	amount, _ := strconv.Atoi(matches[2])
	unit := matches[3]

	now := time.Now()
	var duration time.Duration

	switch unit {
	case "h":
		duration = time.Hour * time.Duration(amount)
	case "d":
		duration = time.Hour * 24 * time.Duration(amount)
	case "w":
		duration = time.Hour * 24 * 7 * time.Duration(amount)
	case "m":
		duration = time.Hour * 24 * 30 * time.Duration(amount)
	case "y":
		duration = time.Hour * 24 * 365 * time.Duration(amount)
	default:
		return time.Time{}, fmt.Errorf("invalid time unit: %s", unit)
	}

	if operator == "+" {
		return now.Add(duration), nil
	}
	return now.Add(-duration), nil
}
