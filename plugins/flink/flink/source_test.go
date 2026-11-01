package flink

import (
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "http://localhost:8081"})
	require.NoError(t, err)
}

func TestValidate_MissingHostFails(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_RejectsAHostThatIsNotAURL(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{"host": "flink.example.com"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_TrimsTheTrailingSlashOffTheHost(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:8081/"})
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:8081", s.config.Host)
}

func TestValidate_DefaultsBooleansToTrue(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:8081"})
	require.NoError(t, err)

	assert.True(t, s.config.VerifySSL)
	assert.True(t, s.config.IncludeTasks)
	assert.True(t, s.config.IncludeRunHistory)
	assert.True(t, s.config.IncludeCompleted)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":              "http://localhost:8081",
		"include_completed": false,
		"verify_ssl":        false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeCompleted)
	assert.False(t, s.config.VerifySSL)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeTasks)
	assert.True(t, s.config.IncludeRunHistory)
}

func TestValidate_AcceptsBasicAuth(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host": "http://localhost:8081", "username": "marmot", "password": "secret",
	})
	require.NoError(t, err)

	assert.Equal(t, "marmot", s.config.Username)
	assert.Equal(t, "secret", s.config.Password)
}

func TestValidate_AcceptsABearerToken(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://localhost:8081", "token": "t0k3n"})
	require.NoError(t, err)

	assert.Equal(t, "t0k3n", s.config.Token)
}

func TestValidate_RejectsBasicAuthAndATokenTogether(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"host": "http://localhost:8081", "username": "marmot", "token": "t0k3n",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidate_AcceptsFilters(t *testing.T) {
	_, err := (&Source{}).Validate(pluginsdk.RawConfig{
		"host": "http://localhost:8081",
		"filter": map[string]interface{}{
			"include": []interface{}{"^etl_.*"},
			"exclude": []interface{}{".*_test$"},
		},
	})
	require.NoError(t, err)
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "flink", meta.ID)
	assert.Equal(t, "Apache Flink", meta.Name)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage", "Run History"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestMeta_MarksSecretsAsSensitive(t *testing.T) {
	sensitive := map[string]bool{}
	for _, field := range Meta().ConfigSpec {
		sensitive[field.Name] = field.Sensitive
	}

	assert.True(t, sensitive["password"])
	assert.True(t, sensitive["token"])
	assert.False(t, sensitive["username"])
}

func TestJobURL_FilesARunningJobUnderRunning(t *testing.T) {
	assert.Equal(t, "http://flink:8081/#/job/running/abc/overview", jobURL("http://flink:8081", "abc", "RUNNING"))
}

func TestJobURL_FilesAFinishedJobUnderCompleted(t *testing.T) {
	assert.Equal(t, "http://flink:8081/#/job/completed/abc/overview", jobURL("http://flink:8081", "abc", "FINISHED"))
	assert.Equal(t, "http://flink:8081/#/job/completed/abc/overview", jobURL("http://flink:8081", "abc", "FAILED"))
	assert.Equal(t, "http://flink:8081/#/job/completed/abc/overview", jobURL("http://flink:8081", "abc", "CANCELED"))
}

func TestJobURL_FallsBackToTheOverviewForOtherStates(t *testing.T) {
	assert.Equal(t, "http://flink:8081/#/overview", jobURL("http://flink:8081", "abc", "RESTARTING"))
	assert.Equal(t, "http://flink:8081/#/overview", jobURL("http://flink:8081", "abc", "CANCELLING"))
}

func TestCleanDescription_FlattensThePlanMarkup(t *testing.T) {
	assert.Equal(t, "counter +- Sink: print-sink", cleanDescription("counter<br/>+- Sink: print-sink<br/>"))
}

func TestCleanDescription_LeavesPlainTextAlone(t *testing.T) {
	assert.Equal(t, "Source: Events Generator Source", cleanDescription("Source: Events Generator Source"))
}

func TestPipelineNames_KeepsUniqueNamesBare(t *testing.T) {
	names := pipelineNames([]jobSummary{
		{JID: "a", Name: "WordCount", StartTime: 10},
		{JID: "b", Name: "State machine job", StartTime: 20},
	})

	assert.Equal(t, "WordCount", names["a"])
	assert.Equal(t, "State machine job", names["b"])
}

func TestPipelineNames_NewestOfASharedNameKeepsItBare(t *testing.T) {
	names := pipelineNames([]jobSummary{
		{JID: "old", Name: "WordCount", StartTime: 10},
		{JID: "new", Name: "WordCount", StartTime: 20},
	})

	assert.Equal(t, "WordCount", names["new"])
	assert.Equal(t, "WordCount (old)", names["old"])
}

func TestPipelineNames_BreaksAStartTimeTieByJobID(t *testing.T) {
	names := pipelineNames([]jobSummary{
		{JID: "aaa", Name: "WordCount", StartTime: 10},
		{JID: "bbb", Name: "WordCount", StartTime: 10},
	})

	assert.Equal(t, "WordCount", names["bbb"])
	assert.Equal(t, "WordCount (aaa)", names["aaa"])
}

func TestPipelineNames_FallsBackToTheJobIDForAnUnnamedJob(t *testing.T) {
	names := pipelineNames([]jobSummary{{JID: "abc", Name: ""}})

	assert.Equal(t, "abc", names["abc"])
}

func TestVertexNames_FirstOfASharedNameKeepsItBare(t *testing.T) {
	names := vertexNames([]jobVertex{
		{ID: "v1", Name: "Map"},
		{ID: "v2", Name: "Map"},
		{ID: "v3", Name: "Sink: print"},
	})

	assert.Equal(t, "Map", names["v1"])
	assert.Equal(t, "Map (v2)", names["v2"])
	assert.Equal(t, "Sink: print", names["v3"])
}

func TestTruncate_CutsAtTheLimit(t *testing.T) {
	assert.Equal(t, "abc", truncate("abcdef", 3))
	assert.Equal(t, "abc", truncate("abc", 3))
	assert.Equal(t, "", truncate("", 3))
}

func TestTruncate_CountsRunesNotBytes(t *testing.T) {
	assert.Equal(t, "äöü", truncate("äöüß", 3))
}

func TestRootCause_PrefersTheLegacyRootException(t *testing.T) {
	e := &jobExceptions{RootException: "java.lang.RuntimeException: boom"}
	e.History.Entries = []exceptionEntry{{Stacktrace: "other"}}

	assert.Equal(t, "java.lang.RuntimeException: boom", e.rootCause())
}

func TestRootCause_FallsBackToTheNewestHistoryEntry(t *testing.T) {
	var e jobExceptions
	mustDecode(failedExceptionsJSON, &e)

	assert.True(t, strings.HasPrefix(e.rootCause(), "org.apache.flink.runtime.JobException: Recovery is suppressed"))
}

func TestRootCause_IsEmptyWithoutAFailure(t *testing.T) {
	var e jobExceptions
	mustDecode(finishedExceptionsJSON, &e)

	assert.Equal(t, "", e.rootCause())
}
