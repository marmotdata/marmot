package enrichment

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/marmotdata/marmot/internal/query"
)

const (
	TargetTypeAssetType   = "asset_type"
	TargetTypeProvider    = "provider"
	TargetTypeTag         = "tag"
	TargetTypeMetadataKey = "metadata_key"
	TargetTypeQuery       = "query"
)

type RuleTarget struct {
	RuleID      string
	TargetType  string
	TargetValue string
}

// AssetSignature contains the key fields used for candidate rule lookup.
type AssetSignature struct {
	ID           string
	Type         string
	Providers    []string
	Tags         []string
	MetadataKeys []string
}

type CandidateRule struct {
	RuleID string
}

// ExtractRuleTargets analyzes a rule and extracts what it's targeting.
func ExtractRuleTargets(rule EnrichmentRule) []RuleTarget {
	var targets []RuleTarget

	if rule.GetRuleType() == RuleTypeMetadataMatch {
		if field := rule.GetMetadataField(); field != nil {
			parts := strings.Split(*field, ".")
			targets = append(targets, RuleTarget{
				TargetType:  TargetTypeMetadataKey,
				TargetValue: parts[0],
			})
		}
		return targets
	}

	expr := rule.GetQueryExpression()
	if expr == nil {
		targets = append(targets, RuleTarget{
			TargetType:  TargetTypeQuery,
			TargetValue: "",
		})
		return targets
	}

	parser := query.NewParser()
	parsed, err := parser.Parse(*expr)
	if err != nil {
		targets = append(targets, RuleTarget{
			TargetType:  TargetTypeQuery,
			TargetValue: "",
		})
		return targets
	}

	targets = extractTargetsFromQuery(parsed)

	if len(targets) == 0 {
		targets = append(targets, RuleTarget{
			TargetType:  TargetTypeQuery,
			TargetValue: "",
		})
	}

	return targets
}

func extractTargetsFromQuery(q *query.Query) []RuleTarget {
	var targets []RuleTarget

	if q.Bool == nil {
		return targets
	}

	for _, filter := range q.Bool.Must {
		targets = append(targets, extractTargetsFromFilter(filter)...)
	}

	for _, filter := range q.Bool.Should {
		targets = append(targets, extractTargetsFromFilter(filter)...)
	}

	return targets
}

func extractTargetsFromFilter(filter query.Filter) []RuleTarget {
	var targets []RuleTarget

	switch filter.FieldType {
	case query.FieldAssetType:
		if v, ok := filter.Value.(string); ok {
			targets = append(targets, RuleTarget{
				TargetType:  TargetTypeAssetType,
				TargetValue: v,
			})
		}
	case query.FieldProvider:
		if v, ok := filter.Value.(string); ok {
			targets = append(targets, RuleTarget{
				TargetType:  TargetTypeProvider,
				TargetValue: v,
			})
		}
	case query.FieldMetadata:
		if len(filter.Field) > 0 {
			targets = append(targets, RuleTarget{
				TargetType:  TargetTypeMetadataKey,
				TargetValue: filter.Field[0],
			})
		}
	case query.FieldName:
		targets = append(targets, RuleTarget{
			TargetType:  TargetTypeQuery,
			TargetValue: "",
		})
	}

	if nested, ok := filter.Value.(*query.BooleanQuery); ok {
		for _, f := range nested.Must {
			targets = append(targets, extractTargetsFromFilter(f)...)
		}
		for _, f := range nested.Should {
			targets = append(targets, extractTargetsFromFilter(f)...)
		}
	}

	return targets
}

// ValidateRule validates the rule configuration based on its type.
func ValidateRule(rule EnrichmentRule) error {
	switch rule.GetRuleType() {
	case RuleTypeQuery:
		expr := rule.GetQueryExpression()
		if expr == nil || *expr == "" {
			return fmt.Errorf("query_expression required for query rule type")
		}
		parser := query.NewParser()
		if _, err := parser.Parse(*expr); err != nil {
			return fmt.Errorf("invalid query syntax: %v", err)
		}
	case RuleTypeMetadataMatch:
		if field := rule.GetMetadataField(); field == nil || *field == "" {
			return fmt.Errorf("metadata_field required for metadata_match rule type")
		}
		if pt := rule.GetPatternType(); pt == nil || *pt == "" {
			return fmt.Errorf("pattern_type required for metadata_match rule type")
		}
		if pv := rule.GetPatternValue(); pv == nil || *pv == "" {
			return fmt.Errorf("pattern_value required for metadata_match rule type")
		}
		if pt := rule.GetPatternType(); pt != nil && *pt == PatternTypeRegex {
			if pv := rule.GetPatternValue(); pv != nil {
				if _, err := regexp.Compile(*pv); err != nil {
					return fmt.Errorf("invalid regex pattern: %v", err)
				}
			}
		}
	default:
		return fmt.Errorf("invalid rule_type: %s", rule.GetRuleType())
	}
	return nil
}
