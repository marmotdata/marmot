package plugin

import (
	"fmt"
	"regexp"
	"strings"
)

// TagsConfig is just a string slice that supports interpolation
type TagsConfig []string

// RegEx for matching interpolated variables, e.g ${myvar}
var interpolationRegex = regexp.MustCompile(`\${([^}]+)}`)

// InterpolateTags processes tags and replaces variables with values from metadata
func InterpolateTags(tags TagsConfig, metadata map[string]interface{}) []string {
	result := make([]string, 0, len(tags))

	for _, tag := range tags {
		if tag == "" {
			continue
		}

		// If no interpolation needed, add as-is
		if !strings.Contains(tag, "${") {
			result = append(result, tag)
			continue
		}

		// Process interpolation
		interpolated := interpolationRegex.ReplaceAllStringFunc(tag, func(match string) string {
			varName := match[2 : len(match)-1]

			// Get value from metadata
			if value, ok := getNestedValue(metadata, varName); ok {
				return fmt.Sprintf("%v", value)
			}

			return ""
		})

		if interpolated != "" {
			result = append(result, interpolated)
		}
	}

	return result
}

// getNestedValue retrieves a value from nested metadata using dot notation
func getNestedValue(metadata map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	current := metadata

	for i := 0; i < len(parts)-1; i++ {
		next, ok := current[parts[i]]
		if !ok {
			return nil, false
		}

		current, ok = next.(map[string]interface{})
		if !ok {
			return nil, false
		}
	}

	value, ok := current[parts[len(parts)-1]]
	return value, ok
}
