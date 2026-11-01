package mlflow

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSignatureInputs_DecodesTheJSONStringMLflowWrites(t *testing.T) {
	raw := json.RawMessage(`"[{\"name\": \"age\", \"type\": \"long\", \"required\": true}]"`)

	inputs, err := parseSignatureInputs(raw)
	require.NoError(t, err)
	require.Len(t, inputs, 1)
	assert.Equal(t, "age", inputs[0].Name)
	assert.Equal(t, "long", inputs[0].Type)
	assert.True(t, inputs[0].required())
}

func TestParseSignatureInputs_DecodesABareArray(t *testing.T) {
	raw := json.RawMessage(`[{"name": "age", "type": "long"}]`)

	inputs, err := parseSignatureInputs(raw)
	require.NoError(t, err)
	require.Len(t, inputs, 1)
	assert.Equal(t, "age", inputs[0].Name)
}

func TestParseSignatureInputs_ToleratesAPythonRepr(t *testing.T) {
	raw := json.RawMessage(`"[{'name': 'age', 'type': 'long', 'required': True}, {'name': 'plan', 'type': 'string', 'required': False}]"`)

	inputs, err := parseSignatureInputs(raw)
	require.NoError(t, err)
	require.Len(t, inputs, 2)
	assert.True(t, inputs[0].required())
	assert.False(t, inputs[1].required())
}

func TestParseSignatureInputs_RejectsGarbage(t *testing.T) {
	_, err := parseSignatureInputs(json.RawMessage(`"not a signature"`))
	require.Error(t, err)
}

func TestSignatureInput_IsRequiredWhenTheFieldIsAbsent(t *testing.T) {
	// MLflow only started writing required in 2.x; older signatures mean
	// every input is required.
	assert.True(t, signatureInput{Name: "age"}.required())
}

func TestFeatureColumns_MapsRequiredToNotNullable(t *testing.T) {
	required, optional := true, false
	columns := featureColumns([]signatureInput{
		{Name: "age", Type: "long", Required: &required},
		{Name: "plan", Type: "string", Required: &optional},
	})

	require.Len(t, columns, 2)
	assert.Equal(t, "age", columns[0].Name)
	assert.Equal(t, "long", columns[0].DataType)
	assert.False(t, columns[0].Nullable)
	assert.True(t, columns[1].Nullable)
}

func TestFeatureColumns_SkipsUnnamedInputs(t *testing.T) {
	columns := featureColumns([]signatureInput{
		{Type: "tensor"},
		{Name: "age", Type: "long"},
	})

	require.Len(t, columns, 1)
	assert.Equal(t, "age", columns[0].Name)
}

func TestInputsFromHistory_PrefersTheEntryMatchingTheArtifactPath(t *testing.T) {
	tag := `[
		{"run_id": "r1", "artifact_path": "candidate", "signature": {"inputs": "[{\"name\": \"x\", \"type\": \"long\"}]"}},
		{"run_id": "r1", "artifact_path": "model", "signature": {"inputs": "[{\"name\": \"age\", \"type\": \"long\"}]"}}
	]`
	version := &modelVersion{RunID: "r1", Source: "runs:/r1/model"}

	inputs, ok := inputsFromHistory(tag, version)
	require.True(t, ok)
	require.Len(t, inputs, 1)
	assert.Equal(t, "age", inputs[0].Name)
}

func TestInputsFromHistory_FallsBackToAnyEntryOfTheRun(t *testing.T) {
	tag := `[{"run_id": "r1", "artifact_path": "candidate", "signature": {"inputs": "[{\"name\": \"x\", \"type\": \"long\"}]"}}]`
	version := &modelVersion{RunID: "r1", Source: "models:/m-1"}

	inputs, ok := inputsFromHistory(tag, version)
	require.True(t, ok)
	assert.Equal(t, "x", inputs[0].Name)
}

func TestInputsFromHistory_IgnoresEntriesOfOtherRuns(t *testing.T) {
	tag := `[{"run_id": "r2", "artifact_path": "model", "signature": {"inputs": "[{\"name\": \"x\", \"type\": \"long\"}]"}}]`
	version := &modelVersion{RunID: "r1", Source: "runs:/r1/model"}

	_, ok := inputsFromHistory(tag, version)
	assert.False(t, ok)
}

func TestInputsFromHistory_IgnoresAnEntryWithoutASignature(t *testing.T) {
	tag := `[{"run_id": "r1", "artifact_path": "model", "flavors": {"sklearn": {}}}]`
	version := &modelVersion{RunID: "r1", Source: "runs:/r1/model"}

	_, ok := inputsFromHistory(tag, version)
	assert.False(t, ok)
}

func TestInputsFromHistory_IgnoresAnEmptyTag(t *testing.T) {
	_, ok := inputsFromHistory("", &modelVersion{RunID: "r1"})
	assert.False(t, ok)
}

func TestRunArtifactPath_ReturnsThePathBelowTheRun(t *testing.T) {
	assert.Equal(t, "model", runArtifactPath("runs:/abc/model"))
	assert.Equal(t, "models/candidate", runArtifactPath("runs:/abc/models/candidate/"))
}

func TestRunArtifactPath_IsEmptyForOtherSources(t *testing.T) {
	assert.Equal(t, "", runArtifactPath("models:/m-1"))
	assert.Equal(t, "", runArtifactPath("s3://bucket/model"))
}

func TestProxiedArtifactPath_HandlesTheBareForm(t *testing.T) {
	path, ok := proxiedArtifactPath("mlflow-artifacts:/2/models/m-1/artifacts")
	require.True(t, ok)
	assert.Equal(t, "2/models/m-1/artifacts", path)
}

func TestProxiedArtifactPath_HandlesTheHostQualifiedForm(t *testing.T) {
	path, ok := proxiedArtifactPath("mlflow-artifacts://mlflow:5000/2/models/m-1/artifacts")
	require.True(t, ok)
	assert.Equal(t, "2/models/m-1/artifacts", path)
}

func TestProxiedArtifactPath_RejectsOtherSchemes(t *testing.T) {
	_, ok := proxiedArtifactPath("s3://bucket/2/models/m-1/artifacts")
	assert.False(t, ok)
}

func TestVersionFilter_SingleQuotesAPlainName(t *testing.T) {
	filter, ok := versionFilter("churn-predictor")
	require.True(t, ok)
	assert.Equal(t, "name='churn-predictor'", filter)
}

func TestVersionFilter_DoubleQuotesANameHoldingAnApostrophe(t *testing.T) {
	filter, ok := versionFilter("it's a model")
	require.True(t, ok)
	assert.Equal(t, `name="it's a model"`, filter)
}

func TestVersionFilter_GivesUpOnANameHoldingBothQuotes(t *testing.T) {
	_, ok := versionFilter(`it's "the" model`)
	assert.False(t, ok)
}

func TestMetricValue_DecodesANumber(t *testing.T) {
	var m metric
	require.NoError(t, json.Unmarshal([]byte(`{"key": "auc", "value": 0.97}`), &m))
	assert.Equal(t, 0.97, float64(m.Value))
	assert.True(t, m.Value.finite())
}

func TestMetricValue_DecodesTheNaNStringMLflowWrites(t *testing.T) {
	var m metric
	require.NoError(t, json.Unmarshal([]byte(`{"key": "loss", "value": "NaN"}`), &m))
	assert.False(t, m.Value.finite())
}

func TestMetricValue_RejectsAnUnparsableString(t *testing.T) {
	var m metric
	require.Error(t, json.Unmarshal([]byte(`{"key": "loss", "value": "high"}`), &m))
}

func TestNewestVersion_PicksTheHighestNumber(t *testing.T) {
	versions := []modelVersion{{Version: "2"}, {Version: "10"}, {Version: "9"}}

	newest := newestVersion(versions)
	require.NotNil(t, newest)
	assert.Equal(t, "10", newest.Version)
}

func TestNewestVersion_IsNilWithoutVersions(t *testing.T) {
	assert.Nil(t, newestVersion(nil))
}

func TestDatasetURI_ReadsTheURIOutOfTheSourceDocument(t *testing.T) {
	assert.Equal(t, "s3://ml-data/customers.parquet", datasetURI(`{"uri": "s3://ml-data/customers.parquet"}`))
	assert.Equal(t, "/data/customers.csv", datasetURI(`{"path": "/data/customers.csv"}`))
}

func TestDatasetURI_KeepsAPlainStringSource(t *testing.T) {
	assert.Equal(t, "s3://ml-data/customers.parquet", datasetURI("s3://ml-data/customers.parquet"))
}

func TestDatasetURI_IsEmptyForASourceWithoutALocation(t *testing.T) {
	assert.Equal(t, "", datasetURI(`{"table": "customers"}`))
	assert.Equal(t, "", datasetURI(""))
}

func TestJSONOrString_DecodesJSONAndKeepsTheRest(t *testing.T) {
	assert.Equal(t, map[string]any{"uri": "x"}, jsonOrString(`{"uri": "x"}`))
	assert.Equal(t, "plain text", jsonOrString("plain text"))
	assert.Nil(t, jsonOrString(""))
}
