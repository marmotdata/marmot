package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleSpec() []ConfigField {
	minVal := 1
	return []ConfigField{
		{
			Name:        "bootstrap_servers",
			Type:        FieldTypeString,
			Label:       "Bootstrap Servers",
			Description: "Comma-separated list of bootstrap servers",
			Required:    true,
			Placeholder: "kafka:9092",
		},
		{
			Name: "authentication",
			Type: FieldTypeObject,
			Fields: []ConfigField{
				{
					Name:    "type",
					Type:    FieldTypeSelect,
					Default: "none",
					Options: []FieldOption{
						{Label: "None", Value: "none"},
						{Label: "SASL SSL", Value: "sasl_ssl"},
					},
				},
				{
					Name: "mechanism",
					Type: FieldTypeSelect,
					Options: []FieldOption{
						{Label: "PLAIN", Value: "PLAIN"},
						{Label: "SCRAM-SHA-256", Value: "SCRAM-SHA-256"},
					},
				},
			},
		},
		{
			Name: "timeout",
			Type: FieldTypeInt,
			Validation: &Validation{
				Min: &minVal,
			},
		},
	}
}

func TestCloneConfigSpec_ProducesIndependentCopy(t *testing.T) {
	original := sampleSpec()
	clone := CloneConfigSpec(original)

	// Mutate the clone
	clone[0].Description = "CHANGED"
	clone[0].Placeholder = "CHANGED"
	clone[1].Fields[0].Default = "CHANGED"
	clone[1].Fields[0].Options[0].Label = "CHANGED"
	newMin := 999
	clone[2].Validation.Min = &newMin

	// Original must be untouched
	assert.Equal(t, "Comma-separated list of bootstrap servers", original[0].Description)
	assert.Equal(t, "kafka:9092", original[0].Placeholder)
	assert.Equal(t, "none", original[1].Fields[0].Default)
	assert.Equal(t, "None", original[1].Fields[0].Options[0].Label)
	assert.Equal(t, 1, *original[2].Validation.Min)
}

func TestCloneConfigSpec_PreservesValues(t *testing.T) {
	original := sampleSpec()
	clone := CloneConfigSpec(original)

	require.Len(t, clone, len(original))
	assert.Equal(t, original[0].Name, clone[0].Name)
	assert.Equal(t, original[0].Required, clone[0].Required)
	assert.Equal(t, original[1].Fields[0].Default, clone[1].Fields[0].Default)
	assert.Equal(t, len(original[1].Fields[0].Options), len(clone[1].Fields[0].Options))
	assert.Equal(t, *original[2].Validation.Min, *clone[2].Validation.Min)
}

func TestApplyConfigOverrides_AppliesAllFields(t *testing.T) {
	spec := CloneConfigSpec(sampleSpec())
	reqTrue := true

	spec = ApplyConfigOverrides(spec, map[string]ConfigOverride{
		"bootstrap_servers": {
			Default:     "localhost:9092",
			Description: "New description",
			Placeholder: "new-placeholder:9092",
			Required:    &reqTrue,
		},
	})

	assert.Equal(t, "localhost:9092", spec[0].Default)
	assert.Equal(t, "New description", spec[0].Description)
	assert.Equal(t, "new-placeholder:9092", spec[0].Placeholder)
	assert.True(t, spec[0].Required)
}

func TestApplyConfigOverrides_NestedFields(t *testing.T) {
	spec := CloneConfigSpec(sampleSpec())

	spec = ApplyConfigOverrides(spec, map[string]ConfigOverride{
		"authentication.type":      {Default: "sasl_ssl"},
		"authentication.mechanism": {Default: "PLAIN"},
	})

	assert.Equal(t, "sasl_ssl", spec[1].Fields[0].Default)
	assert.Equal(t, "PLAIN", spec[1].Fields[1].Default)
}

func TestApplyConfigOverrides_EmptyMap(t *testing.T) {
	original := sampleSpec()
	spec := CloneConfigSpec(original)

	spec = ApplyConfigOverrides(spec, map[string]ConfigOverride{})

	assert.Equal(t, original[0].Description, spec[0].Description)
	assert.Equal(t, original[0].Placeholder, spec[0].Placeholder)
	assert.Equal(t, original[1].Fields[0].Default, spec[1].Fields[0].Default)
}

func TestRemoveConfigFields_TopLevel(t *testing.T) {
	spec := CloneConfigSpec(sampleSpec())

	spec = RemoveConfigFields(spec, []string{"timeout"})

	require.Len(t, spec, 2)
	assert.Equal(t, "bootstrap_servers", spec[0].Name)
	assert.Equal(t, "authentication", spec[1].Name)
}

func TestRemoveConfigFields_NestedFields(t *testing.T) {
	spec := CloneConfigSpec(sampleSpec())

	spec = RemoveConfigFields(spec, []string{"authentication.type"})

	// Parent still present, but only mechanism remains
	require.Len(t, spec, 3)
	require.Len(t, spec[1].Fields, 1)
	assert.Equal(t, "mechanism", spec[1].Fields[0].Name)
}

func TestRemoveConfigFields_MultipleAtOnce(t *testing.T) {
	spec := CloneConfigSpec(sampleSpec())

	spec = RemoveConfigFields(spec, []string{
		"timeout",
		"authentication.type",
		"authentication.mechanism",
	})

	require.Len(t, spec, 2)
	assert.Equal(t, "bootstrap_servers", spec[0].Name)
	assert.Equal(t, "authentication", spec[1].Name)
	assert.Empty(t, spec[1].Fields)
}

func TestRemoveConfigFields_NoOpForUnknown(t *testing.T) {
	spec := CloneConfigSpec(sampleSpec())

	spec = RemoveConfigFields(spec, []string{"nonexistent"})

	assert.Len(t, spec, 3)
}
