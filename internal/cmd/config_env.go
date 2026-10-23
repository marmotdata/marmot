package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/marmotdata/marmot/internal/plugin"
)

// envReference matches ${NAME}; the bare $NAME form is not expanded so a literal dollar sign in a value is left alone.
var envReference = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandRunsEnv replaces ${NAME} in every string value of the runs with the environment variable NAME, failing on an unset variable rather than silently using an empty credential. Values are expanded after parsing, so an environment variable can never inject YAML structure.
func expandRunsEnv(runs []plugin.SourceRun) error {
	for _, run := range runs {
		for source, config := range run {
			if err := expandEnvIn(map[string]interface{}(config)); err != nil {
				return fmt.Errorf("%s: %w", source, err)
			}
		}
	}
	return nil
}

func expandEnvIn(value interface{}) error {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, item := range v {
			expanded, err := expandEnvValue(item)
			if err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			v[key] = expanded
		}
	case []interface{}:
		for i, item := range v {
			expanded, err := expandEnvValue(item)
			if err != nil {
				return fmt.Errorf("[%d]: %w", i, err)
			}
			v[i] = expanded
		}
	}
	return nil
}

func expandEnvValue(value interface{}) (interface{}, error) {
	s, ok := value.(string)
	if !ok {
		return value, expandEnvIn(value)
	}

	var missing string
	expanded := envReference.ReplaceAllStringFunc(s, func(ref string) string {
		name := envReference.FindStringSubmatch(ref)[1]
		v, ok := os.LookupEnv(name)
		if !ok && missing == "" {
			missing = name
		}
		return v
	})
	if missing != "" {
		return nil, fmt.Errorf("environment variable %s is not set", missing)
	}
	return expanded, nil
}
