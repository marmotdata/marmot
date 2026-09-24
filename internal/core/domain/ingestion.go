package domain

import (
	"context"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/rs/zerolog/log"
)

type pipelineKey struct{}

// WithPipeline records which ingestion pipeline is writing, so assets it
// creates can inherit the domain of the schedule with that name. The name only
// picks the destination; it never grants permission to write there.
func WithPipeline(ctx context.Context, pipelineName string) context.Context {
	return context.WithValue(ctx, pipelineKey{}, pipelineName)
}

func PipelineFrom(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(pipelineKey{}).(string)
	return name, ok && name != ""
}

// IngestionObserver places each newly created asset in the domain of the
// schedule whose name matches the pipeline in the context. Assets that already
// exist are never moved: a steward may have reassigned them.
type IngestionObserver struct {
	repo Repository
}

func NewIngestionObserver(repo Repository) *IngestionObserver {
	return &IngestionObserver{repo: repo}
}

// OnAssetCreated cannot fail the creation, so a lookup or assignment error is
// logged and the asset stays in Unassigned, which is the safe default.
func (o *IngestionObserver) OnAssetCreated(ctx context.Context, a *asset.Asset) {
	pipeline, ok := PipelineFrom(ctx)
	if !ok {
		return
	}
	domainID, found, err := o.repo.PipelineDomain(ctx, pipeline)
	if err != nil {
		log.Error().Err(err).Str("pipeline", pipeline).Msg("Resolving the pipeline's domain")
		return
	}
	if !found {
		return
	}
	if err := o.repo.Assign(ctx, KindAsset, []string{a.ID}, domainID); err != nil {
		log.Error().Err(err).Str("asset", a.ID).Str("domain", domainID).Msg("Assigning an ingested asset to its pipeline's domain")
	}
}

func (o *IngestionObserver) OnAssetDeleted(context.Context, string) error {
	return nil
}
