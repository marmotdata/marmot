package metamodel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"sigs.k8s.io/yaml"
)

const MaxProfileBytes = 1 << 20

type Presentation struct {
	LabelKey       string `json:"labelKey,omitempty"`
	HelpTextKey    string `json:"helpTextKey,omitempty"`
	DescriptionKey string `json:"descriptionKey,omitempty"`
	Section        string `json:"section,omitempty"`
	Order          int    `json:"order,omitempty"`
	// Control names an alternate editor for a string field's value; the stored
	// value and its validation are unaffected. Only "user" is defined so far,
	// for a string field that holds a native Marmot user ID.
	Control string `json:"control,omitempty"`
}

var supportedControls = []string{"", "user"}

type Constraints struct {
	Minimum   *float64 `json:"minimum,omitempty"`
	Maximum   *float64 `json:"maximum,omitempty"`
	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`
	MinItems  *int     `json:"minItems,omitempty"`
	MaxItems  *int     `json:"maxItems,omitempty"`
}

type Field struct {
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	ItemType     string       `json:"itemType,omitempty"`
	Core         bool         `json:"core"`
	Required     bool         `json:"required"`
	Nullable     bool         `json:"nullable,omitempty"`
	Storage      string       `json:"storage"`
	AppliesTo    string       `json:"appliesTo,omitempty"`
	Values       []string     `json:"values,omitempty"`
	Validation   Constraints  `json:"validation,omitempty"`
	Presentation Presentation `json:"presentation,omitempty"`
}

type Profile struct {
	FormatVersion int     `json:"formatVersion"`
	ID            string  `json:"id"`
	Version       int     `json:"version"`
	DefaultLocale string  `json:"defaultLocale"`
	Fields        []Field `json:"fields"`
	// Messages resolves labelKey/helpTextKey/descriptionKey to text, keyed by
	// locale then by key. A key missing from the current locale falls back to
	// defaultLocale, then to the raw key. Clients own this fallback chain.
	Messages map[string]map[string]string `json:"messages,omitempty"`
}

type Schema struct {
	Profile
	Hash    string `json:"hash"`
	Enabled bool   `json:"enabled"`
}

type Registry struct {
	schema Schema
	byID   map[string]Field
}

type Violation struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

type ValidationError struct {
	Fields []Violation `json:"fields"`
}

func (e *ValidationError) Error() string { return "metamodel validation failed" }

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var profileID = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
var messageKey = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]*$`)

func Native() *Registry {
	r, err := New(nil)
	if err != nil {
		panic(err)
	}
	return r
}

func nativeFields() []Field {
	fields := []Field{
		{ID: "name", Type: "string", Required: true, Storage: "marmot.name", Presentation: Presentation{LabelKey: "common_name"}},
		{ID: "description", Type: "string", Nullable: true, Storage: "marmot.description", Presentation: Presentation{LabelKey: "asset_technical_description"}},
		{ID: "user_description", Type: "string", Nullable: true, Storage: "marmot.user_description", Presentation: Presentation{LabelKey: "common_description"}},
		{ID: "tags", Type: "list", ItemType: "string", Storage: "marmot.tags", Presentation: Presentation{LabelKey: "common_tags"}},
	}
	for i := range fields {
		fields[i].Core = true
		fields[i].Presentation.Section = "general"
		fields[i].Presentation.Order = i
	}
	return fields
}

func LoadFile(path string) (*Registry, error) {
	if path == "" {
		return New(nil)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening metamodel: %w", err)
	}
	defer f.Close()
	return Load(f)
}

func Load(reader io.Reader) (*Registry, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxProfileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading metamodel: %w", err)
	}
	if len(data) > MaxProfileBytes {
		return nil, errors.New("metamodel exceeds 1 MiB")
	}
	data, err = yaml.YAMLToJSONStrict(data)
	if err != nil {
		return nil, fmt.Errorf("decoding metamodel YAML: %w", err)
	}
	var profile Profile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&profile); err != nil {
		return nil, fmt.Errorf("decoding metamodel: %w", err)
	}
	return New(&profile)
}

func New(profile *Profile) (*Registry, error) {
	schema := Schema{Profile: Profile{FormatVersion: 1, ID: "marmot", Version: 1, DefaultLocale: "en", Fields: nativeFields()}}
	if profile != nil {
		if profile.FormatVersion != 1 || profile.Version < 1 || !profileID.MatchString(profile.ID) || !messageKey.MatchString(profile.DefaultLocale) {
			return nil, errors.New("invalid metamodel format, id, version or defaultLocale")
		}
		if len(profile.Fields) > 256 {
			return nil, errors.New("metamodel exceeds 256 fields")
		}
		if err := validateMessages(profile.Messages); err != nil {
			return nil, err
		}
		schema.FormatVersion, schema.ID, schema.Version, schema.DefaultLocale = profile.FormatVersion, profile.ID, profile.Version, profile.DefaultLocale
		schema.Messages = profile.Messages
		schema.Enabled = true
		seen := make(map[string]bool)
		for _, field := range profile.Fields {
			if seen[field.ID] {
				return nil, fmt.Errorf("duplicate field %q", field.ID)
			}
			seen[field.ID] = true
			if err := validateDefinition(field); err != nil {
				return nil, fmt.Errorf("field %q: %w", field.ID, err)
			}
			index := slices.IndexFunc(schema.Fields, func(f Field) bool { return f.ID == field.ID })
			if index >= 0 {
				base := schema.Fields[index]
				if field.Storage != base.Storage || field.Type != base.Type || field.ItemType != base.ItemType || !field.Core || (field.Nullable && !base.Nullable) || (base.Required && !field.Required) || (base.Required && field.AppliesTo != "") {
					return nil, fmt.Errorf("field %q changes a native contract", field.ID)
				}
				if field.Presentation.LabelKey == "" {
					field.Presentation = base.Presentation
				}
				schema.Fields[index] = field
			} else {
				if slices.Contains([]string{"id", "mrn", "type", "providers", "owners", "schema", "sources", "environments", "parent_mrn", "version", "created_at", "updated_at", "created_by", "last_sync_at", "is_stub", "external_links", "query", "query_language", "terms", "has_run_history"}, field.ID) {
					return nil, fmt.Errorf("field %q is reserved by the native asset model", field.ID)
				}
				if strings.HasPrefix(field.Storage, "marmot.") {
					return nil, fmt.Errorf("unregistered native binding %q", field.Storage)
				}
				if field.Presentation.LabelKey == "" {
					return nil, fmt.Errorf("field %q requires labelKey", field.ID)
				}
				schema.Fields = append(schema.Fields, field)
			}
		}
	}
	storage := make(map[string]bool)
	for _, f := range schema.Fields {
		if storage[f.Storage] {
			return nil, fmt.Errorf("duplicate binding %q", f.Storage)
		}
		for previous := range storage {
			if strings.HasPrefix(f.Storage, previous+".") || strings.HasPrefix(previous, f.Storage+".") {
				return nil, fmt.Errorf("overlapping bindings %q and %q", previous, f.Storage)
			}
		}
		storage[f.Storage] = true
	}
	data, err := json.Marshal(schema.Profile)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	schema.Hash = hex.EncodeToString(hash[:])
	r := &Registry{schema: schema, byID: make(map[string]Field)}
	for _, f := range schema.Fields {
		r.byID[f.ID] = f
	}
	return r, nil
}

func validateMessages(messages map[string]map[string]string) error {
	if len(messages) > 64 {
		return errors.New("metamodel exceeds 64 message locales")
	}
	for locale, catalog := range messages {
		if !messageKey.MatchString(locale) {
			return fmt.Errorf("invalid message locale %q", locale)
		}
		if len(catalog) > 2048 {
			return fmt.Errorf("locale %q exceeds 2048 messages", locale)
		}
		for key, value := range catalog {
			if !messageKey.MatchString(key) {
				return fmt.Errorf("invalid message key %q", key)
			}
			if value == "" {
				return fmt.Errorf("empty message for key %q", key)
			}
		}
	}
	return nil
}

func validateDefinition(f Field) error {
	if !identifier.MatchString(f.ID) || len(f.ID) > 80 || !f.Core || (f.Required && f.Nullable) {
		return errors.New("invalid id, core or nullable/required combination")
	}
	if f.AppliesTo != "" && f.AppliesTo != "governed_assets" {
		return errors.New("unknown appliesTo profile")
	}
	if !slices.Contains([]string{"string", "integer", "number", "boolean", "date", "enum", "list"}, f.Type) {
		return errors.New("unsupported type")
	}
	if f.Type == "list" {
		if !slices.Contains([]string{"string", "integer", "number", "boolean", "date", "enum"}, f.ItemType) {
			return errors.New("list requires a supported itemType")
		}
	} else if f.ItemType != "" {
		return errors.New("itemType requires list")
	}
	if f.Type == "enum" || f.ItemType == "enum" {
		if len(f.Values) == 0 || len(f.Values) > 256 {
			return errors.New("enum requires 1–256 values")
		}
		seen := make(map[string]bool)
		for _, value := range f.Values {
			if value == "" || seen[value] {
				return errors.New("empty or duplicate enum value")
			}
			seen[value] = true
		}
	} else if len(f.Values) > 0 {
		return errors.New("values requires enum")
	}
	if strings.HasPrefix(f.Storage, "metadata.") {
		parts := strings.Split(f.Storage, ".")
		if len(parts) < 3 || len(parts) > 8 {
			return errors.New("metadata binding requires a namespace and field")
		}
		for _, part := range parts[1:] {
			if !identifier.MatchString(part) || slices.Contains([]string{"constructor", "prototype", "__proto__"}, part) {
				return errors.New("invalid metadata binding")
			}
		}
	} else if !strings.HasPrefix(f.Storage, "marmot.") {
		return errors.New("unknown storage binding")
	}
	for _, key := range []string{f.Presentation.LabelKey, f.Presentation.HelpTextKey, f.Presentation.DescriptionKey} {
		if key != "" && !messageKey.MatchString(key) {
			return errors.New("invalid message key")
		}
	}
	if !slices.Contains(supportedControls, f.Presentation.Control) {
		return errors.New("unsupported presentation control")
	}
	if f.Presentation.Control == "user" && f.Type != "string" {
		return errors.New("the user control requires type string")
	}
	v := f.Validation
	valueType := f.Type
	if valueType == "list" {
		valueType = f.ItemType
	}
	if (v.Minimum != nil || v.Maximum != nil) && valueType != "number" && valueType != "integer" {
		return errors.New("numeric bounds require a numeric type")
	}
	if (v.MinLength != nil || v.MaxLength != nil) && !slices.Contains([]string{"string", "enum", "date"}, valueType) {
		return errors.New("length bounds require a string type")
	}
	if (v.MinItems != nil || v.MaxItems != nil) && f.Type != "list" {
		return errors.New("item bounds require a list")
	}
	for _, bounds := range [][2]*int{{v.MinLength, v.MaxLength}, {v.MinItems, v.MaxItems}} {
		if (bounds[0] != nil && *bounds[0] < 0) || (bounds[1] != nil && *bounds[1] < 0) || (bounds[0] != nil && bounds[1] != nil && *bounds[0] > *bounds[1]) {
			return errors.New("invalid length bounds")
		}
	}
	if v.Minimum != nil && v.Maximum != nil && *v.Minimum > *v.Maximum {
		return errors.New("invalid numeric bounds")
	}
	return nil
}

func (r *Registry) Schema() Schema {
	data, _ := json.Marshal(r.schema)
	var schema Schema
	_ = json.Unmarshal(data, &schema)
	return schema
}

func (r *Registry) Field(id string) (Field, bool) { f, ok := r.byID[id]; return f, ok }
func (r *Registry) Enabled() bool                 { return r != nil && r.schema.Enabled }

func (r *Registry) Validate(values map[string]any, governed bool) error {
	var violations []Violation
	for _, f := range r.schema.Fields {
		if f.AppliesTo == "governed_assets" && !governed {
			continue
		}
		value, present := values[f.ID]
		if !present || value == nil {
			if f.Required {
				violations = append(violations, Violation{f.ID, "required"})
			} else if present && !f.Nullable {
				violations = append(violations, Violation{f.ID, "not_nullable"})
			}
			continue
		}
		if code := validateValue(f, value); code != "" {
			violations = append(violations, Violation{f.ID, code})
		}
	}
	if len(violations) > 0 {
		return &ValidationError{Fields: violations}
	}
	return nil
}

func validateValue(f Field, value any) string {
	v := f.Validation
	switch f.Type {
	case "string", "enum", "date":
		s, ok := value.(string)
		if !ok {
			return "type"
		}
		if f.Required && strings.TrimSpace(s) == "" {
			return "required"
		}
		length := utf8.RuneCountInString(s)
		if (v.MinLength != nil && length < *v.MinLength) || (v.MaxLength != nil && length > *v.MaxLength) {
			return "length"
		}
		if f.Type == "enum" && !slices.Contains(f.Values, s) {
			return "enum"
		}
		if f.Type == "date" {
			if _, err := time.Parse("2006-01-02", s); err != nil {
				return "date"
			}
		}
	case "integer", "number":
		n, ok := number(value)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) || (f.Type == "integer" && math.Trunc(n) != n) {
			return "type"
		}
		if (v.Minimum != nil && n < *v.Minimum) || (v.Maximum != nil && n > *v.Maximum) {
			return "range"
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return "type"
		}
	case "list":
		var items []any
		switch list := value.(type) {
		case []any:
			items = list
		case []string:
			for _, item := range list {
				items = append(items, item)
			}
		default:
			return "type"
		}
		if f.Required && len(items) == 0 {
			return "required"
		}
		if (v.MinItems != nil && len(items) < *v.MinItems) || (v.MaxItems != nil && len(items) > *v.MaxItems) {
			return "items"
		}
		itemField := f
		itemField.Type = f.ItemType
		for _, item := range items {
			if code := validateValue(itemField, item); code != "" {
				return code
			}
		}
	}
	return ""
}

func number(value any) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
