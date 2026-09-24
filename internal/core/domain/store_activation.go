package domain

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/marmotdata/marmot/internal/core/auth"
)

func (r *PostgresRepository) All(ctx context.Context) ([]*Domain, error) {
	rows, err := r.db.Query(ctx, "SELECT "+columns+" FROM domains ORDER BY depth, lower(name)")
	if err != nil {
		return nil, err
	}
	return scanDomains(rows)
}

func (r *PostgresRepository) EnforcementState(ctx context.Context) (*EnforcementState, error) {
	var state EnforcementState
	var by *string
	var at time.Time
	err := r.db.QueryRow(ctx, "SELECT value = 'true'::jsonb, updated_by, updated_at FROM domain_settings WHERE key = $1", writeEnforcementKey).
		Scan(&state.Write, &by, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return &state, nil
	}
	if err != nil {
		return nil, err
	}
	state.UpdatedBy, state.UpdatedAt = by, &at
	return &state, nil
}

// EditorPrincipals lists active users and service accounts whose roles grant
// assets:manage or glossary:manage, leaving out holders of the admin role,
// who keep global scope under enforcement.
func (r *PostgresRepository) EditorPrincipals(ctx context.Context) ([]EditorPrincipal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT 'user', u.id::text, COALESCE(NULLIF(u.name, ''), u.username),
		       array_agg(DISTINCT p.resource_type || ':' || p.action ORDER BY p.resource_type || ':' || p.action)
		  FROM users u
		  JOIN user_roles ur ON ur.user_id = u.id
		  JOIN role_permissions rp ON rp.role_id = ur.role_id
		  JOIN permissions p ON p.id = rp.permission_id
		 WHERE u.active
		   AND (p.resource_type, p.action) IN (('assets', 'manage'), ('glossary', 'manage'))
		   AND NOT EXISTS (SELECT 1 FROM user_roles a JOIN roles ar ON ar.id = a.role_id WHERE a.user_id = u.id AND ar.name = $1)
		 GROUP BY u.id, u.name, u.username
		UNION ALL
		SELECT 'service_account', sa.id::text, sa.name,
		       array_agg(DISTINCT p.resource_type || ':' || p.action ORDER BY p.resource_type || ':' || p.action)
		  FROM service_accounts sa
		  JOIN service_account_roles sr ON sr.service_account_id = sa.id
		  JOIN role_permissions rp ON rp.role_id = sr.role_id
		  JOIN permissions p ON p.id = rp.permission_id
		 WHERE sa.active AND sa.deleted_at IS NULL
		   AND (p.resource_type, p.action) IN (('assets', 'manage'), ('glossary', 'manage'))
		   AND NOT EXISTS (SELECT 1 FROM service_account_roles a JOIN roles ar ON ar.id = a.role_id WHERE a.service_account_id = sa.id AND ar.name = $1)
		 GROUP BY sa.id, sa.name
		 ORDER BY 1, 3`, auth.AdminRoleName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EditorPrincipal
	for rows.Next() {
		var e EditorPrincipal
		if err := rows.Scan(&e.SubjectType, &e.SubjectID, &e.Name, &e.Permissions); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PipelinesOutsideDomain counts, per schedule, the assets its runs ingested
// that now sit outside the schedule's domain subtree.
func (r *PostgresRepository) PipelinesOutsideDomain(ctx context.Context) ([]PlanPipeline, error) {
	rows, err := r.db.Query(ctx, `
		WITH schedules AS (
			SELECT s.id, s.name, d.id AS domain_id, d.path
			  FROM ingestion_schedules s
			  LEFT JOIN ingestion_schedule_domains sd ON sd.schedule_id = s.id
			  JOIN domains d ON d.id = COALESCE(sd.domain_id, $1::uuid)
		), pipeline_runs AS (
			SELECT jr.schedule_id, jr.plugin_run_id AS run_id
			  FROM ingestion_job_runs jr
			 WHERE jr.plugin_run_id IS NOT NULL
			UNION
			SELECT s.id, r.id
			  FROM runs r
			  JOIN ingestion_schedules s ON s.name = r.pipeline_name
		)
		SELECT s.id::text, s.name, s.domain_id::text, count(DISTINCT a.id)
		  FROM schedules s
		  JOIN pipeline_runs pr ON pr.schedule_id = s.id
		  JOIN run_checkpoints c ON c.run_id = pr.run_id AND c.entity_type = 'asset'
		  JOIN assets a ON a.mrn = c.entity_mrn
		  LEFT JOIN asset_domains ad ON ad.asset_id = a.id
		  LEFT JOIN domains ass ON ass.id = ad.domain_id
		 WHERE COALESCE(ass.path, $2) NOT LIKE s.path || '%'
		 GROUP BY s.id, s.name, s.domain_id
		 ORDER BY s.name`, UnassignedID, unassignedPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PlanPipeline
	for rows.Next() {
		var p PlanPipeline
		if err := rows.Scan(&p.ScheduleID, &p.Name, &p.Domain.ID, &p.AssetsOutside); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
