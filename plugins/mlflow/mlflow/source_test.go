package mlflow

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"tracking_uri": "http://mlflow.internal:5000"})
	require.NoError(t, err)
}

func TestValidate_MissingTrackingURIFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tracking_uri")
}

func TestValidate_RejectsATrackingURIThatIsNotAURL(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"tracking_uri": "mlflow.internal"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tracking_uri")
}

func TestValidate_TrimsATrailingSlash(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"tracking_uri": "http://mlflow.internal:5000/"})
	require.NoError(t, err)
	assert.Equal(t, "http://mlflow.internal:5000", s.config.TrackingURI)
}

func TestValidate_AppliesDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"tracking_uri": "http://mlflow.internal:5000"})
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.True(t, s.config.VerifySSL)
	assert.True(t, s.config.IncludeExperiments)
	assert.True(t, s.config.IncludeDatasets)
	assert.True(t, s.config.IncludeMetrics)
	assert.Equal(t, 0, s.config.MaxModels)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"tracking_uri":     "http://mlflow.internal:5000",
		"verify_ssl":       false,
		"include_datasets": false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.VerifySSL)
	assert.False(t, s.config.IncludeDatasets)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeExperiments)
	assert.True(t, s.config.IncludeMetrics)
}

func TestValidate_RejectsBothAuthMethodsAtOnce(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"tracking_uri": "http://mlflow.internal:5000",
		"username":     "marmot",
		"password":     "pw",
		"token":        "tok",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not both")
}

func TestValidate_RejectsAPasswordWithoutAUsername(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"tracking_uri": "http://mlflow.internal:5000",
		"password":     "pw",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username")
}

func TestValidate_RejectsANegativeMaxModels(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"tracking_uri": "http://mlflow.internal:5000",
		"max_models":   -1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_models")
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"tracking_uri": "http://mlflow.internal:5000",
		"filter": map[string]interface{}{
			"include": []interface{}{"^churn.*"},
			"exclude": []interface{}{".*-test$"},
		},
	})
	require.NoError(t, err)
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "mlflow", meta.ID)
	assert.Equal(t, "MLflow", meta.Name)
	assert.Equal(t, "mlflow", meta.Icon)
	assert.Equal(t, "ml", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestMeta_MarksSecretsAsSensitive(t *testing.T) {
	sensitive := make(map[string]bool)
	for _, field := range Meta().ConfigSpec {
		sensitive[field.Name] = field.Sensitive
	}

	assert.True(t, sensitive["password"])
	assert.True(t, sensitive["token"])
	assert.False(t, sensitive["username"])
	assert.False(t, sensitive["tracking_uri"])
}
