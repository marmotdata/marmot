package openmetadata

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

const searchIndexFields = "owners,tags,domains,dataProducts,fields"

// discoverSearchIndexes catalogues search indices. Marmot's
// Elasticsearch and OpenSearch plugins catalogue an index as a Table, so
// indices on those services do too and merge with them; other search
// engines get the Index type.
func (c *collector) discoverSearchIndexes(ctx context.Context, client *client) error {
	indexes, supported, err := listOptional[searchIndex](ctx, client, "/v1/searchIndexes", searchIndexFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing search indexes: %w", err)
	}
	if !supported {
		return nil
	}

	discovered := 0
	for _, idx := range indexes {
		if !c.wanted(idx.entityBase) {
			continue
		}

		p := projectionFor(idx.ServiceType)
		name := strings.Join(fqnBelowService(idx.FullyQualifiedName), ".")
		if name == "" {
			continue
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "index_type", idx.IndexType)
		putIf(metadata, "field_count", len(idx.Fields))

		asset := c.newAsset(idx.entityBase, "searchIndex", p.IndexType, p, c.mrnName(name, idx.FullyQualifiedName), metadata)
		if c.config.IncludeColumns {
			setColumns(&asset, idx.Fields)
		}
		c.add(idx.ID, asset)
		discovered++
	}

	log.Debug().Int("count", discovered).Msg("Discovered search indexes")
	return nil
}
