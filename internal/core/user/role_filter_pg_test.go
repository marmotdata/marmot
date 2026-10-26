package user

import (
	"context"
	"testing"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestListUsersFiltersByRole(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	var adminRole string
	var seededAdmins int
	if err := pool.QueryRow(ctx, "SELECT r.id, count(ur.user_id) FROM roles r LEFT JOIN user_roles ur ON ur.role_id = r.id WHERE r.name = 'admin' GROUP BY r.id").Scan(&adminRole, &seededAdmins); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ana", "bob"} {
		if _, err := pool.Exec(ctx, "INSERT INTO users (username, name) VALUES ($1, $1)", name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, "INSERT INTO user_roles (user_id, role_id) SELECT id, $1 FROM users WHERE username = 'ana'", adminRole); err != nil {
		t.Fatal(err)
	}

	users, total, err := NewPostgresRepository(pool).ListUsers(ctx, Filter{RoleIDs: []string{adminRole}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	want := seededAdmins + 1
	if total != want || len(users) != want {
		t.Fatalf("got %d users (total %d), want %d admins", len(users), total, want)
	}
	for _, u := range users {
		if u.Username == "bob" {
			t.Fatal("bob has no admin role but was listed")
		}
	}
}
