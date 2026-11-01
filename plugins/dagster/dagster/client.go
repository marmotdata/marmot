package dagster

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client talks to a Dagster webserver's GraphQL endpoint. Dagster exposes a
// single endpoint for everything, so there is one request helper and a method
// per document this plugin sends.
type Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

// ClientConfig configures a Client.
type ClientConfig struct {
	Host      string
	Token     string
	VerifySSL bool
}

// NewClient returns a Client for the Dagster webserver at cfg.Host.
func NewClient(cfg ClientConfig) *Client {
	transport := http.DefaultTransport
	if !cfg.VerifySSL {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return &Client{
		endpoint: cfg.Host + "/graphql",
		token:    cfg.Token,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// PipelineSelector points a query at one job inside one code location.
// Dagster still calls this a pipeline selector for backwards compatibility.
type PipelineSelector struct {
	PipelineName           string `json:"pipelineName"`
	RepositoryName         string `json:"repositoryName"`
	RepositoryLocationName string `json:"repositoryLocationName"`
}

// RepositorySelector points a query at one repository inside one code location.
type RepositorySelector struct {
	RepositoryName         string `json:"repositoryName"`
	RepositoryLocationName string `json:"repositoryLocationName"`
}

// Location is a Dagster code location.
type Location struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Tag is a key/value pair attached to a job or a run.
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Schedule is a cron schedule that launches a job.
type Schedule struct {
	Name          string `json:"name"`
	CronSchedule  string `json:"cronSchedule"`
	ScheduleState *struct {
		Status string `json:"status"`
	} `json:"scheduleState"`
}

// Sensor watches for external state and launches a job.
type Sensor struct {
	Name        string `json:"name"`
	SensorState *struct {
		Status string `json:"status"`
	} `json:"sensorState"`
}

// Job is a Dagster job: a graph of ops that can be launched as one run.
type Job struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	IsJob       bool       `json:"isJob"`
	IsAssetJob  bool       `json:"isAssetJob"`
	Tags        []Tag      `json:"tags"`
	Schedules   []Schedule `json:"schedules"`
	Sensors     []Sensor   `json:"sensors"`
}

// Repository groups the jobs and assets defined by one code location.
type Repository struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Location Location `json:"location"`
	Jobs     []Job    `json:"jobs"`
}

// Selector returns the selector that addresses this repository.
func (r Repository) Selector() RepositorySelector {
	return RepositorySelector{
		RepositoryName:         r.Name,
		RepositoryLocationName: r.Location.Name,
	}
}

// SolidHandle is one op in a job's graph. Dagster keeps the pre-1.0 name
// "solid" in its GraphQL schema; an op and a solid are the same thing.
type SolidHandle struct {
	HandleID string `json:"handleID"`
	Solid    struct {
		Name       string `json:"name"`
		Definition struct {
			Name        string  `json:"name"`
			Description *string `json:"description"`
		} `json:"definition"`
		Inputs []struct {
			Definition struct {
				Name string `json:"name"`
			} `json:"definition"`
			DependsOn []struct {
				Solid struct {
					Name string `json:"name"`
				} `json:"solid"`
				Definition struct {
					Name string `json:"name"`
				} `json:"definition"`
			} `json:"dependsOn"`
		} `json:"inputs"`
		Outputs []struct {
			Definition struct {
				Name string `json:"name"`
			} `json:"definition"`
		} `json:"outputs"`
	} `json:"solid"`
}

// StepStat is the outcome of one op within a run.
type StepStat struct {
	StepKey   string   `json:"stepKey"`
	Status    string   `json:"status"`
	StartTime *float64 `json:"startTime"`
	EndTime   *float64 `json:"endTime"`
}

// Run is one execution of a job. The times are epoch seconds and are null
// while a run is still queued.
type Run struct {
	RunID      string     `json:"runId"`
	Status     string     `json:"status"`
	StartTime  *float64   `json:"startTime"`
	EndTime    *float64   `json:"endTime"`
	UpdateTime *float64   `json:"updateTime"`
	Tags       []Tag      `json:"tags"`
	StepStats  []StepStat `json:"stepStats"`
}

// AssetKey is the multi-part key that identifies a software-defined asset.
type AssetKey struct {
	Path []string `json:"path"`
}

// TableColumn is one column of a TableSchema metadata entry.
type TableColumn struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description *string `json:"description"`
	Constraints *struct {
		Nullable bool `json:"nullable"`
		Unique   bool `json:"unique"`
	} `json:"constraints"`
}

// MetadataEntry is one label/value pair attached to an asset. Dagster returns
// a union, so only the field matching Typename is populated.
type MetadataEntry struct {
	Typename   string `json:"__typename"`
	Label      string `json:"label"`
	Text       string `json:"text"`
	URL        string `json:"url"`
	Path       string `json:"path"`
	JSONString string `json:"jsonString"`
	Schema     *struct {
		Columns []TableColumn `json:"columns"`
	} `json:"schema"`
}

// AssetEdge is one end of an asset-to-asset dependency.
type AssetEdge struct {
	Asset struct {
		AssetKey AssetKey `json:"assetKey"`
	} `json:"asset"`
}

// Materialization records that an asset was written by a run.
// Timestamp is epoch milliseconds, serialised as a string.
type Materialization struct {
	RunID     string `json:"runId"`
	Timestamp string `json:"timestamp"`
}

// AssetNode is a software-defined asset as declared in code.
type AssetNode struct {
	ID                    string            `json:"id"`
	AssetKey              AssetKey          `json:"assetKey"`
	Description           *string           `json:"description"`
	ComputeKind           *string           `json:"computeKind"`
	GroupName             *string           `json:"groupName"`
	OpNames               []string          `json:"opNames"`
	JobNames              []string          `json:"jobNames"`
	IsPartitioned         bool              `json:"isPartitioned"`
	IsObservable          bool              `json:"isObservable"`
	IsMaterializable      bool              `json:"isMaterializable"`
	MetadataEntries       []MetadataEntry   `json:"metadataEntries"`
	Dependencies          []AssetEdge       `json:"dependencies"`
	DependedBy            []AssetEdge       `json:"dependedBy"`
	AssetMaterializations []Materialization `json:"assetMaterializations"`
}

const repositoriesQuery = `query MarmotRepositories {
  repositoriesOrError {
    __typename
    ... on RepositoryConnection {
      nodes {
        id
        name
        location { id name }
        jobs {
          id
          name
          description
          isJob
          isAssetJob
          tags { key value }
          schedules { name cronSchedule scheduleState { status } }
          sensors { name sensorState { status } }
        }
      }
    }
    ... on PythonError { message }
  }
}`

const opHandlesQuery = `query MarmotOpHandles($selector: PipelineSelector!) {
  pipelineOrError(params: $selector) {
    __typename
    ... on Pipeline {
      id
      name
      solidHandles {
        handleID
        solid {
          name
          definition { name description }
          inputs {
            definition { name }
            dependsOn { solid { name } definition { name } }
          }
          outputs { definition { name } }
        }
      }
    }
    ... on PipelineNotFoundError { message }
    ... on PythonError { message }
  }
}`

const runsQuery = `query MarmotRuns($filter: RunsFilter!, $limit: Int!) {
  runsOrError(filter: $filter, limit: $limit) {
    __typename
    ... on Runs {
      results {
        runId
        status
        startTime
        endTime
        updateTime
        tags { key value }
        stepStats { stepKey status startTime endTime }
      }
    }
    ... on InvalidPipelineRunsFilterError { message }
    ... on PythonError { message }
  }
}`

const assetNodesQuery = `query MarmotAssetNodes($selector: RepositorySelector!) {
  repositoryOrError(repositorySelector: $selector) {
    __typename
    ... on Repository {
      assetNodes {
        id
        assetKey { path }
        description
        computeKind
        groupName
        opNames
        jobNames
        isPartitioned
        isObservable
        isMaterializable
        metadataEntries {
          __typename
          label
          ... on TextMetadataEntry { text }
          ... on UrlMetadataEntry { url }
          ... on PathMetadataEntry { path }
          ... on JsonMetadataEntry { jsonString }
          ... on TableSchemaMetadataEntry {
            schema { columns { name type description constraints { nullable unique } } }
          }
        }
        dependencies { asset { assetKey { path } } }
        dependedBy { asset { assetKey { path } } }
        assetMaterializations(limit: 1) { runId timestamp }
      }
    }
    ... on RepositoryNotFoundError { message }
    ... on PythonError { message }
  }
}`

// ListRepositories returns every repository the webserver can load.
func (c *Client) ListRepositories(ctx context.Context) ([]Repository, error) {
	var out struct {
		RepositoriesOrError struct {
			Typename string       `json:"__typename"`
			Nodes    []Repository `json:"nodes"`
			Message  string       `json:"message"`
		} `json:"repositoriesOrError"`
	}

	if err := c.do(ctx, repositoriesQuery, nil, &out); err != nil {
		return nil, err
	}
	if err := unionError(out.RepositoriesOrError.Typename, "RepositoryConnection", out.RepositoriesOrError.Message); err != nil {
		return nil, err
	}

	return out.RepositoriesOrError.Nodes, nil
}

// JobOpHandles returns the ops of one job, each with its input dependencies.
func (c *Client) JobOpHandles(ctx context.Context, repo Repository, jobName string) ([]SolidHandle, error) {
	variables := map[string]any{
		"selector": PipelineSelector{
			PipelineName:           jobName,
			RepositoryName:         repo.Name,
			RepositoryLocationName: repo.Location.Name,
		},
	}

	var out struct {
		PipelineOrError struct {
			Typename     string        `json:"__typename"`
			SolidHandles []SolidHandle `json:"solidHandles"`
			Message      string        `json:"message"`
		} `json:"pipelineOrError"`
	}

	if err := c.do(ctx, opHandlesQuery, variables, &out); err != nil {
		return nil, err
	}
	if err := unionError(out.PipelineOrError.Typename, "Pipeline", out.PipelineOrError.Message); err != nil {
		return nil, err
	}

	return out.PipelineOrError.SolidHandles, nil
}

// JobRuns returns the most recent runs of one job, newest first.
func (c *Client) JobRuns(ctx context.Context, jobName string, limit int) ([]Run, error) {
	variables := map[string]any{
		"filter": map[string]any{"pipelineName": jobName},
		"limit":  limit,
	}

	var out struct {
		RunsOrError struct {
			Typename string `json:"__typename"`
			Results  []Run  `json:"results"`
			Message  string `json:"message"`
		} `json:"runsOrError"`
	}

	if err := c.do(ctx, runsQuery, variables, &out); err != nil {
		return nil, err
	}
	if err := unionError(out.RunsOrError.Typename, "Runs", out.RunsOrError.Message); err != nil {
		return nil, err
	}

	return out.RunsOrError.Results, nil
}

// RepositoryAssetNodes returns the software-defined assets of one repository.
func (c *Client) RepositoryAssetNodes(ctx context.Context, repo Repository) ([]AssetNode, error) {
	variables := map[string]any{"selector": repo.Selector()}

	var out struct {
		RepositoryOrError struct {
			Typename   string      `json:"__typename"`
			AssetNodes []AssetNode `json:"assetNodes"`
			Message    string      `json:"message"`
		} `json:"repositoryOrError"`
	}

	if err := c.do(ctx, assetNodesQuery, variables, &out); err != nil {
		return nil, err
	}
	if err := unionError(out.RepositoryOrError.Typename, "Repository", out.RepositoryOrError.Message); err != nil {
		return nil, err
	}

	return out.RepositoryOrError.AssetNodes, nil
}

// graphQLRequest is the POST body Dagster's endpoint expects.
type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// do posts one GraphQL document and decodes the "data" object into out.
func (c *Client) do(ctx context.Context, query string, variables map[string]any, out any) error {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// Dagster+ authenticates with its own header rather than Authorization.
	if c.token != "" {
		req.Header.Set("Dagster-Cloud-Api-Token", c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("dagster returned status %d: %s", resp.StatusCode, truncate(string(payload), 300))
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("dagster graphql error: %s", envelope.Errors[0].Message)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("dagster returned no data")
	}

	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decoding data: %w", err)
	}
	return nil
}

// unionError turns a GraphQL union that resolved to an error branch into a Go
// error. Dagster answers with HTTP 200 and an error member, so a caller that
// only checks the status code would silently read an empty result.
func unionError(typename, want, message string) error {
	if typename == want {
		return nil
	}
	if message != "" {
		return fmt.Errorf("dagster returned %s: %s", typename, message)
	}
	return fmt.Errorf("dagster returned %s, expected %s", typename, want)
}

// truncate shortens a server response so an error stays readable in a log line.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
