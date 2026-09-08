package prefect

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_AcceptsAHostThatAlreadyHasTheAPIPath(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:4200/api"})

	require.NoError(t, err)
	assert.Equal(t, "http://localhost:4200/api", s.config.Host)
}

func TestValidate_AddsTheAPIPathPeopleLeaveOff(t *testing.T) {
	// The address in the browser is the UI's, which has no /api on it.
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:4200"})

	require.NoError(t, err)
	assert.Equal(t, "http://localhost:4200/api", s.config.Host)
}

func TestValidate_TrimsATrailingSlashBeforeAddingTheAPIPath(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:4200/"})

	require.NoError(t, err)
	assert.Equal(t, "http://localhost:4200/api", s.config.Host)
}

func TestValidate_LeavesACloudWorkspaceURLAlone(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"host":    "https://api.prefect.cloud/api/accounts/acc-1/workspaces/ws-1",
		"api_key": "pnu_secret",
	})

	require.NoError(t, err)
	assert.Equal(t, "https://api.prefect.cloud/api/accounts/acc-1/workspaces/ws-1", s.config.Host)
}

func TestValidate_MissingHostFails(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_RejectsAHostWithNoScheme(t *testing.T) {
	// The url check on its own reads "localhost" as the scheme and lets
	// this through, so the failure would only surface on the first request.
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "localhost:4200"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "http://")
}

func TestValidate_RejectsBothAuthenticationMethodsAtOnce(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"host":        "http://localhost:4200/api",
		"api_key":     "pnu_secret",
		"auth_string": "admin:password",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not both")
}

func TestValidate_AcceptsAuthStringOnItsOwn(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"host":        "http://localhost:4200/api",
		"auth_string": "admin:password",
	})

	require.NoError(t, err)
	assert.Equal(t, "admin:password", s.config.AuthString)
}

func TestValidate_AcceptsNoAuthenticationAtAll(t *testing.T) {
	// A self-hosted server with no auth configured is the default setup.
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:4200/api"})

	require.NoError(t, err)
	assert.Empty(t, s.config.APIKey)
	assert.Empty(t, s.config.AuthString)
}

func TestValidate_DefaultsTheFlagsToTrue(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:4200/api"})

	require.NoError(t, err)
	require.NotNil(t, s.config)
	assert.True(t, s.config.VerifySSL)
	assert.True(t, s.config.IncludeTasks)
	assert.True(t, s.config.IncludeDeployments)
	assert.True(t, s.config.IncludeRunHistory)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"host":                "http://localhost:4200/api",
		"include_tasks":       false,
		"include_run_history": false,
	})

	require.NoError(t, err)
	assert.False(t, s.config.IncludeTasks)
	assert.False(t, s.config.IncludeRunHistory)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeDeployments)
	assert.True(t, s.config.VerifySSL)
}

func TestValidate_DefaultsTheRunHistoryLimit(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:4200/api"})

	require.NoError(t, err)
	assert.Equal(t, 10, s.config.RunHistoryLimit)
}

func TestValidate_RejectsARunHistoryLimitAboveTheMaximum(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"host":              "http://localhost:4200/api",
		"run_history_limit": 500,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "run_history_limit")
}

func TestValidate_RejectsARunHistoryLimitOfZero(t *testing.T) {
	// Prefect answers a limit of 0 with an empty list, so accepting this
	// would quietly discover nothing at all.
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"host":              "http://localhost:4200/api",
		"run_history_limit": 0,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "run_history_limit")
}

func TestValidate_KeepsRunHistoryLimitOptionalInTheUIForm(t *testing.T) {
	// A minimum is not a requirement to fill the field in: the default
	// applies first, so the form must not mark it required.
	for _, field := range Meta().ConfigSpec {
		if field.Name == "run_history_limit" {
			assert.False(t, field.Required)
			assert.EqualValues(t, 10, field.Default)
			return
		}
	}

	t.Fatal("run_history_limit is missing from the config spec")
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"host": "http://localhost:4200/api",
		"filter": map[string]interface{}{
			"include": []interface{}{"^nightly.*"},
			"exclude": []interface{}{".*-test$"},
		},
	})

	require.NoError(t, err)
}

// The zero-value Config keeps Go's false defaults; only Validate promotes
// the flags to true, so a raw struct must not look pre-configured.
func TestConfig_ZeroValueDefaultsAreFalse(t *testing.T) {
	config := &Config{Host: "http://localhost:4200/api"}

	assert.False(t, config.IncludeTasks)
	assert.False(t, config.IncludeDeployments)
	assert.False(t, config.IncludeRunHistory)
	assert.False(t, config.VerifySSL)
}

func TestMeta_DeclaresWhatDiscoverActuallyEmits(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "prefect", meta.ID)
	assert.Equal(t, "Prefect", meta.Name)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage", "Run History"}, meta.Features)
}

func TestMeta_MarksTheCredentialFieldsSensitive(t *testing.T) {
	sensitive := map[string]bool{}
	for _, field := range Meta().ConfigSpec {
		sensitive[field.Name] = field.Sensitive
	}

	assert.True(t, sensitive["api_key"])
	assert.True(t, sensitive["auth_string"])
	assert.False(t, sensitive["host"])
}

func TestFlowURL_PointsAtTheSelfHostedUI(t *testing.T) {
	assert.Equal(t, "http://localhost:4200/flows/flow/abc",
		flowURL("http://localhost:4200/api", "abc"))
}

func TestFlowURL_PointsAtTheCloudUIForAWorkspace(t *testing.T) {
	// Cloud serves its UI on a different host to its API, so the account
	// and workspace ids have to be read back out of the API URL.
	assert.Equal(t, "https://app.prefect.cloud/account/acc-1/workspace/ws-1/flows/flow/abc",
		flowURL("https://api.prefect.cloud/api/accounts/acc-1/workspaces/ws-1", "abc"))
}

func TestTaskName_DropsThePrefectHashSuffix(t *testing.T) {
	// Prefect derives the hash from the task's source, so keeping it would
	// rename the asset every time somebody edits the task.
	assert.Equal(t, "nightly-etl/extract", taskName("nightly-etl", "extract-bf522387"))
}

func TestTaskName_KeepsAKeyThatHasNoHashSuffix(t *testing.T) {
	assert.Equal(t, "nightly-etl/extract", taskName("nightly-etl", "extract"))
}

func TestTaskName_KeepsAHyphenatedNameIntact(t *testing.T) {
	assert.Equal(t, "nightly-etl/load-orders", taskName("nightly-etl", "load-orders-d6bfc39f"))
}

func TestEventTypeForState_MapsCompletedToComplete(t *testing.T) {
	assert.Equal(t, "COMPLETE", eventTypeForState("COMPLETED"))
}

func TestEventTypeForState_MapsFailedAndCrashedToFail(t *testing.T) {
	assert.Equal(t, "FAIL", eventTypeForState("FAILED"))
	assert.Equal(t, "FAIL", eventTypeForState("CRASHED"))
}

func TestEventTypeForState_MapsCancellationToAbort(t *testing.T) {
	assert.Equal(t, "ABORT", eventTypeForState("CANCELLED"))
	assert.Equal(t, "ABORT", eventTypeForState("CANCELLING"))
}

func TestEventTypeForState_MapsUnfinishedStatesToRunning(t *testing.T) {
	assert.Equal(t, "RUNNING", eventTypeForState("RUNNING"))
	assert.Equal(t, "RUNNING", eventTypeForState("PENDING"))
	assert.Equal(t, "RUNNING", eventTypeForState("PAUSED"))
}

func TestEventTypeForState_MapsAnythingElseToOther(t *testing.T) {
	assert.Equal(t, "OTHER", eventTypeForState("SCHEDULED"))
	assert.Equal(t, "OTHER", eventTypeForState(""))
}
