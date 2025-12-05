package team

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTeamNotFound         = errors.New("team not found")
	ErrTeamNameExists       = errors.New("team name already exists")
	ErrMemberNotFound       = errors.New("member not found")
	ErrMemberAlreadyExists  = errors.New("member already exists")
	ErrMappingNotFound      = errors.New("sso mapping not found")
	ErrMappingAlreadyExists = errors.New("sso mapping already exists")
	ErrCannotEditSSOTeam    = errors.New("cannot edit SSO-managed team")
)

type Repository interface {
	CreateTeam(ctx context.Context, team *Team) error
	GetTeam(ctx context.Context, id string) (*Team, error)
	GetTeamByName(ctx context.Context, name string) (*Team, error)
	FindSimilarTeamNames(ctx context.Context, searchTerm string, limit int) ([]string, error)
	UpdateTeam(ctx context.Context, id string, name, description string) error
	UpdateTeamFields(ctx context.Context, id string, name, description *string, metadata map[string]interface{}, tags []string) error
	DeleteTeam(ctx context.Context, id string) error
	ListTeams(ctx context.Context, limit, offset int) ([]*Team, int, error)
	TeamExists(ctx context.Context, id string) (bool, error)

	AddMember(ctx context.Context, member *TeamMember) error
	RemoveMember(ctx context.Context, teamID, userID string) error
	UpdateMemberRole(ctx context.Context, teamID, userID, role string) error
	GetMember(ctx context.Context, teamID, userID string) (*TeamMember, error)
	ListMembers(ctx context.Context, teamID string) ([]*TeamMemberWithUser, error)
	ListUserTeams(ctx context.Context, userID string) ([]*Team, error)
	IsUserInTeam(ctx context.Context, userID, teamID string) (bool, error)
	ConvertMemberToManual(ctx context.Context, teamID, userID string) error

	CreateSSOMapping(ctx context.Context, mapping *SSOTeamMapping) error
	GetSSOMapping(ctx context.Context, id string) (*SSOTeamMapping, error)
	UpdateSSOMapping(ctx context.Context, id, teamID, memberRole string) error
	DeleteSSOMapping(ctx context.Context, id string) error
	ListSSOMappings(ctx context.Context, provider string) ([]*SSOTeamMapping, error)
	GetMappingsForGroups(ctx context.Context, provider string, groups []string) ([]*SSOTeamMapping, error)

	AddAssetOwner(ctx context.Context, assetID, ownerType, ownerID string) error
	RemoveAssetOwner(ctx context.Context, assetID, ownerType, ownerID string) error
	ListAssetOwners(ctx context.Context, assetID string) ([]*Owner, error)
	ListAssetsByOwner(ctx context.Context, ownerType, ownerID string) ([]string, error)

	SearchOwners(ctx context.Context, query string, limit int) ([]*Owner, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateTeam(ctx context.Context, team *Team) error {
	metadataJSON, err := json.Marshal(team.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	query := `
		INSERT INTO teams (name, description, metadata, tags, created_via_sso, sso_provider, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`

	err = r.db.QueryRow(ctx, query,
		team.Name,
		team.Description,
		metadataJSON,
		team.Tags,
		team.CreatedViaSSO,
		team.SSOProvider,
		team.CreatedBy,
	).Scan(&team.ID, &team.CreatedAt, &team.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrTeamNameExists
		}
		return fmt.Errorf("failed to create team: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetTeam(ctx context.Context, id string) (*Team, error) {
	query := `
		SELECT id, name, description, metadata, tags, created_via_sso, sso_provider, created_by, created_at, updated_at
		FROM teams
		WHERE id = $1`

	team := &Team{}
	var metadataJSON []byte
	err := r.db.QueryRow(ctx, query, id).Scan(
		&team.ID,
		&team.Name,
		&team.Description,
		&metadataJSON,
		&team.Tags,
		&team.CreatedViaSSO,
		&team.SSOProvider,
		&team.CreatedBy,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("failed to get team: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &team.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshaling metadata: %w", err)
	}

	return team, nil
}

func (r *PostgresRepository) GetTeamByName(ctx context.Context, name string) (*Team, error) {
	query := `
		SELECT id, name, description, metadata, tags, created_via_sso, sso_provider, created_by, created_at, updated_at
		FROM teams
		WHERE name = $1`

	team := &Team{}
	var metadataJSON []byte
	err := r.db.QueryRow(ctx, query, name).Scan(
		&team.ID,
		&team.Name,
		&team.Description,
		&metadataJSON,
		&team.Tags,
		&team.CreatedViaSSO,
		&team.SSOProvider,
		&team.CreatedBy,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if err == nil {
		if err := json.Unmarshal(metadataJSON, &team.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
		return team, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}

	queryILike := `
		SELECT id, name, description, metadata, tags, created_via_sso, sso_provider, created_by, created_at, updated_at
		FROM teams
		WHERE name ILIKE $1
		LIMIT 1`

	team = &Team{}
	err = r.db.QueryRow(ctx, queryILike, name).Scan(
		&team.ID,
		&team.Name,
		&team.Description,
		&metadataJSON,
		&team.Tags,
		&team.CreatedViaSSO,
		&team.SSOProvider,
		&team.CreatedBy,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("failed to get team (case-insensitive): %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &team.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshaling metadata: %w", err)
	}

	return team, nil
}

// FindSimilarTeamNames finds team names similar to the given search term
func (r *PostgresRepository) FindSimilarTeamNames(ctx context.Context, searchTerm string, limit int) ([]string, error) {
	if limit == 0 {
		limit = 5
	}

	query := `
		SELECT name
		FROM teams
		WHERE name ILIKE $1
		ORDER BY name
		LIMIT $2`

	// Use wildcards for partial matching
	pattern := "%" + searchTerm + "%"
	rows, err := r.db.Query(ctx, query, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find similar teams: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan team name: %w", err)
		}
		names = append(names, name)
	}

	return names, rows.Err()
}

func (r *PostgresRepository) UpdateTeam(ctx context.Context, id, name, description string) error {
	query := `
		UPDATE teams
		SET name = $1, description = $2, updated_at = NOW()
		WHERE id = $3 AND created_via_sso = FALSE
		RETURNING id`

	var returnedID string
	err := r.db.QueryRow(ctx, query, name, description, id).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			team, err := r.GetTeam(ctx, id)
			if err != nil {
				return err
			}
			if team.CreatedViaSSO {
				return ErrCannotEditSSOTeam
			}
			return ErrTeamNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrTeamNameExists
		}
		return fmt.Errorf("failed to update team: %w", err)
	}

	return nil
}

func (r *PostgresRepository) UpdateTeamFields(ctx context.Context, id string, name, description *string, metadata map[string]interface{}, tags []string) error {
	// Get existing team to check if it's SSO-managed
	team, err := r.GetTeam(ctx, id)
	if err != nil {
		return err
	}
	if team.CreatedViaSSO {
		return ErrCannotEditSSOTeam
	}

	// Build dynamic update query
	updates := []string{}
	params := []interface{}{}
	paramCount := 1

	if name != nil {
		updates = append(updates, fmt.Sprintf("name = $%d", paramCount))
		params = append(params, *name)
		paramCount++
	}

	if description != nil {
		updates = append(updates, fmt.Sprintf("description = $%d", paramCount))
		params = append(params, *description)
		paramCount++
	}

	if metadata != nil {
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshaling metadata: %w", err)
		}
		updates = append(updates, fmt.Sprintf("metadata = $%d", paramCount))
		params = append(params, metadataJSON)
		paramCount++
	}

	if tags != nil {
		updates = append(updates, fmt.Sprintf("tags = $%d", paramCount))
		params = append(params, tags)
		paramCount++
	}

	if len(updates) == 0 {
		return nil // Nothing to update
	}

	updates = append(updates, "updated_at = NOW()")
	params = append(params, id)

	query := fmt.Sprintf(`
		UPDATE teams
		SET %s
		WHERE id = $%d AND created_via_sso = FALSE
		RETURNING id`, strings.Join(updates, ", "), paramCount)

	var returnedID string
	err = r.db.QueryRow(ctx, query, params...).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTeamNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrTeamNameExists
		}
		return fmt.Errorf("failed to update team fields: %w", err)
	}

	return nil
}

func (r *PostgresRepository) DeleteTeam(ctx context.Context, id string) error {
	query := `DELETE FROM teams WHERE id = $1 AND created_via_sso = FALSE RETURNING id`

	var returnedID string
	err := r.db.QueryRow(ctx, query, id).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			team, err := r.GetTeam(ctx, id)
			if err != nil {
				return err
			}
			if team.CreatedViaSSO {
				return ErrCannotEditSSOTeam
			}
			return ErrTeamNotFound
		}
		return fmt.Errorf("failed to delete team: %w", err)
	}

	return nil
}

func (r *PostgresRepository) ListTeams(ctx context.Context, limit, offset int) ([]*Team, int, error) {
	countQuery := `SELECT COUNT(*) FROM teams`
	var total int
	err := r.db.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count teams: %w", err)
	}

	query := `
		SELECT id, name, description, metadata, tags, created_via_sso, sso_provider, created_by, created_at, updated_at
		FROM teams
		ORDER BY name
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list teams: %w", err)
	}
	defer rows.Close()

	teams := []*Team{}
	for rows.Next() {
		team := &Team{}
		var metadataJSON []byte
		err := rows.Scan(
			&team.ID,
			&team.Name,
			&team.Description,
			&metadataJSON,
			&team.Tags,
			&team.CreatedViaSSO,
			&team.SSOProvider,
			&team.CreatedBy,
			&team.CreatedAt,
			&team.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan team: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &team.Metadata); err != nil {
			return nil, 0, fmt.Errorf("unmarshaling metadata: %w", err)
		}
		teams = append(teams, team)
	}

	return teams, total, nil
}

func (r *PostgresRepository) TeamExists(ctx context.Context, id string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM teams WHERE id = $1)`
	var exists bool
	err := r.db.QueryRow(ctx, query, id).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) AddMember(ctx context.Context, member *TeamMember) error {
	query := `
		INSERT INTO team_members (team_id, user_id, role, source, sso_provider)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, joined_at`

	err := r.db.QueryRow(ctx, query,
		member.TeamID,
		member.UserID,
		member.Role,
		member.Source,
		member.SSOProvider,
	).Scan(&member.ID, &member.JoinedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrMemberAlreadyExists
		}
		return fmt.Errorf("failed to add member: %w", err)
	}

	return nil
}

func (r *PostgresRepository) RemoveMember(ctx context.Context, teamID, userID string) error {
	query := `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2 RETURNING id`

	var id string
	err := r.db.QueryRow(ctx, query, teamID, userID).Scan(&id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("failed to remove member: %w", err)
	}

	return nil
}

func (r *PostgresRepository) UpdateMemberRole(ctx context.Context, teamID, userID, role string) error {
	query := `
		UPDATE team_members
		SET role = $1
		WHERE team_id = $2 AND user_id = $3
		RETURNING id`

	var id string
	err := r.db.QueryRow(ctx, query, role, teamID, userID).Scan(&id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("failed to update member role: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetMember(ctx context.Context, teamID, userID string) (*TeamMember, error) {
	query := `
		SELECT id, team_id, user_id, role, source, sso_provider, joined_at
		FROM team_members
		WHERE team_id = $1 AND user_id = $2`

	member := &TeamMember{}
	err := r.db.QueryRow(ctx, query, teamID, userID).Scan(
		&member.ID,
		&member.TeamID,
		&member.UserID,
		&member.Role,
		&member.Source,
		&member.SSOProvider,
		&member.JoinedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMemberNotFound
		}
		return nil, fmt.Errorf("failed to get member: %w", err)
	}

	return member, nil
}

func (r *PostgresRepository) ListMembers(ctx context.Context, teamID string) ([]*TeamMemberWithUser, error) {
	query := `
		SELECT
			tm.id, tm.team_id, tm.user_id, tm.role, tm.source, tm.sso_provider, tm.joined_at,
			u.username, u.name, ui.provider_email
		FROM team_members tm
		JOIN users u ON tm.user_id = u.id
		LEFT JOIN user_identities ui ON tm.user_id = ui.user_id AND ui.provider = tm.sso_provider
		WHERE tm.team_id = $1
		ORDER BY tm.joined_at`

	rows, err := r.db.Query(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to list members: %w", err)
	}
	defer rows.Close()

	members := []*TeamMemberWithUser{}
	for rows.Next() {
		member := &TeamMemberWithUser{}
		err := rows.Scan(
			&member.ID,
			&member.TeamID,
			&member.UserID,
			&member.Role,
			&member.Source,
			&member.SSOProvider,
			&member.JoinedAt,
			&member.Username,
			&member.Name,
			&member.Email,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}
		members = append(members, member)
	}

	return members, nil
}

func (r *PostgresRepository) ListUserTeams(ctx context.Context, userID string) ([]*Team, error) {
	query := `
		SELECT t.id, t.name, t.description, t.metadata, t.tags, t.created_via_sso, t.sso_provider, t.created_by, t.created_at, t.updated_at
		FROM teams t
		JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = $1
		ORDER BY t.name`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user teams: %w", err)
	}
	defer rows.Close()

	teams := []*Team{}
	for rows.Next() {
		team := &Team{}
		var metadataJSON []byte
		err := rows.Scan(
			&team.ID,
			&team.Name,
			&team.Description,
			&metadataJSON,
			&team.Tags,
			&team.CreatedViaSSO,
			&team.SSOProvider,
			&team.CreatedBy,
			&team.CreatedAt,
			&team.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &team.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
		teams = append(teams, team)
	}

	return teams, nil
}

func (r *PostgresRepository) IsUserInTeam(ctx context.Context, userID, teamID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM team_members WHERE user_id = $1 AND team_id = $2)`
	var exists bool
	err := r.db.QueryRow(ctx, query, userID, teamID).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) ConvertMemberToManual(ctx context.Context, teamID, userID string) error {
	query := `
		UPDATE team_members
		SET source = $1, sso_provider = NULL
		WHERE team_id = $2 AND user_id = $3 AND source = $4
		RETURNING id`

	var id string
	err := r.db.QueryRow(ctx, query, SourceManual, teamID, userID, SourceSSO).Scan(&id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("failed to convert member: %w", err)
	}

	return nil
}

func (r *PostgresRepository) CreateSSOMapping(ctx context.Context, mapping *SSOTeamMapping) error {
	query := `
		INSERT INTO sso_team_mappings (provider, sso_group_name, team_id, member_role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRow(ctx, query,
		mapping.Provider,
		mapping.SSOGroupName,
		mapping.TeamID,
		mapping.MemberRole,
	).Scan(&mapping.ID, &mapping.CreatedAt, &mapping.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrMappingAlreadyExists
		}
		return fmt.Errorf("failed to create sso mapping: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetSSOMapping(ctx context.Context, id string) (*SSOTeamMapping, error) {
	query := `
		SELECT id, provider, sso_group_name, team_id, member_role, created_at, updated_at
		FROM sso_team_mappings
		WHERE id = $1`

	mapping := &SSOTeamMapping{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&mapping.ID,
		&mapping.Provider,
		&mapping.SSOGroupName,
		&mapping.TeamID,
		&mapping.MemberRole,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMappingNotFound
		}
		return nil, fmt.Errorf("failed to get sso mapping: %w", err)
	}

	return mapping, nil
}

func (r *PostgresRepository) UpdateSSOMapping(ctx context.Context, id, teamID, memberRole string) error {
	query := `
		UPDATE sso_team_mappings
		SET team_id = $1, member_role = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING id`

	var returnedID string
	err := r.db.QueryRow(ctx, query, teamID, memberRole, id).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMappingNotFound
		}
		return fmt.Errorf("failed to update sso mapping: %w", err)
	}

	return nil
}

func (r *PostgresRepository) DeleteSSOMapping(ctx context.Context, id string) error {
	query := `DELETE FROM sso_team_mappings WHERE id = $1 RETURNING id`

	var returnedID string
	err := r.db.QueryRow(ctx, query, id).Scan(&returnedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMappingNotFound
		}
		return fmt.Errorf("failed to delete sso mapping: %w", err)
	}

	return nil
}

func (r *PostgresRepository) ListSSOMappings(ctx context.Context, provider string) ([]*SSOTeamMapping, error) {
	query := `
		SELECT id, provider, sso_group_name, team_id, member_role, created_at, updated_at
		FROM sso_team_mappings
		WHERE provider = $1
		ORDER BY sso_group_name`

	rows, err := r.db.Query(ctx, query, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to list sso mappings: %w", err)
	}
	defer rows.Close()

	mappings := []*SSOTeamMapping{}
	for rows.Next() {
		mapping := &SSOTeamMapping{}
		err := rows.Scan(
			&mapping.ID,
			&mapping.Provider,
			&mapping.SSOGroupName,
			&mapping.TeamID,
			&mapping.MemberRole,
			&mapping.CreatedAt,
			&mapping.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sso mapping: %w", err)
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}

func (r *PostgresRepository) GetMappingsForGroups(ctx context.Context, provider string, groups []string) ([]*SSOTeamMapping, error) {
	query := `
		SELECT id, provider, sso_group_name, team_id, member_role, created_at, updated_at
		FROM sso_team_mappings
		WHERE provider = $1 AND sso_group_name = ANY($2)`

	rows, err := r.db.Query(ctx, query, provider, groups)
	if err != nil {
		return nil, fmt.Errorf("failed to get mappings for groups: %w", err)
	}
	defer rows.Close()

	mappings := []*SSOTeamMapping{}
	for rows.Next() {
		mapping := &SSOTeamMapping{}
		err := rows.Scan(
			&mapping.ID,
			&mapping.Provider,
			&mapping.SSOGroupName,
			&mapping.TeamID,
			&mapping.MemberRole,
			&mapping.CreatedAt,
			&mapping.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sso mapping: %w", err)
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}

func (r *PostgresRepository) AddAssetOwner(ctx context.Context, assetID, ownerType, ownerID string) error {
	var query string
	if ownerType == OwnerTypeUser {
		query = `
			INSERT INTO asset_owners (asset_id, user_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING`
	} else {
		query = `
			INSERT INTO asset_owners (asset_id, team_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING`
	}

	_, err := r.db.Exec(ctx, query, assetID, ownerID)
	if err != nil {
		return fmt.Errorf("failed to add asset owner: %w", err)
	}

	_, err = r.db.Exec(ctx, `UPDATE assets SET updated_at = NOW() WHERE id = $1`, assetID)
	if err != nil {
		return fmt.Errorf("failed to update asset timestamp: %w", err)
	}

	return nil
}

func (r *PostgresRepository) RemoveAssetOwner(ctx context.Context, assetID, ownerType, ownerID string) error {
	var query string
	if ownerType == OwnerTypeUser {
		query = `DELETE FROM asset_owners WHERE asset_id = $1 AND user_id = $2`
	} else {
		query = `DELETE FROM asset_owners WHERE asset_id = $1 AND team_id = $2`
	}

	result, err := r.db.Exec(ctx, query, assetID, ownerID)
	if err != nil {
		return fmt.Errorf("failed to remove asset owner: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("owner not found")
	}

	_, err = r.db.Exec(ctx, `UPDATE assets SET updated_at = NOW() WHERE id = $1`, assetID)
	if err != nil {
		return fmt.Errorf("failed to update asset timestamp: %w", err)
	}

	return nil
}

func (r *PostgresRepository) ListAssetOwners(ctx context.Context, assetID string) ([]*Owner, error) {
	query := `
		SELECT
			CASE
				WHEN ao.user_id IS NOT NULL THEN 'user'
				ELSE 'team'
			END as owner_type,
			COALESCE(ao.user_id, ao.team_id) as owner_id,
			COALESCE(u.name, t.name) as name,
			ui.provider_email
		FROM asset_owners ao
		LEFT JOIN users u ON ao.user_id = u.id
		LEFT JOIN teams t ON ao.team_id = t.id
		LEFT JOIN user_identities ui ON ao.user_id = ui.user_id
		WHERE ao.asset_id = $1
		ORDER BY owner_type, name`

	rows, err := r.db.Query(ctx, query, assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to list asset owners: %w", err)
	}
	defer rows.Close()

	owners := []*Owner{}
	for rows.Next() {
		owner := &Owner{}
		err := rows.Scan(
			&owner.Type,
			&owner.ID,
			&owner.Name,
			&owner.Email,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan owner: %w", err)
		}
		owners = append(owners, owner)
	}

	return owners, nil
}

func (r *PostgresRepository) ListAssetsByOwner(ctx context.Context, ownerType, ownerID string) ([]string, error) {
	var query string
	if ownerType == OwnerTypeUser {
		query = `
			SELECT asset_id
			FROM asset_owners
			WHERE user_id = $1
			ORDER BY created_at DESC`
	} else {
		query = `
			SELECT asset_id
			FROM asset_owners
			WHERE team_id = $1
			ORDER BY created_at DESC`
	}

	rows, err := r.db.Query(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list assets by owner: %w", err)
	}
	defer rows.Close()

	assetIDs := []string{}
	for rows.Next() {
		var assetID string
		if err := rows.Scan(&assetID); err != nil {
			return nil, fmt.Errorf("failed to scan asset id: %w", err)
		}
		assetIDs = append(assetIDs, assetID)
	}

	return assetIDs, nil
}

func (r *PostgresRepository) SearchOwners(ctx context.Context, query string, limit int) ([]*Owner, error) {
	owners := []*Owner{}

	// Search users
	usersQuery := `
		SELECT 'user' as type, u.id, u.name, ui.provider_email
		FROM users u
		LEFT JOIN user_identities ui ON u.id = ui.user_id
		WHERE u.name ILIKE '%' || $1 || '%' OR u.username ILIKE '%' || $1 || '%'
		ORDER BY u.name
		LIMIT $2`

	userRows, err := r.db.Query(ctx, usersQuery, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer userRows.Close()

	for userRows.Next() {
		owner := &Owner{}
		if err := userRows.Scan(&owner.Type, &owner.ID, &owner.Name, &owner.Email); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		owners = append(owners, owner)
	}

	// Search teams
	teamsQuery := `
		SELECT 'team' as type, id, name, NULL as email
		FROM teams
		WHERE name ILIKE '%' || $1 || '%'
		ORDER BY name
		LIMIT $2`

	teamRows, err := r.db.Query(ctx, teamsQuery, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search teams: %w", err)
	}
	defer teamRows.Close()

	for teamRows.Next() {
		owner := &Owner{}
		if err := teamRows.Scan(&owner.Type, &owner.ID, &owner.Name, &owner.Email); err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}
		owners = append(owners, owner)
	}

	return owners, nil
}
