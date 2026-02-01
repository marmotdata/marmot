package query

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// TableConfig specifies column names for different tables
type TableConfig struct {
	TypeColumn     string // "type" for assets, "asset_type" for search_index
	ProviderColumn string // "providers" for both
	NameColumn     string // "name" for both
	MetadataColumn string // "metadata" for both
}

// DefaultAssetsConfig returns column config for the assets table
func DefaultAssetsConfig() TableConfig {
	return TableConfig{
		TypeColumn:     "type",
		ProviderColumn: "providers",
		NameColumn:     "name",
		MetadataColumn: "metadata",
	}
}

// SearchIndexConfig returns column config for the search_index table
func SearchIndexConfig() TableConfig {
	return TableConfig{
		TypeColumn:     "asset_type",
		ProviderColumn: "providers",
		NameColumn:     "name",
		MetadataColumn: "metadata",
	}
}

// Builder handles converting queries into SQL
type Builder struct {
	config TableConfig
}

// NewBuilder creates a new query builder with default assets table config
func NewBuilder() *Builder {
	return &Builder{config: DefaultAssetsConfig()}
}

// NewSearchIndexBuilder creates a builder optimized for search_index table
func NewSearchIndexBuilder() *Builder {
	return &Builder{config: SearchIndexConfig()}
}

// BuildSQL converts a Query into a SQL query with parameters
func (b *Builder) BuildSQL(q *Query, baseQuery string) (string, []interface{}, error) {
	var conditions []string
	var params []interface{}

	// Add placeholder param for $1 (used by baseQuery for search ranking)
	params = append(params, "")
	paramCount := 1 // $1 is now used

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
		paramCount++
		conditions = append(conditions, fmt.Sprintf("(search_text @@ websearch_to_tsquery('english', $%d) OR similarity(name, $%d) > 0.3)", paramCount, paramCount))
		params = append(params, q.FreeText)
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
	// Pass 0 meaning "no params used yet" - first filter will use $1
	conditions, params, _, err := b.buildBooleanConditions(bq, 0)
	return conditions, params, err
}

// BuildSearchConditions builds WHERE clause conditions from a Query struct.
// Returns conditions, params, next param index, and any error.
// This is designed for integration with custom query builders like the search store.
func (b *Builder) BuildSearchConditions(q *Query, startParam int) ([]string, []interface{}, int, error) {
	var conditions []string
	var params []interface{}
	paramCount := startParam

	// Handle boolean conditions
	if q.Bool != nil {
		boolConditions, boolParams, newParamCount, err := b.buildBooleanConditions(q.Bool, paramCount)
		if err != nil {
			return nil, nil, paramCount, err
		}
		if len(boolConditions) > 0 {
			conditions = append(conditions, boolConditions...)
		}
		params = append(params, boolParams...)
		paramCount = newParamCount
	}

	return conditions, params, paramCount, nil
}

// GetFreeText returns the free text portion of the query
func (q *Query) GetFreeText() string {
	return q.FreeText
}

// HasStructuredFilters returns true if the query has any structured filters
func (q *Query) HasStructuredFilters() bool {
	return q.Bool != nil && (len(q.Bool.Must) > 0 || len(q.Bool.Should) > 0 || len(q.Bool.MustNot) > 0)
}

// CanUseCompositeIndex checks if the query can efficiently use a composite index.
// Returns true if it's a simple single-field exact match on type or provider.
func (q *Query) CanUseCompositeIndex() bool {
	if q.Bool == nil {
		return false
	}

	// Must have exactly one Must condition and no Should/MustNot
	if len(q.Bool.Must) != 1 || len(q.Bool.Should) > 0 || len(q.Bool.MustNot) > 0 {
		return false
	}

	filter := q.Bool.Must[0]

	// Check for nested boolean (can't use composite index)
	if _, ok := filter.Value.(*BooleanQuery); ok {
		return false
	}

	// Only exact match on type or provider can use composite index
	if filter.Operator != OpEquals {
		return false
	}

	// Check for wildcards in value
	if strVal, ok := filter.Value.(string); ok && strings.Contains(strVal, "*") {
		return false
	}

	return filter.FieldType == FieldAssetType || filter.FieldType == FieldProvider
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

// buildFilterCondition creates SQL condition for a single filter.
// Uses "increment first, then use" semantic to match buildFilterClauses pattern.
// paramCount represents the last used param index; returns the new last used index.
func (b *Builder) buildFilterCondition(filter Filter, paramCount int) (string, []interface{}, int, error) {
	var condition string
	var params []interface{}

	// Validate field names
	for _, field := range filter.Field {
		if !isValidIdentifier(field) {
			return "", nil, paramCount, fmt.Errorf("invalid field name: %s", field)
		}
	}

	// Skip @kind filters - they're used for table selection, not WHERE clauses
	// Don't increment paramCount since we're not using a parameter
	if filter.FieldType == FieldKind {
		return "TRUE", nil, paramCount, nil
	}

	// Handle nested boolean queries before incrementing - they handle their own params
	if filter.Operator == OpEquals {
		if boolQuery, ok := filter.Value.(*BooleanQuery); ok {
			conditions, nestedParams, newParamCount, err := b.buildBooleanConditions(boolQuery, paramCount)
			if err != nil {
				return "", nil, paramCount, err
			}
			return fmt.Sprintf("(%s)", strings.Join(conditions, " AND ")), nestedParams, newParamCount, nil
		}
	}

	// Increment first to get the next available param index
	paramCount++

	// Handle special case for freetext
	if filter.Field[0] == "freetext" {
		condition = fmt.Sprintf("(search_text @@ websearch_to_tsquery('english', $%d) OR similarity(name, $%d) > 0.3)", paramCount, paramCount)
		params = append(params, filter.Value)
		return condition, params, paramCount, nil
	}

	// Build column reference based on field type
	var columnRef string
	switch filter.FieldType {
	case FieldAssetType:
		// @type queries the type column (asset_type for search_index)
		columnRef = b.config.TypeColumn
	case FieldProvider:
		// @provider queries the providers array column
		columnRef = b.config.ProviderColumn
	case FieldName:
		// @name queries the name column directly
		columnRef = b.config.NameColumn
	case FieldMetadata:
		// @metadata.* queries the metadata JSONB column
		// Build JSON path with validated field names
		if len(filter.Field) > 1 {
			jsonPath := ""
			for i, field := range filter.Field[:len(filter.Field)-1] {
				if i > 0 {
					jsonPath += "->"
				}
				jsonPath += fmt.Sprintf("'%s'", field)
			}
			columnRef = fmt.Sprintf("%s->%s->>'%s'", b.config.MetadataColumn, jsonPath, filter.Field[len(filter.Field)-1])
		} else {
			columnRef = fmt.Sprintf("%s->>'%s'", b.config.MetadataColumn, filter.Field[0])
		}
	default:
		return "", nil, paramCount, fmt.Errorf("unsupported field type: %s", filter.FieldType)
	}

	// Handle operators differently based on field type
	switch filter.Operator {
	case OpWildcard:
		if filter.FieldType == FieldProvider {
			return "", nil, paramCount, fmt.Errorf("wildcard operator not supported for provider fields")
		}
		value := strings.ReplaceAll(fmt.Sprintf("%v", filter.Value), "*", "%")
		// Use ILIKE for case-insensitive wildcard matching
		condition = fmt.Sprintf("%s ILIKE $%d", columnRef, paramCount)
		params = append(params, value)

	case OpEquals:
		// Note: nested BooleanQuery is handled before the increment above
		switch {
		case filter.FieldType == FieldProvider:
			condition = fmt.Sprintf("%s && ARRAY[$%d]::text[]", columnRef, paramCount)
			params = append(params, filter.Value)
		case filter.FieldType == FieldAssetType:
			// Use lower() for case-insensitive match with functional index
			condition = fmt.Sprintf("lower(%s) = lower($%d)", columnRef, paramCount)
			params = append(params, filter.Value)
		case filter.FieldType == FieldName:
			// Use lower() for case-insensitive match with functional index
			condition = fmt.Sprintf("lower(%s) = lower($%d)", columnRef, paramCount)
			params = append(params, filter.Value)
		case filter.FieldType == FieldKind:
			// Kind is handled at table selection level
			condition = "TRUE"
			return condition, nil, paramCount, nil
		case filter.FieldType == FieldMetadata:
			// Use JSONB containment for GIN jsonb_path_ops index
			jsonbValue := buildNestedJSONB(filter.Field, filter.Value)
			condition = fmt.Sprintf("%s @> $%d::jsonb", b.config.MetadataColumn, paramCount)
			params = append(params, jsonbValue)
		default:
			condition = fmt.Sprintf("%s = $%d", columnRef, paramCount)
			params = append(params, filter.Value)
		}

	case OpContains:
		if filter.FieldType == FieldProvider {
			// For array fields, check if any element contains the value (must use unnest for partial match)
			condition = fmt.Sprintf("EXISTS (SELECT 1 FROM unnest(%s) AS elem WHERE lower(elem) LIKE lower($%d))", columnRef, paramCount)
			params = append(params, fmt.Sprintf("%%%v%%", filter.Value))
		} else {
			// Use ILIKE for partial matching (can't use functional index for contains)
			condition = fmt.Sprintf("%s ILIKE $%d", columnRef, paramCount)
			params = append(params, fmt.Sprintf("%%%v%%", filter.Value))
		}

	case OpNotEquals:
		switch {
		case filter.FieldType == FieldProvider:
			// Use negated array overlap for GIN index compatibility
			condition = fmt.Sprintf("NOT (%s && ARRAY[$%d]::text[])", columnRef, paramCount)
			params = append(params, filter.Value)
		case filter.FieldType == FieldAssetType:
			// Use lower() for case-insensitive not-equal with null handling
			condition = fmt.Sprintf("(%s IS NULL OR lower(%s) != lower($%d))", columnRef, columnRef, paramCount)
			params = append(params, filter.Value)
		case filter.FieldType == FieldName:
			// Use lower() for case-insensitive not-equal
			condition = fmt.Sprintf("lower(%s) != lower($%d)", columnRef, paramCount)
			params = append(params, filter.Value)
		case filter.FieldType == FieldKind:
			condition = "TRUE"
			return condition, nil, paramCount, nil
		default:
			// For metadata fields, use exact !=
			condition = fmt.Sprintf("%s != $%d", columnRef, paramCount)
			params = append(params, filter.Value)
		}

	case OpGreater:
		if filter.FieldType == FieldProvider {
			return "", nil, paramCount, fmt.Errorf("comparison operators not supported for provider fields")
		}
		condition = fmt.Sprintf("(%s)::numeric > $%d::numeric", columnRef, paramCount)
		params = append(params, fmt.Sprintf("%v", filter.Value))

	case OpLess:
		if filter.FieldType == FieldProvider {
			return "", nil, paramCount, fmt.Errorf("comparison operators not supported for provider fields")
		}
		condition = fmt.Sprintf("(%s)::numeric < $%d::numeric", columnRef, paramCount)
		params = append(params, fmt.Sprintf("%v", filter.Value))

	case OpGreaterEqual:
		if filter.FieldType == FieldProvider {
			return "", nil, paramCount, fmt.Errorf("comparison operators not supported for provider fields")
		}
		condition = fmt.Sprintf("(%s)::numeric >= $%d::numeric", columnRef, paramCount)
		params = append(params, fmt.Sprintf("%v", filter.Value))

	case OpLessEqual:
		if filter.FieldType == FieldProvider {
			return "", nil, paramCount, fmt.Errorf("comparison operators not supported for provider fields")
		}
		condition = fmt.Sprintf("(%s)::numeric <= $%d::numeric", columnRef, paramCount)
		params = append(params, fmt.Sprintf("%v", filter.Value))

	case OpRange:
		if filter.FieldType == FieldProvider {
			return "", nil, paramCount, fmt.Errorf("range operator not supported for provider fields")
		}
		if filter.Range == nil {
			return "", nil, paramCount, fmt.Errorf("range values missing")
		}
		// Range needs two parameters - use current and next
		condition = fmt.Sprintf("(%s)::numeric >= $%d::numeric AND (%s)::numeric <= $%d::numeric",
			columnRef, paramCount, columnRef, paramCount+1)
		params = append(params, filter.Range.From, filter.Range.To)
		return condition, params, paramCount + 1, nil

	default:
		return "", nil, paramCount, fmt.Errorf("unsupported operator: %s", filter.Operator)
	}

	return condition, params, paramCount, nil
}

// isValidIdentifier checks if a field name contains only allowed characters
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}

	// Only allow alphanumeric characters and underscores
	for _, char := range s {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '_' {
			return false
		}
	}
	return true
}

// buildNestedJSONB creates a nested JSONB structure from a path and value.
// For example, path ["cloud", "accountId"] with value "123" becomes:
// {"cloud": {"accountId": "123"}}
// String values that look like numbers are converted to numbers for proper JSONB matching.
func buildNestedJSONB(path []string, value interface{}) string {
	if len(path) == 0 {
		return "{}"
	}

	// Build the nested structure from inside out
	result := make(map[string]interface{})
	current := result

	for i := 0; i < len(path)-1; i++ {
		next := make(map[string]interface{})
		current[path[i]] = next
		current = next
	}

	// Convert string values to appropriate types for proper JSONB matching
	finalValue := value
	if strVal, ok := value.(string); ok {
		finalValue = parseJSONValue(strVal)
	}

	current[path[len(path)-1]] = finalValue

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "{}"
	}

	return string(jsonBytes)
}

// parseJSONValue attempts to convert a string to its native JSON type.
// Returns float64 for numbers, bool for true/false, or the original string.
func parseJSONValue(s string) interface{} {
	// Try parsing as float (handles integers too)
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	// Try parsing as bool
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	// Return as string
	return s
}
