package asset

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseWriter(t *testing.T) {
	assert.Equal(t, WriterInfo{Kind: "pipeline", Pipeline: "shop-pg", Source: "postgresql", Scheduled: true}, ParseWriter("run:shop-pg/postgresql#schedule:42"))
	assert.Equal(t, WriterInfo{Kind: "pipeline", Pipeline: "local-duckdb", Source: "duckdb", PrincipalType: "user", PrincipalID: "u1", Zone: "alice-laptop"}, ParseWriter("run:local-duckdb/duckdb#user:u1/alice-laptop"))
	assert.Equal(t, WriterInfo{Kind: "pipeline", Pipeline: "shop-pg", Source: "postgresql"}, ParseWriter("run:shop-pg/postgresql"))
	assert.Equal(t, WriterInfo{Kind: "api", PrincipalType: "service_account", PrincipalID: "sa1"}, ParseWriter("api:service_account:sa1"))
	assert.Equal(t, WriterInfo{Kind: "openlineage"}, ParseWriter("openlineage"))
	assert.Equal(t, WriterInfo{Kind: "other"}, ParseWriter("something-else"))
}

func TestMergeContributionsRanksWritersAndUnionsTags(t *testing.T) {
	now := time.Now()
	native, manual, ref := "from the database", "by a person", "from openlineage"
	merged := MergeContributions([]Contribution{
		{Writer: "openlineage", Role: RoleReference, LastSeenAt: now, Fields: ContributionFields{Description: &ref, Metadata: map[string]interface{}{"namespace": "x", "owner": "ol"}, Tags: []string{"openlineage"}}},
		{Writer: "run:pg/postgresql", Role: RoleNative, LastSeenAt: now, Fields: ContributionFields{Description: &native, Metadata: map[string]interface{}{"owner": "db", "rows": 1}, Tags: []string{"pii"}}},
		{Writer: "api:user:1", Role: RoleManual, LastSeenAt: now, Fields: ContributionFields{Description: &manual, Metadata: map[string]interface{}{"owner": "team"}}},
	})
	assert.Equal(t, manual, *merged.Description)
	assert.Equal(t, map[string]interface{}{"namespace": "x", "owner": "team", "rows": 1}, merged.Metadata)
	assert.ElementsMatch(t, []string{"pii", "openlineage"}, merged.Tags)
}
