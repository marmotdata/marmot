package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/marmotdata/plugin-sdk/mrn"
)

// Roles rank what a writer says about an asset. A lower rank wins a field both set: a person or Terraform first, then the system that owns the asset, then a system that only federates it, then one that merely mentions it in lineage.
const (
	RoleManual    = "manual"
	RoleNative    = "native"
	RoleFederated = "federated"
	RoleReference = "reference"
)

var roleRank = map[string]int{RoleManual: 0, RoleNative: 1, RoleFederated: 2, RoleReference: 3}

// ErrInvalidMRN is returned for a string that is not an asset MRN in either form.
var ErrInvalidMRN = errors.New("invalid asset MRN")

// ExistsError is ErrAlreadyExists carrying the asset that already holds the MRN, so an API caller can be told which asset to import or update instead.
type ExistsError struct {
	Asset *Asset
}

func (e *ExistsError) Error() string { return ErrAlreadyExists.Error() }

func (e *ExistsError) Is(target error) bool { return target == ErrAlreadyExists }

// CanonicalMRN returns the one spelling of an asset MRN given in either the legacy or the kinded form, or ErrInvalidMRN.
func CanonicalMRN(s string) (string, error) {
	m, err := mrn.ParseMRN(s)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidMRN, err)
	}
	if m.Kind != mrn.KindAsset {
		return "", fmt.Errorf("%w: %q names a %s", ErrInvalidMRN, s, m.Kind)
	}
	return m.String(), nil
}

// MRNParts is an asset MRN split into its parts for API responses, so clients never have to split the string themselves.
type MRNParts struct {
	Kind     string   `json:"kind"`
	Type     string   `json:"type"`
	Provider string   `json:"provider"`
	Zone     string   `json:"zone,omitempty"`
	Name     string   `json:"name"`
	Path     []string `json:"path"`
} // @name MRNParts

// DiscoverPath is the asset page's path for an MRN in either form, /discover/<type>/<provider>[@<zone>]/<name>, the same path the search index stores.
func DiscoverPath(s string) string {
	m, err := mrn.ParseMRN(s)
	if err != nil || m.Kind != mrn.KindAsset {
		return "/discover/" + strings.TrimPrefix(s, mrn.Prefix)
	}
	return "/discover/" + m.Type + "/" + m.ProviderSegment() + "/" + m.Name
}

// PartsOf splits an MRN for an API response, or returns nil when it does not parse.
func PartsOf(s string) *MRNParts {
	m, err := mrn.ParseMRN(s)
	if err != nil || m.Kind != mrn.KindAsset {
		return nil
	}
	return &MRNParts{Kind: string(m.Kind), Type: m.Type, Provider: m.Provider, Zone: m.Zone, Name: m.Name, Path: m.Path()}
}

// ContributionFields is what one writer says about an asset.
type ContributionFields struct {
	Name          *string                `json:"name,omitempty"`
	Type          string                 `json:"type,omitempty"`
	Providers     []string               `json:"providers,omitempty"`
	Description   *string                `json:"description,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Schema        map[string]string      `json:"schema,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	ExternalLinks []ExternalLink         `json:"external_links,omitempty"`
	Query         *string                `json:"query,omitempty"`
	QueryLanguage *string                `json:"query_language,omitempty"`
}

// Contribution is one writer's latest word on an asset. Writer names the pipeline run owner, API principal or integration, and Role ranks it.
type Contribution struct {
	AssetID    string             `json:"asset_id"`
	Writer     string             `json:"writer"`
	Role       string             `json:"role"`
	Fields     ContributionFields `json:"fields"`
	FirstSeen  time.Time          `json:"first_seen_at"`
	LastSeenAt time.Time          `json:"last_seen_at"`
	// WriterInfo is Writer split into what a person reading it needs, filled in by the API so clients never parse writer keys.
	WriterInfo *WriterInfo `json:"writer_info,omitempty"`
} // @name AssetContribution

// WriterInfo says who a writer is. Kind is pipeline, api, openlineage, ui, marmot or legacy; the other fields are set when they apply.
type WriterInfo struct {
	Kind      string `json:"kind"`
	Pipeline  string `json:"pipeline,omitempty"`
	Source    string `json:"source,omitempty"`
	Scheduled bool   `json:"scheduled,omitempty"`
	// PrincipalType and PrincipalID name the person or service account, and PrincipalName is their display name when it could be looked up.
	PrincipalType string `json:"principal_type,omitempty"`
	PrincipalID   string `json:"principal_id,omitempty"`
	PrincipalName string `json:"principal_name,omitempty"`
	Zone          string `json:"zone,omitempty"`
} // @name AssetWriterInfo

// ParseWriter splits a writer key into its parts: run:<pipeline>/<source>[#<owner>] for pipeline runs, where the owner is schedule:<id> or <principal type>:<id>[/<zone>], api:<principal type>:<id> for REST and Terraform, and the fixed writers openlineage, ui, marmot and legacy.
func ParseWriter(writer string) WriterInfo {
	switch {
	case strings.HasPrefix(writer, "run:"):
		rest := strings.TrimPrefix(writer, "run:")
		rest, owner, _ := strings.Cut(rest, "#")
		info := WriterInfo{Kind: "pipeline"}
		if i := strings.LastIndex(rest, "/"); i >= 0 {
			info.Pipeline, info.Source = rest[:i], rest[i+1:]
		} else {
			info.Pipeline = rest
		}
		if strings.HasPrefix(owner, "schedule:") {
			info.Scheduled = true
		} else if owner != "" {
			principal, zone, _ := strings.Cut(owner, "/")
			info.PrincipalType, info.PrincipalID, _ = strings.Cut(principal, ":")
			info.Zone = zone
		}
		return info
	case strings.HasPrefix(writer, "api:"):
		info := WriterInfo{Kind: "api"}
		info.PrincipalType, info.PrincipalID, _ = strings.Cut(strings.TrimPrefix(writer, "api:"), ":")
		return info
	case writer == "openlineage", writer == "ui", writer == "marmot", writer == "legacy":
		return WriterInfo{Kind: writer}
	default:
		return WriterInfo{Kind: "other"}
	}
}

// ZoneBinding records that a provider reached at a host is a named zone.
type ZoneBinding struct {
	Provider  string    `json:"provider"`
	Host      string    `json:"host"`
	Zone      string    `json:"zone"`
	Origin    string    `json:"origin"`
	UpdatedAt time.Time `json:"updated_at"`
} // @name ZoneBinding

// MergeContributions works out an asset's fields from every contribution: a single-valued field comes from the best-ranked writer that set it, metadata is overlaid so the best-ranked writer wins each key, and tags are the union.
func MergeContributions(contribs []Contribution) ContributionFields {
	sorted := slices.Clone(contribs)
	sort.SliceStable(sorted, func(i, j int) bool {
		ri, rj := roleRank[sorted[i].Role], roleRank[sorted[j].Role]
		if ri != rj {
			return ri < rj
		}
		return sorted[i].LastSeenAt.After(sorted[j].LastSeenAt)
	})

	var out ContributionFields
	for _, c := range sorted {
		f := c.Fields
		if out.Name == nil && f.Name != nil && *f.Name != "" {
			out.Name = f.Name
		}
		if out.Type == "" && f.Type != "" {
			out.Type = f.Type
		}
		if len(out.Providers) == 0 && len(f.Providers) > 0 {
			out.Providers = f.Providers
		}
		if out.Description == nil && f.Description != nil && *f.Description != "" {
			out.Description = f.Description
		}
		if out.Schema == nil && len(f.Schema) > 0 {
			out.Schema = f.Schema
		}
		if out.ExternalLinks == nil && len(f.ExternalLinks) > 0 {
			out.ExternalLinks = f.ExternalLinks
		}
		if out.Query == nil && f.Query != nil && *f.Query != "" {
			out.Query = f.Query
			out.QueryLanguage = f.QueryLanguage
		}
	}

	out.Metadata = map[string]interface{}{}
	for i := len(sorted) - 1; i >= 0; i-- {
		maps.Copy(out.Metadata, sorted[i].Fields.Metadata)
	}

	seen := map[string]bool{}
	out.Tags = []string{}
	for _, c := range sorted {
		for _, t := range c.Fields.Tags {
			if !seen[t] {
				seen[t] = true
				out.Tags = append(out.Tags, t)
			}
		}
	}
	return out
}

// ResolveMRNs turns incoming MRNs, legacy, kinded or carrying a host hint, into the MRNs of the assets they name, following aliases. An MRN nothing answers to comes back canonical. The result is index-aligned with the input.
func (r *PostgresRepository) ResolveMRNs(ctx context.Context, mrns []string) ([]string, error) {
	if len(mrns) == 0 {
		return []string{}, nil
	}
	rows, err := r.db.Query(ctx, `SELECT marmot_resolve_asset_mrn(m) FROM unnest($1::text[]) WITH ORDINALITY AS u(m, i) ORDER BY i`, mrns)
	if err != nil {
		return nil, fmt.Errorf("resolving MRNs: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, len(mrns))
	for rows.Next() {
		var s *string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("scanning resolved MRN: %w", err)
		}
		if s == nil {
			out = append(out, "")
		} else {
			out = append(out, *s)
		}
	}
	return out, rows.Err()
}

// MoveMRN gives an asset a new MRN in place, keeping its id and so everything attached to it, and leaves the old MRN as an alias. Lineage follows through the foreign key; documentation keyed by MRN is moved here.
func (r *PostgresRepository) MoveMRN(ctx context.Context, assetID, newMRN, reason string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var oldMRN string
	if err := tx.QueryRow(ctx, `SELECT mrn FROM assets WHERE id = $1 FOR UPDATE`, assetID).Scan(&oldMRN); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("locking asset: %w", err)
	}
	if oldMRN == newMRN {
		return nil
	}

	if _, err := tx.Exec(ctx, `DELETE FROM mrn_aliases WHERE alias = $1`, newMRN); err != nil {
		return fmt.Errorf("clearing alias: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE assets SET mrn = $2, updated_at = NOW() WHERE id = $1`, assetID, newMRN); err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("moving asset MRN: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE doc_pages SET entity_id = $2 WHERE entity_type = 'asset' AND entity_id = $1`, oldMRN, newMRN); err != nil {
		return fmt.Errorf("moving doc pages: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE documentation d SET mrn = $2 WHERE mrn = $1 AND NOT EXISTS (SELECT 1 FROM documentation o WHERE o.mrn = $2 AND o.source = d.source)`, oldMRN, newMRN); err != nil {
		return fmt.Errorf("moving documentation: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO mrn_aliases (alias, kind, entity_id, reason) VALUES ($1, 'asset', $2, $3)
		ON CONFLICT (alias) DO UPDATE SET entity_id = EXCLUDED.entity_id, kind = EXCLUDED.kind, reason = EXCLUDED.reason, created_at = NOW()`,
		oldMRN, assetID, reason); err != nil {
		return fmt.Errorf("recording alias: %w", err)
	}
	return tx.Commit(ctx)
}

// MergeAssets folds loser into winner; see marmot_merge_assets.
func (r *PostgresRepository) MergeAssets(ctx context.Context, loserID, winnerID string) error {
	if _, err := r.db.Exec(ctx, `SELECT marmot_merge_assets($1, $2)`, loserID, winnerID); err != nil {
		if strings.Contains(err.Error(), "asset not found") {
			return ErrNotFound
		}
		return fmt.Errorf("merging assets: %w", err)
	}
	return nil
}

// Aliases lists the MRNs an asset used to have.
func (r *PostgresRepository) Aliases(ctx context.Context, assetID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT alias FROM mrn_aliases WHERE kind = 'asset' AND entity_id = $1 ORDER BY created_at`, assetID)
	if err != nil {
		return nil, fmt.Errorf("listing aliases: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// ListContributions returns every writer's contribution to an asset.
func (r *PostgresRepository) ListContributions(ctx context.Context, assetID string) ([]Contribution, error) {
	rows, err := r.db.Query(ctx, `
		SELECT asset_id, writer, role, fields, first_seen_at, last_seen_at
		FROM asset_contributions WHERE asset_id = $1`, assetID)
	if err != nil {
		return nil, fmt.Errorf("listing contributions: %w", err)
	}
	defer rows.Close()
	var out []Contribution
	for rows.Next() {
		var c Contribution
		var raw []byte
		if err := rows.Scan(&c.AssetID, &c.Writer, &c.Role, &raw, &c.FirstSeen, &c.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scanning contribution: %w", err)
		}
		if err := json.Unmarshal(raw, &c.Fields); err != nil {
			return nil, fmt.Errorf("decoding contribution: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpsertContribution records a writer's latest word on an asset.
func (r *PostgresRepository) UpsertContribution(ctx context.Context, c Contribution) error {
	raw, err := json.Marshal(c.Fields)
	if err != nil {
		return fmt.Errorf("encoding contribution: %w", err)
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO asset_contributions (asset_id, writer, role, fields)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (asset_id, writer) DO UPDATE SET role = EXCLUDED.role, fields = EXCLUDED.fields, last_seen_at = NOW()`,
		c.AssetID, c.Writer, c.Role, raw)
	if err != nil {
		return fmt.Errorf("writing contribution: %w", err)
	}
	return nil
}

// TouchContribution marks a writer as having seen an asset again without changing what it said, so a run that found nothing new still shows when it last saw the asset.
func (r *PostgresRepository) TouchContribution(ctx context.Context, assetID, writer string) (bool, error) {
	tag, err := r.db.Exec(ctx, `UPDATE asset_contributions SET last_seen_at = NOW() WHERE asset_id = $1 AND writer = $2`, assetID, writer)
	if err != nil {
		return false, fmt.Errorf("touching contribution: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// DeleteContribution withdraws a writer's contribution and reports how many contributions the asset has left.
func (r *PostgresRepository) DeleteContribution(ctx context.Context, assetID, writer string) (int, error) {
	if _, err := r.db.Exec(ctx, `DELETE FROM asset_contributions WHERE asset_id = $1 AND writer = $2`, assetID, writer); err != nil {
		return 0, fmt.Errorf("withdrawing contribution: %w", err)
	}
	var left int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM asset_contributions WHERE asset_id = $1`, assetID).Scan(&left); err != nil {
		return 0, fmt.Errorf("counting contributions: %w", err)
	}
	return left, nil
}

// AssetIDsByWriter lists the assets a writer contributes to. A writer ending in * matches every writer with that prefix.
func (r *PostgresRepository) AssetIDsByWriter(ctx context.Context, writer string) ([]string, error) {
	q := `SELECT DISTINCT asset_id FROM asset_contributions WHERE writer = $1`
	arg := writer
	if strings.HasSuffix(writer, "*") {
		q = `SELECT DISTINCT asset_id FROM asset_contributions WHERE starts_with(writer, $1)`
		arg = strings.TrimSuffix(writer, "*")
	}
	rows, err := r.db.Query(ctx, q, arg)
	if err != nil {
		return nil, fmt.Errorf("listing assets by writer: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// WritersFor lists which writers contribute to an asset, best-ranked first.
func (r *PostgresRepository) WritersFor(ctx context.Context, assetID string) ([]string, error) {
	contribs, err := r.ListContributions(ctx, assetID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(contribs, func(i, j int) bool { return roleRank[contribs[i].Role] < roleRank[contribs[j].Role] })
	out := make([]string, 0, len(contribs))
	for _, c := range contribs {
		out = append(out, c.Writer)
	}
	return out, nil
}

// UpsertZoneBindings records which zone each host belongs to. A discovered binding never overrides one an admin pinned.
func (r *PostgresRepository) UpsertZoneBindings(ctx context.Context, bindings []ZoneBinding, origin string) error {
	for _, b := range bindings {
		provider := strings.ReplaceAll(sanitizeMRNSegment(b.Provider), "@", "-")
		host := strings.ToLower(strings.TrimSpace(b.Host))
		if provider == "" || host == "" {
			continue
		}
		zone := sanitizeMRNSegment(b.Zone)
		_, err := r.db.Exec(ctx, `
			INSERT INTO zone_bindings (provider, host, zone, origin) VALUES ($1, $2, $3, $4)
			ON CONFLICT (provider, host) DO UPDATE SET zone = EXCLUDED.zone, origin = EXCLUDED.origin, updated_at = NOW()
			WHERE zone_bindings.origin = 'discovered' OR EXCLUDED.origin = 'manual'`,
			provider, host, zone, origin)
		if err != nil {
			return fmt.Errorf("writing zone binding: %w", err)
		}
	}
	return nil
}

// ListZoneBindings returns every binding.
func (r *PostgresRepository) ListZoneBindings(ctx context.Context) ([]ZoneBinding, error) {
	rows, err := r.db.Query(ctx, `SELECT provider, host, zone, origin, updated_at FROM zone_bindings ORDER BY provider, host`)
	if err != nil {
		return nil, fmt.Errorf("listing zone bindings: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ZoneBinding, error) {
		var b ZoneBinding
		err := row.Scan(&b.Provider, &b.Host, &b.Zone, &b.Origin, &b.UpdatedAt)
		return b, err
	})
}

// DeleteZoneBinding removes one binding.
func (r *PostgresRepository) DeleteZoneBinding(ctx context.Context, provider, host string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM zone_bindings WHERE provider = $1 AND host = $2`, provider, strings.ToLower(host))
	if err != nil {
		return fmt.Errorf("deleting zone binding: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func sanitizeMRNSegment(s string) string {
	m := mrn.Asset("x", s, "x")
	return m.Provider
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "SQLSTATE 23505")
}

// --- service ---

// ResolveMRN resolves one incoming MRN; see PostgresRepository.ResolveMRNs.
func (s *service) ResolveMRN(ctx context.Context, m string) (string, error) {
	out, err := s.ResolveMRNs(ctx, []string{m})
	if err != nil {
		return "", err
	}
	return out[0], nil
}

// ResolveMRNs resolves incoming MRNs. A string that is not an MRN at all comes back unchanged, which matches nothing.
func (s *service) ResolveMRNs(ctx context.Context, mrns []string) ([]string, error) {
	return s.repo.ResolveMRNs(ctx, mrns)
}

// MoveMRN moves an asset to a new MRN, keeping its id; reason is recorded on the alias left behind.
func (s *service) MoveMRN(ctx context.Context, assetID, newMRN, reason string) error {
	canonical, err := CanonicalMRN(newMRN)
	if err != nil {
		return err
	}
	if err := s.repo.MoveMRN(ctx, assetID, canonical, reason); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrAssetNotFound
		}
		if errors.Is(err, ErrConflict) {
			return ErrAlreadyExists
		}
		return err
	}
	return nil
}

// Merge folds loser into winner and recomputes the winner from the contributions it now holds.
func (s *service) Merge(ctx context.Context, loserID, winnerID string) (*Asset, error) {
	if loserID == winnerID {
		return nil, fmt.Errorf("%w: an asset cannot be merged into itself", ErrInvalidInput)
	}
	if err := s.repo.MergeAssets(ctx, loserID, winnerID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrAssetNotFound
		}
		return nil, err
	}
	return s.recompute(ctx, winnerID, true)
}

// GetByExactMRNs looks assets up by the MRN they have now, without following aliases, keyed by canonical MRN. An asset that has moved away from an MRN has already been claimed by whoever moved it, so a caller looking for an asset to take over must not find it through its old MRN.
func (s *service) GetByExactMRNs(ctx context.Context, mrns []string) (map[string]*Asset, error) {
	canonical := make([]string, 0, len(mrns))
	for _, m := range mrns {
		if c, err := CanonicalMRN(m); err == nil {
			canonical = append(canonical, c)
		}
	}
	assets, err := s.repo.GetByMRNs(ctx, canonical)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*Asset, len(assets))
	for _, a := range assets {
		if a.MRN != nil {
			out[*a.MRN] = a
		}
	}
	return out, nil
}

// Aliases lists the MRNs an asset used to have.
func (s *service) Aliases(ctx context.Context, assetID string) ([]string, error) {
	return s.repo.Aliases(ctx, assetID)
}

// Contributions lists who says what about an asset.
func (s *service) Contributions(ctx context.Context, assetID string) ([]Contribution, error) {
	return s.repo.ListContributions(ctx, assetID)
}

// Contribute records a writer's word on an asset and rewrites the asset from every contribution it has.
func (s *service) Contribute(ctx context.Context, assetID, writer, role string, fields ContributionFields, skipNotification bool) (*Asset, error) {
	if _, ok := roleRank[role]; !ok {
		return nil, fmt.Errorf("%w: unknown role %q", ErrInvalidInput, role)
	}
	if err := s.repo.UpsertContribution(ctx, Contribution{AssetID: assetID, Writer: writer, Role: role, Fields: fields}); err != nil {
		return nil, err
	}
	return s.recompute(ctx, assetID, skipNotification)
}

// TouchContribution notes that a writer saw an asset again unchanged. It reports false when the writer had no contribution yet.
func (s *service) TouchContribution(ctx context.Context, assetID, writer string) (bool, error) {
	return s.repo.TouchContribution(ctx, assetID, writer)
}

// Withdraw removes a writer's contribution. The asset is deleted when nobody contributes to it any more, and the asset is rewritten from what is left otherwise. It reports whether the asset was deleted.
func (s *service) Withdraw(ctx context.Context, assetID, writer string) (bool, error) {
	left, err := s.repo.DeleteContribution(ctx, assetID, writer)
	if err != nil {
		return false, err
	}
	if left == 0 {
		if err := s.Delete(ctx, assetID); err != nil && !errors.Is(err, ErrAssetNotFound) {
			return false, err
		}
		return true, nil
	}
	if _, err := s.recompute(ctx, assetID, true); err != nil {
		return false, err
	}
	return false, nil
}

// AssetIDsByWriter lists the assets a writer contributes to; a trailing * matches a writer prefix.
func (s *service) AssetIDsByWriter(ctx context.Context, writer string) ([]string, error) {
	return s.repo.AssetIDsByWriter(ctx, writer)
}

// UpsertZoneBindings records hosts plugins reported (origin discovered) or an admin pinned (origin manual).
func (s *service) UpsertZoneBindings(ctx context.Context, bindings []ZoneBinding, origin string) error {
	if origin != "discovered" && origin != "manual" {
		return fmt.Errorf("%w: origin must be discovered or manual", ErrInvalidInput)
	}
	return s.repo.UpsertZoneBindings(ctx, bindings, origin)
}

// ListZoneBindings returns every host binding.
func (s *service) ListZoneBindings(ctx context.Context) ([]ZoneBinding, error) {
	return s.repo.ListZoneBindings(ctx)
}

// DeleteZoneBinding removes a host binding.
func (s *service) DeleteZoneBinding(ctx context.Context, provider, host string) error {
	if err := s.repo.DeleteZoneBinding(ctx, provider, host); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrAssetNotFound
		}
		return err
	}
	return nil
}

// recompute rewrites an asset's writer-owned fields from its contributions. Fields people edit in Marmot itself, such as the user description, are not writer-owned and are left alone.
func (s *service) recompute(ctx context.Context, assetID string, skipNotification bool) (*Asset, error) {
	contribs, err := s.repo.ListContributions(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if len(contribs) == 0 {
		return s.Get(ctx, assetID)
	}
	merged := MergeContributions(contribs)
	input := UpdateInput{
		Name:             merged.Name,
		Type:             merged.Type,
		Providers:        merged.Providers,
		Description:      merged.Description,
		Metadata:         merged.Metadata,
		Schema:           merged.Schema,
		Tags:             merged.Tags,
		ExternalLinks:    merged.ExternalLinks,
		Query:            merged.Query,
		QueryLanguage:    merged.QueryLanguage,
		SkipNotification: skipNotification,
	}
	return s.Update(ctx, assetID, input)
}
