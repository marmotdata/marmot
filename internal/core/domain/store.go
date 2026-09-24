package domain

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, in CreateInput) (*Domain, error)
	Get(ctx context.Context, id string) (*Domain, error)
	Children(ctx context.Context, parentID *string) ([]*Domain, error)
	Named(ctx context.Context, name string) ([]*Domain, error)
	Subtree(ctx context.Context, id string) ([]*Domain, error)
	Update(ctx context.Context, id string, in UpdateInput) (*Domain, error)
	Delete(ctx context.Context, id string) error
	Move(ctx context.Context, id string, parentID *string) (*Domain, error)
	Assign(ctx context.Context, kind Kind, entityIDs []string, domainID string) error
	DomainOf(ctx context.Context, kind Kind, entityID string) (string, error)
	PipelineDomain(ctx context.Context, pipelineName string) (string, bool, error)
	PipelineAssetsIn(ctx context.Context, scheduleID, domainID string) ([]string, error)
	EntityExists(ctx context.Context, kind Kind, id string) (bool, error)
	Grants(ctx context.Context, subject SubjectType, subjectID string) ([]Grant, error)
	Roles(ctx context.Context, d *Domain) ([]RoleAssignment, error)
	SubjectExists(ctx context.Context, subject SubjectType, id string) (bool, error)
	GrantRole(ctx context.Context, d *Domain, in GrantInput, createdBy string) (*RoleAssignment, error)
	RevokeRole(ctx context.Context, domainID, assignmentID string) error
	AssignPipeline(ctx context.Context, scheduleID, domainID string, moveAssets bool) (int, error)
	ImportCandidates(ctx context.Context, kind Kind, path []string) ([]ImportCandidate, error)
	ApplyImport(ctx context.Context, plan map[Kind]map[string][]string) error
	WriteEnforced(ctx context.Context) (bool, error)
	SetWriteEnforced(ctx context.Context, on bool, by string) error
	Placements(ctx context.Context, kind Kind, ids []string) (map[string]Placement, error)
	TermPlacements(ctx context.Context, names []string) (map[string]Placement, error)
	Audit(ctx context.Context, entries []AuditEntry) error
	AuditLog(ctx context.Context, entityKind, entityID string) ([]AuditEntry, error)
	AssetIDsByMRN(ctx context.Context, mrns []string) (map[string]string, error)
	DocOwner(ctx context.Context, pageID, imageID string) (entityType, entityID string, found bool, err error)
	All(ctx context.Context) ([]*Domain, error)
	EnforcementState(ctx context.Context) (*EnforcementState, error)
	EditorPrincipals(ctx context.Context) ([]EditorPrincipal, error)
	PipelinesOutsideDomain(ctx context.Context) ([]PlanPipeline, error)
}

type membership struct {
	table, column, columnType, entityTable string
	// live excludes soft-deleted entities, where the kind has them.
	live string
}

var memberships = map[Kind]membership{
	KindAsset:             {"asset_domains", "asset_id", "varchar", "assets", "true"},
	KindDataProduct:       {"data_product_domains", "data_product_id", "uuid", "data_products", "true"},
	KindGlossaryTerm:      {"glossary_term_domains", "glossary_term_id", "uuid", "glossary_terms", "e.deleted_at IS NULL"},
	KindIngestionSchedule: {"ingestion_schedule_domains", "schedule_id", "uuid", "ingestion_schedules", "true"},
}

// treeLock serializes structural changes. A move rewrites the paths of a
// subtree in one statement; a concurrent insert under that subtree would keep
// the old prefix, and two crossing moves could form a cycle.
const treeLock = 7_361_024_190_551

const columns = `id, parent_id, path, depth, name, description, metadata, tags, restricted, created_by, created_at, updated_at`

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func scanDomain(row pgx.Row) (*Domain, error) {
	var d Domain
	err := row.Scan(&d.ID, &d.ParentID, &d.Path, &d.Depth, &d.Name, &d.Description,
		&d.Metadata, &d.Tags, &d.Restricted, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func scanDomains(rows pgx.Rows) ([]*Domain, error) {
	defer rows.Close()
	out := []*Domain{}
	for rows.Next() {
		d, err := scanDomain(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// notFound maps a missing row, and an id that is not a valid UUID, to target.
func notFound(err error, target error) error {
	if errors.Is(err, pgx.ErrNoRows) || pgCode(err) == "22P02" {
		return target
	}
	return err
}

func lockTree(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", treeLock)
	return err
}

func nodePosition(ctx context.Context, tx pgx.Tx, id string) (path string, depth int, err error) {
	err = tx.QueryRow(ctx, "SELECT path, depth FROM domains WHERE id = $1 FOR UPDATE", id).Scan(&path, &depth)
	return path, depth, notFound(err, ErrNotFound)
}

func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (*Domain, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockTree(ctx, tx); err != nil {
		return nil, err
	}

	parentPath, parentDepth := "/", 0
	if in.ParentID != nil {
		if parentPath, parentDepth, err = nodePosition(ctx, tx, *in.ParentID); err != nil {
			return nil, err
		}
	}
	if parentDepth+1 > MaxDepth {
		return nil, ErrTooDeep
	}
	metadata, tags := in.Metadata, in.Tags
	if metadata == nil {
		metadata = map[string]any{}
	}
	if tags == nil {
		tags = []string{}
	}

	d, err := scanDomain(tx.QueryRow(ctx, `
		WITH g AS (SELECT gen_random_uuid() AS id)
		INSERT INTO domains (id, parent_id, path, depth, name, description, metadata, tags, created_by)
		SELECT g.id, $1::uuid, $2::text || g.id::text || '/', $3::int, $4::text, $5::text, $6::jsonb, $7::text[], NULLIF($8::text, '') FROM g
		RETURNING `+columns,
		in.ParentID, parentPath, parentDepth+1, in.Name, in.Description, metadata, tags, in.CreatedBy))
	if pgCode(err) == "23505" {
		return nil, ErrNameConflict
	}
	if err != nil {
		return nil, fmt.Errorf("inserting domain: %w", err)
	}
	return d, tx.Commit(ctx)
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (*Domain, error) {
	d, err := scanDomain(r.db.QueryRow(ctx, "SELECT "+columns+" FROM domains WHERE id = $1", id))
	return d, notFound(err, ErrNotFound)
}

func (r *PostgresRepository) Children(ctx context.Context, parentID *string) ([]*Domain, error) {
	var rows pgx.Rows
	var err error
	if parentID == nil {
		rows, err = r.db.Query(ctx, "SELECT "+columns+" FROM domains WHERE parent_id IS NULL ORDER BY lower(name)")
	} else {
		rows, err = r.db.Query(ctx, "SELECT "+columns+" FROM domains WHERE parent_id = $1 ORDER BY lower(name)", *parentID)
	}
	if err != nil {
		return nil, notFound(err, ErrNotFound)
	}
	return scanDomains(rows)
}

func (r *PostgresRepository) Named(ctx context.Context, name string) ([]*Domain, error) {
	rows, err := r.db.Query(ctx, "SELECT "+columns+" FROM domains WHERE lower(name) = lower($1) ORDER BY depth, lower(name)", name)
	if err != nil {
		return nil, err
	}
	return scanDomains(rows)
}

func (r *PostgresRepository) Subtree(ctx context.Context, id string) ([]*Domain, error) {
	root, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, "SELECT "+columns+" FROM domains WHERE path LIKE $1::text || '%' ORDER BY depth, lower(name)", root.Path)
	if err != nil {
		return nil, err
	}
	return scanDomains(rows)
}

func (r *PostgresRepository) Update(ctx context.Context, id string, in UpdateInput) (*Domain, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanDomain(tx.QueryRow(ctx, "SELECT "+columns+" FROM domains WHERE id = $1 FOR UPDATE", id))
	if err != nil {
		return nil, notFound(err, ErrNotFound)
	}
	if in.Name != nil {
		current.Name = *in.Name
	}
	if in.Description != nil {
		current.Description = in.Description
	}
	if in.Metadata != nil {
		current.Metadata = in.Metadata
	}
	if in.Tags != nil {
		current.Tags = in.Tags
	}

	d, err := scanDomain(tx.QueryRow(ctx, `
		UPDATE domains SET name = $2, description = $3, metadata = $4, tags = $5, updated_at = now()
		WHERE id = $1 RETURNING `+columns,
		id, current.Name, current.Description, current.Metadata, current.Tags))
	if pgCode(err) == "23505" {
		return nil, ErrNameConflict
	}
	if err != nil {
		return nil, fmt.Errorf("updating domain: %w", err)
	}
	return d, tx.Commit(ctx)
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockTree(ctx, tx); err != nil {
		return err
	}
	if _, _, err := nodePosition(ctx, tx, id); err != nil {
		return err
	}

	var hasChildren bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM domains WHERE parent_id = $1)", id).Scan(&hasChildren); err != nil {
		return err
	}
	if hasChildren {
		return ErrHasChildren
	}
	for _, m := range memberships {
		// A soft-deleted entity keeps its membership row but is invisible;
		// it must not pin the domain forever.
		if _, err := tx.Exec(ctx, "DELETE FROM "+m.table+" mm USING "+m.entityTable+" e WHERE e.id = mm."+m.column+" AND mm.domain_id = $1 AND NOT ("+m.live+")", id); err != nil {
			return fmt.Errorf("dropping memberships of deleted %s: %w", m.entityTable, err)
		}
		var hasMembers bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+m.table+" WHERE domain_id = $1)", id).Scan(&hasMembers); err != nil {
			return err
		}
		if hasMembers {
			return ErrNotEmpty
		}
	}

	if _, err := tx.Exec(ctx, "DELETE FROM domains WHERE id = $1", id); err != nil {
		if pgCode(err) == "23503" {
			return ErrNotEmpty
		}
		return fmt.Errorf("deleting domain: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Move(ctx context.Context, id string, parentID *string) (*Domain, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockTree(ctx, tx); err != nil {
		return nil, err
	}

	oldPath, oldDepth, err := nodePosition(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	parentPath, parentDepth := "/", 0
	if parentID != nil {
		if parentPath, parentDepth, err = nodePosition(ctx, tx, *parentID); err != nil {
			return nil, err
		}
		if len(parentPath) >= len(oldPath) && parentPath[:len(oldPath)] == oldPath {
			return nil, ErrCycle
		}
	}

	var deepest int
	if err := tx.QueryRow(ctx, "SELECT max(depth) FROM domains WHERE path LIKE $1::text || '%'", oldPath).Scan(&deepest); err != nil {
		return nil, err
	}
	shift := parentDepth + 1 - oldDepth
	if deepest+shift > MaxDepth {
		return nil, ErrTooDeep
	}

	_, err = tx.Exec(ctx, `
		UPDATE domains
		   SET path = $3::text || substr(path, $4::int),
		       depth = depth + $5::int,
		       parent_id = CASE WHEN id = $1 THEN $2::uuid ELSE parent_id END,
		       updated_at = CASE WHEN id = $1 THEN now() ELSE updated_at END
		 WHERE path LIKE $6::text || '%'`,
		id, parentID, parentPath+id+"/", len(oldPath)+1, shift, oldPath)
	if pgCode(err) == "23505" {
		return nil, ErrNameConflict
	}
	if err != nil {
		return nil, fmt.Errorf("moving domain: %w", err)
	}

	d, err := scanDomain(tx.QueryRow(ctx, "SELECT "+columns+" FROM domains WHERE id = $1", id))
	if err != nil {
		return nil, err
	}
	return d, tx.Commit(ctx)
}

// Assign sets the owning domain of every listed entity, all or none.
// Assigning Unassigned removes membership rows, since a missing row already
// means Unassigned.
func (r *PostgresRepository) Assign(ctx context.Context, kind Kind, entityIDs []string, domainID string) error {
	m, ok := memberships[kind]
	if !ok {
		return ErrInvalidInput
	}
	ids := unique(entityIDs)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Comparing as text keeps a malformed UUID a plain miss, not a cast error.
	var found int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+m.entityTable+" WHERE id::text = ANY($1::text[])", ids).Scan(&found); err != nil {
		return err
	}
	if found != len(ids) {
		return ErrEntityNotFound
	}

	if domainID == UnassignedID {
		_, err = tx.Exec(ctx, "DELETE FROM "+m.table+" WHERE "+m.column+"::text = ANY($1::text[])", ids)
	} else {
		_, err = tx.Exec(ctx, `
			INSERT INTO `+m.table+` (`+m.column+`, domain_id)
			SELECT unnest($1::text[])::`+m.columnType+`, $2::uuid
			ON CONFLICT (`+m.column+`) DO UPDATE SET domain_id = EXCLUDED.domain_id, assigned_at = now()`,
			ids, domainID)
	}
	switch pgCode(err) {
	case "23503", "22P02":
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("assigning %s to domain: %w", kind, err)
	}
	return tx.Commit(ctx)
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func (r *PostgresRepository) DomainOf(ctx context.Context, kind Kind, entityID string) (string, error) {
	m, ok := memberships[kind]
	if !ok {
		return "", ErrInvalidInput
	}
	var id string
	err := r.db.QueryRow(ctx, "SELECT domain_id FROM "+m.table+" WHERE "+m.column+" = $1", entityID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) || pgCode(err) == "22P02" {
		return UnassignedID, nil
	}
	return id, err
}

// PipelineDomain returns the domain assigned to the ingestion schedule named
// pipelineName. Runs started by the scheduler use the schedule name as their
// pipeline name; other pipelines have no schedule and report found=false.
func (r *PostgresRepository) PipelineDomain(ctx context.Context, pipelineName string) (string, bool, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		SELECT sd.domain_id
		  FROM ingestion_schedule_domains sd
		  JOIN ingestion_schedules s ON s.id = sd.schedule_id
		 WHERE s.name = $1`, pipelineName).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

func (r *PostgresRepository) ImportCandidates(ctx context.Context, kind Kind, path []string) ([]ImportCandidate, error) {
	m, ok := memberships[kind]
	if !ok {
		return nil, ErrInvalidInput
	}
	rows, err := r.db.Query(ctx, `
		SELECT e.id::text, e.metadata #>> $1::text[], mm.`+m.column+` IS NOT NULL
		  FROM `+m.entityTable+` e
		  LEFT JOIN `+m.table+` mm ON mm.`+m.column+` = e.id
		 WHERE COALESCE(e.metadata #>> $1::text[], '') <> '' AND `+m.live, path)
	if err != nil {
		return nil, fmt.Errorf("reading import candidates for %s: %w", kind, err)
	}
	defer rows.Close()
	var out []ImportCandidate
	for rows.Next() {
		var c ImportCandidate
		if err := rows.Scan(&c.ID, &c.Value, &c.Assigned); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ApplyImport assigns every planned entity in one transaction. An entity
// assigned since the plan was read keeps that assignment.
func (r *PostgresRepository) ApplyImport(ctx context.Context, plan map[Kind]map[string][]string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for kind, byDomain := range plan {
		m, ok := memberships[kind]
		if !ok {
			return ErrInvalidInput
		}
		for domainID, ids := range byDomain {
			if _, err := tx.Exec(ctx, `
				INSERT INTO `+m.table+` (`+m.column+`, domain_id)
				SELECT unnest($1::text[])::`+m.columnType+`, $2::uuid
				ON CONFLICT (`+m.column+`) DO NOTHING`, ids, domainID); err != nil {
				return fmt.Errorf("importing %s memberships: %w", kind, err)
			}
		}
	}
	return tx.Commit(ctx)
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// pipelineAssetsIn lists the assets a schedule has ingested that are in
// domainID now. Runs are found through the job that started them, which
// survives a rename, and by the schedule's current name, which covers runs
// reported from outside the scheduler.
func pipelineAssetsIn(ctx context.Context, q querier, scheduleID, domainID string) ([]string, error) {
	rows, err := q.Query(ctx, `
		WITH pipeline_runs AS (
			SELECT jr.plugin_run_id AS run_id
			  FROM ingestion_job_runs jr
			 WHERE jr.schedule_id::text = $1 AND jr.plugin_run_id IS NOT NULL
			UNION
			SELECT r.id
			  FROM runs r
			  JOIN ingestion_schedules s ON s.name = r.pipeline_name
			 WHERE s.id::text = $1
		)
		SELECT DISTINCT a.id
		  FROM run_checkpoints c
		  JOIN pipeline_runs pr ON pr.run_id = c.run_id
		  JOIN assets a ON a.mrn = c.entity_mrn
		  LEFT JOIN asset_domains ad ON ad.asset_id = a.id
		 WHERE c.entity_type = 'asset'
		   AND COALESCE(ad.domain_id, $2::uuid) = $3::uuid`,
		scheduleID, UnassignedID, domainID)
	if err != nil {
		return nil, fmt.Errorf("listing pipeline assets: %w", err)
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *PostgresRepository) PipelineAssetsIn(ctx context.Context, scheduleID, domainID string) ([]string, error) {
	return pipelineAssetsIn(ctx, r.db, scheduleID, domainID)
}

func (r *PostgresRepository) AssignPipeline(ctx context.Context, scheduleID, domainID string, moveAssets bool) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM ingestion_schedules WHERE id::text = $1)", scheduleID).Scan(&exists); err != nil {
		return 0, err
	}
	if !exists {
		return 0, ErrEntityNotFound
	}

	current := UnassignedID
	err = tx.QueryRow(ctx, "SELECT domain_id FROM ingestion_schedule_domains WHERE schedule_id::text = $1 FOR UPDATE", scheduleID).Scan(&current)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	if current == domainID {
		return 0, nil
	}

	var assets []string
	if moveAssets {
		if assets, err = pipelineAssetsIn(ctx, tx, scheduleID, current); err != nil {
			return 0, err
		}
	}

	set := func(m membership, ids []string) error {
		if len(ids) == 0 {
			return nil
		}
		var err error
		if domainID == UnassignedID {
			_, err = tx.Exec(ctx, "DELETE FROM "+m.table+" WHERE "+m.column+"::text = ANY($1::text[])", ids)
		} else {
			_, err = tx.Exec(ctx, `
				INSERT INTO `+m.table+` (`+m.column+`, domain_id)
				SELECT unnest($1::text[])::`+m.columnType+`, $2::uuid
				ON CONFLICT (`+m.column+`) DO UPDATE SET domain_id = EXCLUDED.domain_id, assigned_at = now()`,
				ids, domainID)
		}
		return err
	}
	if err := set(memberships[KindIngestionSchedule], []string{scheduleID}); err != nil {
		return 0, fmt.Errorf("assigning schedule: %w", err)
	}
	if err := set(memberships[KindAsset], assets); err != nil {
		return 0, fmt.Errorf("moving pipeline assets: %w", err)
	}
	return len(assets), tx.Commit(ctx)
}

func (r *PostgresRepository) EntityExists(ctx context.Context, kind Kind, id string) (bool, error) {
	m, ok := memberships[kind]
	if !ok {
		return false, ErrInvalidInput
	}
	var exists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+m.entityTable+" WHERE id::text = $1)", id).Scan(&exists)
	return exists, err
}
