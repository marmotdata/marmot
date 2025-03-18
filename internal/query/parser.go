package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Parser handles parsing of search queries into structured formats
type Parser struct {
	tokeniser *Tokeniser
}

// NewParser creates a new query parser
func NewParser() *Parser {
	return &Parser{
		tokeniser: NewTokeniser(),
	}
}

// Parse parses a search string into a structured query
func (p *Parser) Parse(queryStr string) (*Query, error) {
	tokens, err := p.tokeniser.Tokenise(queryStr)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return &Query{}, nil
	}

	query := &Query{
		Bool: &BooleanQuery{
			Must:    make([]Filter, 0),
			Should:  make([]Filter, 0),
			MustNot: make([]Filter, 0),
		},
	}

	if tokens[0] == "(" {
		depth := 1
		var nestedTokens []string
		var i int

		for i = 1; i < len(tokens) && depth > 0; i++ {
			if tokens[i] == "(" {
				depth++
			} else if tokens[i] == ")" {
				depth--
			}
			if depth > 0 {
				nestedTokens = append(nestedTokens, tokens[i])
			}
		}

		if depth != 0 {
			return nil, fmt.Errorf("unmatched parentheses")
		}

		nestedQuery, err := p.Parse(strings.Join(nestedTokens, " "))
		if err != nil {
			return nil, err
		}

		if i < len(tokens) {
			switch strings.ToUpper(tokens[i]) {
			case "OR":
				if i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "NOT" {
					// Handle OR NOT case
					query.Bool.Must = append(query.Bool.Must, Filter{Value: nestedQuery.Bool, Operator: OpEquals})

					// Parse the NOT part
					remainingQuery, err := p.Parse(strings.Join(tokens[i+2:], " "))
					if err != nil {
						return nil, err
					}
					query.Bool.Should = append(query.Bool.Should, Filter{
						Field:    remainingQuery.Bool.Must[0].Field,
						Value:    remainingQuery.Bool.Must[0].Value,
						Operator: OpNotEquals,
					})
					return query, nil
				}
				// Regular OR
				query.Bool.Must = append(query.Bool.Must, Filter{Value: nestedQuery.Bool, Operator: OpEquals})
				remainingQuery, err := p.Parse(strings.Join(tokens[i+1:], " "))
				if err != nil {
					return nil, err
				}
				query.Bool.Should = append(query.Bool.Should, remainingQuery.Bool.Must...)
				return query, nil

			case "AND":
				query.Bool.Must = append(query.Bool.Must, Filter{Value: nestedQuery.Bool, Operator: OpEquals})
				if i+1 < len(tokens) && strings.ToUpper(tokens[i+1]) == "NOT" {
					remainingQuery, err := p.Parse(strings.Join(tokens[i+2:], " "))
					if err != nil {
						return nil, err
					}
					query.Bool.MustNot = append(query.Bool.MustNot, remainingQuery.Bool.Must...)
				} else {
					remainingQuery, err := p.Parse(strings.Join(tokens[i+1:], " "))
					if err != nil {
						return nil, err
					}
					query.Bool.Must = append(query.Bool.Must, remainingQuery.Bool.Must...)
				}
				return query, nil
			}
		}
		return nestedQuery, nil
	}

	var freeTextTokens []string
	var notOperatorNext bool
	var nextIsOrClause bool
	i := 0

	for i < len(tokens) {
		token := tokens[i]

		if strings.HasPrefix(token, "@metadata.") {
			if len(freeTextTokens) > 0 {
				query.FreeText = strings.Join(freeTextTokens, " ")
				freeTextTokens = []string{}
			}

			filter, consumed, err := p.parseFilter(tokens[i:])
			if err != nil {
				return nil, err
			}
			filter.OrigQuery = queryStr

			if notOperatorNext {
				query.Bool.MustNot = append(query.Bool.MustNot, filter)
				notOperatorNext = false
			} else if nextIsOrClause {
				query.Bool.Should = append(query.Bool.Should, filter)
				nextIsOrClause = false
			} else {
				query.Bool.Must = append(query.Bool.Must, filter)
			}

			i += consumed
			continue
		}

		if isBooleanOperator(token) {
			if len(freeTextTokens) > 0 {
				query.FreeText = strings.Join(freeTextTokens, " ")
				freeTextTokens = []string{}
			}

			op := strings.ToUpper(token)
			if op == "OR" {
				nextIsOrClause = true
			} else if op == "NOT" {
				notOperatorNext = true
			}

			i++
			continue
		}

		freeTextTokens = append(freeTextTokens, token)
		i++
	}

	if len(freeTextTokens) > 0 {
		query.FreeText = strings.Join(freeTextTokens, " ")
	}

	return query, nil
}

func (p *Parser) parseFilter(tokens []string) (Filter, int, error) {
	if len(tokens) < 3 {
		return Filter{}, 0, fmt.Errorf("incomplete filter expression")
	}

	// Split the nested field path
	fieldPath := strings.Split(strings.TrimPrefix(tokens[0], "@metadata."), ".")
	if len(fieldPath) > 5 { // Optional depth limit
		return Filter{}, 0, fmt.Errorf("metadata nesting depth exceeds limit of 5")
	}

	op := tokens[1]
	consumed := 2

	operator, err := parseOperator(op)
	if err != nil {
		return Filter{}, 0, err
	}

	if operator == OpRange {
		// Special handling for range queries
		rangeStr := ""
		var rangeConsumed int
		for _, t := range tokens[2:] {
			rangeStr += t + " "
			rangeConsumed++
			if strings.HasSuffix(t, "]") {
				break
			}
		}
		rangeStr = strings.TrimSpace(rangeStr)

		re := regexp.MustCompile(`\[(.*?)\s+TO\s+(.*?)\]`)
		matches := re.FindStringSubmatch(rangeStr)
		if matches == nil {
			return Filter{}, 0, fmt.Errorf("invalid range format")
		}

		from := strings.TrimSpace(matches[1])
		to := strings.TrimSpace(matches[2])

		fromVal, _ := strconv.ParseFloat(from, 64)
		toVal, _ := strconv.ParseFloat(to, 64)

		return Filter{
			Field:    fieldPath,
			Operator: operator,
			Range: &RangeValue{
				From:      fromVal,
				To:        toVal,
				Inclusive: true,
			},
		}, consumed + rangeConsumed, nil
	}

	// Handle quoted values
	isQuoted := strings.HasPrefix(tokens[2], "\"") || strings.HasPrefix(tokens[2], "'")
	var value string

	if isQuoted {
		quoteChar := tokens[2][0]
		valueTokens := []string{}

		closed := false
		for i := 2; i < len(tokens); i++ {
			currentToken := tokens[i]

			if strings.Contains(currentToken, string(quoteChar)) && !strings.HasSuffix(currentToken, fmt.Sprintf("\\%c", quoteChar)) {
				// Handle closing quote
				if strings.HasPrefix(currentToken, string(quoteChar)) {
					currentToken = currentToken[1:]
				}
				if strings.HasSuffix(currentToken, string(quoteChar)) {
					currentToken = currentToken[:len(currentToken)-1]
					closed = true
				}
				valueTokens = append(valueTokens, currentToken)
				consumed = i + 1
				break
			} else {
				if i == 2 && strings.HasPrefix(currentToken, string(quoteChar)) {
					valueTokens = append(valueTokens, currentToken[1:])
				} else {
					valueTokens = append(valueTokens, currentToken)
				}
			}
		}

		if !closed {
			return Filter{}, 0, fmt.Errorf("unclosed quotes in query")
		}

		value = strings.Join(valueTokens, " ")
	} else {
		value = tokens[2]
		consumed = 3
	}

	// Check for wildcard operator
	if strings.Contains(value, "*") {
		operator = OpWildcard
	}

	return Filter{
		Field:    fieldPath,
		Operator: operator,
		Value:    value,
	}, consumed, nil
}

func (p *Parser) parseNestedQuery(tokens []string) (*BooleanQuery, int, error) {
	tokens = tokens[1:] // Skip first "("
	depth := 1
	var nestedTokens []string
	var i int

	for i = 0; i < len(tokens); i++ {
		token := tokens[i]
		if token == "(" {
			depth++
			nestedTokens = append(nestedTokens, token)
		} else if token == ")" {
			depth--
			if depth == 0 {
				break
			}
			nestedTokens = append(nestedTokens, token)
		} else {
			nestedTokens = append(nestedTokens, token)
		}
	}

	if depth != 0 {
		return nil, 0, fmt.Errorf("unmatched parentheses")
	}

	query, err := p.Parse(strings.Join(nestedTokens, " "))
	if err != nil {
		return nil, 0, err
	}

	return query.Bool, i + 2, nil // +2 for initial "(" and final ")"
}

func (p *Parser) parseRangeValue(tokens []string) (*RangeValue, int, error) {
	var rangeStr string
	consumed := 0
	for i, t := range tokens {
		rangeStr += t + " "
		consumed++
		if strings.HasSuffix(t, "]") {
			break
		}
		if i >= 3 { // Prevent infinite loop
			break
		}
	}
	rangeStr = strings.TrimSpace(rangeStr)

	re := regexp.MustCompile(`\[(.*?)\s+TO\s+(.*?)\]`)
	matches := re.FindStringSubmatch(rangeStr)
	if matches == nil {
		return nil, 0, fmt.Errorf("invalid range format")
	}

	from := strings.TrimSpace(matches[1])
	to := strings.TrimSpace(matches[2])

	fromVal, _ := strconv.ParseFloat(from, 64)
	toVal, _ := strconv.ParseFloat(to, 64)

	return &RangeValue{
		From:      fromVal,
		To:        toVal,
		Inclusive: true,
	}, consumed, nil
}
