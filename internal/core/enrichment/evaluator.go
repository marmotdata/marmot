package enrichment

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/query"
)

type RuleType string

const (
	RuleTypeQuery         RuleType = "query"
	RuleTypeMetadataMatch RuleType = "metadata_match"
)

const (
	PatternTypeExact    = "exact"
	PatternTypeWildcard = "wildcard"
	PatternTypeRegex    = "regex"
	PatternTypePrefix   = "prefix"
)

// EnrichmentRule is the common interface implemented by asset rules.
type EnrichmentRule interface {
	GetID() string
	GetRuleType() RuleType
	GetQueryExpression() *string
	GetMetadataField() *string
	GetPatternType() *string
	GetPatternValue() *string
	GetIsEnabled() bool
}

type Evaluator struct {
	db *pgxpool.Pool
}

// NewEvaluator creates a new rule evaluator.
func NewEvaluator(db *pgxpool.Pool) *Evaluator {
	return &Evaluator{db: db}
}

// ExecuteRule runs a rule and returns matching asset IDs.
func (e *Evaluator) ExecuteRule(ctx context.Context, rule EnrichmentRule) ([]string, error) {
	switch {
	case rule.GetRuleType() == RuleTypeQuery && rule.GetQueryExpression() != nil:
		return e.executeQueryRule(ctx, *rule.GetQueryExpression())
	case rule.GetRuleType() == RuleTypeMetadataMatch:
		return e.executeMetadataMatchRule(ctx, rule)
	default:
		return nil, fmt.Errorf("unsupported rule type: %s", rule.GetRuleType())
	}
}

// EvaluateRuleForAsset checks if a specific asset matches a rule.
func (e *Evaluator) EvaluateRuleForAsset(ctx context.Context, rule EnrichmentRule, assetID string) (bool, error) {
	if rule.GetRuleType() == RuleTypeQuery && rule.GetQueryExpression() != nil {
		return e.evaluateQueryRuleForAsset(ctx, *rule.GetQueryExpression(), assetID)
	}
	if rule.GetRuleType() == RuleTypeMetadataMatch {
		return e.evaluateMetadataRuleForAsset(ctx, rule, assetID)
	}
	return false, fmt.Errorf("unsupported rule type: %s", rule.GetRuleType())
}

func (e *Evaluator) executeQueryRule(ctx context.Context, queryExpression string) ([]string, error) {
	parser := query.NewParser()
	builder := query.NewBuilder()

	parsedQuery, err := parser.Parse(queryExpression)
	if err != nil {
		return nil, fmt.Errorf("parsing query: %w", err)
	}

	baseQuery := `WITH search_results AS (SELECT id, 1.0 as search_rank FROM assets`
	sqlQuery, queryParams, err := builder.BuildSQL(parsedQuery, baseQuery)
	if err != nil {
		return nil, fmt.Errorf("building SQL: %w", err)
	}

	sqlQuery = strings.Replace(sqlQuery,
		") SELECT * FROM search_results",
		" AND is_stub = FALSE) SELECT id, search_rank FROM search_results",
		1)

	if !strings.Contains(sqlQuery, "WHERE") {
		sqlQuery = strings.Replace(sqlQuery,
			" AND is_stub = FALSE)",
			" WHERE is_stub = FALSE)",
			1)
	}

	sqlQuery = RenumberParameters(sqlQuery)

	var params []interface{}
	if len(queryParams) > 1 {
		params = queryParams[1:]
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := e.db.Query(ctx, sqlQuery, params...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	var assetIDs []string
	for rows.Next() {
		var id string
		var rank float64
		if err := rows.Scan(&id, &rank); err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}
		assetIDs = append(assetIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating results: %w", err)
	}

	return assetIDs, nil
}

func (e *Evaluator) executeMetadataMatchRule(ctx context.Context, rule EnrichmentRule) ([]string, error) {
	metadataField := rule.GetMetadataField()
	patternType := rule.GetPatternType()
	patternValue := rule.GetPatternValue()

	if metadataField == nil || patternType == nil || patternValue == nil {
		return nil, fmt.Errorf("metadata match rule missing required fields")
	}

	columnRef := BuildMetadataColumnRef(*metadataField)
	condition, args, err := BuildPatternCondition(columnRef, *patternType, *patternValue, 1)
	if err != nil {
		return nil, err
	}

	q := fmt.Sprintf("SELECT id FROM assets WHERE is_stub = FALSE AND %s", condition)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := e.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("executing metadata query: %w", err)
	}
	defer rows.Close()

	var assetIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}
		assetIDs = append(assetIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating results: %w", err)
	}

	return assetIDs, nil
}

func (e *Evaluator) evaluateQueryRuleForAsset(ctx context.Context, queryExpression, assetID string) (bool, error) {
	parser := query.NewParser()
	builder := query.NewBuilder()

	parsedQuery, err := parser.Parse(queryExpression)
	if err != nil {
		return false, fmt.Errorf("parsing query: %w", err)
	}

	baseQuery := `WITH search_results AS (SELECT id, 1.0 as search_rank FROM assets`
	sqlQuery, queryParams, err := builder.BuildSQL(parsedQuery, baseQuery)
	if err != nil {
		return false, fmt.Errorf("building SQL: %w", err)
	}

	sqlQuery = strings.Replace(sqlQuery,
		") SELECT * FROM search_results",
		" AND is_stub = FALSE) SELECT id, search_rank FROM search_results",
		1)

	if !strings.Contains(sqlQuery, "WHERE") {
		sqlQuery = strings.Replace(sqlQuery,
			" AND is_stub = FALSE)",
			" WHERE is_stub = FALSE)",
			1)
	}

	sqlQuery = RenumberParameters(sqlQuery)

	var params []interface{}
	if len(queryParams) > 1 {
		params = queryParams[1:]
	}

	nextParam := len(params) + 1
	checkQuery := fmt.Sprintf(
		"SELECT EXISTS(SELECT 1 FROM (%s) AS results WHERE id = $%d)",
		sqlQuery, nextParam,
	)
	params = append(params, assetID)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var exists bool
	err = e.db.QueryRow(ctx, checkQuery, params...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("executing query: %w", err)
	}

	return exists, nil
}

func (e *Evaluator) evaluateMetadataRuleForAsset(ctx context.Context, rule EnrichmentRule, assetID string) (bool, error) {
	metadataField := rule.GetMetadataField()
	patternType := rule.GetPatternType()
	patternValue := rule.GetPatternValue()

	if metadataField == nil || patternType == nil || patternValue == nil {
		return false, fmt.Errorf("metadata match rule missing required fields")
	}

	columnRef := BuildMetadataColumnRef(*metadataField)
	condition, args, err := BuildPatternCondition(columnRef, *patternType, *patternValue, 2)
	if err != nil {
		return false, err
	}

	q := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM assets WHERE id = $1 AND is_stub = FALSE AND %s)", condition)
	allArgs := append([]interface{}{assetID}, args...)

	var exists bool
	err = e.db.QueryRow(ctx, q, allArgs...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("executing metadata query: %w", err)
	}

	return exists, nil
}

// EvaluateMetadataRuleInMemory checks if an asset matches a metadata rule without DB access.
func EvaluateMetadataRuleInMemory(rule EnrichmentRule, metadata map[string]interface{}) bool {
	metadataField := rule.GetMetadataField()
	patternType := rule.GetPatternType()
	patternValue := rule.GetPatternValue()

	if metadataField == nil || patternType == nil || patternValue == nil {
		return false
	}

	value := GetNestedMetadataValue(metadata, *metadataField)
	if value == nil {
		return false
	}

	strValue, ok := value.(string)
	if !ok {
		return false
	}

	switch *patternType {
	case PatternTypeExact:
		return strValue == *patternValue
	case PatternTypePrefix:
		return strings.HasPrefix(strValue, *patternValue)
	case PatternTypeWildcard:
		return MatchWildcard(*patternValue, strValue)
	case PatternTypeRegex:
		return false // let DB handle regex
	}

	return false
}

// BuildMetadataColumnRef builds a PostgreSQL column reference for a metadata field path.
func BuildMetadataColumnRef(field string) string {
	fieldPath := strings.Split(field, ".")
	if len(fieldPath) > 1 {
		jsonPath := ""
		for i, f := range fieldPath[:len(fieldPath)-1] {
			if i > 0 {
				jsonPath += "->"
			}
			jsonPath += fmt.Sprintf("'%s'", f)
		}
		return fmt.Sprintf("metadata->%s->>'%s'", jsonPath, fieldPath[len(fieldPath)-1])
	}
	return fmt.Sprintf("metadata->>'%s'", fieldPath[0])
}

// BuildPatternCondition builds a SQL condition for pattern matching.
// startParam is the first $N parameter number to use.
func BuildPatternCondition(columnRef, patternType, patternValue string, startParam int) (string, []interface{}, error) {
	switch patternType {
	case PatternTypeExact:
		return fmt.Sprintf("%s = $%d", columnRef, startParam), []interface{}{patternValue}, nil
	case PatternTypeWildcard:
		pattern := strings.ReplaceAll(patternValue, "*", "%")
		return fmt.Sprintf("%s ILIKE $%d", columnRef, startParam), []interface{}{pattern}, nil
	case PatternTypeRegex:
		if _, err := regexp.Compile(patternValue); err != nil {
			return "", nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
		return fmt.Sprintf("%s ~ $%d", columnRef, startParam), []interface{}{patternValue}, nil
	case PatternTypePrefix:
		return fmt.Sprintf("%s LIKE $%d", columnRef, startParam), []interface{}{patternValue + "%"}, nil
	default:
		return "", nil, fmt.Errorf("unsupported pattern type: %s", patternType)
	}
}

// RenumberParameters renumbers SQL parameters from $2, $3, ... to $1, $2, ...
func RenumberParameters(sql string) string {
	for i := 20; i >= 2; i-- {
		old := fmt.Sprintf("$%d", i)
		new := fmt.Sprintf("$%d", i-1)
		sql = strings.ReplaceAll(sql, old, new)
	}
	return sql
}

// GetNestedMetadataValue extracts a value from nested metadata using dot notation.
func GetNestedMetadataValue(metadata map[string]interface{}, field string) interface{} {
	parts := strings.Split(field, ".")
	var current interface{} = metadata

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}

	return current
}

// MatchWildcard performs simple wildcard matching (* = any characters).
func MatchWildcard(pattern, value string) bool {
	pattern = strings.ToLower(pattern)
	value = strings.ToLower(value)

	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == value
	}

	if parts[0] != "" && !strings.HasPrefix(value, parts[0]) {
		return false
	}

	lastPart := parts[len(parts)-1]
	if lastPart != "" && !strings.HasSuffix(value, lastPart) {
		return false
	}

	pos := len(parts[0])
	for i := 1; i < len(parts)-1; i++ {
		if parts[i] == "" {
			continue
		}
		idx := strings.Index(value[pos:], parts[i])
		if idx == -1 {
			return false
		}
		pos += idx + len(parts[i])
	}

	return true
}

// ExtractMetadataKeys extracts the top-level keys from metadata.
func ExtractMetadataKeys(metadata map[string]interface{}) []string {
	if metadata == nil {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	return keys
}
