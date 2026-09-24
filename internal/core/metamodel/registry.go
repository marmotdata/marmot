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
	// Facet asks Discover to offer this field as a segmented filter. Only enum and boolean fields qualify
	Facet bool `json:"facet,omitempty"`
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

// AppliesTo scopes a field to entity kinds and, within asset, to specific asset types. Kinds
// defaults to ["asset"] when empty. Stub exemption is not part of this; see Registry.Missing.
type AppliesTo struct {
	Kinds      []string `json:"kinds,omitempty"`
	AssetTypes []string `json:"assetTypes,omitempty"`
}

func (a AppliesTo) EffectiveKinds() []string {
	if len(a.Kinds) == 0 {
		return []string{"asset"}
	}
	return a.Kinds
}

type Field struct {
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	ItemType     string       `json:"itemType,omitempty"`
	Core         bool         `json:"core"`
	Required     bool         `json:"required"`
	Nullable     bool         `json:"nullable,omitempty"`
	Storage      string       `json:"storage"`
	AppliesTo    AppliesTo    `json:"appliesTo,omitempty"`
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
				overridesScope := len(field.AppliesTo.Kinds) > 0 || len(field.AppliesTo.AssetTypes) > 0
				if field.Storage != base.Storage || field.Type != base.Type || field.ItemType != base.ItemType || !field.Core || (field.Nullable && !base.Nullable) || (base.Required && !field.Required) || (base.Required && overridesScope) {
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
	// Unique per kind, not globally: different kinds never share a row.
	storageByKind := make(map[string]map[string]bool)
	for _, f := range schema.Fields {
		for _, kind := range f.AppliesTo.EffectiveKinds() {
			seen := storageByKind[kind]
			if seen == nil {
				seen = make(map[string]bool)
				storageByKind[kind] = seen
			}
			if seen[f.Storage] {
				return nil, fmt.Errorf("duplicate binding %q for kind %q", f.Storage, kind)
			}
			for previous := range seen {
				if strings.HasPrefix(f.Storage, previous+".") || strings.HasPrefix(previous, f.Storage+".") {
					return nil, fmt.Errorf("overlapping bindings %q and %q for kind %q", previous, f.Storage, kind)
				}
			}
			seen[f.Storage] = true
		}
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

var supportedKinds = []string{"asset", "data_product"}

func validateAppliesTo(a AppliesTo) error {
	if len(a.Kinds) > 8 {
		return errors.New("appliesTo exceeds 8 kinds")
	}
	seen := make(map[string]bool, len(a.Kinds))
	for _, kind := range a.Kinds {
		if seen[kind] {
			return fmt.Errorf("duplicate appliesTo kind %q", kind)
		}
		seen[kind] = true
		if kind == "glossary_term" {
			return errors.New("appliesTo kind \"glossary_term\" is reserved, not yet supported")
		}
		if !slices.Contains(supportedKinds, kind) {
			return fmt.Errorf("unknown appliesTo kind %q", kind)
		}
	}
	if len(a.AssetTypes) > 0 {
		if len(a.Kinds) > 0 && !slices.Contains(a.Kinds, "asset") {
			return errors.New("appliesTo assetTypes requires kind asset")
		}
		if len(a.AssetTypes) > 64 {
			return errors.New("appliesTo exceeds 64 assetTypes")
		}
		seenTypes := make(map[string]bool, len(a.AssetTypes))
		for _, t := range a.AssetTypes {
			if t == "" || seenTypes[t] {
				return errors.New("empty or duplicate appliesTo assetType")
			}
			seenTypes[t] = true
		}
	}
	return nil
}

func validateDefinition(f Field) error {
	if !identifier.MatchString(f.ID) || len(f.ID) > 80 || !f.Core || (f.Required && f.Nullable) {
		return errors.New("invalid id, core or nullable/required combination")
	}
	if err := validateAppliesTo(f.AppliesTo); err != nil {
		return err
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
		if len(parts) < 2 || len(parts) > 8 {
			return errors.New("metadata binding requires at least one field name")
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
	if f.Presentation.Facet && f.Type != "enum" && f.Type != "boolean" {
		return errors.New("facet requires type enum or boolean")
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

// SchemaForKind is Schema with Fields narrowed to those that apply to kind.
func (r *Registry) SchemaForKind(kind string) Schema {
	schema := r.Schema()
	filtered := []Field{}
	for _, f := range schema.Fields {
		if slices.Contains(f.AppliesTo.EffectiveKinds(), kind) {
			filtered = append(filtered, f)
		}
	}
	schema.Fields = filtered
	return schema
}

func (r *Registry) Field(id string) (Field, bool) { f, ok := r.byID[id]; return f, ok }
func (r *Registry) Enabled() bool                 { return r != nil && r.schema.Enabled }

// ValueAt reads the value a metadata.* storage binding points to, shared by every entity kind that stores governed fields under its own metadata JSON.
func ValueAt(metadata map[string]any, storage string) (any, bool) {
	parts := strings.Split(strings.TrimPrefix(storage, "metadata."), ".")
	var value any = metadata
	for _, part := range parts {
		child, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok = child[part]
		if !ok {
			return nil, false
		}
	}
	return value, true
}

// Fields returns the fields that apply to kind ("asset", "data_product"; more may be added).
func (r *Registry) Fields(kind string) []Field {
	var out []Field
	for _, f := range r.schema.Fields {
		if slices.Contains(f.AppliesTo.EffectiveKinds(), kind) {
			out = append(out, f)
		}
	}
	return out
}

func isGoverned(f Field) bool {
	return strings.HasPrefix(f.Storage, "metadata.")
}

func (r *Registry) Validate(values map[string]any, kind string, governed bool) error {
	var violations []Violation
	for _, f := range r.Fields(kind) {
		if isGoverned(f) && !governed {
			continue
		}
		value, present := values[f.ID]
		if !present || value == nil {
			if f.Required && !isGoverned(f) {
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

// Missing reports required fields with no value, for audit — it never blocks a write. Stubs
// (governed=false) are unconditionally exempt.
func (r *Registry) Missing(values map[string]any, kind string, governed bool) []Violation {
	if !governed {
		return nil
	}
	var violations []Violation
	for _, f := range r.Fields(kind) {
		if !f.Required {
			continue
		}
		if value, present := values[f.ID]; !present || value == nil {
			violations = append(violations, Violation{f.ID, "required"})
		}
	}
	return violations
}

func validateValue(f Field, value any) string {
	v := f.Validation
	switch f.Type {
	case "string", "enum", "date":
		s, ok := value.(string)
		if !ok {
			return "type"
		}
		if f.Required && !isGoverned(f) && strings.TrimSpace(s) == "" {
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
		if f.Required && !isGoverned(f) && len(items) == 0 {
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
