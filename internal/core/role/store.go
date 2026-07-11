package role

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound      = errors.New("role not found")
	ErrAlreadyExists = errors.New("role already exists")
)

type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	IsSystem    bool         `json:"is_system"`
	UserCount   int          `json:"user_count,omitempty"`
	Permissions []Permission `json:"permissions,omitempty"`
	DeletedAt   *time.Time   `json:"deleted_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
} // @name Role

type Permission struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ResourceType string `json:"resource_type"`
	Action       string `json:"action"`
} // @name RolePermission

type CreateInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	PermIDs     []string `json:"permission_ids,omitempty"`
}

type UpdateInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type Store interface {
	List(ctx context.Context, includeDeleted bool) ([]*Role, error)
	Get(ctx context.Context, id string) (*Role, error)
	GetByName(ctx context.Context, name string) (*Role, error)
	Create(ctx context.Context, input CreateInput) (*Role, error)
	Update(ctx context.Context, id string, input UpdateInput) (*Role, error)
	SoftDelete(ctx context.Context, id string) error
	AttachedPermissionIDs(ctx context.Context, roleID string) ([]string, error)
	ReplacePermissions(ctx context.Context, roleID string, permIDs []string) error
	HasUsers(ctx context.Context, roleID string) (bool, error)
	ListPermissions(ctx context.Context) ([]Permission, error)
}

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(db *pgxpool.Pool) Store {
	return &PostgresStore{db: db}
}

const roleSelectBase = `
	SELECT r.id, r.name, r.description, r.is_system, r.deleted_at, r.created_at, r.updated_at,
	       COUNT(DISTINCT ur.user_id) AS user_count,
	       COALESCE(json_agg(json_build_object(
	           'id', p.id, 'name', p.name, 'description', p.description,
	           'resource_type', p.resource_type, 'action', p.action
	       )) FILTER (WHERE p.id IS NOT NULL), '[]'::json) AS permissions
	FROM roles r
	LEFT JOIN user_roles ur ON ur.role_id = r.id
	LEFT JOIN role_permissions rp ON rp.role_id = r.id
	LEFT JOIN permissions p ON p.id = rp.permission_id`

func (s *PostgresStore) List(ctx context.Context, includeDeleted bool) ([]*Role, error) {
	q := roleSelectBase
	if !includeDeleted {
		q += ` WHERE r.deleted_at IS NULL`
	}
	q += ` GROUP BY r.id ORDER BY r.created_at ASC`

	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}
	defer rows.Close()
	return scanRoleRows(rows)
}

func (s *PostgresStore) Get(ctx context.Context, id string) (*Role, error) {
	q := roleSelectBase + ` WHERE r.id = $1 GROUP BY r.id`
	rows, err := s.db.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("getting role: %w", err)
	}
	defer rows.Close()

	roles, err := scanRoleRows(rows)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, ErrNotFound
	}
	return roles[0], nil
}

func (s *PostgresStore) GetByName(ctx context.Context, name string) (*Role, error) {
	q := roleSelectBase + ` WHERE r.name = $1 AND r.deleted_at IS NULL GROUP BY r.id`
	rows, err := s.db.Query(ctx, q, name)
	if err != nil {
		return nil, fmt.Errorf("getting role by name: %w", err)
	}
	defer rows.Close()

	roles, err := scanRoleRows(rows)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, ErrNotFound
	}
	return roles[0], nil
}

func (s *PostgresStore) Create(ctx context.Context, input CreateInput) (*Role, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx,
		`INSERT INTO roles (name, description) VALUES ($1, $2) RETURNING id`,
		input.Name, input.Description,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("inserting role: %w", err)
	}

	if len(input.PermIDs) > 0 {
		if _, err := tx.Exec(ctx,
			`INSERT INTO role_permissions (role_id, permission_id) SELECT $1, unnest($2::uuid[])`,
			id, input.PermIDs,
		); err != nil {
			return nil, fmt.Errorf("attaching permissions: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return s.Get(ctx, id)
}

func (s *PostgresStore) Update(ctx context.Context, id string, input UpdateInput) (*Role, error) {
	if input.Name == nil && input.Description == nil {
		return s.Get(ctx, id)
	}

	sets := []string{}
	args := []any{}
	n := 1

	if input.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", n))
		args = append(args, *input.Name)
		n++
	}
	if input.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", n))
		args = append(args, *input.Description)
		n++
	}
	sets = append(sets, fmt.Sprintf("updated_at = $%d", n))
	args = append(args, time.Now())
	n++
	args = append(args, id)

	q := fmt.Sprintf(
		"UPDATE roles SET %s WHERE id = $%d AND deleted_at IS NULL",
		strings.Join(sets, ", "), n,
	)

	tag, err := s.db.Exec(ctx, q, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("updating role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}

	return s.Get(ctx, id)
}

func (s *PostgresStore) SoftDelete(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE roles SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft-deleting role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) AttachedPermissionIDs(ctx context.Context, roleID string) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT permission_id FROM role_permissions WHERE role_id = $1`, roleID)
	if err != nil {
		return nil, fmt.Errorf("querying permission IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning permission ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *PostgresStore) ReplacePermissions(ctx context.Context, roleID string, permIDs []string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return fmt.Errorf("clearing permissions: %w", err)
	}

	if len(permIDs) > 0 {
		if _, err := tx.Exec(ctx,
			`INSERT INTO role_permissions (role_id, permission_id) SELECT $1, unnest($2::uuid[])`,
			roleID, permIDs,
		); err != nil {
			return fmt.Errorf("inserting permissions: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) HasUsers(ctx context.Context, roleID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM user_roles WHERE role_id = $1)`, roleID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking role users: %w", err)
	}
	return exists, nil
}

func (s *PostgresStore) ListPermissions(ctx context.Context) ([]Permission, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, name, description, resource_type, action FROM permissions ORDER BY resource_type, action`)
	if err != nil {
		return nil, fmt.Errorf("listing permissions: %w", err)
	}
	defer rows.Close()

	var perms []Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.ResourceType, &p.Action); err != nil {
			return nil, fmt.Errorf("scanning permission: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func scanRoleRows(rows pgx.Rows) ([]*Role, error) {
	var roles []*Role
	for rows.Next() {
		var r Role
		var permsJSON []byte
		if err := rows.Scan(
			&r.ID, &r.Name, &r.Description, &r.IsSystem,
			&r.DeletedAt, &r.CreatedAt, &r.UpdatedAt,
			&r.UserCount, &permsJSON,
		); err != nil {
			return nil, fmt.Errorf("scanning role: %w", err)
		}
		if len(permsJSON) > 0 && string(permsJSON) != "null" {
			if err := json.Unmarshal(permsJSON, &r.Permissions); err != nil {
				return nil, fmt.Errorf("parsing permissions: %w", err)
			}
		}
		roles = append(roles, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating roles: %w", err)
	}
	return roles, nil
}
