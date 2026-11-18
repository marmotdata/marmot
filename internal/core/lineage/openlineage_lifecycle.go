package lineage

import (
	"context"
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/rs/zerolog/log"
)

// The lifecycleStateChange facet says what a statement did to an output dataset: created, altered, overwritten, truncated, renamed or dropped it. Every state is recorded on the asset; a rename and a drop also change which asset there is, but only once the statement has finished, from a COMPLETE run event or a DatasetEvent.

const (
	lifecycleDrop   = "DROP"
	lifecycleRename = "RENAME"
)

func lifecycleState(ds *Dataset) string {
	lc := facetMap(ds.Facets, "lifecycleStateChange")
	state, _ := lc["lifecycleStateChange"].(string)
	return strings.ToUpper(state)
}

// processDatasetOutput files a dataset an event wrote, or a DatasetEvent's dataset, following a rename or drop the event reports. It returns the MRN to link the job to, or an empty string when there is no asset to link: a dropped dataset, or a renamed one whose rename has not finished yet.
func (s *service) processDatasetOutput(ctx context.Context, ds *Dataset, producer, createdBy string, final bool) (string, error) {
	switch lifecycleState(ds) {
	case lifecycleDrop:
		return "", s.dropDataset(ctx, ds, producer, final)
	case lifecycleRename:
		if wait, err := s.renameDataset(ctx, ds, producer, final); err != nil || wait {
			return "", err
		}
	}
	return s.upsertDataset(ctx, ds, producer, createdBy)
}

// dropDataset handles a dataset a statement dropped. It is never created, so a DROP of a table Marmot has not seen leaves nothing behind. Once the drop has finished, an asset only OpenLineage knew about is deleted; one its own system or a person wrote to is kept, marked as dropped, for that system's next run to remove.
func (s *service) dropDataset(ctx context.Context, ds *Dataset, producer string, final bool) error {
	target := s.resolveDataset(ctx, ds, producer)
	existing, err := s.findAsset(ctx, target)
	if err != nil || existing == nil || !final {
		return err
	}
	if existing.IsStub && s.ownedByOpenLineage(ctx, existing) {
		if err := s.assetSvc.Delete(ctx, existing.ID); err != nil {
			return fmt.Errorf("deleting dropped dataset %s: %w", target, err)
		}
		log.Info().Str("mrn", target).Msg("Deleted a dataset an OpenLineage event reported dropped")
		return nil
	}
	id := identifyDataset(ds, producer)
	s.contribute(ctx, existing, asset.RoleReference, datasetFields(ds, id), datasetSource(ds, id))
	return nil
}

// renameDataset handles a dataset a statement renamed, which the facet's previousIdentifier names. Once the rename has finished, the asset under the old name moves to the new one, keeping its id, lineage and everything people added, and the old MRN stays as an alias. When both names already have an asset, an OpenLineage-only stub under either is merged into the other. Before the rename has finished it reports wait when the old asset exists and the new one does not, so the new name is not created early.
func (s *service) renameDataset(ctx context.Context, ds *Dataset, producer string, final bool) (wait bool, err error) {
	lc := facetMap(ds.Facets, "lifecycleStateChange")
	prev, _ := lc["previousIdentifier"].(map[string]interface{})
	prevNamespace, _ := prev["namespace"].(string)
	prevName, _ := prev["name"].(string)
	if prevNamespace == "" || prevName == "" {
		return false, nil
	}
	// The old name lives in the same system, so it is resolved with the same facets.
	oldMRN := s.resolveDataset(ctx, &Dataset{Namespace: prevNamespace, Name: prevName, Facets: ds.Facets}, producer)
	newMRN := s.resolveDataset(ctx, ds, producer)
	if oldMRN == newMRN {
		return false, nil
	}
	found, err := s.assetSvc.GetByExactMRNs(ctx, []string{oldMRN, newMRN})
	if err != nil {
		return false, fmt.Errorf("looking up renamed dataset: %w", err)
	}
	was, now := found[oldMRN], found[newMRN]
	if was == nil {
		return false, nil
	}
	if !final {
		return now == nil, nil
	}
	if now != nil && now.ID != was.ID {
		switch {
		case now.IsStub && s.ownedByOpenLineage(ctx, now):
			if _, err := s.assetSvc.Merge(ctx, now.ID, was.ID); err != nil {
				return false, fmt.Errorf("merging %s into the renamed asset: %w", newMRN, err)
			}
		case was.IsStub && s.ownedByOpenLineage(ctx, was):
			if _, err := s.assetSvc.Merge(ctx, was.ID, now.ID); err != nil {
				return false, fmt.Errorf("merging %s into %s: %w", oldMRN, newMRN, err)
			}
			return false, nil
		default:
			log.Info().Str("from", oldMRN).Str("to", newMRN).Msg("Renamed dataset already has an asset under both names; leaving both")
			return false, nil
		}
	}
	if err := s.assetSvc.MoveMRN(ctx, was.ID, newMRN, "openlineage-rename"); err != nil {
		return false, fmt.Errorf("moving %s to %s: %w", oldMRN, newMRN, err)
	}
	return false, nil
}

// resolveDataset returns the MRN of the asset a dataset is, following aliases.
func (s *service) resolveDataset(ctx context.Context, ds *Dataset, producer string) string {
	id := identifyDataset(ds, producer)
	resolved, err := s.assetSvc.ResolveMRN(ctx, id.MRN)
	if err != nil || resolved == "" {
		return id.MRN
	}
	return resolved
}
