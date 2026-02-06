package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

const (
	apiRunsStart        = "/api/v1/runs/start"
	apiRunsComplete     = "/api/v1/runs/complete"
	apiRunsAssetsBatch  = "/api/v1/runs/assets/batch"
	apiPipelineTemplate = "/api/v1/pipelines/%s"
	apiLineageBatch     = "/api/v1/lineage/batch"
	apiDocsBatch        = "/api/v1/assets/documentation/batch"

	statusCreated   = "created"
	statusUpdated   = "updated"
	statusUnchanged = "unchanged"
	statusDeleted   = "deleted"
	statusFailed    = "failed"

	symbolAdd      = "+"
	symbolUpdate   = "~"
	symbolUnchange = "="
	symbolDelete   = "-"

	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorRed    = "\033[31m"
	colorReset  = "\033[0m"

	httpTimeout = 120 * time.Second

	confirmYes = "yes"
	confirmY   = "y"

	maxDisplayEntities = 10
)

var (
	configFile string
	host       string
	apiKey     string
	quiet      bool
	destroy    bool
)

type StartRunRequest struct {
	PipelineName string                 `json:"pipeline_name"`
	SourceName   string                 `json:"source_name"`
	Config       plugin.RawPluginConfig `json:"config"`
}

type CompleteRunRequest struct {
	RunID   string             `json:"run_id"`
	Status  plugin.RunStatus   `json:"status"`
	Summary *plugin.RunSummary `json:"summary"`
	Error   string             `json:"error,omitempty"`
}

type BatchCreateRequest struct {
	Assets        []CreateAssetRequest     `json:"assets"`
	Lineage       []CreateLineageRequest   `json:"lineage"`
	Documentation []CreateDocRequest       `json:"documentation"`
	Statistics    []CreateStatisticRequest `json:"statistics"`
	Config        plugin.RawPluginConfig   `json:"config"`
	PipelineName  string                   `json:"pipeline_name"`
	SourceName    string                   `json:"source_name"`
	RunID         string                   `json:"run_id"`
}

type CreateAssetRequest struct {
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	Providers     []string               `json:"providers"`
	Description   *string                `json:"description"`
	Metadata      map[string]interface{} `json:"metadata"`
	Schema        map[string]interface{} `json:"schema"`
	Tags          []string               `json:"tags"`
	Sources       []string               `json:"sources"`
	ExternalLinks []map[string]string    `json:"external_links"`
}

type CreateLineageRequest struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type CreateDocRequest struct {
	AssetMRN string `json:"asset_mrn"`
	Content  string `json:"content"`
	Type     string `json:"type"`
}

type CreateStatisticRequest struct {
	AssetMRN   string  `json:"asset_mrn"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
}

type BatchCreateResponse struct {
	Assets               []AssetResult         `json:"assets"`
	Lineage              []LineageResult       `json:"lineage"`
	Documentation        []DocumentationResult `json:"documentation"`
	StaleEntitiesRemoved []string              `json:"stale_entities_removed,omitempty"`
}

type AssetResult struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Provider string      `json:"provider"`
	MRN      string      `json:"mrn"`
	Asset    interface{} `json:"asset"`
	Status   string      `json:"status"`
	Error    string      `json:"error,omitempty"`
}

type LineageResult struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type DocumentationResult struct {
	AssetMRN string `json:"asset_mrn"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

type Summary struct {
	AssetsCreated      int
	AssetsUpdated      int
	AssetsDeleted      int
	AssetsUnchanged    int
	LineageCreated     int
	LineageUpdated     int
	DocumentationAdded int
	ErrorsEncountered  int
}

type DestroyRunResponse struct {
	AssetsDeleted        int      `json:"assets_deleted"`
	LineageDeleted       int      `json:"lineage_deleted"`
	DocumentationDeleted int      `json:"documentation_deleted"`
	DeletedEntityMRNs    []string `json:"deleted_entity_mrns"`
}

func init() {
	ingestCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to ingestion config file (required)")
	ingestCmd.Flags().StringVarP(&host, "host", "H", "http://localhost:8080", "Marmot API host")
	ingestCmd.Flags().StringVarP(&apiKey, "api-key", "k", "", "API key for authentication")
	ingestCmd.Flags().BoolVarP(&quiet, "quiet", "q", true, "Hide info logs, show errors only")
	ingestCmd.Flags().BoolVarP(&destroy, "destroy", "d", false, "Delete all resources for this pipeline (requires confirmation)")
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
		client:  &http.Client{Timeout: httpTimeout},
	}
}

func (c *apiClient) startRun(ctx context.Context, request StartRunRequest) (*plugin.Run, error) {
	req, err := c.newRequest(ctx, http.MethodPost, apiRunsStart, request)
	if err != nil {
		return nil, err
	}

	var run plugin.Run
	if err := c.do(req, &run); err != nil {
		return nil, err
	}

	return &run, nil
}

func (c *apiClient) completeRun(ctx context.Context, request CompleteRunRequest) error {
	req, err := c.newRequest(ctx, http.MethodPost, apiRunsComplete, request)
	if err != nil {
		return err
	}

	return c.do(req, nil)
}

func (c *apiClient) batchCreateAssets(ctx context.Context, request BatchCreateRequest) (*BatchCreateResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, apiRunsAssetsBatch, request)
	if err != nil {
		return nil, err
	}

	var response BatchCreateResponse
	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *apiClient) destroyPipeline(ctx context.Context, pipelineName string) (*DestroyRunResponse, error) {
	path := fmt.Sprintf(apiPipelineTemplate, pipelineName)
	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}

	var response DestroyRunResponse
	if err := c.do(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
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

	if v != nil {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	return nil
}

func runIngestion(ctx context.Context) error {
	if quiet {
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	var config plugin.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	if config.Name == "" {
		return fmt.Errorf("pipeline name is required")
	}

	client := newAPIClient(host, apiKey)

	if destroy {
		return runDestroy(ctx, config, client)
	}

	fmt.Printf("Pipeline: %s\n\n", config.Name)

	overallSummary := &Summary{}

	for _, run := range config.Runs {
		if err := executeRun(ctx, run, client, overallSummary, config); err != nil {
			return err
		}
	}

	printSummary(overallSummary)
	return nil
}

func runDestroy(ctx context.Context, config plugin.Config, client *apiClient) error {
	fmt.Printf("Pipeline: %s\n\n", config.Name)

	fmt.Printf("⚠️  WARNING: This will permanently delete ALL resources from pipeline: %s\n", config.Name)
	fmt.Printf("   This includes all assets, lineage, and documentation created by any source in this pipeline.\n")
	fmt.Printf("\n")

	fmt.Printf("This action cannot be undone. Are you sure you want to continue? (y/N): ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != confirmY && response != confirmYes {
		fmt.Println("Operation cancelled.")
		return nil
	}

	fmt.Println()

	printStep("Destroying pipeline resources...")
	destroyResp, err := client.destroyPipeline(ctx, config.Name)
	if err != nil {
		printError(fmt.Sprintf("Failed to destroy pipeline: %v", err))
		return err
	}

	totalDeleted := destroyResp.AssetsDeleted + destroyResp.LineageDeleted + destroyResp.DocumentationDeleted

	if totalDeleted == 0 {
		printWarning("No resources found to delete")
	} else {
		printSuccess(fmt.Sprintf("Deleted %d assets, %d lineage edges, %d documentation entries",
			destroyResp.AssetsDeleted, destroyResp.LineageDeleted, destroyResp.DocumentationDeleted))

		if len(destroyResp.DeletedEntityMRNs) > 0 {
			fmt.Printf("  Deleted entities:\n")
			for i, mrn := range destroyResp.DeletedEntityMRNs {
				if i >= maxDisplayEntities {
					fmt.Printf("    ... and %d more\n", len(destroyResp.DeletedEntityMRNs)-maxDisplayEntities)
					break
				}
				fmt.Printf("    - %s\n", mrn)
			}
		}
	}

	fmt.Println()
	printDestroySummary(destroyResp, totalDeleted, config.Name)
	fmt.Println()
	return nil
}

func executeRun(ctx context.Context, run plugin.SourceRun, client *apiClient, overallSummary *Summary, config plugin.Config) error {
	registry := plugin.GetRegistry()

	for sourceName, rawConfig := range run {
		entry, err := registry.Get(sourceName)
		if err != nil {
			return fmt.Errorf("unknown source: %s", sourceName)
		}

		source := entry.Source

		printSourceHeader(sourceName)

		_, err = source.Validate(rawConfig)
		if err != nil {
			printError(fmt.Sprintf("Config validation failed: %v", err))
			return err
		}

		maskedConfig := plugin.MaskSensitiveFieldsFromSpec(rawConfig, entry.Meta.ConfigSpec)

		printStep("Starting run...")
		startTime := time.Now()

		runStartReq := StartRunRequest{
			PipelineName: config.Name,
			SourceName:   sourceName,
			Config:       maskedConfig,
		}

		ingestionRun, err := client.startRun(ctx, runStartReq)
		if err != nil {
			printError(fmt.Sprintf("Failed to start run: %v", err))
			return err
		}

		printSuccess(fmt.Sprintf("Run started (ID: %s)", ingestionRun.RunID))

		printStep("Discovering assets...")
		result, err := source.Discover(ctx, rawConfig)
		discoveryTime := time.Since(startTime)

		if err != nil {
			printError(fmt.Sprintf("Discovery failed: %v", err))
			_ = client.completeRun(ctx, CompleteRunRequest{
				RunID:  ingestionRun.RunID,
				Status: plugin.StatusFailed,
				Error:  err.Error(),
			})
			return err
		}

		plugin.FilterDiscoveryResult(result, rawConfig)

		if len(result.Assets) == 0 {
			printWarning("No assets discovered")
			summary := &plugin.RunSummary{
				TotalEntities:   0,
				DurationSeconds: int(discoveryTime.Seconds()),
			}
			_ = client.completeRun(ctx, CompleteRunRequest{
				RunID:   ingestionRun.RunID,
				Status:  plugin.StatusCompleted,
				Summary: summary,
			})
			continue
		}

		printSuccess(fmt.Sprintf("Discovered %d assets in %v", len(result.Assets), discoveryTime))

		runSummary := &plugin.RunSummary{
			TotalEntities: len(result.Assets) + len(result.Lineage) + len(result.Documentation),
		}

		if len(result.Assets) > 0 {
			printStep("Syncing assets...")

			totalAssets := len(result.Assets)
			assets := make([]CreateAssetRequest, 0, len(result.Assets))

			for i, asset := range result.Assets {
				printProgress(fmt.Sprintf("Preparing assets (%d/%d)", i+1, totalAssets))

				name := ""
				if asset.Name != nil {
					name = *asset.Name
				}

				schema := make(map[string]interface{})
				for k, v := range asset.Schema {
					schema[k] = v
				}

				sources := make([]string, len(asset.Sources))
				for j, source := range asset.Sources {
					sources[j] = source.Name
				}

				assets = append(assets, CreateAssetRequest{
					Name:          name,
					Type:          asset.Type,
					Providers:     asset.Providers,
					Description:   asset.Description,
					Metadata:      asset.Metadata,
					Schema:        schema,
					Tags:          asset.Tags,
					Sources:       sources,
					ExternalLinks: convertExternalLinks(asset.ExternalLinks),
				})
			}

			lineage := make([]CreateLineageRequest, 0, len(result.Lineage))
			for _, edge := range result.Lineage {
				lineage = append(lineage, CreateLineageRequest{
					Source: edge.Source,
					Target: edge.Target,
					Type:   edge.Type,
				})
			}

			documentation := make([]CreateDocRequest, 0, len(result.Documentation))
			for _, doc := range result.Documentation {
				documentation = append(documentation, CreateDocRequest{
					AssetMRN: doc.MRN,
					Content:  doc.Content,
					Type:     doc.Source,
				})
			}

			batchReq := BatchCreateRequest{
				Assets:        assets,
				Lineage:       lineage,
				Documentation: documentation,
				Config:        maskedConfig,
				PipelineName:  config.Name,
				SourceName:    sourceName,
				RunID:         ingestionRun.RunID,
			}

			printProgress("Syncing with server...")
			assetResponse, err := client.batchCreateAssets(ctx, batchReq)
			if err != nil {
				clearProgress()
				printError(fmt.Sprintf("Asset sync failed: %v", err))
				_ = client.completeRun(ctx, CompleteRunRequest{
					RunID:  ingestionRun.RunID,
					Status: plugin.StatusFailed,
					Error:  err.Error(),
				})
				return err
			}

			processAssetResults(assetResponse.Assets, runSummary, overallSummary)
			processLineageResults(assetResponse.Lineage, runSummary, overallSummary)
			processDocumentationResults(assetResponse.Documentation, runSummary, overallSummary)

			runSummary.AssetsDeleted = len(assetResponse.StaleEntitiesRemoved)
			overallSummary.AssetsDeleted += runSummary.AssetsDeleted

			for _, staleMRN := range assetResponse.StaleEntitiesRemoved {
				printChange(symbolDelete, "asset", "", staleMRN, statusDeleted)
			}

			printAssetSummary(runSummary, overallSummary)
		}

		totalTime := time.Since(startTime)
		runSummary.DurationSeconds = int(totalTime.Seconds())

		status := plugin.StatusCompleted
		if runSummary.ErrorsCount > 0 {
			status = plugin.StatusFailed
		}

		err = client.completeRun(ctx, CompleteRunRequest{
			RunID:   ingestionRun.RunID,
			Status:  status,
			Summary: runSummary,
		})

		if err != nil {
			printWarning(fmt.Sprintf("Failed to complete run: %v", err))
		}

		printSuccess(fmt.Sprintf("Run completed in %v", totalTime))

		if runSummary.ErrorsCount > 0 {
			printWarning(fmt.Sprintf("%d errors encountered", runSummary.ErrorsCount))
		}

		fmt.Println()
	}

	return nil
}

func processAssetResults(results []AssetResult, runSummary *plugin.RunSummary, overallSummary *Summary) {
	clearProgress()

	for _, result := range results {
		if result.Error != "" {
			runSummary.ErrorsCount++
			overallSummary.ErrorsEncountered++
			printChange(symbolDelete, result.Type, result.Provider, result.Name, "error")
		} else {
			switch result.Status {
			case statusCreated:
				runSummary.AssetsCreated++
				overallSummary.AssetsCreated++
				printChange(symbolAdd, result.Type, result.Provider, result.Name, statusCreated)
			case statusUpdated:
				runSummary.AssetsUpdated++
				overallSummary.AssetsUpdated++
				printChange(symbolUpdate, result.Type, result.Provider, result.Name, statusUpdated)
			case statusUnchanged:
				overallSummary.AssetsUnchanged++
				printChange(symbolUnchange, result.Type, result.Provider, result.Name, statusUnchanged)
			}
		}
	}
}

func processLineageResults(results []LineageResult, runSummary *plugin.RunSummary, overallSummary *Summary) {
	for _, result := range results {
		if result.Error != "" {
			runSummary.ErrorsCount++
		} else {
			switch result.Status {
			case statusCreated:
				runSummary.LineageCreated++
				overallSummary.LineageCreated++
				printChange(symbolAdd, "lineage", result.Type, fmt.Sprintf("%s -> %s", result.Source, result.Target), statusCreated)
			case statusUpdated:
				runSummary.LineageUpdated++
				overallSummary.LineageUpdated++
				printChange(symbolUpdate, "lineage", result.Type, fmt.Sprintf("%s -> %s", result.Source, result.Target), statusUpdated)
			}
		}
	}
}

func processDocumentationResults(results []DocumentationResult, runSummary *plugin.RunSummary, overallSummary *Summary) {
	for _, result := range results {
		if result.Error != "" {
			runSummary.ErrorsCount++
		} else if result.Status == statusCreated {
			runSummary.DocumentationAdded++
			overallSummary.DocumentationAdded++
			printChange(symbolAdd, "documentation", result.Type, result.AssetMRN, statusCreated)
		}
	}
}

func convertExternalLinks(links []asset.ExternalLink) []map[string]string {
	result := make([]map[string]string, 0, len(links))
	for _, link := range links {
		result = append(result, map[string]string{
			"name": link.Name,
			"url":  link.URL,
		})
	}
	return result
}

func printProgress(message string) {
	fmt.Printf("\r  %s", message)
}

func clearProgress() {
	fmt.Print("\r\033[K")
}

func printAssetSummary(runSummary *plugin.RunSummary, overallSummary *Summary) {
	if overallSummary.AssetsUnchanged > 0 {
		printSuccess(fmt.Sprintf("Assets synced: %d created, %d updated, %d unchanged, %d deleted",
			runSummary.AssetsCreated, runSummary.AssetsUpdated, overallSummary.AssetsUnchanged, runSummary.AssetsDeleted))
	} else {
		printSuccess(fmt.Sprintf("Assets synced: %d created, %d updated, %d deleted",
			runSummary.AssetsCreated, runSummary.AssetsUpdated, runSummary.AssetsDeleted))
	}
}

func printSourceHeader(sourceName string) {
	fmt.Printf("📊 %s\n", sourceName)
	fmt.Printf("%s\n", strings.Repeat("-", len(sourceName)+4))
}

func printStep(message string) {
	fmt.Printf("  %s\n", message)
}

func printSuccess(message string) {
	fmt.Printf("  ✅ %s\n", message)
}

func printWarning(message string) {
	fmt.Printf("  ⚠️  %s\n", message)
}

func printError(message string) {
	fmt.Printf("  ❌ %s\n", message)
}

func printChange(symbol, resourceType, provider, name, action string) {
	var color string
	switch symbol {
	case symbolAdd:
		color = colorGreen
	case symbolUpdate:
		color = colorYellow
	case symbolUnchange:
		color = colorCyan
	case symbolDelete:
		color = colorRed
	default:
		color = colorReset
	}

	if provider != "" {
		fmt.Printf("  %s%s %s.%s.%s%s\n", color, symbol, resourceType, provider, name, colorReset)
	} else {
		fmt.Printf("  %s%s %s.%s%s\n", color, symbol, resourceType, name, colorReset)
	}
}

func printSummary(summary *Summary) {
	changes := []string{}

	if summary.AssetsCreated > 0 {
		changes = append(changes, fmt.Sprintf("%s%d created%s", colorGreen, summary.AssetsCreated, colorReset))
	}
	if summary.AssetsUpdated > 0 {
		changes = append(changes, fmt.Sprintf("%s%d updated%s", colorYellow, summary.AssetsUpdated, colorReset))
	}
	if summary.AssetsUnchanged > 0 {
		changes = append(changes, fmt.Sprintf("%s%d unchanged%s", colorCyan, summary.AssetsUnchanged, colorReset))
	}
	if summary.AssetsDeleted > 0 {
		changes = append(changes, fmt.Sprintf("%s%d deleted%s", colorRed, summary.AssetsDeleted, colorReset))
	}

	if len(changes) > 0 {
		fmt.Printf("Assets: %s\n", strings.Join(changes, ", "))
	}

	changes = []string{}
	if summary.LineageCreated > 0 {
		changes = append(changes, fmt.Sprintf("%s%d created%s", colorGreen, summary.LineageCreated, colorReset))
	}
	if summary.LineageUpdated > 0 {
		changes = append(changes, fmt.Sprintf("%s%d updated%s", colorYellow, summary.LineageUpdated, colorReset))
	}

	if len(changes) > 0 {
		fmt.Printf("Lineage: %s\n", strings.Join(changes, ", "))
	}

	if summary.DocumentationAdded > 0 {
		fmt.Printf("Documentation: %s%d added%s\n", colorGreen, summary.DocumentationAdded, colorReset)
	}

	if summary.ErrorsEncountered > 0 {
		fmt.Printf("Errors: %s%d encountered%s\n", colorRed, summary.ErrorsEncountered, colorReset)
		fmt.Printf("\n%s⚠️  Some operations completed with errors%s\n", colorYellow, colorReset)
	} else {
		fmt.Printf("\n%s✅ All operations completed successfully%s\n", colorGreen, colorReset)
	}

	fmt.Println()
}

func printDestroySummary(response *DestroyRunResponse, totalDeleted int, pipelineName string) {
	fmt.Printf("🗑️  Destruction Summary\n")
	fmt.Printf("Pipeline: %s\n", pipelineName)

	if totalDeleted > 0 {
		changes := []string{}
		if response.AssetsDeleted > 0 {
			changes = append(changes, fmt.Sprintf("%s%d assets%s", colorRed, response.AssetsDeleted, colorReset))
		}
		if response.LineageDeleted > 0 {
			changes = append(changes, fmt.Sprintf("%s%d lineage%s", colorRed, response.LineageDeleted, colorReset))
		}
		if response.DocumentationDeleted > 0 {
			changes = append(changes, fmt.Sprintf("%s%d docs%s", colorRed, response.DocumentationDeleted, colorReset))
		}

		fmt.Printf("Deleted: %s\n", strings.Join(changes, ", "))
		fmt.Printf("\n%s✅ All pipeline resources have been permanently deleted%s\n", colorGreen, colorReset)
	} else {
		fmt.Printf("\n%s⚠️  No resources were found to delete%s\n", colorYellow, colorReset)
	}
}

