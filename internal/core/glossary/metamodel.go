package glossary

import (
	"maps"
	"strings"

	"github.com/marmotdata/marmot/internal/core/metamodel"
)

const metamodelKind = "glossary_term"

// WithMetamodel validates a term's metadata against the profile fields that
// apply to glossary_term on every Create and Update.
func WithMetamodel(registry *metamodel.Registry) ServiceOption {
	return func(s *service) {
		s.metamodel = registry
	}
}

// validateMetamodel checks metadata against the governed glossary_term fields.
// Terms written through the API are always governed.
func validateMetamodel(registry *metamodel.Registry, metadata map[string]interface{}) error {
	if registry == nil || !registry.Enabled() {
		return nil
	}
	values := make(map[string]any)
	for _, f := range registry.Fields(metamodelKind) {
		if value, present := metamodel.ValueAt(metadata, f.Storage); present {
			values[f.ID] = value
		}
	}
	return registry.Validate(values, metamodelKind, true)
}

// keepGoverned carries the profile's governed values over from current into
// incoming. A sync owns the source's metadata, not what people curated in the
// fields the profile defines, so a run never wipes them.
func keepGoverned(registry *metamodel.Registry, current, incoming map[string]interface{}) map[string]interface{} {
	if registry == nil || !registry.Enabled() {
		return incoming
	}
	for _, f := range registry.Fields(metamodelKind) {
		if !strings.HasPrefix(f.Storage, "metadata.") {
			continue
		}
		if value, present := metamodel.ValueAt(current, f.Storage); present {
			incoming = setAt(incoming, strings.Split(strings.TrimPrefix(f.Storage, "metadata."), "."), value)
		}
	}
	return incoming
}

func setAt(m map[string]interface{}, path []string, value interface{}) map[string]interface{} {
	if m == nil {
		m = map[string]interface{}{}
	}
	if len(path) == 1 {
		m[path[0]] = value
		return m
	}
	// Nested maps may still belong to the caller's input: copy before writing.
	child := map[string]interface{}{}
	if current, ok := m[path[0]].(map[string]interface{}); ok {
		maps.Copy(child, current)
	}
	m[path[0]] = setAt(child, path[1:], value)
	return m
}
