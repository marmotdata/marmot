package lineage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/core/asset"
)

type Repository interface {
	GetAssetLineage(ctx context.Context, assetID string, limit int, direction string) (*LineageResponse, error)
	CreateDirectLineage(ctx context.Context, sourceMRN string, targetMRN string, lineageType string) (string, error)
	EdgeExists(ctx context.Context, source, target string) (bool, error)
	DeleteDirectLineage(ctx context.Context, edgeID string) error
	GetDirectLineage(ctx context.Context, edgeID string) (*LineageEdge, error)
	StoreRunHistory(ctx context.Context, entry *RunHistoryEntry) error
}

type LineageResponse struct {
	Nodes []LineageNode `json:"nodes"`
	Edges []LineageEdge `json:"edges"`
}

type LineageNode struct {
	ID    string       `json:"id"`
	Type  string       `json:"type"`
	Asset *asset.Asset `json:"asset"`
	Depth int          `json:"depth"`
}

type LineageEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	JobMRN string `json:"job_mrn,omitempty"`
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) GetDirectLineage(ctx context.Context, edgeID string) (*LineageEdge, error) {
	query := `
        SELECT e.id, e.source_mrn, e.target_mrn, e.job_mrn,
            CASE 
                WHEN e.job_mrn IS NOT NULL THEN 'JOB'
                WHEN a1.type = 'Service' OR a2.type = 'Service' THEN 'SERVICE'
                ELSE 'DEFAULT'
            END as type
        FROM lineage_edges e
        JOIN assets a1 ON e.source_mrn = a1.mrn
        JOIN assets a2 ON e.target_mrn = a2.mrn
        WHERE e.id = $1`

	var edge LineageEdge
	var jobMRN *string

	err := r.db.QueryRow(ctx, query, edgeID).Scan(
		&edge.ID,
		&edge.Source,
		&edge.Target,
		&jobMRN,
		&edge.Type,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying edge: %w", err)
	}

	if jobMRN != nil {
		edge.JobMRN = *jobMRN
	}

	return &edge, nil
}

func (r *PostgresRepository) EdgeExists(ctx context.Context, source, target string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM lineage_edges WHERE source_mrn = $1 AND target_mrn = $2)",
		source, target,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking lineage edge existence: %w", err)
	}
	return exists, nil
}

func (r *PostgresRepository) DeleteDirectLineage(ctx context.Context, edgeID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var eventID string
	err = tx.QueryRow(ctx, `
        SELECT event_id 
        FROM lineage_edges 
        WHERE id = $1`,
		edgeID,
	).Scan(&eventID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return fmt.Errorf("getting event ID: %w", err)
	}

	_, err = tx.Exec(ctx, `
        DELETE FROM lineage_edges 
        WHERE id = $1`,
		edgeID,
	)
	if err != nil {
		return fmt.Errorf("deleting edge: %w", err)
	}

	_, err = tx.Exec(ctx, `
        DELETE FROM lineage_events 
        WHERE event_id = $1`,
		eventID,
	)
	if err != nil {
		return fmt.Errorf("deleting event: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) CreateDirectLineage(ctx context.Context, sourceMRN string, targetMRN string, lineageType string) (string, error) {
	// Check if edge already exists
	exists, err := r.EdgeExists(ctx, sourceMRN, targetMRN)
	if err != nil {
		return "", fmt.Errorf("checking edge existence: %w", err)
	}
	if exists {
		// Return existing edge ID
		var edgeID string
		err := r.db.QueryRow(ctx,
			"SELECT id FROM lineage_edges WHERE source_mrn = $1 AND target_mrn = $2 LIMIT 1",
			sourceMRN, targetMRN).Scan(&edgeID)
		if err != nil {
			return "", fmt.Errorf("getting existing edge ID: %w", err)
		}
		return edgeID, nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	eventID := uuid.New()
	edgeID := uuid.New()
	now := time.Now()

	// Use the actual lineage type (DEPENDS_ON, CREATES, TRANSFORMS, etc.)
	if lineageType == "" {
		lineageType = "DIRECT"
	}

	eventData := map[string]interface{}{
		"source": sourceMRN,
		"target": targetMRN,
		"type":   lineageType,
	}
	eventDataJSON, err := json.Marshal(eventData)
	if err != nil {
		return "", fmt.Errorf("marshaling event data: %w", err)
	}

	_, err = tx.Exec(ctx, `
        INSERT INTO lineage_events (
            event_id, 
            event_time, 
            event_type, 
            event_data 
        )
        VALUES ($1, $2, $3, $4)`,
		eventID,
		now,
		"DIRECT",
		eventDataJSON,
	)
	if err != nil {
		return "", fmt.Errorf("inserting lineage event: %w", err)
	}

	if err := r.ensureAssetsExist(ctx, tx, sourceMRN, targetMRN); err != nil {
		return "", err
	}

	_, err = tx.Exec(ctx, `
        INSERT INTO lineage_edges (id, source_mrn, target_mrn, event_id)
        VALUES ($1, $2, $3, $4)`,
		edgeID, sourceMRN, targetMRN, eventID,
	)
	if err != nil {
		return "", fmt.Errorf("inserting lineage edge: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("committing transaction: %w", err)
	}

	return edgeID.String(), nil
}

func (r *PostgresRepository) ensureAssetsExist(ctx context.Context, tx pgx.Tx, sourceMRN, targetMRN string) error {
	var count int
	err := tx.QueryRow(ctx, `
        SELECT COUNT(*) FROM assets 
        WHERE mrn = ANY($1)`, []string{sourceMRN, targetMRN}).Scan(&count)
	if err != nil {
		return fmt.Errorf("checking assets existence: %w", err)
	}

	if count != 2 {
		return fmt.Errorf("one or both assets do not exist: %s, %s", sourceMRN, targetMRN)
	}
	return nil
}

func (r *PostgresRepository) GetAssetLineage(ctx context.Context, assetID string, limit int, direction string) (*LineageResponse, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var mrn string
	err = tx.QueryRow(ctx, "SELECT mrn FROM assets WHERE id = $1", assetID).Scan(&mrn)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("asset not found: %s", assetID)
		}
		return nil, fmt.Errorf("getting asset mrn: %w", err)
	}

	nodes, err := r.scanLineageNodes(ctx, tx, `
	SELECT id, name, mrn, type, providers, description,
	metadata, schema, sources, tags,
	created_by, created_at, updated_at, last_sync_at, is_stub,
	0 as depth
	FROM assets WHERE mrn = $1`, mrn)
	if err != nil {
		return nil, fmt.Errorf("scanning root node: %w", err)
	}

	if direction != "downstream" {
		upstreamNodes, err := r.getUpstreamNodes(ctx, tx, mrn, limit)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, upstreamNodes...)
	}

	if direction != "upstream" {
		downstreamNodes, err := r.getDownstreamNodes(ctx, tx, mrn, limit)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, downstreamNodes...)
	}

	edges, err := r.getLineageEdges(ctx, tx, nodes)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &LineageResponse{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

func (r *PostgresRepository) getUpstreamNodes(ctx context.Context, tx pgx.Tx, mrn string, limit int) ([]LineageNode, error) {
	return r.scanLineageNodes(ctx, tx, `
	WITH RECURSIVE upstream AS (
		SELECT DISTINCT
			source_mrn as mrn,
			-1::integer as depth,
			job_mrn
		FROM lineage_edges
		WHERE target_mrn = $1

		UNION ALL

		SELECT DISTINCT
			e.source_mrn,
			(u.depth - 1)::integer as depth,
			e.job_mrn
		FROM lineage_edges e
		JOIN upstream u ON e.target_mrn = u.mrn
		WHERE e.source_mrn <> $1
		AND u.depth > -$2::integer
	)
	SELECT DISTINCT ON (a.mrn)
		a.id, a.name, a.mrn, a.type, a.providers, a.description,
		a.metadata, a.schema, a.sources, a.tags,
		a.created_by, a.created_at, a.updated_at, a.last_sync_at, a.is_stub,
		u.depth
	FROM upstream u
	JOIN assets a ON a.mrn = u.mrn
	ORDER BY a.mrn, abs(u.depth)`, mrn, limit)
}

func (r *PostgresRepository) getDownstreamNodes(ctx context.Context, tx pgx.Tx, mrn string, limit int) ([]LineageNode, error) {
	return r.scanLineageNodes(ctx, tx, `
	WITH RECURSIVE downstream AS (
		SELECT DISTINCT
			target_mrn as mrn,
			1 as depth,
			job_mrn
		FROM lineage_edges
		WHERE source_mrn = $1

		UNION ALL

		SELECT DISTINCT
			e.target_mrn,
			CASE 
				WHEN d.depth < $2 THEN d.depth + 1
				ELSE d.depth
			END as depth,
			e.job_mrn
		FROM lineage_edges e
		JOIN downstream d ON e.source_mrn = d.mrn
		WHERE e.target_mrn <> $1
		AND d.depth < ($2)
	)
	SELECT DISTINCT ON (a.mrn)
		a.id, a.name, a.mrn, a.type, a.providers, a.description,
		a.metadata, a.schema, a.sources, a.tags,
		a.created_by, a.created_at, a.updated_at, a.last_sync_at, a.is_stub,
		d.depth
	FROM downstream d
	JOIN assets a ON a.mrn = d.mrn
	ORDER BY a.mrn, abs(d.depth)`, mrn, limit)
}

func (r *PostgresRepository) getLineageEdges(ctx context.Context, tx pgx.Tx, nodes []LineageNode) ([]LineageEdge, error) {
	if len(nodes) == 0 {
		return []LineageEdge{}, nil
	}

	nodeMRNs := make([]string, len(nodes))
	for i, node := range nodes {
		if node.Asset.MRN != nil && *node.Asset.MRN != "" {
			nodeMRNs[i] = *node.Asset.MRN
		} else {
			nodeMRNs[i] = node.ID
		}
	}

	rows, err := tx.Query(ctx, `
		SELECT DISTINCT
			e.id,
			e.source_mrn,
			e.target_mrn,
			e.job_mrn,
			CASE 
				WHEN e.job_mrn IS NOT NULL THEN 'JOB'
				WHEN a1.type = 'Service' OR a2.type = 'Service' THEN 'SERVICE'
				ELSE 'DEFAULT'
			END as type
		FROM lineage_edges e
		JOIN assets a1 ON e.source_mrn = a1.mrn
		JOIN assets a2 ON e.target_mrn = a2.mrn
		WHERE e.source_mrn = ANY($1) AND e.target_mrn = ANY($1)
		ORDER BY e.source_mrn, e.target_mrn`, nodeMRNs)
	if err != nil {
		return nil, fmt.Errorf("querying edges: %w", err)
	}
	defer rows.Close()

	var edges []LineageEdge
	for rows.Next() {
		var edge LineageEdge
		var jobMRN *string
		if err := rows.Scan(&edge.ID, &edge.Source, &edge.Target, &jobMRN, &edge.Type); err != nil {
			return nil, fmt.Errorf("scanning edge: %w", err)
		}
		if jobMRN != nil {
			edge.JobMRN = *jobMRN
		}
		edges = append(edges, edge)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating edges: %w", err)
	}

	return edges, nil
}

func (r *PostgresRepository) scanLineageNodes(ctx context.Context, tx pgx.Tx, query string, args ...interface{}) ([]LineageNode, error) {
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying nodes: %w", err)
	}
	defer rows.Close()

	var nodes []LineageNode
	for rows.Next() {
		var a asset.Asset
		var node LineageNode

		err := rows.Scan(
			&a.ID,
			&a.Name,
			&a.MRN,
			&a.Type,
			&a.Providers,
			&a.Description,
			&a.Metadata,
			&a.Schema,
			&a.Sources,
			&a.Tags,
			&a.CreatedBy,
			&a.CreatedAt,
			&a.UpdatedAt,
			&a.LastSyncAt,
			&a.IsStub,
			&node.Depth,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning node: %w", err)
		}

		// Use MRN as the node ID for proper lineage relationships
		if a.MRN != nil && *a.MRN != "" {
			node.ID = *a.MRN
		} else {
			node.ID = a.ID
		}
		node.Type = a.Type
		node.Asset = &a
		nodes = append(nodes, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating nodes: %w", err)
	}

	return nodes, nil
}

func (r *PostgresRepository) StoreRunHistory(ctx context.Context, entry *RunHistoryEntry) error {
	runFacetsJSON, err := json.Marshal(entry.RunFacets)
	if err != nil {
		return fmt.Errorf("failed to marshal run facets: %w", err)
	}

	jobFacetsJSON, err := json.Marshal(entry.JobFacets)
	if err != nil {
		return fmt.Errorf("failed to marshal job facets: %w", err)
	}

	inputsJSON, err := json.Marshal(entry.Inputs)
	if err != nil {
		return fmt.Errorf("failed to marshal inputs: %w", err)
	}

	outputsJSON, err := json.Marshal(entry.Outputs)
	if err != nil {
		return fmt.Errorf("failed to marshal outputs: %w", err)
	}

	query := `
		INSERT INTO run_history (
			id, asset_id, run_id, job_namespace, job_name, 
			event_type, event_time, producer, run_facets, job_facets, 
			inputs, outputs, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err = r.db.Exec(ctx, query,
		entry.ID, entry.AssetID, entry.RunID, entry.JobNamespace, entry.JobName,
		entry.EventType, entry.EventTime, entry.Producer, runFacetsJSON, jobFacetsJSON,
		inputsJSON, outputsJSON, entry.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to store run history: %w", err)
	}

	return nil
}
