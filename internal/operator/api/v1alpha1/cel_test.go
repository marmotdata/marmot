package v1alpha1

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

// validateSpec runs the CRD's CEL rules on spec the way the API server does at admission.
func validateSpec(t *testing.T, spec map[string]any) field.ErrorList {
	t.Helper()
	data, err := os.ReadFile("../../../../charts/marmot/crds/runs.marmotdata.io_runs.yaml")
	require.NoError(t, err)
	var crd apiextensionsv1.CustomResourceDefinition
	require.NoError(t, yaml.Unmarshal(data, &crd))

	var props apiextensions.JSONSchemaProps
	require.NoError(t, apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(crd.Spec.Versions[0].Schema.OpenAPIV3Schema, &props, nil))
	structural, err := schema.NewStructural(&props)
	require.NoError(t, err)

	obj := map[string]any{"apiVersion": "runs.marmotdata.io/v1alpha1", "kind": "Run", "metadata": map[string]any{"name": "r"}, "spec": spec}
	errs, _ := cel.NewValidator(structural, true, celconfig.PerCallLimit).Validate(context.Background(), field.NewPath("root"), structural, obj, nil, celconfig.RuntimeCELCostBudget)
	return errs
}

func runs() []any {
	return []any{map[string]any{"postgresql": map[string]any{"host": "db"}}}
}

func TestRunRejectsMarmotEnv(t *testing.T) {
	assert.Empty(t, validateSpec(t, map[string]any{"runs": runs(), "env": []any{map[string]any{"name": "PG_PASSWORD", "value": "x"}}}))

	for _, name := range []string{"MARMOT_HOST", "MARMOT_PLUGINS_REGISTRY", "MARMOT_API_KEY"} {
		errs := validateSpec(t, map[string]any{"runs": runs(), "env": []any{map[string]any{"name": name, "value": "x"}}})
		assert.NotEmpty(t, errs, name)
	}
}

func TestRunRejectsScheduleWhenServing(t *testing.T) {
	assert.Empty(t, validateSpec(t, map[string]any{"runs": runs(), "serve": true}))
	assert.Empty(t, validateSpec(t, map[string]any{"runs": runs(), "schedule": "0 * * * *"}))
	assert.NotEmpty(t, validateSpec(t, map[string]any{"runs": runs(), "serve": true, "schedule": "0 * * * *"}))
}
