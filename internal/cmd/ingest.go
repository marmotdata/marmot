package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/plugin/providers/asyncapi"
	"github.com/marmotdata/marmot/internal/plugin/providers/kafka"
	"github.com/marmotdata/marmot/internal/plugin/providers/mongodb"
	"github.com/marmotdata/marmot/internal/plugin/providers/postgresql"
	"github.com/marmotdata/marmot/internal/plugin/providers/sns"
	"github.com/marmotdata/marmot/internal/plugin/providers/sqs"

	//TODO: structs from here should be in the API package
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/sync"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var (
	configFile string
	host       string
	apiKey     string
)

type BatchCreateInput struct {
	Assets []asset.Asset          `json:"assets"`
	Config plugin.RawPluginConfig `json:"config"`
}

func init() {
	ingestCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to ingestion config file (required)")
	ingestCmd.Flags().StringVarP(&host, "host", "H", "http://localhost:8080", "Marmot API host")
	ingestCmd.Flags().StringVarP(&apiKey, "api-key", "k", "", "API key for authentication")
	ingestCmd.MarkFlagRequired("config")
	rootCmd.AddCommand(ingestCmd)
}

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest data from sources into Marmot",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIngestion(cmd.Context())
	},
}

type apiClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func newAPIClient(baseURL, apiKey string) *apiClient {
	return &apiClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *apiClient) batchCreateAssets(ctx context.Context, assets []asset.Asset, config plugin.RawPluginConfig) (map[string]sync.ChangeType, error) {
	input := BatchCreateInput{
		Assets: assets,
		Config: config,
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/assets/batch", input)
	if err != nil {
		return nil, err
	}

	var response struct {
		Assets []struct {
			Asset  *asset.Asset `json:"asset"`
			Status string       `json:"status"`
			Error  string       `json:"error,omitempty"`
		} `json:"assets"`
	}

	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	changes := make(map[string]sync.ChangeType)
	for _, r := range response.Assets {
		if r.Asset != nil && r.Asset.MRN != nil {
			changes[*r.Asset.MRN] = sync.ChangeType(r.Status)
		}
	}
	return changes, nil
}

func (c *apiClient) batchCreateLineage(ctx context.Context, edges []lineage.LineageEdge) (map[string]sync.ChangeType, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/lineage/batch", edges)
	if err != nil {
		return nil, err
	}

	var results []struct {
		Edge   lineage.LineageEdge `json:"edge"`
		Status string              `json:"status"`
	}
	if err := c.do(req, &results); err != nil {
		return nil, err
	}

	changes := make(map[string]sync.ChangeType)
	for _, r := range results {
		edgeKey := fmt.Sprintf("%s -> %s", r.Edge.Source, r.Edge.Target)
		changes[edgeKey] = sync.ChangeType(r.Status)
	}
	return changes, nil
}

func (c *apiClient) batchCreateDocumentation(ctx context.Context, docs []assetdocs.Documentation) (map[string]sync.ChangeType, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/assets/documentation/batch", docs)
	if err != nil {
		return nil, err
	}

	var results []struct {
		Documentation assetdocs.Documentation `json:"documentation"`
		Status        string                  `json:"status"`
	}
	if err := c.do(req, &results); err != nil {
		return nil, err
	}

	changes := make(map[string]sync.ChangeType)
	for _, r := range results {
		changes[r.Documentation.MRN] = sync.ChangeType(r.Status)
	}
	return changes, nil
}

func (c *apiClient) newRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	return req, nil
}

func (c *apiClient) do(req *http.Request, v interface{}) error {
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

func runIngestion(ctx context.Context) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	var config plugin.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	client := newAPIClient(host, apiKey)
	syncer := sync.NewAssetSyncer()
	summary := sync.Summary{
		Assets:        make(map[string]sync.ChangeType),
		Lineage:       make(map[string]sync.ChangeType),
		Documentation: make(map[string]sync.ChangeType),
	}

	for _, run := range config.Runs {
		if err := executeRun(ctx, run, syncer, client, &summary); err != nil {
			return err
		}
	}

	summary.Print()
	return nil
}

func executeRun(ctx context.Context, run plugin.SourceRun, syncer *sync.AssetSyncer, client *apiClient, summary *sync.Summary) error {
	for sourceName, rawConfig := range run {
		src, ok := sourceRegistry[sourceName]
		if !ok {
			return fmt.Errorf("unknown source: %s", sourceName)
		}

		log.Info().Str("source", sourceName).Msg("Starting discovery")

		if err := src().Validate(rawConfig); err != nil {
			return fmt.Errorf("validating source config: %w", err)
		}

		result, err := src().Discover(ctx, rawConfig)
		if err != nil {
			return fmt.Errorf("running discovery: %w", err)
		}

		// Batch sync assets
		if len(result.Assets) > 0 {
			log.Info().Msgf("Syncing %d assets...", len(result.Assets))
			statuses, err := client.batchCreateAssets(ctx, result.Assets, rawConfig)
			if err != nil {
				return fmt.Errorf("syncing assets: %w", err)
			}
			for mrn, status := range statuses {
				summary.Assets[mrn] = status
			}
		}

		// Batch create lineage
		if len(result.Lineage) > 0 {
			log.Info().Msgf("Creating %d lineage edges...", len(result.Lineage))
			statuses, err := client.batchCreateLineage(ctx, result.Lineage)
			if err != nil {
				return fmt.Errorf("creating lineage: %w", err)
			}
			for edge, status := range statuses {
				summary.Lineage[edge] = status
			}
		}

		// Batch create documentation
		if len(result.Documentation) > 0 {
			log.Info().Msgf("Creating %d documentation entries...", len(result.Documentation))
			statuses, err := client.batchCreateDocumentation(ctx, result.Documentation)
			if err != nil {
				return fmt.Errorf("creating documentation: %w", err)
			}
			for mrn, status := range statuses {
				summary.Documentation[mrn] = status
			}
		}

		log.Info().
			Str("source", sourceName).
			Int("assets_synced", len(summary.Assets)).
			Int("lineage_created", len(summary.Lineage)).
			Int("docs_created", len(summary.Documentation)).
			Msg("Source sync complete")
	}

	return nil
}

// sourceRegistry maps source names to their constructor functions
var sourceRegistry = map[string]func() plugin.Source{
	"asyncapi":   func() plugin.Source { return &asyncapi.Source{} },
	"sns":        func() plugin.Source { return &sns.Source{} },
	"sqs":        func() plugin.Source { return &sqs.Source{} },
	"kafka":      func() plugin.Source { return &kafka.Source{} },
	"postgresql": func() plugin.Source { return &postgresql.Source{} },
	"mongodb":    func() plugin.Source { return &mongodb.Source{} },
}
