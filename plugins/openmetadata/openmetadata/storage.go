package openmetadata

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

const containerFields = "owners,tags,domains,dataProducts,parent,dataModel"

// discoverContainers catalogues object storage. OpenMetadata nests
// containers: the top level one is the bucket, anything below it is a
// prefix inside that bucket. Marmot's S3 and GCS plugins catalogue
// buckets, so top level containers take the Bucket type for those
// services and everything deeper stays a Container.
func (c *collector) discoverContainers(ctx context.Context, client *client) error {
	containers, supported, err := listOptional[container](ctx, client, "/v1/containers", containerFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
	}
	if !supported {
		return nil
	}

	mrnByFQN := make(map[string]string, len(containers))
	discovered := 0

	for _, ct := range containers {
		if !c.wanted(ct.entityBase) {
			continue
		}

		p := projectionFor(ct.ServiceType)
		parts := fqnBelowService(ct.FullyQualifiedName)
		if len(parts) == 0 {
			continue
		}

		assetType := "Container"
		if len(parts) == 1 {
			assetType = p.ContainerType
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "bucket", parts[0])
		putIf(metadata, "prefix", ct.Prefix)
		putIf(metadata, "size", int64(ct.Size))
		putIf(metadata, "object_count", int64(ct.NumberOfObjects))
		putIf(metadata, "file_formats", ct.FileFormats)
		if ct.DataModel != nil {
			putIf(metadata, "partitioned", ct.DataModel.IsPartitioned)
		}

		name := strings.Join(parts, "/")
		if len(parts) == 1 {
			name = parts[0]
		}

		asset := c.newAsset(ct.entityBase, "container", assetType, p, c.mrnName(name, ct.FullyQualifiedName), metadata)
		if ct.DataModel != nil && c.config.IncludeColumns {
			setColumns(&asset, ct.DataModel.Columns)
		}

		c.add(ct.ID, asset)
		mrnByFQN[ct.FullyQualifiedName] = *asset.MRN
		discovered++
	}

	// Link children to their parents once every container has an MRN.
	for _, ct := range containers {
		if ct.Parent == nil || ct.Parent.FullyQualifiedName == "" {
			continue
		}
		child, ok := mrnByFQN[ct.FullyQualifiedName]
		if !ok {
			continue
		}
		if parent, ok := mrnByFQN[ct.Parent.FullyQualifiedName]; ok {
			c.link(parent, child, "CONTAINS")
		}
	}

	log.Debug().Int("count", discovered).Msg("Discovered containers")
	return nil
}
