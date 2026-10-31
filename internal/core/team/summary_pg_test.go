package team

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
	"github.com/marmotdata/marmot/pkg/config"
)

func TestGroupMappingsAndSummaries_Postgres(t *testing.T) {
	runGroupMappingChecks(t, pgtest.TempDB(t))
}

// runGroupMappingChecks covers the counts on the team list and group mappings that give a role, a team or both, including that a role-only mapping never touches team membership at sign-in.
func runGroupMappingChecks(t *testing.T, db *pgxpool.Pool) {
	ctx := context.Background()
	svc := NewService(NewPostgresRepository(db))

	var userID, adminRoleID string
	if err := db.QueryRow(ctx, `INSERT INTO users (username, name) VALUES ('grace', 'Grace') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT id FROM roles WHERE name = 'admin'`).Scan(&adminRoleID); err != nil {
		t.Fatal(err)
	}

	platform, err := svc.CreateTeam(ctx, "Data Platform", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateTeam(ctx, "Finance", "", nil); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.CreateGroupMapping(ctx, GroupMappingInput{Provider: "okta", SSOGroupName: "nobody"}); err != ErrEmptyGroupMapping {
		t.Fatalf("empty mapping: err = %v", err)
	}
	both, err := svc.CreateGroupMapping(ctx, GroupMappingInput{Provider: "okta", SSOGroupName: "data-platform", TeamID: platform.ID, RoleID: adminRoleID})
	if err != nil {
		t.Fatalf("CreateGroupMapping: %v", err)
	}
	if both.TeamName == nil || *both.TeamName != "Data Platform" || both.RoleName == nil || *both.RoleName != "admin" || both.MemberRole != RoleMember {
		t.Fatalf("mapping = %+v", both)
	}
	roleOnly, err := svc.CreateGroupMapping(ctx, GroupMappingInput{Provider: "okta", SSOGroupName: "marmot-admins", RoleID: adminRoleID})
	if err != nil || roleOnly.TeamID != "" {
		t.Fatalf("role-only mapping = %+v, %v", roleOnly, err)
	}

	all, err := svc.ListSSOMappings(ctx, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("ListSSOMappings(all) = %d, %v", len(all), err)
	}

	if err := svc.SyncUserTeamsFromSSO(ctx, userID, "okta", []string{"data-platform", "marmot-admins"}, config.TeamSyncConfig{}); err != nil {
		t.Fatalf("SyncUserTeamsFromSSO: %v", err)
	}
	roles, err := svc.RoleNamesForGroups(ctx, "okta", []string{"data-platform", "marmot-admins"})
	if err != nil || len(roles) != 1 || roles[0] != "admin" {
		t.Fatalf("RoleNamesForGroups = %v, %v", roles, err)
	}

	if _, err := db.Exec(ctx, `INSERT INTO team_webhooks (team_id, name, provider, webhook_url) VALUES ($1, 'alerts', 'slack', 'https://example.com/hook')`, platform.ID); err != nil {
		t.Fatal(err)
	}
	assetID := pgtest.SeedAsset(t, db)
	if err := svc.AddAssetOwner(ctx, assetID, OwnerTypeTeam, platform.ID); err != nil {
		t.Fatal(err)
	}

	sums, total, err := svc.ListTeamSummaries(ctx, 50, 0)
	if err != nil || total != 2 {
		t.Fatalf("ListTeamSummaries = %d, %v", total, err)
	}
	got := map[string]*TeamSummary{}
	for _, s := range sums {
		got[s.Name] = s
	}
	p := got["Data Platform"]
	if p.MemberCount != 1 || p.AssetCount != 1 || p.WebhookCount != 1 || len(p.SSOGroups) != 1 || p.SSOGroups[0].SSOGroupName != "data-platform" {
		t.Fatalf("Data Platform summary = %+v", p)
	}
	if f := got["Finance"]; f.MemberCount != 0 || f.AssetCount != 0 || len(f.SSOGroups) != 0 {
		t.Fatalf("Finance summary = %+v", f)
	}

	// Dropping the team from a mapping keeps the role it gives.
	updated, err := svc.UpdateGroupMapping(ctx, both.ID, GroupMappingInput{RoleID: adminRoleID})
	if err != nil || updated.TeamID != "" || updated.RoleName == nil {
		t.Fatalf("UpdateGroupMapping = %+v, %v", updated, err)
	}
	if err := svc.AddDefaultMember(ctx, platform.ID, userID); err != nil {
		t.Fatalf("AddDefaultMember on an existing member: %v", err)
	}
}
