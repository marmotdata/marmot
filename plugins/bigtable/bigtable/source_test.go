package bigtable

import (
	"encoding/binary"
	"testing"
	"time"

	bt "cloud.google.com/go/bigtable"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeta_DescribesTheBigtablePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "bigtable", meta.ID)
	assert.Equal(t, "Google Bigtable", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestValidate_RejectsAConfigWithoutAProject(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "project_id")
}

func TestValidate_FillsInTheSamplingDefaults(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "analytics"})

	require.NoError(t, err)
	assert.True(t, s.config.IncludeColumns, "columns are worth having by default")
	assert.Equal(t, 100, s.config.SampleRows)
	assert.Equal(t, 100000, s.config.MaxCountRows)
}

func TestValidate_LeavesTheScanningOptionsOffByDefault(t *testing.T) {
	// Both of these read whole tables, so they have to be asked for.
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "analytics"})

	require.NoError(t, err)
	assert.False(t, s.config.IncludeStatistics)
	assert.False(t, s.config.IncludeBackups)
}

func TestValidate_KeepsAnExplicitFalseForIncludeColumns(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "analytics", "include_columns": false})

	require.NoError(t, err)
	assert.False(t, s.config.IncludeColumns)
}

func TestValidate_RejectsASampleSizeAboveTheLimit(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "analytics", "sample_rows": 20000})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sample_rows")
}

func TestValidate_RequiresInstancesWhenPointedAtAnEmulator(t *testing.T) {
	// The emulator cannot list its own instances, so discovery would find
	// nothing at all and look like an empty project.
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id":    "test-project",
		"emulator_host": "localhost:8086",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "instances")
}

func TestValidate_RejectsCredentialsAlongsideAnEmulator(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id":    "test-project",
		"emulator_host": "localhost:8086",
		"instances":     []string{"test-instance"},
		"credentials":   map[string]any{"credentials_file": "/etc/key.json"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "emulator_host")
}

func TestValidate_AcceptsAnEmulatorWithInstancesListed(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"project_id":    "test-project",
		"emulator_host": " localhost:8086 ",
		"instances":     []string{"test-instance"},
	})

	require.NoError(t, err)
	assert.Equal(t, "localhost:8086", s.config.EmulatorHost, "surrounding spaces would break the dial")
}

func TestClientOptions_TurnOffAuthForAnEmulator(t *testing.T) {
	// Three options: the endpoint, no authentication, and an insecure
	// transport, because the emulator serves plain gRPC.
	s := &Source{config: &Config{ProjectID: "test-project", EmulatorHost: "localhost:8086"}}

	assert.Len(t, s.clientOptions(), 3)
}

func TestClientOptions_AreEmptyWhenTheEnvironmentSuppliesCredentials(t *testing.T) {
	s := &Source{config: &Config{ProjectID: "analytics"}}

	assert.Empty(t, s.clientOptions(), "no options means Application Default Credentials")
}

func TestTableName_PutsTheInstanceInFront(t *testing.T) {
	assert.Equal(t, "prod-metrics.events", tableName("prod-metrics", "events"))
}

func TestConsoleURL_LinksToTheTableOverview(t *testing.T) {
	s := &Source{config: &Config{ProjectID: "analytics"}}

	assert.Equal(t,
		"https://console.cloud.google.com/bigtable/instances/prod-metrics/tables/events/overview?project=analytics",
		s.consoleURL("prod-metrics", "events"))
}

func TestConsoleURL_IsEmptyForAnEmulator(t *testing.T) {
	s := &Source{config: &Config{ProjectID: "test-project", EmulatorHost: "localhost:8086"}}

	assert.Empty(t, s.consoleURL("test-instance", "events"), "an emulator has no console page")
}

func TestSplitColumn_SeparatesFamilyFromQualifier(t *testing.T) {
	family, qualifier := splitColumn("d:name")

	assert.Equal(t, "d", family)
	assert.Equal(t, "name", qualifier)
}

func TestSplitColumn_KeepsAColonInsideTheQualifier(t *testing.T) {
	// Only the first colon separates: a qualifier is arbitrary bytes and
	// may hold colons of its own.
	family, qualifier := splitColumn("d:user:id")

	assert.Equal(t, "d", family)
	assert.Equal(t, "user:id", qualifier)
}

func TestSplitColumn_TreatsANameWithoutAColonAsAFamily(t *testing.T) {
	family, qualifier := splitColumn("d")

	assert.Equal(t, "d", family)
	assert.Empty(t, qualifier)
}

func TestInferValueType_CallsReadableBytesText(t *testing.T) {
	assert.Equal(t, "text", inferValueType([]byte("alice@example.com")))
}

func TestInferValueType_CallsAnEightByteNumberInt64(t *testing.T) {
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, 42)

	assert.Equal(t, "int64", inferValueType(value))
}

func TestInferValueType_CallsEightPrintableBytesText(t *testing.T) {
	// An eight character string is the ambiguous case, and text wins:
	// readable bytes were almost certainly written as text.
	assert.Equal(t, "text", inferValueType([]byte("12345678")))
}

func TestInferValueType_CallsAnythingElseBinary(t *testing.T) {
	assert.Equal(t, "binary", inferValueType([]byte{0x00, 0x01, 0xff, 0xfe, 0x03}))
}

func TestInferValueType_CallsAnEmptyValueBinary(t *testing.T) {
	// An empty cell carries no evidence at all, so it claims nothing.
	assert.Equal(t, "binary", inferValueType(nil))
}

func TestMergeType_TakesTheFirstTypeSeen(t *testing.T) {
	assert.Equal(t, "text", mergeType("", "text"))
}

func TestMergeType_KeepsATypeEveryRowAgreedOn(t *testing.T) {
	assert.Equal(t, "int64", mergeType("int64", "int64"))
}

func TestMergeType_FallsBackToBinaryWhenRowsDisagree(t *testing.T) {
	assert.Equal(t, "binary", mergeType("text", "int64"))
}

func TestRenderValue_ShowsReadableBytesAsTheyAre(t *testing.T) {
	assert.Equal(t, "alice", renderValue([]byte("alice")))
}

func TestRenderValue_Base64EncodesBinary(t *testing.T) {
	assert.Equal(t, "AAH//g==", renderValue([]byte{0x00, 0x01, 0xff, 0xfe}))
}

func TestGCPolicyText_RendersAMaxVersionsRule(t *testing.T) {
	text := gcPolicyText(bt.FamilyInfo{
		Name:         "d",
		GCPolicy:     "versions() > 3",
		FullGCPolicy: bt.MaxVersionsPolicy(3),
	})

	assert.Equal(t, "versions() > 3", text)
}

func TestGCPolicyText_RendersAMaxAgeRule(t *testing.T) {
	text := gcPolicyText(bt.FamilyInfo{
		Name:         "m",
		GCPolicy:     "age() > 24h",
		FullGCPolicy: bt.MaxAgePolicy(24 * time.Hour),
	})

	assert.Equal(t, "age() > 1d", text)
}

func TestGCPolicyText_RendersACompositeRule(t *testing.T) {
	text := gcPolicyText(bt.FamilyInfo{
		Name:         "d",
		FullGCPolicy: bt.UnionPolicy(bt.MaxVersionsPolicy(1), bt.MaxAgePolicy(time.Hour)),
	})

	assert.Equal(t, "(versions() > 1 || age() > 1h)", text)
}

func TestGCPolicyText_IsEmptyForAFamilyWithNoRule(t *testing.T) {
	// The client renders "no rule" two ways depending on which field is
	// read, and neither is worth putting in front of a person.
	assert.Empty(t, gcPolicyText(bt.FamilyInfo{Name: "d", FullGCPolicy: bt.NoGcPolicy()}))
	assert.Empty(t, gcPolicyText(bt.FamilyInfo{Name: "d", GCPolicy: "<never>"}))
}

func TestChangeStreamRetentionText_RendersTheRetention(t *testing.T) {
	assert.Equal(t, "24h0m0s", changeStreamRetentionText(24*time.Hour))
}

func TestChangeStreamRetentionText_IsEmptyWhenChangeStreamsAreOff(t *testing.T) {
	assert.Empty(t, changeStreamRetentionText(nil))
	assert.Empty(t, changeStreamRetentionText(time.Duration(0)))
}

func TestInstanceStateText_NamesTheStates(t *testing.T) {
	assert.Equal(t, "READY", instanceStateText(bt.Ready))
	assert.Equal(t, "CREATING", instanceStateText(bt.Creating))
	assert.Empty(t, instanceStateText(bt.NotKnown))
}

func TestInstanceTypeText_NamesTheTypes(t *testing.T) {
	assert.Equal(t, "PRODUCTION", instanceTypeText(bt.PRODUCTION))
	assert.Equal(t, "DEVELOPMENT", instanceTypeText(bt.DEVELOPMENT))
	assert.Empty(t, instanceTypeText(bt.UNSPECIFIED))
}

func TestStorageTypeText_NamesTheStorageTypes(t *testing.T) {
	assert.Equal(t, "SSD", storageTypeText(bt.SSD))
	assert.Equal(t, "HDD", storageTypeText(bt.HDD))
}

func TestRowKeyColumn_IsTheTablesPrimaryKey(t *testing.T) {
	c := rowKeyColumn()

	assert.Equal(t, "row_key", c.Name)
	assert.Equal(t, "bytes", c.DataType)
	assert.True(t, c.PrimaryKey)
	assert.False(t, c.Nullable, "every row has a key")
}
