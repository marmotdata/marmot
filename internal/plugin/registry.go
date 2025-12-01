package plugin

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

type FieldType string

const (
	FieldTypeString      FieldType = "string"
	FieldTypeInt         FieldType = "int"
	FieldTypeBool        FieldType = "bool"
	FieldTypeSelect      FieldType = "select"
	FieldTypeMultiselect FieldType = "multiselect"
	FieldTypePassword    FieldType = "password"
	FieldTypeObject      FieldType = "object"
)

type ConfigField struct {
	Name        string        `json:"name"`
	Type        FieldType     `json:"type"`
	Label       string        `json:"label"`
	Description string        `json:"description"`
	Required    bool          `json:"required"`
	Default     interface{}   `json:"default,omitempty"`
	Options     []FieldOption `json:"options,omitempty"`
	Validation  *Validation   `json:"validation,omitempty"`
	Sensitive   bool          `json:"sensitive"`
	Placeholder string        `json:"placeholder,omitempty"`
	Fields      []ConfigField `json:"fields,omitempty"`
	IsArray     bool          `json:"is_array,omitempty"`
}

type FieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type Validation struct {
	Pattern string      `json:"pattern,omitempty"`
	Min     *int        `json:"min,omitempty"`
	Max     *int        `json:"max,omitempty"`
	MinLen  *int        `json:"min_len,omitempty"`
	MaxLen  *int        `json:"max_len,omitempty"`
}

type PluginMeta struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Icon        string        `json:"icon"`
	Category    string        `json:"category"`
	ConfigSpec  []ConfigField `json:"config_spec"`
}

func GenerateConfigSpec(configType interface{}) []ConfigField {
	return generateConfigSpecRecursive(configType, "")
}

func generateConfigSpecRecursive(configType interface{}, prefix string) []ConfigField {
	var fields []ConfigField

	t := reflect.TypeOf(configType)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		jsonTag := field.Tag.Get("json")

		// Handle inline embedded structs by recursively processing their fields
		if jsonTag != "" && strings.Contains(jsonTag, "inline") {
			// Get the type of the embedded struct
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Ptr {
				embeddedType = embeddedType.Elem()
			}

			// Create a zero value instance and recursively process it
			if embeddedType.Kind() == reflect.Struct {
				embeddedInstance := reflect.New(embeddedType).Interface()
				embeddedFields := generateConfigSpecRecursive(embeddedInstance, prefix)
				fields = append(fields, embeddedFields...)
			}
			continue
		}

		if field.Anonymous {
			continue
		}

		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		jsonName := strings.Split(jsonTag, ",")[0]

		description := field.Tag.Get("description")
		sensitive := field.Tag.Get("sensitive") == "true"
		defaultValue := field.Tag.Get("default")
		validateTag := field.Tag.Get("validate")

		fieldType := inferFieldType(field.Type, sensitive)

		// Determine if field is required based on validate tag
		required := false
		if validateTag != "" {
			// Check if validate tag contains "required" (handles required, required_without, required_with, etc.)
			required = strings.Contains(validateTag, "required")
		}

		configField := ConfigField{
			Name:        jsonName,
			Type:        fieldType,
			Label:       toLabel(jsonName),
			Description: description,
			Required:    required,
			Sensitive:   sensitive,
		}

		if defaultValue != "" {
			configField.Default = parseDefault(defaultValue, field.Type)
		}

		// Parse oneof validation and convert to dropdown options
		if validateTag != "" && strings.Contains(validateTag, "oneof=") {
			options := parseOneOfOptions(validateTag)
			if len(options) > 0 {
				configField.Type = FieldTypeSelect
				configField.Options = options
			}
		}

		// Handle nested structs and arrays of structs
		if fieldType == FieldTypeObject {
			nestedType := field.Type
			if nestedType.Kind() == reflect.Ptr {
				nestedType = nestedType.Elem()
			}

			// Check if it's a slice of structs (array of objects)
			if nestedType.Kind() == reflect.Slice {
				elemType := nestedType.Elem()
				if elemType.Kind() == reflect.Struct {
					configField.IsArray = true
					// Generate fields from the struct element
					nestedInstance := reflect.New(elemType).Interface()
					configField.Fields = generateConfigSpecRecursive(nestedInstance, prefix+jsonName+".")
				}
			} else if nestedType.Kind() == reflect.Struct {
				// Single nested object
				nestedInstance := reflect.New(nestedType).Interface()
				configField.Fields = generateConfigSpecRecursive(nestedInstance, prefix+jsonName+".")
			}
		}

		fields = append(fields, configField)
	}

	return fields
}

func parseOneOfOptions(validateTag string) []FieldOption {
	// Extract the oneof part from validate tag
	// Example: "omitempty,oneof=disable require verify-ca verify-full" -> "disable require verify-ca verify-full"
	parts := strings.Split(validateTag, "oneof=")
	if len(parts) < 2 {
		return nil
	}

	// Get the values part and split by space or comma
	oneofPart := parts[1]
	// Take only up to the next comma (in case there are other validation rules after)
	if idx := strings.Index(oneofPart, ","); idx != -1 {
		oneofPart = oneofPart[:idx]
	}

	// Split by space to get individual values
	values := strings.Fields(oneofPart)
	if len(values) == 0 {
		return nil
	}

	options := make([]FieldOption, 0, len(values))
	for _, value := range values {
		options = append(options, FieldOption{
			Label: toLabel(value),
			Value: value,
		})
	}

	return options
}

func inferFieldType(t reflect.Type, sensitive bool) FieldType {
	if sensitive {
		return FieldTypePassword
	}

	// Dereference pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return FieldTypeString
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return FieldTypeInt
	case reflect.Bool:
		return FieldTypeBool
	case reflect.Slice:
		// Check if it's a slice of structs (array of objects)
		elemType := t.Elem()
		if elemType.Kind() == reflect.Struct {
			return FieldTypeObject // Will be handled as array of objects
		}
		return FieldTypeMultiselect
	case reflect.Struct:
		return FieldTypeObject
	default:
		return FieldTypeString
	}
}

func toLabel(fieldName string) string {
	parts := strings.Split(fieldName, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[0:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

func parseDefault(value string, t reflect.Type) interface{} {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if val, err := strconv.ParseInt(value, 10, 64); err == nil {
			return val
		}
	case reflect.Bool:
		if val, err := strconv.ParseBool(value); err == nil {
			return val
		}
	case reflect.String:
		return value
	}
	return value
}

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*RegistryEntry
	sources map[string]Source
}

type RegistryEntry struct {
	Meta   PluginMeta
	Source Source
}

var globalRegistry = &Registry{
	plugins: make(map[string]*RegistryEntry),
	sources: make(map[string]Source),
}

func GetRegistry() *Registry {
	return globalRegistry
}

func (r *Registry) Register(meta PluginMeta, source Source) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[meta.ID]; exists {
		return fmt.Errorf("plugin %s already registered", meta.ID)
	}

	r.plugins[meta.ID] = &RegistryEntry{
		Meta:   meta,
		Source: source,
	}
	r.sources[meta.ID] = source

	return nil
}

func (r *Registry) Get(id string) (*RegistryEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.plugins[id]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", id)
	}

	return entry, nil
}

func (r *Registry) GetSource(id string) (Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	source, exists := r.sources[id]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", id)
	}

	return source, nil
}

func (r *Registry) List() []PluginMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metas := make([]PluginMeta, 0, len(r.plugins))
	for _, entry := range r.plugins {
		metas = append(metas, entry.Meta)
	}

	return metas
}
