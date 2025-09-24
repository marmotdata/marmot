package plugin

import (
	"reflect"
	"strings"
)

// MetadataField defines a metadata field with documentation and validation
type MetadataField struct {
	Path        string
	Description string
	Type        string
	Required    bool
	Sensitive   bool
}

// GetMetadataFields extracts metadata field definitions from struct tags
func GetMetadataFields(v interface{}) []MetadataField {
	var fields []MetadataField
	t := reflect.TypeOf(v)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		path := field.Tag.Get("metadata")
		if path == "" {
			continue
		}

		_, sensitive := field.Tag.Lookup("sensitive")

		fields = append(fields, MetadataField{
			Path:        path,
			Description: field.Tag.Get("description"),
			Type:        field.Type.String(),
			Required:    field.Tag.Get("required") == "true",
			Sensitive:   sensitive,
		})
	}

	return fields
}

// MapToMetadata converts a struct with metadata tags to a metadata map
func MapToMetadata(source interface{}) map[string]interface{} {
	metadata := make(map[string]interface{})
	t := reflect.TypeOf(source)
	v := reflect.ValueOf(source)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		metadataTag := field.Tag.Get("metadata")

		if metadataTag == "" {
			continue
		}

		value := v.Field(i).Interface()

		if isNilValue(value) {
			continue
		}

		_, sensitive := field.Tag.Lookup("sensitive")
		if sensitive {
			value = SensitiveMask
		}

		if field.Type.Kind() == reflect.Struct && !sensitive {
			nestedMetadata := MapToMetadata(value)
			for k, v := range nestedMetadata {
				setNestedValue(metadata, metadataTag+"."+k, v)
			}
		} else if field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Struct && !sensitive {
			sliceValue := v.Field(i)
			for j := 0; j < sliceValue.Len(); j++ {
				nestedMetadata := MapToMetadata(sliceValue.Index(j).Interface())
				for k, v := range nestedMetadata {
					setNestedValue(metadata, metadataTag+"."+k, v)
				}
			}
		} else {
			setNestedValue(metadata, metadataTag, value)
		}
	}

	return metadata
}

func isNilValue(v interface{}) bool {
	switch v := v.(type) {
	case string:
		return v == ""
	case int:
		return v == 0
	case bool:
		return !v
	case []string:
		return len(v) == 0
	case nil:
		return true
	default:
		return false
	}
}

func setNestedValue(metadata map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := metadata

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if _, exists := current[part]; !exists {
			current[part] = make(map[string]interface{})
		}
		current = current[part].(map[string]interface{})
	}

	current[parts[len(parts)-1]] = value
}

