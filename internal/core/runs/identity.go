package runs

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// WriterKey names the contribution a pipeline run makes to the assets it finds. Runs from before runs had owners have no owner and share the pipeline and source alone.
func WriterKey(pipelineName, sourceName, owner string) string {
	key := "run:" + pipelineName + "/" + sourceName
	if owner != "" {
		key += "#" + owner
	}
	return key
}

// writerPrefix matches every run contribution of a pipeline, whatever the source or owner.
func writerPrefix(pipelineName string) string {
	return "run:" + pipelineName + "/"
}

// writerRole ranks a plugin's contributions: one that federates assets other systems own, such as Trino or OpenMetadata, ranks below the system's own plugin.
func writerRole(sourceName string) string {
	if entry, err := plugin.GetRegistry().Get(sourceName); err == nil && entry.Meta.Federates {
		return asset.RoleFederated
	}
	return asset.RoleNative
}

// unresolvedHeirs holds the MRNs of old assets several of this run's assets could be. They are kept out of stale removal until someone merges them into the right asset, because removing them would lose everything people added to them.
type unresolvedHeirs map[string]bool

// assetIdentity is what a run worked out about who one of its assets is.
type assetIdentity struct {
	mrn       string
	err       string
	existing  *asset.Asset
	movedFrom string
}

// resolveAssetIdentities works out the MRN of every asset in a run and which existing asset, if any, it is. An asset whose MRN another asset in the same run also has is failed rather than silently merged with it. An asset not found under its MRN may be one the catalog holds under an MRN an earlier plugin version gave it, either declared in PreviousMRNs or the legacy type, provider and name form; that asset is moved to the new MRN so it keeps its id and everything attached to it. When several assets in the run could be the same old asset, the one whose metadata matches it best takes it over, and if none stands out none does.
func (s *service) resolveAssetIdentities(ctx context.Context, assets []CreateAssetInput, response *ProcessAssetsResponse) ([]assetIdentity, unresolvedHeirs, error) {
	ids := make([]assetIdentity, len(assets))
	unresolved := unresolvedHeirs{}

	var toResolve []string
	var at []int
	for i, ast := range assets {
		if len(ast.Providers) == 0 {
			ids[i].err = "asset has no provider"
			continue
		}
		raw := mrn.New(ast.Type, ast.Providers[0], ast.Name)
		if ast.MRN != nil && *ast.MRN != "" {
			raw = *ast.MRN
		}
		canonical, err := asset.CanonicalMRN(raw)
		if err != nil {
			ids[i].err = err.Error()
			continue
		}
		toResolve = append(toResolve, canonical)
		at = append(at, i)
	}
	resolved, err := s.assetService.ResolveMRNs(ctx, toResolve)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving asset MRNs: %w", err)
	}
	for k, i := range at {
		ids[i].mrn = resolved[k]
	}

	first := map[string]int{}
	for i := range ids {
		if ids[i].err != "" || ids[i].mrn == "" {
			continue
		}
		if j, ok := first[ids[i].mrn]; ok {
			ids[i].err = fmt.Sprintf("another asset in this run, %q, has the same MRN %s; the plugin has to tell them apart, for example by schema or database", assets[j].Name, ids[i].mrn)
			continue
		}
		first[ids[i].mrn] = i
	}

	produced := make([]string, 0, len(first))
	for m := range first {
		produced = append(produced, m)
	}
	existing, err := s.assetService.GetByMRNs(ctx, produced)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching existing assets: %w", err)
	}
	for i := range ids {
		if ids[i].err == "" && ids[i].mrn != "" {
			ids[i].existing = existing[ids[i].mrn]
		}
	}

	// Every asset in the run lists where it may have lived before, the ones that already exist too, so an old asset several of them could be is recognised on every run until someone merges it, not only on the run that first created them.
	candidates := make([][]string, len(ids))
	var allCandidates []string
	for i, ast := range assets {
		if ids[i].err != "" || ids[i].mrn == "" {
			continue
		}
		candidates[i] = previousMRNs(ast, ids[i].mrn)
		allCandidates = append(allCandidates, candidates[i]...)
	}
	if len(allCandidates) == 0 {
		return ids, unresolved, nil
	}
	// Exact matches only: an asset that has already moved away from a candidate MRN belongs to whoever moved it, and following its alias would let a second system of the same name take it over.
	found, err := s.assetService.GetByExactMRNs(ctx, allCandidates)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching assets under previous MRNs: %w", err)
	}

	claims := map[string][]int{}
	byID := map[string]*asset.Asset{}
	for i := range ids {
		for _, c := range candidates[i] {
			a := found[c]
			if a == nil || a.MRN == nil {
				continue
			}
			// An asset this run still produces under its own MRN is not up for grabs.
			if _, stillProduced := first[*a.MRN]; stillProduced {
				continue
			}
			if ids[i].existing != nil && ids[i].existing.ID == a.ID {
				continue
			}
			claims[a.ID] = append(claims[a.ID], i)
			byID[a.ID] = a
			break
		}
	}
	// An asset that already exists under its own MRN only counts towards ambiguity; it never takes over another.
	for id, claimants := range claims {
		open := 0
		for _, i := range claimants {
			if ids[i].existing == nil {
				open++
			}
		}
		if open == 0 && len(claimants) > 1 {
			unresolved[*byID[id].MRN] = true
			delete(claims, id)
		}
	}

	claimed := make([]string, 0, len(claims))
	for id := range claims {
		claimed = append(claimed, id)
	}
	sort.Strings(claimed)
	for _, assetID := range claimed {
		old := byID[assetID]
		winner := pickHeir(assets, claims[assetID], old)
		if winner >= 0 && ids[winner].existing != nil {
			winner = -1
		}
		if winner < 0 {
			names := make([]string, 0, len(claims[assetID]))
			for _, i := range claims[assetID] {
				names = append(names, ids[i].mrn)
			}
			unresolved[*old.MRN] = true
			response.Warnings = append(response.Warnings, fmt.Sprintf("%s could be any of %s; it was left as it is and they were created as new assets. Merge it into the right one to keep what was added to it", *old.MRN, strings.Join(names, ", ")))
			continue
		}
		if err := s.assetService.MoveMRN(ctx, assetID, ids[winner].mrn, "renamed"); err != nil {
			log.Warn().Err(err).Str("from", *old.MRN).Str("to", ids[winner].mrn).Msg("Failed to move asset to its new MRN")
			continue
		}
		moved, err := s.assetService.Get(ctx, assetID)
		if err != nil {
			return nil, nil, fmt.Errorf("reading moved asset: %w", err)
		}
		ids[winner].existing = moved
		ids[winner].movedFrom = *old.MRN
	}
	return ids, unresolved, nil
}

// previousMRNs lists the MRNs an asset may be stored under from before, best first: what the plugin declares, the legacy form built from its name, and the same MRN without a zone for a pipeline that has just been given one.
func previousMRNs(ast CreateAssetInput, current string) []string {
	var out []string
	add := func(m string) {
		c, err := asset.CanonicalMRN(m)
		if err == nil && c != current && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	for _, p := range ast.PreviousMRNs {
		add(p)
	}
	if parsed, err := mrn.ParseMRN(current); err == nil && parsed.Zone != "" {
		add(parsed.WithZone("").String())
	}
	if len(ast.Providers) > 0 {
		add(mrn.New(ast.Type, ast.Providers[0], ast.Name))
	}
	return out
}

// pickHeir chooses which of several new assets takes over an old one: the one sharing the most metadata values with it, or -1 when that does not single one out.
func pickHeir(assets []CreateAssetInput, claimants []int, old *asset.Asset) int {
	if len(claimants) == 1 {
		return claimants[0]
	}
	best, bestScore, tied := -1, -1, false
	for _, i := range claimants {
		score := 0
		for k, v := range assets[i].Metadata {
			if ov, ok := old.Metadata[k]; ok && fmt.Sprint(ov) == fmt.Sprint(v) {
				score++
			}
		}
		switch {
		case score > bestScore:
			best, bestScore, tied = i, score, false
		case score == bestScore:
			tied = true
		}
	}
	if tied || bestScore <= 0 {
		return -1
	}
	return best
}

// currentAssetCheckpoints keys a baseline's asset checkpoints by the MRN each asset has today, so a checkpoint written under a legacy MRN or one the asset has since moved from still counts.
func (s *service) currentAssetCheckpoints(ctx context.Context, checkpoints map[string]*plugin.RunCheckpoint) (map[string]*plugin.RunCheckpoint, error) {
	keys := make([]string, 0, len(checkpoints))
	for k, cp := range checkpoints {
		if cp.EntityType == "asset" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	resolved, err := s.assetService.ResolveMRNs(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("resolving checkpoint MRNs: %w", err)
	}
	out := make(map[string]*plugin.RunCheckpoint, len(keys))
	for i, k := range keys {
		cp := checkpoints[k]
		m := resolved[i]
		if m == "" {
			m = k
		}
		if prev, ok := out[m]; ok && prev.Operation != StatusDeleted {
			continue
		}
		out[m] = cp
	}
	return out, nil
}

// removeStale withdraws this run's contribution from every asset its baseline had and this run did not find. An asset is deleted only when no other writer still contributes to it: another pipeline, an import, a person or Terraform.
func (s *service) removeStale(ctx context.Context, run *plugin.Run, runID, writer string, baseline map[string]*plugin.RunCheckpoint, current []string, keep unresolvedHeirs, response *ProcessAssetsResponse) {
	seen := make(map[string]bool, len(current))
	for _, m := range current {
		seen[m] = true
	}
	for m := range keep {
		if cp, ok := baseline[m]; ok && cp.Operation != StatusDeleted && !seen[m] {
			// Still in this run's baseline so the next run protects it again, until it is merged.
			if err := s.AddCheckpoint(ctx, runID, "asset", m, StatusUnchanged, cp.SourceFields); err != nil {
				log.Error().Err(err).Str("entity_mrn", m).Msg("Failed to keep an unresolved asset in the baseline")
			}
			seen[m] = true
		}
	}
	stale := make([]string, 0)
	for m, cp := range baseline {
		if cp.Operation != StatusDeleted && !seen[m] {
			stale = append(stale, m)
		}
	}
	if len(stale) == 0 {
		return
	}
	sort.Strings(stale)
	found, err := s.assetService.GetByMRNs(ctx, stale)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read stale assets")
		return
	}

	for _, staleMRN := range stale {
		if a := found[staleMRN]; a != nil {
			deleted, err := s.assetService.Withdraw(ctx, a.ID, writer)
			switch {
			case err != nil:
				log.Error().Err(err).Str("asset_mrn", staleMRN).Msg("Failed to withdraw stale asset")
				continue
			case deleted:
				response.StaleEntitiesRemoved = append(response.StaleEntitiesRemoved, staleMRN)
			default:
				response.StaleContributionsWithdrawn = append(response.StaleContributionsWithdrawn, staleMRN)
			}
		}

		entity := &RunEntity{
			ID:         uuid.New().String(),
			RunID:      runID,
			EntityType: "asset",
			EntityMRN:  staleMRN,
			Status:     StatusDeleted,
			CreatedAt:  time.Now(),
		}
		if err := s.repo.AddRunEntity(ctx, run.ID, entity); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", staleMRN).Msg("Failed to add deleted run entity")
		}
		if err := s.AddCheckpoint(ctx, runID, "asset", staleMRN, StatusDeleted, []string{}); err != nil {
			log.Error().Err(err).Str("run_id", runID).Str("entity_mrn", staleMRN).Msg("Failed to add deleted checkpoint")
		}
	}
}

// lineageCheckpointKey names an edge in a run's checkpoints. MRNs never hold whitespace, so " -> " cannot occur inside either end and the key splits back cleanly.
func lineageCheckpointKey(lineageType, source, target string) string {
	return "lineage:" + strings.ToLower(lineageType) + ":" + source + " -> " + target
}

// parseLineageCheckpointKey splits a key from lineageCheckpointKey.
func parseLineageCheckpointKey(key string) (lineageType, source, target string, ok bool) {
	rest, found := strings.CutPrefix(key, "lineage:")
	if !found {
		return "", "", "", false
	}
	lineageType, rest, found = strings.Cut(rest, ":")
	if !found {
		return "", "", "", false
	}
	source, target, found = strings.Cut(rest, " -> ")
	return lineageType, source, target, found
}

func documentationCheckpointKey(docType, assetMRN string) string {
	return "documentation:" + strings.ToLower(docType) + ":" + assetMRN
}

// contributionFields is what a run's asset says, in the shape contributions are stored.
func contributionFields(ast CreateAssetInput) asset.ContributionFields {
	name := ast.Name
	return asset.ContributionFields{
		Name:          &name,
		Type:          ast.Type,
		Providers:     ast.Providers,
		Description:   ast.Description,
		Metadata:      ast.Metadata,
		Schema:        convertSchemaToStringMap(ast.Schema),
		Tags:          ast.Tags,
		ExternalLinks: convertToAssetExternalLinks(ast.ExternalLinks),
		Query:         ast.Query,
		QueryLanguage: ast.QueryLanguage,
	}
}

func firstProvider(providers []string) string {
	if len(providers) == 0 {
		return ""
	}
	return providers[0]
}

// RecordZoneBindings stores the hosts a run's plugin reported connecting to and the zone each belongs to.
func (s *service) RecordZoneBindings(ctx context.Context, bindings []asset.ZoneBinding) error {
	if len(bindings) == 0 {
		return nil
	}
	return s.assetService.UpsertZoneBindings(ctx, bindings, "discovered")
}
