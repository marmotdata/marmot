// +marmot:name=Azure Blob Storage
// +marmot:description=Discovers containers and blobs from Azure Blob Storage accounts.
// +marmot:status=experimental
// +marmot:features=Assets
package azureblob

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for Azure Blob Storage plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	// Connection options (choose one)
	ConnectionString string `json:"connection_string,omitempty" description:"Azure Storage connection string" sensitive:"true"`
	AccountName      string `json:"account_name,omitempty" description:"Azure Storage account name"`
	AccountKey       string `json:"account_key,omitempty" description:"Azure Storage account key" sensitive:"true"`
	Endpoint         string `json:"endpoint,omitempty" description:"Custom endpoint URL (for Azurite or other emulators)"`

	// Discovery options
	IncludeMetadata  bool `json:"include_metadata" description:"Include container metadata" default:"true"`
	IncludeBlobCount bool `json:"include_blob_count" description:"Count blobs in each container (can be slow for large containers)" default:"false"`

	// Filtering
	Filter *plugin.Filter `json:"filter,omitempty" description:"Filter containers by name pattern"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
connection_string: "${AZURE_STORAGE_CONNECTION_STRING}"
include_metadata: true
include_blob_count: false
filter:
  include:
    - "^data-.*"
  exclude:
    - ".*-temp$"
tags:
  - "azure"
  - "storage"
`

type Source struct {
	config *Config
	client *azblob.Client
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if config.ConnectionString == "" && config.AccountName == "" {
		return nil, fmt.Errorf("either connection_string or account_name must be provided")
	}

	if config.AccountName != "" && config.AccountKey == "" && config.ConnectionString == "" {
		return nil, fmt.Errorf("account_key is required when using account_name")
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	s.config = config

	client, err := s.createClient()
	if err != nil {
		return nil, fmt.Errorf("creating Azure Blob client: %w", err)
	}
	s.client = client

	containers, err := s.discoverContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering containers: %w", err)
	}

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	for _, containerItem := range containers {
		containerName := *containerItem.Name

		filter := plugin.Filter{}
		if s.config.Filter != nil {
			filter = *s.config.Filter
		}
		if !plugin.ShouldIncludeResource(containerName, filter) {
			log.Debug().Str("container", containerName).Msg("Skipping container due to filter")
			continue
		}

		asset, err := s.createContainerAsset(ctx, containerItem)
		if err != nil {
			log.Warn().Err(err).Str("container", containerName).Msg("Failed to create asset for container")
			continue
		}
		assets = append(assets, asset)
	}

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

func (s *Source) createClient() (*azblob.Client, error) {
	if s.config.ConnectionString != "" {
		return azblob.NewClientFromConnectionString(s.config.ConnectionString, nil)
	}

	endpoint := s.config.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.blob.core.windows.net/", s.config.AccountName)
	}

	cred, err := azblob.NewSharedKeyCredential(s.config.AccountName, s.config.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("creating shared key credential: %w", err)
	}

	return azblob.NewClientWithSharedKeyCredential(endpoint, cred, nil)
}

func (s *Source) discoverContainers(ctx context.Context) ([]*service.ContainerItem, error) {
	var containers []*service.ContainerItem

	pager := s.client.NewListContainersPager(&azblob.ListContainersOptions{
		Include: azblob.ListContainersInclude{
			Metadata: s.config.IncludeMetadata,
		},
	})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing containers: %w", err)
		}
		containers = append(containers, page.ContainerItems...)
	}

	return containers, nil
}

func (s *Source) createContainerAsset(ctx context.Context, containerItem *service.ContainerItem) (asset.Asset, error) {
	containerName := *containerItem.Name

	metadata := make(map[string]interface{})
	metadata["container_name"] = containerName

	if containerItem.Properties != nil {
		props := containerItem.Properties

		if props.LastModified != nil {
			metadata["last_modified"] = props.LastModified.Format(time.RFC3339)
		}

		if props.ETag != nil {
			metadata["etag"] = string(*props.ETag)
		}

		if props.LeaseStatus != nil {
			metadata["lease_status"] = string(*props.LeaseStatus)
		}

		if props.LeaseState != nil {
			metadata["lease_state"] = string(*props.LeaseState)
		}

		if props.HasImmutabilityPolicy != nil {
			metadata["has_immutability_policy"] = *props.HasImmutabilityPolicy
		}

		if props.HasLegalHold != nil {
			metadata["has_legal_hold"] = *props.HasLegalHold
		}

		if props.PublicAccess != nil {
			metadata["public_access"] = string(*props.PublicAccess)
		} else {
			metadata["public_access"] = "none"
		}
	}

	if s.config.IncludeMetadata && containerItem.Metadata != nil {
		for key, value := range containerItem.Metadata {
			if value != nil {
				metadata["custom_"+key] = *value
			}
		}
	}

	if s.config.IncludeBlobCount {
		count, err := s.countBlobs(ctx, containerName)
		if err != nil {
			log.Warn().Err(err).Str("container", containerName).Msg("Failed to count blobs")
		} else {
			metadata["blob_count"] = count
		}
	}

	mrnValue := mrn.New("Container", "AzureBlob", containerName)
	description := fmt.Sprintf("Azure Blob Storage container %s", containerName)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &containerName,
		MRN:         &mrnValue,
		Type:        "Container",
		Providers:   []string{"AzureBlob"},
		Description: &description,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AzureBlob",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}, nil
}

func (s *Source) countBlobs(ctx context.Context, containerName string) (int64, error) {
	containerClient := s.client.ServiceClient().NewContainerClient(containerName)

	var count int64
	pager := containerClient.NewListBlobsFlatPager(nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return 0, err
		}
		count += int64(len(page.Segment.BlobItems))
	}

	return count, nil
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "azureblob",
		Name:        "Azure Blob Storage",
		Description: "Discover containers from Azure Blob Storage accounts",
		Icon:        "azureblob",
		Category:    "storage",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register Azure Blob Storage plugin")
	}
}
