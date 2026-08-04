package openmetadata

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

const (
	apiCollectionFields = "owners,tags,domains,dataProducts"
	apiEndpointFields   = "owners,tags,domains,dataProducts"
)

// discoverAPIs catalogues API collections and their endpoints, named the
// way Marmot's OpenAPI plugin names them: a collection is a Service, an
// endpoint is "<METHOD> <path>" under that service.
func (c *collector) discoverAPIs(ctx context.Context, client *client) error {
	collections, supported, err := listOptional[apiCollection](ctx, client, "/v1/apiCollections", apiCollectionFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing API collections: %w", err)
	}
	if !supported {
		log.Debug().Msg("OpenMetadata does not expose API collections, skipping")
		return nil
	}

	collectionMRNs := make(map[string]string, len(collections))
	collectionNames := make(map[string]string, len(collections))

	for _, coll := range collections {
		if !c.wanted(coll.entityBase) {
			continue
		}

		p := projectionFor(coll.ServiceType)
		name := strings.Join(fqnBelowService(coll.FullyQualifiedName), ".")
		if name == "" {
			continue
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "endpoint_url", coll.EndpointURL)

		asset := c.newAsset(coll.entityBase, "apiCollection", "Service", p, c.mrnName(name, coll.FullyQualifiedName), metadata)
		c.add(coll.ID, asset)

		collectionMRNs[coll.FullyQualifiedName] = *asset.MRN
		collectionNames[coll.FullyQualifiedName] = name
	}

	endpoints, _, err := listOptional[apiEndpoint](ctx, client, "/v1/apiEndpoints", apiEndpointFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing API endpoints: %w", err)
	}

	discovered := 0
	for _, ep := range endpoints {
		if !c.wanted(ep.entityBase) {
			continue
		}

		collectionFQN := ep.APICollection.FullyQualifiedName
		collectionName := collectionNames[collectionFQN]
		if collectionName == "" {
			collectionName = ep.APICollection.Name
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "collection", collectionName)
		putIf(metadata, "method", ep.RequestMethod)
		putIf(metadata, "endpoint_url", ep.EndpointURL)
		putIf(metadata, "path", endpointPath(ep.EndpointURL))

		// Endpoints are named by collection and operation. Unlike the
		// other entity kinds this does not try to match the OpenAPI
		// plugin, which names an endpoint after its HTTP method and
		// path: OpenMetadata's endpointURL is a documentation link
		// rather than the request path, so the method and path are not
		// reliably recoverable.
		p := projectionFor(ep.ServiceType)
		name := strings.Join(fqnBelowService(ep.FullyQualifiedName), ".")
		if name == "" {
			continue
		}

		asset := c.newAsset(ep.entityBase, "apiEndpoint", "Endpoint", p, name, metadata)

		// Containment is carried by the CONTAINS edge. Asset.ParentMRN
		// exists on the SDK type but Marmot does not persist it, so
		// setting it would look like a link and store nothing.
		if parent, ok := collectionMRNs[collectionFQN]; ok {
			c.link(parent, *asset.MRN, "CONTAINS")
		}

		c.add(ep.ID, asset)
		discovered++
	}

	log.Debug().Int("collections", len(collectionMRNs)).Int("endpoints", discovered).Msg("Discovered APIs")
	return nil
}

// endpointPath is the request path of an endpoint URL, when there is
// one worth recording. OpenMetadata often stores a documentation link
// here, whose path is "/" or points at an anchor.
func endpointPath(endpointURL string) string {
	if endpointURL == "" {
		return ""
	}
	parsed, err := url.Parse(endpointURL)
	if err != nil || parsed.Path == "" || parsed.Path == "/" {
		return ""
	}
	return parsed.Path
}
