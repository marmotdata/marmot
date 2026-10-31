package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// MaxJobTitleLength matches the column, so an over-long title is refused with a clear error rather than a database one.
const MaxJobTitleLength = 255

// ProfileTeam is a team the user belongs to, with where the membership came from so the account page can say when an identity provider group put them there.
type ProfileTeam struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Source      string `json:"source"`
	SSOProvider string `json:"sso_provider,omitempty"`
	SSOGroup    string `json:"sso_group,omitempty"`
} // @name ProfileTeam

// ProfileRoleSource names an identity provider group that maps to one of the user's roles. Sign-in replaces mapped roles with what the groups give, so for someone who signs in with that provider this is where the role came from.
type ProfileRoleSource struct {
	Role     string `json:"role"`
	Provider string `json:"provider"`
	Group    string `json:"group"`
} // @name ProfileRoleSource

// Profile holds the parts of an account that people describe themselves with, kept apart from User so the hot authentication path does not load them.
type Profile struct {
	JobTitle    string              `json:"job_title"`
	Teams       []ProfileTeam       `json:"teams"`
	RoleSources []ProfileRoleSource `json:"role_sources"`
} // @name UserProfile

func (s *service) GetProfile(ctx context.Context, userID string) (*Profile, error) {
	return s.repo.GetProfile(ctx, userID)
}

func (s *service) UpdateJobTitle(ctx context.Context, userID string, jobTitle string) error {
	jobTitle = strings.TrimSpace(jobTitle)
	if utf8.RuneCountInString(jobTitle) > MaxJobTitleLength {
		return &InvalidInputError{Fields: []FieldError{{Field: "job_title", Message: fmt.Sprintf("must be at most %d characters", MaxJobTitleLength)}}}
	}
	return s.repo.UpdateJobTitle(ctx, userID, jobTitle)
}

func (r *PostgresRepository) GetProfile(ctx context.Context, userID string) (*Profile, error) {
	profile := &Profile{Teams: []ProfileTeam{}, RoleSources: []ProfileRoleSource{}}
	err := r.db.QueryRow(ctx, `SELECT COALESCE(job_title, '') FROM users WHERE id = $1`, userID).Scan(&profile.JobTitle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("getting job title: %w", err)
	}

	// The group comes from the team's SSO mapping for the provider that added the member; a team can map several groups, so the first by name is shown.
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.name, tm.role, tm.source, COALESCE(tm.sso_provider, ''),
		       COALESCE((SELECT m.sso_group_name FROM sso_team_mappings m
		                  WHERE m.team_id = t.id AND m.provider = tm.sso_provider
		                  ORDER BY m.sso_group_name LIMIT 1), '')
		  FROM team_members tm
		  JOIN teams t ON t.id = tm.team_id
		 WHERE tm.user_id = $1
		 ORDER BY t.name`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing teams: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t ProfileTeam
		if err := rows.Scan(&t.ID, &t.Name, &t.Role, &t.Source, &t.SSOProvider, &t.SSOGroup); err != nil {
			return nil, fmt.Errorf("scanning team: %w", err)
		}
		if t.Source != "sso" {
			t.SSOGroup = ""
		}
		profile.Teams = append(profile.Teams, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating teams: %w", err)
	}

	// Only providers the user actually signs in with count, so a mapping for some other identity provider is never shown as the source.
	srcRows, err := r.db.Query(ctx, `
		SELECT r.name, m.provider, m.sso_group_name
		  FROM user_roles ur
		  JOIN roles r ON r.id = ur.role_id
		  JOIN sso_team_mappings m ON m.role_id = r.id
		  JOIN user_identities ui ON ui.user_id = ur.user_id AND ui.provider = m.provider
		 WHERE ur.user_id = $1
		 ORDER BY r.name, m.sso_group_name`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing role sources: %w", err)
	}
	defer srcRows.Close()
	for srcRows.Next() {
		var src ProfileRoleSource
		if err := srcRows.Scan(&src.Role, &src.Provider, &src.Group); err != nil {
			return nil, fmt.Errorf("scanning role source: %w", err)
		}
		profile.RoleSources = append(profile.RoleSources, src)
	}
	if err := srcRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating role sources: %w", err)
	}
	return profile, nil
}

func (r *PostgresRepository) UpdateJobTitle(ctx context.Context, userID string, jobTitle string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET job_title = NULLIF($2, ''), updated_at = NOW() WHERE id = $1`,
		userID, jobTitle)
	if err != nil {
		return fmt.Errorf("updating job title: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
