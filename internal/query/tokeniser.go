package query

import (
	"fmt"
	"strings"
	"unicode"
)

// Tokeniser handles breaking down search queries into tokens
type Tokeniser struct{}

// NewTokeniser creates a new tokenizer
func NewTokeniser() *Tokeniser {
	return &Tokeniser{}
}

// Tokenise splits a query string into tokens while preserving quoted strings and operators
func (t *Tokeniser) Tokenise(query string) ([]string, error) {
	var tokens []string
	var currentToken strings.Builder
	inQuotes := false
	quoteChar := rune(0)

	for _, char := range query {
		switch {
		case char == '(' || char == ')':
			// If we have a current token, add it
			if currentToken.Len() > 0 && !inQuotes {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
			if inQuotes {
				currentToken.WriteRune(char)
			} else {
				// Add parenthesis as a separate token
				tokens = append(tokens, string(char))
			}
		case char == '"' || char == '\'':
			currentToken.WriteRune(char)
			if !inQuotes {
				inQuotes = true
				quoteChar = char
			} else if char == quoteChar {
				inQuotes = false
				if currentToken.Len() > 0 {
					tokens = append(tokens, currentToken.String())
					currentToken.Reset()
				}
			}
		case char == ':':
			if inQuotes {
				currentToken.WriteRune(char)
			} else {
				if currentToken.Len() > 0 {
					tokens = append(tokens, currentToken.String())
				}
				tokens = append(tokens, ":")
				currentToken.Reset()
			}
		case unicode.IsSpace(char):
			if inQuotes {
				currentToken.WriteRune(char)
			} else if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
		default:
			currentToken.WriteRune(char)
		}
	}

	if currentToken.Len() > 0 {
		tokens = append(tokens, currentToken.String())
	}

	if inQuotes {
		return nil, fmt.Errorf("unclosed quotes in query")
	}

	// Check for mixed quotes
	if strings.Count(query, "'") > 0 && strings.Count(query, "\"") > 0 {
		return nil, fmt.Errorf("mixed quote types in query")
	}

	return cleanTokens(tokens), nil
}

func cleanTokens(tokens []string) []string {
	var cleaned []string
	for _, token := range tokens {
		if token = strings.TrimSpace(token); token != "" {
			cleaned = append(cleaned, token)
		}
	}
	return cleaned
}
