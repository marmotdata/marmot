package domain

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const roleColumns = `a.id, a.domain_id, d.name, a.subject_type, a.subject_id::text,
	COALESCE(u.name, t.name, sa.name, ''),
	u.id IS NULL AND t.id IS NULL AND sa.id IS NULL,
	a.role, a.created_by, a.created_at`

const roleJoins = `domain_role_assignments a
	JOIN domains d ON d.id = a.domain_id
	LEFT JOIN users u ON a.subject_type = 'user' AND u.id = a.subject_id
	LEFT JOIN teams t ON a.subject_type = 'team' AND t.id = a.subject_id
	LEFT JOIN service_accounts sa ON a.subject_type = 'service_account' AND sa.id = a.subject_id AND sa.deleted_at IS NULL`

func scanRole(row pgx.Row) (*RoleAssignment, error) {
	var r RoleAssignment
	err := row.Scan(&r.ID, &r.DomainID, &r.DomainName, &r.SubjectType, &r.SubjectID,
		&r.SubjectName, &r.SubjectMissing, &r.Role, &r.CreatedBy, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// Grants returns the domain paths a subject holds a role over, directly or,
// for a user, through its teams.
func (r *PostgresRepository) Grants(ctx context.Context, subject SubjectType, subjectID string) ([]Grant, error) {
	rows, err := r.db.Query(ctx, `
		SELECT d.path, a.role
		  FROM domain_role_assignments a
		  JOIN domains d ON d.id = a.domain_id
		 WHERE (a.subject_type = $1 AND a.subject_id::text = $2)
		    OR ($1 = 'user' AND a.subject_type = 'team'
		        AND a.subject_id IN (SELECT team_id FROM team_members WHERE user_id::text = $2))`,
		string(subject), subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := []Grant{}
	for rows.Next() {
		var g Grant
		if err := rows.Scan(&g.Path, &g.Role); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

// Roles lists the assignments on d and on its ancestors, nearest last.
func (r *PostgresRepository) Roles(ctx context.Context, d *Domain) ([]RoleAssignment, error) {
	ids := strings.Split(strings.Trim(d.Path, "/"), "/")
	rows, err := r.db.Query(ctx, `
		SELECT `+roleColumns+`
		  FROM `+roleJoins+`
		 WHERE a.domain_id::text = ANY($1::text[])
		 ORDER BY d.depth, a.role, 6`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RoleAssignment{}
	for rows.Next() {
		ra, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		ra.Inherited = ra.DomainID != d.ID
		out = append(out, *ra)
	}
	return out, rows.Err()
}

var subjectTables = map[SubjectType]string{
	SubjectUser:           "users WHERE id::text = $1",
	SubjectTeam:           "teams WHERE id::text = $1",
	SubjectServiceAccount: "service_accounts WHERE id::text = $1 AND deleted_at IS NULL",
}

func (r *PostgresRepository) SubjectExists(ctx context.Context, subject SubjectType, id string) (bool, error) {
	from, ok := subjectTables[subject]
	if !ok {
		return false, ErrInvalidInput
	}
	var exists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+from+")", id).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) GrantRole(ctx context.Context, d *Domain, in GrantInput, createdBy string) (*RoleAssignment, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO domain_role_assignments (domain_id, subject_type, subject_id, role, created_by)
		VALUES ($1, $2, $3::uuid, $4, NULLIF($5, ''))
		RETURNING id`, d.ID, string(in.SubjectType), in.SubjectID, string(in.Role), createdBy).Scan(&id)
	switch pgCode(err) {
	case "23505":
		return nil, ErrDuplicate
	case "22P02":
		return nil, ErrEntityNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("granting role: %w", err)
	}
	return scanRole(r.db.QueryRow(ctx, "SELECT "+roleColumns+" FROM "+roleJoins+" WHERE a.id = $1", id))
}

func (r *PostgresRepository) RevokeRole(ctx context.Context, domainID, assignmentID string) error {
	tag, err := r.db.Exec(ctx, "DELETE FROM domain_role_assignments WHERE id::text = $1 AND domain_id::text = $2", assignmentID, domainID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
