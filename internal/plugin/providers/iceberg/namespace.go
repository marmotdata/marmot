package iceberg

import (
	"context"
	"strings"
	"time"

	"github.com/apache/iceberg-go/table"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

func (s *Source) discoverNamespaces(ctx context.Context) ([]asset.Asset, map[string]table.Identifier, error) {
	var assets []asset.Asset
	nsMap := make(map[string]table.Identifier)

	if err := s.listNamespacesRecursive(ctx, nil, &assets, nsMap); err != nil {
		return nil, nil, err
	}

	return assets, nsMap, nil
}

func (s *Source) listNamespacesRecursive(ctx context.Context, parent table.Identifier, assets *[]asset.Asset, nsMap map[string]table.Identifier) error {
	namespaces, err := s.cat.ListNamespaces(ctx, parent)
	if err != nil {
		return err
	}

	for _, ns := range namespaces {
		nsPath := strings.Join(ns, ".")
		nsMap[nsPath] = ns

		if s.config.IncludeNamespaces {
			a, err := s.createNamespaceAsset(ctx, ns)
			if err != nil {
				log.Warn().Err(err).Str("namespace", nsPath).Msg("Failed to create namespace asset")
				continue
			}
			*assets = append(*assets, a)
		}

		if err := s.listNamespacesRecursive(ctx, ns, assets, nsMap); err != nil {
			log.Warn().Err(err).Str("namespace", nsPath).Msg("Failed to list child namespaces")
		}
	}

	return nil
}

func (s *Source) createNamespaceAsset(ctx context.Context, ns table.Identifier) (asset.Asset, error) {
	nsPath := strings.Join(ns, ".")
	metadata := map[string]interface{}{
		"namespace": nsPath,
	}

	var description *string
	props, err := s.cat.LoadNamespaceProperties(ctx, ns)
	if err != nil {
		log.Warn().Err(err).Str("namespace", nsPath).Msg("Failed to load namespace properties")
	} else {
		if loc, ok := props["location"]; ok {
			metadata["location"] = loc
		}
		for k, v := range props {
			if k == "description" {
				desc := v
				description = &desc
				continue
			}
			metadata["property."+k] = v
		}
	}

	mrnValue := mrn.New("Namespace", "Iceberg", nsPath)
	name := nsPath
	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Namespace",
		Providers:   []string{"Iceberg"},
		Description: description,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Iceberg",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}, nil
}
