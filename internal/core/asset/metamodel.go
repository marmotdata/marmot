package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/marmotdata/marmot/internal/core/metamodel"
)

var (
	ErrVersionConflict = errors.New("asset version conflict")
	ErrVersionRequired = errors.New("asset version required")
)

func WithMetamodel(registry *metamodel.Registry) ServiceOption {
	return func(s *service) { s.metamodel = registry }
}

func (s *service) registry() *metamodel.Registry {
	if s.metamodel == nil {
		return metamodel.Native()
	}
	return s.metamodel
}

func (s *service) Metamodel() metamodel.Schema {
	return s.registry().Schema()
}

func (s *service) PatchFields(ctx context.Context, id string, version int64, fields map[string]any) (*Asset, error) {
	if version < 1 {
		return nil, ErrVersionRequired
	}
	return s.Update(ctx, id, UpdateInput{
		ExpectedVersion: &version,
		GovernedFields:  fields,
	})
}

func (s *service) validateAsset(a *Asset) error {
	if !s.registry().Enabled() {
		return nil
	}
	return s.registry().Validate(MetamodelValues(s.registry(), a), !a.IsStub)
}

func applyFields(registry *metamodel.Registry, asset *Asset, fields map[string]any) error {
	if asset.Metadata == nil {
		asset.Metadata = make(map[string]any)
	}
	for id, value := range fields {
		field, ok := registry.Field(id)
		if !ok {
			return &metamodel.ValidationError{Fields: []metamodel.Violation{{Field: id, Code: "unknown_field"}}}
		}
		if value == nil && (!field.Nullable || field.Required) {
			return &metamodel.ValidationError{Fields: []metamodel.Violation{{Field: id, Code: "not_nullable"}}}
		}
		switch field.Storage {
		case "marmot.name", "marmot.description", "marmot.user_description":
			var ptr *string
			if value != nil {
				text, ok := value.(string)
				if !ok {
					return fieldTypeError(id)
				}
				ptr = &text
			}
			switch field.Storage {
			case "marmot.name":
				asset.Name = ptr
			case "marmot.description":
				asset.Description = ptr
			case "marmot.user_description":
				if ptr != nil && *ptr == "" {
					ptr = nil
				}
				asset.UserDescription = ptr
			}
		case "marmot.tags":
			list, ok := stringList(value)
			if !ok {
				return fieldTypeError(id)
			}
			asset.Tags = list
		default:
			parts := strings.Split(strings.TrimPrefix(field.Storage, "metadata."), ".")
			if err := setMetadataValue(asset.Metadata, parts, value); err != nil {
				return fmt.Errorf("%w: field %s: %v", ErrInvalidInput, id, err)
			}
		}
	}
	return registry.Validate(MetamodelValues(registry, asset), !asset.IsStub)
}

func fieldTypeError(id string) error {
	return &metamodel.ValidationError{Fields: []metamodel.Violation{{Field: id, Code: "type"}}}
}

func stringList(value any) ([]string, bool) {
	if list, ok := value.([]string); ok {
		return append([]string{}, list...), true
	}
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		result = append(result, s)
	}
	return result, true
}

func setMetadataValue(object map[string]any, parts []string, value any) error {
	for _, part := range parts[:len(parts)-1] {
		next, ok := object[part]
		if !ok {
			next = map[string]any{}
			object[part] = next
		}
		child, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("metadata path %q is not an object", part)
		}
		object = child
	}
	if value == nil {
		delete(object, parts[len(parts)-1])
	} else {
		object[parts[len(parts)-1]] = value
	}
	return nil
}

func metadataValue(object map[string]any, binding string) (any, bool) {
	parts := strings.Split(strings.TrimPrefix(binding, "metadata."), ".")
	var value any = object
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

func MetamodelValues(registry *metamodel.Registry, asset *Asset) map[string]any {
	values := make(map[string]any)
	for _, f := range registry.Schema().Fields {
		var value any
		present := true
		switch f.Storage {
		case "marmot.name":
			if asset.Name != nil {
				value = *asset.Name
			} else {
				present = false
			}
		case "marmot.description":
			if asset.Description != nil {
				value = *asset.Description
			} else {
				present = false
			}
		case "marmot.user_description":
			if asset.UserDescription != nil {
				value = *asset.UserDescription
			} else {
				present = false
			}
		case "marmot.tags":
			value = asset.Tags
		default:
			value, present = metadataValue(asset.Metadata, f.Storage)
		}
		if present {
			values[f.ID] = value
		}
	}
	return values
}

func cloneMetadata(value map[string]any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = json.Unmarshal(data, &result)
	return result, err
}

func (s *service) preserveGoverned(current *Asset, input *UpdateInput) error {
	if input.ExpectedVersion != nil && current.Version != *input.ExpectedVersion {
		return ErrVersionConflict
	}
	registry := s.registry()
	if !registry.Enabled() || input.Metadata == nil {
		return nil
	}
	metadata, err := cloneMetadata(input.Metadata)
	if err != nil {
		return err
	}
	for _, field := range registry.Schema().Fields {
		if !strings.HasPrefix(field.Storage, "metadata.") {
			continue
		}
		previous, existed := metadataValue(current.Metadata, field.Storage)
		next, supplied := metadataValue(metadata, field.Storage)
		if supplied && (!existed || !reflect.DeepEqual(previous, next)) && input.ExpectedVersion == nil {
			if _, patched := input.GovernedFields[field.ID]; !patched {
				return ErrVersionRequired
			}
		}
		_, patched := input.GovernedFields[field.ID]
		if !supplied && existed && !patched {
			if err := setMetadataValue(metadata, strings.Split(strings.TrimPrefix(field.Storage, "metadata."), "."), previous); err != nil {
				return err
			}
		}
	}
	input.Metadata = metadata
	return nil
}
