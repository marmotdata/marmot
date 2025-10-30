package plugin

import (
	"reflect"
	"strings"
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
	parts := strings.Split(path, ".")

	current := m
	for i := 0; i < len(parts)-1; i++ {
		if next, ok := current[parts[i]].(map[string]interface{}); ok {
			current = next
		} else {
			return
		}
	}

	lastKey := parts[len(parts)-1]
	if val, ok := current[lastKey]; ok {
		if str, ok := val.(string); ok && str != "" {
			current[lastKey] = SensitiveMask
		}
	}
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

	if t.Kind() == reflect.Ptr {
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
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct {
			extractSensitiveFieldsRecursive(fieldType, fullPath+".", fields)
		}
	}
}
