package openmetadata

import (
	"context"
	"fmt"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

const mlModelFields = "owners,tags,domains,dataProducts,dashboard"

// discoverMLModels catalogues machine learning models, and links each
// model to the tables its features were built from.
func (c *collector) discoverMLModels(ctx context.Context, client *client) error {
	models, supported, err := listOptional[mlModel](ctx, client, "/v1/mlmodels", mlModelFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing ML models: %w", err)
	}
	if !supported {
		return nil
	}

	type featureSource struct {
		modelMRN string
		sourceID string
	}
	var sources []featureSource

	discovered := 0
	for _, m := range models {
		if !c.wanted(m.entityBase) {
			continue
		}

		p := projectionFor(m.ServiceType)
		name := strings.Join(fqnBelowService(m.FullyQualifiedName), ".")
		if name == "" {
			continue
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "algorithm", m.Algorithm)
		putIf(metadata, "target", m.Target)
		putIf(metadata, "server", m.Server)
		putIf(metadata, "feature_count", len(m.MlFeatures))
		if m.MlStore != nil {
			putIf(metadata, "storage", m.MlStore.Storage)
			putIf(metadata, "image_repository", m.MlStore.ImageRepository)
		}
		if params := hyperParameters(m.MlHyperParams); len(params) > 0 {
			metadata["hyper_parameters"] = params
		}

		asset := c.newAsset(m.entityBase, "mlmodel", "Model", p, c.mrnName(name, m.FullyQualifiedName), metadata)
		if c.config.IncludeColumns {
			setFeatures(&asset, m.MlFeatures)
		}
		c.add(m.ID, asset)
		discovered++

		for _, feature := range m.MlFeatures {
			for _, source := range feature.FeatureSources {
				if source.DataSource != nil && source.DataSource.ID != "" {
					sources = append(sources, featureSource{modelMRN: *asset.MRN, sourceID: source.DataSource.ID})
				}
			}
		}
	}

	// Feature sources point at tables, which are only resolvable once
	// every asset has been collected.
	c.deferred = append(c.deferred, func() {
		for _, s := range sources {
			if sourceMRN, ok := c.mrnByID[s.sourceID]; ok {
				c.link(sourceMRN, s.modelMRN, "FEEDS")
			}
		}
	})

	log.Debug().Int("count", discovered).Msg("Discovered ML models")
	return nil
}

func hyperParameters(params []mlHyperParameter) map[string]interface{} {
	if len(params) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(params))
	for _, p := range params {
		out[p.Name] = p.Value
	}
	return out
}

// setFeatures records a model's features in the same place table
// columns go, so the UI renders both with the same field list.
func setFeatures(asset *pluginsdk.Asset, features []mlFeature) {
	columns := make([]column, 0, len(features))
	for _, f := range features {
		columns = append(columns, column{
			Name:        f.Name,
			DataType:    f.DataType,
			Description: f.Description,
		})
	}
	setColumns(asset, columns)
}
