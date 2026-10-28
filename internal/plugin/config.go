package plugin

import (
	"reflect"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

const SensitiveMask = "********"

func (r RawPluginConfig) MaskSensitiveFields(configStruct interface{}) RawPluginConfig {
	if r == nil {
		return nil
	}

	result := make(RawPluginConfig)
	for key, value := range r {
		result[key] = value
	}

	sensitiveFields := extractSensitiveFields(configStruct)

	for _, fieldPath := range sensitiveFields {
		maskFieldInMap(result, fieldPath)
	}

	return result
}

func maskFieldInMap(m map[string]interface{}, path string) {
	parent, key := writableParentMap(m, path)
	if parent == nil {
		return
	}

	if str, ok := parent[key].(string); ok && str != "" {
		parent[key] = SensitiveMask
	}
}

// parentMap walks a dotted path and returns the map holding its final key, or nil if any segment along the way is not a map
func parentMap(m map[string]interface{}, path string) (map[string]interface{}, string) {
	parts := strings.Split(path, ".")

	current := m
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]interface{})
		if !ok {
			return nil, ""
		}
		current = next
	}

	return current, parts[len(parts)-1]
}

func extractSensitiveFields(v interface{}) []string {
	var fields []string
	extractSensitiveFieldsRecursive(reflect.TypeOf(v), "", &fields)
	return fields
}

func extractSensitiveFieldsRecursive(t reflect.Type, prefix string, fields *[]string) {
	if t == nil {
		return
	}

	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		jsonName := strings.Split(jsonTag, ",")[0]
		fullPath := jsonName
		if prefix != "" {
			fullPath = prefix + jsonName
		}

		if _, hasSensitive := field.Tag.Lookup("sensitive"); hasSensitive {
			*fields = append(*fields, fullPath)
		}

		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct {
			extractSensitiveFieldsRecursive(fieldType, fullPath+".", fields)
		}
	}
}

// MaskSensitiveFieldsFromSpec masks sensitive fields in a config map using the ConfigSpec
func MaskSensitiveFieldsFromSpec(config RawPluginConfig, configSpec []pluginsdk.ConfigField) RawPluginConfig {
	if config == nil {
		return nil
	}

	result := make(RawPluginConfig)
	for key, value := range config {
		result[key] = value
	}

	sensitiveFields := extractSensitiveFieldsFromSpec(configSpec, "")

	for _, fieldPath := range sensitiveFields {
		maskFieldInMap(result, fieldPath)
	}

	return result
}

// RestoreMaskedFieldsFromSpec swaps sensitive fields still holding SensitiveMask for their value in previous, so a config read back from the API can be saved without wiping its secrets; masked fields with no previous value are removed
func RestoreMaskedFieldsFromSpec(config, previous map[string]interface{}, configSpec []pluginsdk.ConfigField) {
	for _, path := range extractSensitiveFieldsFromSpec(configSpec, "") {
		parent, key := parentMap(config, path)
		if parent == nil || parent[key] != SensitiveMask {
			continue
		}

		if prevParent, _ := parentMap(previous, path); prevParent != nil {
			if value, ok := prevParent[key]; ok {
				parent[key] = value
				continue
			}
		}
		delete(parent, key)
	}
}

func extractSensitiveFieldsFromSpec(fields []pluginsdk.ConfigField, prefix string) []string {
	var sensitive []string

	for _, field := range fields {
		fullPath := field.Name
		if prefix != "" {
			fullPath = prefix + field.Name
		}

		if field.Sensitive {
			sensitive = append(sensitive, fullPath)
		}

		// Recursively check nested fields
		if field.Type == pluginsdk.FieldTypeObject && len(field.Fields) > 0 {
			nestedFields := extractSensitiveFieldsFromSpec(field.Fields, fullPath+".")
			sensitive = append(sensitive, nestedFields...)
		}
	}

	return sensitive
}
