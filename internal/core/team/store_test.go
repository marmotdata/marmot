package team

import (
	"context"
	"testing"

	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestSearchOwnersByExactID(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	users := user.NewPostgresRepository(pool)
	teams := NewPostgresRepository(pool)

	created := &user.User{Username: "steward", Name: "Data Steward"}
	if err := users.CreateUser(ctx, created, "hash"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// A stored reference value is an id, not text a person typed; it must
	// resolve on its own even though it never appears in name or username.
	byID, err := teams.SearchOwners(ctx, created.ID, 10)
	if err != nil {
		t.Fatalf("SearchOwners by id: %v", err)
	}
	if len(byID) != 1 || byID[0].ID != created.ID || byID[0].Type != "user" {
		t.Fatalf("SearchOwners by id = %+v, want exactly %q", byID, created.ID)
	}

	byName, err := teams.SearchOwners(ctx, "Data Steward", 10)
	if err != nil {
		t.Fatalf("SearchOwners by name: %v", err)
	}
	if len(byName) != 1 || byName[0].ID != created.ID {
		t.Fatalf("SearchOwners by name = %+v, want exactly %q", byName, created.ID)
	}

	miss, err := teams.SearchOwners(ctx, "00000000-0000-0000-0000-000000000000", 10)
	if err != nil {
		t.Fatalf("SearchOwners by unknown id: %v", err)
	}
	if len(miss) != 0 {
		t.Fatalf("SearchOwners by unknown id = %+v, want none", miss)
	}
}
