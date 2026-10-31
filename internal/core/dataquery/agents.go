package dataquery

import (
	"context"
	"fmt"
	"time"
)

// PrincipalTypeServiceAccount is the principal type agents query as. Agents reach data through a service account's key, so that is the identity their activity is summarised under.
const PrincipalTypeServiceAccount = "service_account"

const (
	defaultAgentSummaryLimit = 10
	maxAgentSummaryLimit     = 100
)

// AgentSummaryFilter selects whose activity to summarise. PrincipalType defaults to service accounts; PrincipalID narrows it to one.
type AgentSummaryFilter struct {
	PrincipalType string
	PrincipalID   string
	// Now anchors the 24 hour and 7 day windows, and defaults to the current time.
	Now   time.Time
	Limit int
}

// AgentSummary is one caller's recent data query activity, enough for a row on Home or the head of an agent's page.
type AgentSummary struct {
	PrincipalType string     `json:"principal_type"`
	PrincipalID   string     `json:"principal_id"`
	Name          string     `json:"name"`
	AuditSubject  string     `json:"audit_subject"`
	Queries24h    int64      `json:"queries_24h"`
	Queries7d     int64      `json:"queries_7d"`
	Refused24h    int64      `json:"refused_24h"`
	Refused7d     int64      `json:"refused_7d"`
	TablesRead7d  int64      `json:"tables_read_7d"`
	LastActiveAt  *time.Time `json:"last_active_at,omitempty"`
} // @name QueryAgentSummary

// AgentSummaries returns the busiest callers of the last 7 days from the hourly rollups, with the time each last queried read from the log's principal index. Asking for one principal returns it even when it has been quiet all week.
func (r *PostgresRepository) AgentSummaries(ctx context.Context, f AgentSummaryFilter) ([]AgentSummary, error) {
	if f.PrincipalType == "" {
		f.PrincipalType = PrincipalTypeServiceAccount
	}
	if f.Now.IsZero() {
		f.Now = time.Now()
	}
	if f.Limit <= 0 {
		f.Limit = defaultAgentSummaryLimit
	}
	f.Limit = min(f.Limit, maxAgentSummaryLimit)
	weekAgo := f.Now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	dayAgo := f.Now.Add(-24 * time.Hour).Truncate(time.Hour)

	args := []any{weekAgo, dayAgo, f.PrincipalType}
	principal := ""
	if f.PrincipalID != "" {
		args = append(args, f.PrincipalID)
		principal = fmt.Sprintf(" AND principal_id = $%d", len(args))
	}
	args = append(args, f.Limit)

	// The named principal is seeded into the callers so it is returned with zeros rather than missing when it has no rollups this week.
	seed := ""
	if f.PrincipalID != "" {
		seed = ` UNION ALL SELECT $3::varchar, $4::text, NULL, 0, 0, 0, 0`
	}

	rows, err := r.db.Query(ctx, `
		WITH raw AS (
			SELECT principal_type, principal_id, max(audit_subject) AS subject,
			       COALESCE(sum(queries) FILTER (WHERE bucket >= $2), 0) AS q24,
			       COALESCE(sum(queries), 0) AS q7,
			       COALESCE(sum(queries) FILTER (WHERE bucket >= $2 AND decision = 'denied'), 0) AS r24,
			       COALESCE(sum(queries) FILTER (WHERE decision = 'denied'), 0) AS r7
			FROM data_query_stats_hourly
			WHERE bucket >= $1 AND principal_type = $3`+principal+`
			GROUP BY principal_type, principal_id`+seed+`
		), callers AS (
			SELECT principal_type, principal_id, max(subject) AS subject,
			       sum(q24)::bigint AS q24, sum(q7)::bigint AS q7, sum(r24)::bigint AS r24, sum(r7)::bigint AS r7
			FROM raw GROUP BY principal_type, principal_id
		)
		SELECT c.principal_type, c.principal_id,
		       COALESCE(sa.name, ''), COALESCE(c.subject, l.subject, ''),
		       c.q24, c.q7, c.r24, c.r7, COALESCE(t.tables, 0), l.last
		FROM callers c
		LEFT JOIN service_accounts sa ON c.principal_type = 'service_account' AND sa.id::text = c.principal_id
		LEFT JOIN LATERAL (
			SELECT count(DISTINCT ts.mrn) AS tables
			FROM data_query_table_stats_hourly ts
			WHERE ts.principal_type = c.principal_type AND ts.principal_id = c.principal_id AND ts.bucket >= $1
		) t ON true
		LEFT JOIN LATERAL (
			SELECT dq.started_at AS last, dq.audit_subject AS subject
			FROM data_queries dq
			WHERE dq.principal_type = c.principal_type AND dq.principal_id = c.principal_id
			ORDER BY dq.started_at DESC LIMIT 1
		) l ON true
		WHERE c.q7 > 0 OR l.last IS NOT NULL
		ORDER BY c.q7 DESC, l.last DESC NULLS LAST, c.principal_id
		LIMIT $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("summarising agents: %w", err)
	}
	defer rows.Close()

	out := []AgentSummary{}
	for rows.Next() {
		var s AgentSummary
		if err := rows.Scan(&s.PrincipalType, &s.PrincipalID, &s.Name, &s.AuditSubject,
			&s.Queries24h, &s.Queries7d, &s.Refused24h, &s.Refused7d, &s.TablesRead7d, &s.LastActiveAt); err != nil {
			return nil, fmt.Errorf("scanning agent summary: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
