package teams

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/marmotdata/marmot/internal/core/user"
)

// Only CreateTeam is reachable from createTeam, so the rest of the interface is
// embedded rather than stubbed out method by method.
type stubTeamRepo struct {
	team.Repository
	created *team.Team
}

func (s *stubTeamRepo) CreateTeam(_ context.Context, t *team.Team) error {
	t.ID = "team1"
	s.created = t
	return nil
}

func createTeamRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/teams", strings.NewReader(`{"name":"platform"}`))
	return req.WithContext(ctx)
}

// WithAuth sets UserContextKey only when the principal has a user behind it, so a
// service-account key reaches this handler with a principal and no user. Reading
// the user unconditionally panicked, which net/http turns into a dropped
// connection rather than a response.
func TestCreateTeam_PrincipalWithoutUser(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		wantCreatedBy *string
	}{
		{
			name: "service account records no creator",
			ctx: context.WithValue(context.Background(), common.PrincipalContextKey,
				auth.NewServiceAccountPrincipal("sa1", "ci-bot", []string{"admin"}, nil)),
			wantCreatedBy: nil,
		},
		{
			name: "user records itself as creator",
			ctx: context.WithValue(context.Background(), common.UserContextKey,
				&user.User{ID: "user1", Active: true}),
			wantCreatedBy: strPtr("user1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubTeamRepo{}
			h := &Handler{teamService: team.NewService(repo)}
			rec := httptest.NewRecorder()

			h.createTeam(rec, createTeamRequest(tt.ctx))

			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
			}
			if repo.created == nil {
				t.Fatal("no team reached the repository")
			}
			switch {
			case tt.wantCreatedBy == nil && repo.created.CreatedBy != nil:
				t.Fatalf("created_by = %q, want NULL", *repo.created.CreatedBy)
			case tt.wantCreatedBy != nil && repo.created.CreatedBy == nil:
				t.Fatalf("created_by = NULL, want %q", *tt.wantCreatedBy)
			case tt.wantCreatedBy != nil && *repo.created.CreatedBy != *tt.wantCreatedBy:
				t.Fatalf("created_by = %q, want %q", *repo.created.CreatedBy, *tt.wantCreatedBy)
			}

			var body team.Team
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decoding response: %v", err)
			}
			if body.ID != "team1" {
				t.Fatalf("response id = %q, want team1", body.ID)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
