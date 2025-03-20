package query

import (
	"fmt"
	"strings"
	"unicode"
)

// Builder handles converting queries into SQL
type Builder struct{}

// NewBuilder creates a new query builder
func NewBuilder() *Builder {
	return &Builder{}
}

// BuildSQL converts a Query into a SQL query with parameters
func (b *Builder) BuildSQL(q *Query, baseQuery string) (string, []interface{}, error) {
	var conditions []string
	var params []interface{}

	params = append(params, "")
	paramCount := 2

	// Handle boolean conditions first
	if q.Bool != nil {
		boolConditions, boolParams, newParamCount, err := b.buildBooleanConditions(q.Bool, paramCount)
		if err != nil {
			return "", nil, err
		}
		if len(boolConditions) > 0 {
			conditions = append(conditions, strings.Join(boolConditions, " AND "))
		}
		params = append(params, boolParams...)
		paramCount = newParamCount
	}

	// Then handle free text search
	if q.FreeText != "" {
		conditions = append(conditions, fmt.Sprintf("(search_text @@ websearch_to_tsquery('english', $%d) OR similarity(name, $%d) > 0.3)", paramCount, paramCount))
		params = append(params, q.FreeText)
		paramCount++
	}

	// Build the complete query
	var query string
	baseQuery = strings.TrimRight(strings.TrimSpace(baseQuery), ")")

	if len(conditions) > 0 {
		query = fmt.Sprintf("%s WHERE %s) SELECT * FROM search_results ORDER BY search_rank DESC", baseQuery, strings.Join(conditions, " AND "))
	} else {
		query = fmt.Sprintf("%s) SELECT * FROM search_results ORDER BY search_rank DESC", baseQuery)
	}

	return query, params, nil
}

func (b *Builder) BuildConditions(bq *BooleanQuery) ([]string, []interface{}, error) {
	conditions, params, _, err := b.buildBooleanConditions(bq, 1)
	return conditions, params, err
}

// buildBooleanConditions creates SQL conditions for boolean queries
func (b *Builder) buildBooleanConditions(bq *BooleanQuery, paramCount int) ([]string, []interface{}, int, error) {
	var conditions []string
	var params []interface{}

	// Handle nested queries and regular must conditions
	for _, filter := range bq.Must {
		if nestedBool, ok := filter.Value.(*BooleanQuery); ok {
			nestedConds, nestedParams, newCount, err := b.buildBooleanConditions(nestedBool, paramCount)
			if err != nil {
				return nil, nil, paramCount, err
			}
			conditions = append(conditions, fmt.Sprintf("(%s)", strings.Join(nestedConds, " AND ")))
			params = append(params, nestedParams...)
			paramCount = newCount
		} else {
			cond, filterParams, newCount, err := b.buildFilterCondition(filter, paramCount)
			if err != nil {
				return nil, nil, paramCount, err
			}
			conditions = append(conditions, cond)
			params = append(params, filterParams...)
			paramCount = newCount
		}
	}

	// If we have any conditions and Should/MustNot, wrap existing conditions
	if len(conditions) > 0 && (len(bq.Should) > 0 || len(bq.MustNot) > 0) {
		conditions = []string{fmt.Sprintf("(%s)", strings.Join(conditions, " AND "))}
	}

	// Handle OR conditions
	for _, filter := range bq.Should {
		cond, filterParams, newCount, err := b.buildFilterCondition(filter, paramCount)
		if err != nil {
			return nil, nil, paramCount, err
		}
		if len(conditions) > 0 {
			conditions[0] = fmt.Sprintf("%s OR %s", conditions[0], cond)
		} else {
			conditions = append(conditions, cond)
		}
		params = append(params, filterParams...)
		paramCount = newCount
	}

	// Handle NOT conditions
	for _, filter := range bq.MustNot {
		cond, filterParams, newCount, err := b.buildFilterCondition(filter, paramCount)
		if err != nil {
			return nil, nil, paramCount, err
		}
		if len(conditions) > 0 {
			conditions[0] = fmt.Sprintf("%s OR NOT (%s)", conditions[0], cond)
		} else {
			conditions = append(conditions, fmt.Sprintf("NOT (%s)", cond))
		}
		params = append(params, filterParams...)
		paramCount = newCount
	}

	return conditions, params, paramCount, nil
}

// buildFilterCondition creates SQL condition for a single filter
func (b *Builder) buildFilterCondition(filter Filter, paramCount int) (string, []interface{}, int, error) {
	var condition string
	var params []interface{}

	// Validate field names
	for _, field := range filter.Field {
		if !isValidIdentifier(field) {
			return "", nil, paramCount, fmt.Errorf("invalid field name: %s", field)
		}
	}

	// Build JSON path with validated field names
	jsonPath := ""
	if len(filter.Field) > 1 {
		for i, field := range filter.Field[:len(filter.Field)-1] {
			if i > 0 {
				jsonPath += "->"
			}
			jsonPath += fmt.Sprintf("'%s'", field)
		}
		jsonPath = fmt.Sprintf("->%s->>'%s'", jsonPath, filter.Field[len(filter.Field)-1])
	} else {
		jsonPath = fmt.Sprintf("->>'%s'", filter.Field[0])
	}

	if filter.Field[0] == "freetext" {
		condition = fmt.Sprintf("(search_text @@ websearch_to_tsquery('english', $%d) OR similarity(name, $%d) > 0.3)", paramCount, paramCount)
		params = append(params, filter.Value)
		return condition, params, paramCount + 1, nil
	}

	switch filter.Operator {
	case OpWildcard:
		condition = fmt.Sprintf("metadata%s LIKE $%d", jsonPath, paramCount)
		value := strings.ReplaceAll(fmt.Sprintf("%v", filter.Value), "*", "%")
		params = append(params, value)

	case OpEquals:
		if boolQuery, ok := filter.Value.(*BooleanQuery); ok {
			conditions, nestedParams, newParamCount, err := b.buildBooleanConditions(boolQuery, paramCount)
			if err != nil {
				return "", nil, paramCount, err
			}
			return fmt.Sprintf("(%s)", strings.Join(conditions, " AND ")), nestedParams, newParamCount, nil
		}
		condition = fmt.Sprintf("metadata%s = $%d", jsonPath, paramCount)
		params = append(params, filter.Value)

	case OpContains:
		condition = fmt.Sprintf("metadata%s ILIKE $%d", jsonPath, paramCount)
		params = append(params, fmt.Sprintf("%%%v%%", filter.Value))

	case OpNotEquals:
		condition = fmt.Sprintf("metadata%s != $%d", jsonPath, paramCount)
		params = append(params, filter.Value)

	case OpGreater:
		condition = fmt.Sprintf("(metadata%s)::numeric > $%d", jsonPath, paramCount)
		params = append(params, fmt.Sprintf("%v", filter.Value))

	case OpLess:
		condition = fmt.Sprintf("(metadata%s)::numeric < $%d", jsonPath, paramCount)
		params = append(params, fmt.Sprintf("%v", filter.Value))

	case OpGreaterEqual:
		condition = fmt.Sprintf("(metadata%s)::numeric >= $%d", jsonPath, paramCount)
		params = append(params, fmt.Sprintf("%v", filter.Value))

	case OpLessEqual:
		condition = fmt.Sprintf("(metadata%s)::numeric <= $%d", jsonPath, paramCount)
		params = append(params, fmt.Sprintf("%v", filter.Value))

	case OpRange:
		if filter.Range == nil {
			return "", nil, paramCount, fmt.Errorf("range values missing")
		}
		condition = fmt.Sprintf("(metadata%s)::numeric >= $%d AND (metadata%s)::numeric <= $%d",
			jsonPath, paramCount, jsonPath, paramCount+1)
		params = append(params, filter.Range.From, filter.Range.To)
		return condition, params, paramCount + 2, nil

	default:
		return "", nil, paramCount, fmt.Errorf("unsupported operator: %s", filter.Operator)
	}

	return condition, params, paramCount + 1, nil
}

// isValidIdentifier checks if a field name contains only allowed characters
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}

	// Only allow alphanumeric characters and underscores
	// Can be adjusted to allow more characters if needed
	for _, char := range s {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '_' {
			return false
		}
	}
	return true
}
