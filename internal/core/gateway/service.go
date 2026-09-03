package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/crypto"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

var (
	ErrDenied           = errors.New("query denied by policy")
	ErrTargetDisabled   = errors.New("query target is disabled")
	ErrQueryUnsupported = errors.New("plugin does not support queries")
	ErrNotSessionOwner  = errors.New("session belongs to another principal")
)

// QueryRunner dispatches plan and query calls to long-running plugin
// instances. Implemented by plugin.InstanceManager.
type QueryRunner interface {
	PlanQuery(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error)
	ExecuteQuery(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.QueryRequest) (pluginsdk.QueryStream, error)
	Status() []plugin.InstanceStatus
}

// Options carries the gateway's tunables from the config file.
type Options struct {
	SessionTTL     time.Duration
	QueryTimeout   time.Duration
	MaxRowsDefault int64
	MaxRowsCap     int64
	// IngestOnQuery throttles catalog refreshes triggered by queries.
	// SourceInterval is the minimum gap between auto-ingests of the same
	// source; GlobalInterval is a hard floor between any two auto-ingests
	// across all sources, so query traffic cannot spam ingestion.
	IngestSourceInterval time.Duration
	IngestGlobalInterval time.Duration
}

// IngestTrigger enqueues an asynchronous catalog refresh for a source. It is
// implemented at the wiring layer over the ingestion job machinery; a nil
// trigger disables ingest-on-query.
type IngestTrigger interface {
	TriggerIngest(ctx context.Context, sourceID string) error
}

// CreateGrantInput grants a principal access to resources matching a
// selector.
type CreateGrantInput struct {
	PrincipalType    string     `json:"principal_type" validate:"required,oneof=service_account user"`
	PrincipalID      string     `json:"principal_id" validate:"required,uuid"`
	ResourceSelector string     `json:"resource_selector" validate:"required"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	Reason           *string    `json:"reason,omitempty"`
} // @name GatewayCreateGrantInput

// QueryInput is one statement to run through the gateway.
type QueryInput struct {
	SessionID string `json:"session_id" validate:"required,uuid"`
	Target    string `json:"target" validate:"required"`
	Statement string `json:"statement" validate:"required"`
	MaxRows   int64  `json:"max_rows,omitempty"`
} // @name GatewayQueryInput

// QueryColumn describes one result column.
type QueryColumn struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
} // @name GatewayQueryColumn

// AssetContext is the catalogue context fused onto a query result for one
// referenced asset.
type AssetContext struct {
	MRN         string   `json:"mrn"`
	Name        string   `json:"name,omitempty"`
	Type        string   `json:"type,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
} // @name GatewayAssetContext

// QueryResult carries rows back together with the policy decision and the
// catalogue context of everything the query touched.
type QueryResult struct {
	SessionID      string         `json:"session_id"`
	AuditID        string         `json:"audit_id"`
	Columns        []QueryColumn  `json:"columns"`
	Rows           [][]any        `json:"rows"`
	RowCount       int64          `json:"row_count"`
	Truncated      bool           `json:"truncated"`
	ReferencedMRNs []string       `json:"referenced_mrns,omitempty"`
	Context        []AssetContext `json:"context,omitempty"`
} // @name GatewayQueryResult

// TargetProvider resolves query targets from queryable ingestion sources.
// It is implemented at the wiring layer over the schedule service so the
// gateway core stays decoupled from the ingestion domain. Targets carry the
// source's stored (encrypted) config; the gateway decrypts at query time.
type TargetProvider interface {
	ListQueryTargets(ctx context.Context) ([]*Target, error)
	GetQueryTarget(ctx context.Context, name string) (*Target, error)
}

// Service is the query gateway: queryable sources, grants, sessions,
// policy-checked query execution and the audit trail.
type Service interface {
	ListTargets(ctx context.Context) ([]*Target, error)
	InstanceStatus() []plugin.InstanceStatus

	CreateGrant(ctx context.Context, input CreateGrantInput, createdBy string) (*Grant, error)
	ListGrants(ctx context.Context, principalType, principalID string) ([]*Grant, error)
	RevokeGrant(ctx context.Context, id, revokedBy, reason string) error

	OpenSession(ctx context.Context, principal auth.Principal, purpose string) (*Session, error)
	GetSession(ctx context.Context, id string) (*Session, error)
	ListSessions(ctx context.Context, limit, offset int) ([]*Session, error)
	RevokeSession(ctx context.Context, id string, principal auth.Principal) error

	Query(ctx context.Context, principal auth.Principal, input QueryInput) (*QueryResult, error)
	QueryForPrincipal(ctx context.Context, principal auth.Principal, target, statement string, maxRows int64) (*QueryResult, error)

	ListAudit(ctx context.Context, filter AuditFilter) ([]*AuditEntry, error)
}

type service struct {
	repo      Repository
	targets   TargetProvider
	assets    asset.Service
	runner    QueryRunner
	ingest    *ingestThrottle
	encryptor *crypto.Encryptor
	opts      Options
	validator *validator.Validate
}

func NewService(repo Repository, targets TargetProvider, assets asset.Service, runner QueryRunner, ingest IngestTrigger, encryptor *crypto.Encryptor, opts Options) Service {
	return &service{
		repo:      repo,
		targets:   targets,
		assets:    assets,
		runner:    runner,
		ingest:    newIngestThrottle(ingest, opts.IngestSourceInterval, opts.IngestGlobalInterval),
		encryptor: encryptor,
		opts:      opts,
		validator: validator.New(),
	}
}

// ingestThrottle fires asynchronous catalog refreshes, but never faster than
// its per-source and global minimum intervals, so heavy query traffic cannot
// turn into heavy ingestion. Intervals are tracked in memory (per replica),
// which is a soft global bound; the per-source ingestion machinery dedupes
// the actual runs. A nil trigger makes maybeTrigger a no-op.
type ingestThrottle struct {
	trigger        IngestTrigger
	sourceInterval time.Duration
	globalInterval time.Duration

	mu            sync.Mutex
	lastGlobal    time.Time
	lastPerSource map[string]time.Time
}

func newIngestThrottle(trigger IngestTrigger, sourceInterval, globalInterval time.Duration) *ingestThrottle {
	return &ingestThrottle{
		trigger:        trigger,
		sourceInterval: sourceInterval,
		globalInterval: globalInterval,
		lastPerSource:  make(map[string]time.Time),
	}
}

// maybeTrigger fires an async refresh for the source if both the per-source
// and global rate limits allow it. It returns immediately; the ingest runs
// in the background and never blocks the query.
func (t *ingestThrottle) maybeTrigger(sourceID string) {
	if t.trigger == nil || sourceID == "" {
		return
	}

	t.mu.Lock()
	now := time.Now()
	if !t.lastGlobal.IsZero() && now.Sub(t.lastGlobal) < t.globalInterval {
		t.mu.Unlock()
		return
	}
	if last, ok := t.lastPerSource[sourceID]; ok && now.Sub(last) < t.sourceInterval {
		t.mu.Unlock()
		return
	}
	// Reserve the slot before releasing the lock so concurrent queries cannot
	// both pass the check and double-fire.
	t.lastGlobal = now
	t.lastPerSource[sourceID] = now
	t.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := t.trigger.TriggerIngest(ctx, sourceID); err != nil {
			log.Warn().Err(err).Str("source", sourceID).Msg("ingest-on-query trigger failed")
		} else {
			log.Debug().Str("source", sourceID).Msg("ingest-on-query: enqueued catalog refresh")
		}
	}()
}

func (s *service) ListTargets(ctx context.Context) ([]*Target, error) {
	targets, err := s.targets.ListQueryTargets(ctx)
	if err != nil {
		return nil, err
	}
	for i, t := range targets {
		targets[i] = redactTarget(t)
	}
	return targets, nil
}

func (s *service) InstanceStatus() []plugin.InstanceStatus {
	return s.runner.Status()
}

func (s *service) CreateGrant(ctx context.Context, input CreateGrantInput, createdBy string) (*Grant, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	grant := &Grant{
		PrincipalType:    input.PrincipalType,
		PrincipalID:      input.PrincipalID,
		ResourceSelector: input.ResourceSelector,
		Actions:          []string{ActionQuery},
		ExpiresAt:        input.ExpiresAt,
		Reason:           input.Reason,
	}
	if createdBy != "" {
		grant.CreatedBy = &createdBy
	}
	if err := s.repo.CreateGrant(ctx, grant); err != nil {
		return nil, err
	}
	return grant, nil
}

func (s *service) ListGrants(ctx context.Context, principalType, principalID string) ([]*Grant, error) {
	return s.repo.ListGrants(ctx, principalType, principalID)
}

func (s *service) RevokeGrant(ctx context.Context, id, revokedBy, reason string) error {
	return s.repo.RevokeGrant(ctx, id, revokedBy, reason)
}

func (s *service) OpenSession(ctx context.Context, principal auth.Principal, purpose string) (*Session, error) {
	session := &Session{
		PrincipalType: string(principal.Type()),
		PrincipalID:   principal.ID(),
		ExpiresAt:     time.Now().Add(s.opts.SessionTTL),
	}
	if purpose != "" {
		session.Purpose = &purpose
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *service) GetSession(ctx context.Context, id string) (*Session, error) {
	return s.repo.GetSession(ctx, id)
}

func (s *service) ListSessions(ctx context.Context, limit, offset int) ([]*Session, error) {
	return s.repo.ListSessions(ctx, limit, offset)
}

func (s *service) RevokeSession(ctx context.Context, id string, principal auth.Principal) error {
	session, err := s.repo.GetSession(ctx, id)
	if err != nil {
		return err
	}
	if session.PrincipalID != principal.ID() && !principal.HasPermission("gateway", "manage") {
		return ErrNotSessionOwner
	}
	return s.repo.RevokeSession(ctx, id, principal.ID())
}

// Query is the session-based direct query path: resolve and validate the
// session, then run the statement against the principal's grants, auditing
// the attempt to that session.
func (s *service) Query(ctx context.Context, principal auth.Principal, input QueryInput) (*QueryResult, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	session, err := s.repo.GetSession(ctx, input.SessionID)
	if err != nil {
		return nil, err
	}
	if session.PrincipalID != principal.ID() {
		return nil, ErrNotSessionOwner
	}
	if session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionClosed
	}

	if err := s.repo.TouchSession(ctx, session.ID); err != nil {
		log.Warn().Err(err).Str("session", session.ID).Msg("Failed to touch gateway session")
	}
	return s.runQuery(ctx, principal, &session.ID, input.Target, input.Statement, input.MaxRows)
}

// QueryForPrincipal is the session-less path used by MCP, where the client's
// authenticated connection stands in for a session. Same policy check, same
// audit trail (with a null session), same fused context.
func (s *service) QueryForPrincipal(ctx context.Context, principal auth.Principal, target, statement string, maxRows int64) (*QueryResult, error) {
	if target == "" || statement == "" {
		return nil, fmt.Errorf("%w: target and statement are required", ErrInvalidInput)
	}
	return s.runQuery(ctx, principal, nil, target, statement, maxRows)
}

// runQuery is the shared core: plan the statement through the target's
// plugin, check the plan against the principal's grants, execute, audit and
// fuse catalogue context onto the result.
func (s *service) runQuery(ctx context.Context, principal auth.Principal, sessionID *string, targetName, statement string, requestedRows int64) (*QueryResult, error) {
	target, err := s.targets.GetQueryTarget(ctx, targetName)
	if err != nil {
		return nil, err
	}
	if !target.Enabled {
		return nil, ErrTargetDisabled
	}

	entry, err := plugin.GetRegistry().Get(target.PluginID)
	if err != nil {
		return nil, fmt.Errorf("plugin %q for target %q not loaded: %w", target.PluginID, target.Name, err)
	}
	if !entry.Meta.SupportsQuery {
		return nil, fmt.Errorf("%w: plugin %q", ErrQueryUnsupported, target.PluginID)
	}

	config := make(map[string]any, len(target.Config))
	for k, v := range target.Config {
		config[k] = v
	}
	if s.encryptor != nil {
		if err := plugin.DecryptConfigForPlugin(target.PluginID, config, s.encryptor); err != nil {
			return nil, fmt.Errorf("decrypting target config: %w", err)
		}
	}

	maxRows := s.opts.MaxRowsDefault
	if requestedRows > 0 && requestedRows < s.opts.MaxRowsCap {
		maxRows = requestedRows
	}
	req := pluginsdk.QueryRequest{
		Statement: statement,
		MaxRows:   maxRows,
		Identity:  principal.AuditSubject(),
	}

	queryCtx, cancel := context.WithTimeout(ctx, s.opts.QueryTimeout)
	defer cancel()

	resources, planErr := s.planResources(queryCtx, target, config, req)
	if planErr != nil {
		return nil, planErr
	}

	grants, err := s.repo.ListLiveGrants(ctx, string(principal.Type()), principal.ID())
	if err != nil {
		return nil, fmt.Errorf("loading grants: %w", err)
	}
	decision := Decide(principal, grants, resources)

	audit := &AuditEntry{
		SessionID:      sessionID,
		PrincipalType:  string(principal.Type()),
		PrincipalID:    principal.ID(),
		AuditSubject:   principal.AuditSubject(),
		TargetID:       &target.ID,
		TargetName:     target.Name,
		QueryText:      statement,
		ReferencedMRNs: resources,
		Decision:       decisionLabel(decision),
		DecisionDetail: decisionDetail(decision),
		Status:         "running",
		Source:         "direct",
	}
	if !decision.Allowed {
		audit.Status = "denied"
	}
	if err := s.repo.CreateAuditEntry(ctx, audit); err != nil {
		return nil, fmt.Errorf("writing audit entry: %w", err)
	}

	if !decision.Allowed {
		s.completeAudit(ctx, audit.ID, "denied", nil, nil)
		return nil, fmt.Errorf("%w: %s", ErrDenied, decision.Reason)
	}

	result, execErr := s.execute(queryCtx, target.PluginID, config, req, maxRows)
	if execErr != nil {
		msg := execErr.Error()
		s.completeAudit(ctx, audit.ID, "failed", nil, &msg)
		return nil, fmt.Errorf("executing query: %w", execErr)
	}
	s.completeAudit(ctx, audit.ID, "completed", &result.RowCount, nil)

	// Refresh the source's catalog off the back of real usage, so query-only
	// sources self-populate and hot sources stay fresh. Async and rate-limited
	// — it never touches the query's latency.
	if target.IngestOnQuery {
		s.ingest.maybeTrigger(target.ID)
	}

	if sessionID != nil {
		result.SessionID = *sessionID
	}
	result.AuditID = audit.ID
	result.ReferencedMRNs = resources
	result.Context = s.fuseContext(ctx, resources)
	return result, nil
}

// planResources asks the plugin what the statement touches. A plugin that
// cannot plan (Unimplemented, or a nil plan for this statement) falls back
// to the target-level resource so a grant must cover the whole target.
func (s *service) planResources(ctx context.Context, target *Target, config map[string]any, req pluginsdk.QueryRequest) ([]string, error) {
	plan, err := s.runner.PlanQuery(ctx, target.PluginID, config, req)
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return []string{TargetMRN(target.Name)}, nil
		}
		return nil, fmt.Errorf("planning query: %w", err)
	}
	if plan == nil || len(plan.ReferencedMRNs) == 0 {
		return []string{TargetMRN(target.Name)}, nil
	}
	return plan.ReferencedMRNs, nil
}

func (s *service) execute(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.QueryRequest, maxRows int64) (*QueryResult, error) {
	stream, err := s.runner.ExecuteQuery(ctx, pluginID, config, req)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	result := &QueryResult{Rows: [][]any{}}
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return result, nil
			}
			return nil, err
		}
		if len(chunk.Columns) > 0 && result.Columns == nil {
			result.Columns = make([]QueryColumn, len(chunk.Columns))
			for i, c := range chunk.Columns {
				result.Columns[i] = QueryColumn{Name: c.Name, Type: c.Type}
			}
		}
		for _, row := range chunk.Rows {
			if result.RowCount >= maxRows {
				result.Truncated = true
				return result, nil
			}
			result.Rows = append(result.Rows, row)
			result.RowCount++
		}
	}
}

// fuseContext attaches catalogue knowledge about every referenced asset to
// the query result so agents receive meaning together with rows.
func (s *service) fuseContext(ctx context.Context, mrns []string) []AssetContext {
	var real []string
	for _, m := range mrns {
		if !strings.HasPrefix(m, "mrn://target/") {
			real = append(real, m)
		}
	}
	if len(real) == 0 {
		return nil
	}

	assets, err := s.assets.GetByMRNs(ctx, real)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fuse asset context onto query result")
		return nil
	}

	var contexts []AssetContext
	for _, m := range real {
		a, ok := assets[m]
		if !ok || a == nil {
			continue
		}
		ac := AssetContext{MRN: m, Type: a.Type, Tags: a.Tags}
		if a.Name != nil {
			ac.Name = *a.Name
		}
		if a.UserDescription != nil && *a.UserDescription != "" {
			ac.Description = *a.UserDescription
		} else if a.Description != nil {
			ac.Description = *a.Description
		}
		contexts = append(contexts, ac)
	}
	return contexts
}

func (s *service) completeAudit(ctx context.Context, id, auditStatus string, rows *int64, queryErr *string) {
	if err := s.repo.CompleteAuditEntry(ctx, id, auditStatus, rows, queryErr); err != nil {
		log.Error().Err(err).Str("audit_id", id).Msg("Failed to complete gateway audit entry")
	}
}

func (s *service) ListAudit(ctx context.Context, filter AuditFilter) ([]*AuditEntry, error) {
	if filter.Limit <= 0 || filter.Limit > 500 {
		filter.Limit = 50
	}
	return s.repo.ListAuditEntries(ctx, filter)
}

func decisionLabel(d Decision) string {
	if d.Allowed {
		return "allowed"
	}
	return "denied"
}

func decisionDetail(d Decision) map[string]any {
	detail := map[string]any{}
	if d.Reason != "" {
		detail["reason"] = d.Reason
	}
	if len(d.MatchedGrants) > 0 {
		detail["matched_grants"] = d.MatchedGrants
	}
	if len(detail) == 0 {
		return nil
	}
	return detail
}

// redactTarget masks sensitive config fields for API responses using the
// plugin's config spec; the stored row keeps the encrypted values.
func redactTarget(t *Target) *Target {
	entry, err := plugin.GetRegistry().Get(t.PluginID)
	if err != nil || t.Config == nil {
		return t
	}
	sensitive := plugin.GetSensitiveFields(entry.Meta.ConfigSpec)
	redacted := make(map[string]any, len(t.Config))
	for k, v := range t.Config {
		if sensitive[k] {
			redacted[k] = "********"
		} else {
			redacted[k] = v
		}
	}
	t.Config = redacted
	return t
}
