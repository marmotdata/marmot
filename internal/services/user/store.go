package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Repository interface {
	CreateUser(ctx context.Context, user *User, password string) error
	GetUser(ctx context.Context, id string) (*User, error)
	GetUserByUsername(ctx context.Context, email string) (*User, error)
	GetUserByProviderID(ctx context.Context, provider string, providerUserID string) (*User, error)
	UpdateUser(ctx context.Context, id string, updates map[string]interface{}) error
	UpdatePreferences(ctx context.Context, userID string, preferences map[string]interface{}) error
	DeleteUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context, filter Filter) ([]*User, int, error)

	CreateUserIdentity(ctx context.Context, identity *UserIdentity) error
	GetUserIdentities(ctx context.Context, userID string) ([]*UserIdentity, error)
	DeleteUserIdentity(ctx context.Context, userID string, provider string) error

	CreateAPIKey(ctx context.Context, apiKey *APIKey, keyHash string) error
	GetAPIKey(ctx context.Context, id string) (*APIKey, error)
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id string) error
	DeleteAPIKey(ctx context.Context, id string) error
	ListAPIKeys(ctx context.Context, userID string) ([]*APIKey, error)

	AssignRoles(ctx context.Context, userID string, roleNames []string) error
	UpdateRoles(ctx context.Context, userID string, roleNames []string) error
	HasPermission(ctx context.Context, userID string, resourceType string, action string) (bool, error)
	ValidatePassword(ctx context.Context, userID string, password string) error

	UsernameExists(ctx context.Context, username string) (bool, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

// Helper function to scan a user from a row
func scanUser(row pgx.Row) (*User, error) {
	var user User
	var preferencesJSON, rolesJSON []byte

	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.Name,
		&user.Active,
		&preferencesJSON,
		&user.CreatedAt,
		&user.UpdatedAt,
		&rolesJSON,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("scanning user: %w", err)
	}

	// Initialize preferences map
	user.Preferences = make(map[string]interface{})
	if preferencesJSON != nil {
		if err := json.Unmarshal(preferencesJSON, &user.Preferences); err != nil {
			return nil, fmt.Errorf("parsing preferences: %w", err)
		}
	}

	if err := json.Unmarshal(rolesJSON, &user.Roles); err != nil {
		return nil, fmt.Errorf("parsing roles: %w", err)
	}

	return &user, nil
}

// Helper function to scan multiple users from rows
func scanUsers(rows pgx.Rows) ([]*User, error) {
	var users []*User
	for rows.Next() {
		var user User
		var preferencesJSON, rolesJSON []byte
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Name,
			&user.Active,
			&preferencesJSON,
			&user.CreatedAt,
			&user.UpdatedAt,
			&rolesJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}

		// Initialize preferences map
		user.Preferences = make(map[string]interface{})
		if preferencesJSON != nil {
			if err := json.Unmarshal(preferencesJSON, &user.Preferences); err != nil {
				return nil, fmt.Errorf("parsing preferences: %w", err)
			}
		}

		if err := json.Unmarshal(rolesJSON, &user.Roles); err != nil {
			return nil, fmt.Errorf("parsing roles: %w", err)
		}

		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating users: %w", err)
	}

	return users, nil
}

func (r *PostgresRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)",
		username,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking username existence: %w", err)
	}
	return exists, nil
}

func (r *PostgresRepository) CreateUser(ctx context.Context, user *User, passwordHash string) error {
	preferencesJSON, err := json.Marshal(user.Preferences)
	if err != nil {
		return fmt.Errorf("marshaling preferences: %w", err)
	}

	query := `
    INSERT INTO users (username, name, password_hash, active, preferences, created_at, updated_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7)
    RETURNING id`

	err = r.db.QueryRow(ctx, query,
		user.Username,
		user.Name,
		passwordHash,
		user.Active,
		preferencesJSON,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(&user.ID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyExists
		}
		return fmt.Errorf("creating user: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetUser(ctx context.Context, id string) (*User, error) {
	query := `
		WITH user_roles AS (
			SELECT ur.user_id,
				   json_agg(json_build_object(
					   'id', r.id,
					   'name', r.name,
					   'description', r.description,
					   'permissions', (
						   SELECT json_agg(json_build_object(
							   'id', p.id,
							   'name', p.name,
							   'description', p.description,
							   'resource_type', p.resource_type,
							   'action', p.action
						   ))
						   FROM permissions p
						   JOIN role_permissions rp ON rp.permission_id = p.id
						   WHERE rp.role_id = r.id
					   )
				   )) as roles
			FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = $1
			GROUP BY ur.user_id
		)
		SELECT u.id, u.username, u.name, u.active, u.preferences, u.created_at, u.updated_at,
			   COALESCE(ur.roles, '[]'::json)
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		WHERE u.id = $1`

	return scanUser(r.db.QueryRow(ctx, query, id))
}

func (r *PostgresRepository) ListUsers(ctx context.Context, filter Filter) ([]*User, int, error) {
	conditions := []string{"1=1"}
	args := []interface{}{}
	argNum := 1

	if filter.Query != "" {
		conditions = append(conditions, fmt.Sprintf("(username ILIKE $%d OR name ILIKE $%d)", argNum, argNum))
		args = append(args, "%"+filter.Query+"%")
		argNum++
	}

	if len(filter.RoleIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("EXISTS (SELECT 1 FROM user_roles WHERE user_id = u.id AND role_id = ANY($%d))", argNum))
		args = append(args, filter.RoleIDs)
		argNum++
	}

	if filter.Active != nil {
		conditions = append(conditions, fmt.Sprintf("active = $%d", argNum))
		args = append(args, *filter.Active)
		argNum++
	}

	// Count total matching users
	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT u.id)
		FROM users u
		WHERE %s`, strings.Join(conditions, " AND "))

	var total int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting users: %w", err)
	}

	// Query users with pagination
	query := fmt.Sprintf(`
		WITH user_roles AS (
			SELECT ur.user_id,
				   json_agg(json_build_object(
					   'id', r.id,
					   'name', r.name,
					   'description', r.description
				   )) as roles
			FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			GROUP BY ur.user_id
		)
		SELECT u.id, u.username, u.name, u.active, u.preferences, u.created_at, u.updated_at,
			   COALESCE(ur.roles, '[]'::json)
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		WHERE %s
		ORDER BY u.created_at DESC
		LIMIT $%d OFFSET $%d`, strings.Join(conditions, " AND "), argNum, argNum+1)

	// Append pagination parameters
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying users: %w", err)
	}
	defer rows.Close()

	users, err := scanUsers(rows)
	if err != nil {
		return nil, 0, fmt.Errorf("scanning users: %w", err)
	}

	return users, total, nil
}

func (r *PostgresRepository) UpdateUser(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	setStatements := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates)+1)
	argNum := 1

	for field, value := range updates {
		setStatements = append(setStatements, fmt.Sprintf("%s = $%d", field, argNum))
		args = append(args, value)
		argNum++
	}

	// Add updated_at only if it's not already being updated
	if _, ok := updates["updated_at"]; !ok {
		setStatements = append(setStatements, "updated_at = NOW()")
	}

	query := fmt.Sprintf(`
		UPDATE users
		SET %s
		WHERE id = $%d`,
		strings.Join(setStatements, ", "),
		argNum,
	)

	args = append(args, id)

	commandTag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyExists
		}
		return fmt.Errorf("updating user: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *PostgresRepository) UpdatePreferences(ctx context.Context, userID string, preferences map[string]interface{}) error {
	var currentPrefsJSON []byte
	err := r.db.QueryRow(ctx,
		"SELECT COALESCE(preferences, '{}'::jsonb) FROM users WHERE id = $1",
		userID,
	).Scan(&currentPrefsJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("getting current preferences: %w", err)
	}

	// Unmarshal current preferences
	var currentPrefs map[string]interface{}
	if err := json.Unmarshal(currentPrefsJSON, &currentPrefs); err != nil {
		return fmt.Errorf("unmarshaling current preferences: %w", err)
	}

	// If currentPrefs is nil, initialize it
	if currentPrefs == nil {
		currentPrefs = make(map[string]interface{})
	}

	// Update the preferences map with new values
	for k, v := range preferences {
		currentPrefs[k] = v
	}

	// Marshal the updated preferences to JSON
	prefsJSON, err := json.Marshal(currentPrefs)
	if err != nil {
		return fmt.Errorf("marshaling preferences: %w", err)
	}

	// Update the entire preferences column
	commandTag, err := r.db.Exec(ctx,
		"UPDATE users SET preferences = $1::jsonb, updated_at = NOW() WHERE id = $2",
		prefsJSON, userID,
	)
	if err != nil {
		return fmt.Errorf("updating preferences: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *PostgresRepository) DeleteUser(ctx context.Context, id string) error {
	commandTag, err := r.db.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *PostgresRepository) AssignRoles(ctx context.Context, userID string, roleNames []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Delete existing roles
	if _, err := tx.Exec(ctx, "DELETE FROM user_roles WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("deleting existing roles: %w", err)
	}

	// Insert new roles
	query := `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = ANY($2)`

	if _, err := tx.Exec(ctx, query, userID, roleNames); err != nil {
		return fmt.Errorf("assigning roles: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) HasPermission(ctx context.Context, userID string, resourceType string, action string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id = ur.role_id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE ur.user_id = $1
			AND p.resource_type = $2
			AND p.action = $3
		)`

	var hasPermission bool
	err := r.db.QueryRow(ctx, query, userID, resourceType, action).Scan(&hasPermission)
	if err != nil {
		return false, fmt.Errorf("checking permission: %w", err)
	}

	return hasPermission, nil
}

func (r *PostgresRepository) ValidatePassword(ctx context.Context, userID string, password string) error {
	var storedHash string
	err := r.db.QueryRow(ctx,
		"SELECT password_hash FROM users WHERE id = $1",
		userID,
	).Scan(&storedHash)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("getting password hash: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)); err != nil {
		return ErrInvalidPassword
	}

	return nil
}

func (r *PostgresRepository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	query := `
		WITH user_roles AS (
			SELECT ur.user_id,
				   json_agg(json_build_object(
					   'id', r.id,
					   'name', r.name,
					   'description', r.description
				   )) as roles
			FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			GROUP BY ur.user_id
		)
		SELECT u.id, u.username, u.name, u.active, u.preferences, u.created_at, u.updated_at,
			   COALESCE(ur.roles, '[]'::json)
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		WHERE u.username = $1`

	return scanUser(r.db.QueryRow(ctx, query, username))
}

func (r *PostgresRepository) GetUserByProviderID(ctx context.Context, provider string, providerUserID string) (*User, error) {
	query := `
		WITH user_roles AS (
			SELECT ur.user_id,
				   json_agg(json_build_object(
					   'id', r.id,
					   'name', r.name,
					   'description', r.description
				   )) as roles
			FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			GROUP BY ur.user_id
		)
		SELECT u.id, u.username, u.name, u.active, u.preferences, u.created_at, u.updated_at,
			   COALESCE(ur.roles, '[]'::json)
		FROM users u
		JOIN user_identities ui ON ui.user_id = u.id
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		WHERE ui.provider = $1 AND ui.provider_user_id = $2`

	return scanUser(r.db.QueryRow(ctx, query, provider, providerUserID))
}

func (r *PostgresRepository) UpdateRoles(ctx context.Context, userID string, roleNames []string) error {
	return r.AssignRoles(ctx, userID, roleNames)
}

func (r *PostgresRepository) CreateUserIdentity(ctx context.Context, identity *UserIdentity) error {
	providerDataJSON, err := json.Marshal(identity.ProviderData)
	if err != nil {
		return fmt.Errorf("marshaling provider data: %w", err)
	}

	query := `
        INSERT INTO user_identities (
            user_id, provider, provider_user_id, provider_email, provider_data,
            created_at, updated_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id`

	err = r.db.QueryRow(ctx, query,
		identity.UserID,
		identity.Provider,
		identity.ProviderUserID,
		identity.ProviderEmail,
		providerDataJSON,
		identity.CreatedAt,
		identity.UpdatedAt,
	).Scan(&identity.ID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyExists
		}
		return fmt.Errorf("creating user identity: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetUserIdentities(ctx context.Context, userID string) ([]*UserIdentity, error) {
	query := `
        SELECT id, user_id, provider, provider_user_id, provider_email, 
               provider_data, created_at, updated_at
        FROM user_identities
        WHERE user_id = $1`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("querying user identities: %w", err)
	}
	defer rows.Close()

	var identities []*UserIdentity
	for rows.Next() {
		var identity UserIdentity
		var providerDataJSON []byte

		err := rows.Scan(
			&identity.ID,
			&identity.UserID,
			&identity.Provider,
			&identity.ProviderUserID,
			&identity.ProviderEmail,
			&providerDataJSON,
			&identity.CreatedAt,
			&identity.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning user identity: %w", err)
		}

		// Initialize the provider data map
		identity.ProviderData = make(map[string]interface{})

		// Only try to unmarshal if we have provider data
		if len(providerDataJSON) > 0 {
			if err := json.Unmarshal(providerDataJSON, &identity.ProviderData); err != nil {
				return nil, fmt.Errorf("parsing provider data: %w", err)
			}
		}

		identities = append(identities, &identity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user identities: %w", err)
	}

	return identities, nil
}

func (r *PostgresRepository) DeleteUserIdentity(ctx context.Context, userID string, provider string) error {
	commandTag, err := r.db.Exec(ctx,
		"DELETE FROM user_identities WHERE user_id = $1 AND provider = $2",
		userID, provider)
	if err != nil {
		return fmt.Errorf("deleting user identity: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *PostgresRepository) CreateAPIKey(ctx context.Context, apiKey *APIKey, keyHash string) error {
	query := `
		INSERT INTO api_keys (
			user_id, name, key_hash, expires_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	err := r.db.QueryRow(ctx, query,
		apiKey.UserID,
		apiKey.Name,
		keyHash,
		apiKey.ExpiresAt,
		apiKey.CreatedAt,
	).Scan(&apiKey.ID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyExists
		}
		return fmt.Errorf("creating API key: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetAPIKey(ctx context.Context, id string) (*APIKey, error) {
	var apiKey APIKey
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, name, expires_at, last_used_at, created_at
		FROM api_keys
		WHERE id = $1`,
		id,
	).Scan(
		&apiKey.ID,
		&apiKey.UserID,
		&apiKey.Name,
		&apiKey.ExpiresAt,
		&apiKey.LastUsedAt,
		&apiKey.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("getting API key: %w", err)
	}

	return &apiKey, nil
}

func (r *PostgresRepository) GetAPIKeyByHash(ctx context.Context, keyToValidate string) (*APIKey, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, user_id, name, key_hash, expires_at, last_used_at, created_at
        FROM api_keys
        WHERE (expires_at IS NULL OR expires_at > NOW())`)
	if err != nil {
		return nil, fmt.Errorf("querying API keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var apiKey APIKey
		var keyHash string
		err := rows.Scan(
			&apiKey.ID,
			&apiKey.UserID,
			&apiKey.Name,
			&keyHash,
			&apiKey.ExpiresAt,
			&apiKey.LastUsedAt,
			&apiKey.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning API key: %w", err)
		}

		if err := bcrypt.CompareHashAndPassword([]byte(keyHash), []byte(keyToValidate)); err == nil {
			return &apiKey, nil
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating API keys: %w", err)
	}

	return nil, ErrUserNotFound
}

func (r *PostgresRepository) UpdateAPIKeyLastUsed(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx,
		"UPDATE api_keys SET last_used_at = NOW() WHERE id = $1",
		id)
	if err != nil {
		return fmt.Errorf("updating API key last used: %w", err)
	}
	return nil
}

func (r *PostgresRepository) DeleteAPIKey(ctx context.Context, id string) error {
	commandTag, err := r.db.Exec(ctx,
		"DELETE FROM api_keys WHERE id = $1",
		id,
	)
	if err != nil {
		return fmt.Errorf("deleting API key: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *PostgresRepository) ListAPIKeys(ctx context.Context, userID string) ([]*APIKey, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, name, expires_at, last_used_at, created_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("querying API keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		var key APIKey
		err := rows.Scan(
			&key.ID,
			&key.UserID,
			&key.Name,
			&key.ExpiresAt,
			&key.LastUsedAt,
			&key.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning API key: %w", err)
		}
		keys = append(keys, &key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating API keys: %w", err)
	}

	return keys, nil
}
