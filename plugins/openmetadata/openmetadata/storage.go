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
// prefix inside that bucket. Marmot's S3, GCS and Azure Blob plugins
// catalogue the bucket, or container in Azure's words, and nothing
// inside it, so only the top level is imported. A prefix imported here
// would have no counterpart in a later native run and would sit in the
// catalog forever without ever being updated again. Set
// include_container_prefixes to import the hierarchy anyway, which is
// worth doing when nothing else is going to catalogue that bucket.
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

		parts := fqnBelowService(ct.FullyQualifiedName)
		if len(parts) == 0 {
			continue
		}

		root := isRootContainer(ct, parts)
		if !root && !c.config.IncludeContainerPrefixes {
			c.skipped["nested inside a storage container"]++
			continue
		}

		p := projectionFor(ct.ServiceType)
		assetType := "Container"
		if root {
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

// isRootContainer reports whether a container is the bucket itself
// rather than something inside one.
//
// OpenMetadata states the hierarchy twice: a container points at its
// parent, and its fully qualified name is the service followed by the
// path down to it, so a bucket has exactly one part below the service.
// A root has to satisfy both statements. The parent reference alone is
// not enough because a prefix whose parent OpenMetadata never recorded
// would then be catalogued as a bucket called raw/events, which is not a
// bucket that exists. The path alone is not enough either, because it
// ignores what OpenMetadata says the tree actually is. Where the two
// disagree the container is treated as nested, which leaves it out
// instead of inventing a bucket.
func isRootContainer(ct container, parts []string) bool {
	if ct.Parent != nil && ct.Parent.FullyQualifiedName != "" {
		return false
	}
	return len(parts) == 1
}
