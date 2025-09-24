package plugin

import (
	"reflect"
	"strings"
)

const SensitiveMask = "********"

// MaskSensitiveInfo masks sensitive values in a plugin configuration
func (r RawPluginConfig) MaskSensitiveFields(configStruct interface{}) RawPluginConfig {
	if r == nil {
		return nil
	}

	result := make(RawPluginConfig)
	for key, value := range r {
		result[key] = value
	}

	maskSensitiveFields(reflect.ValueOf(configStruct), result, "")

	return result
}

func maskSensitiveFields(v reflect.Value, configMap RawPluginConfig, prefix string) {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if !fieldValue.CanInterface() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		jsonName := jsonTag
		if idx := strings.Index(jsonTag, ","); idx != -1 {
			jsonName = jsonTag[:idx]
		}

		fullPath := jsonName
		if prefix != "" {
			fullPath = prefix + "." + jsonName
		}

		if _, hasSensitive := field.Tag.Lookup("sensitive"); hasSensitive {
			if !isEmptyValue(fieldValue) {
				setNestedValueInConfig(configMap, fullPath, SensitiveMask)
			}
			continue
		}

		if fieldValue.Kind() == reflect.Struct || (fieldValue.Kind() == reflect.Ptr && fieldValue.Type().Elem().Kind() == reflect.Struct) {
			maskSensitiveFields(fieldValue, configMap, fullPath)
		}
	}
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Bool:
		return !v.Bool()
	default:
		return false
	}
}

func setNestedValueInConfig(configMap RawPluginConfig, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := configMap

	for _, part := range parts[:len(parts)-1] {
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return
		}
	}

	current[parts[len(parts)-1]] = value
}
