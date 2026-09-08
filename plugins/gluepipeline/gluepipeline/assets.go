package gluepipeline

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
)

// provider is the service name shared with the Glue plugin, so a workflow,
// the jobs it runs and the tables they read all live in one namespace.
const provider = "Glue"

// assetMRN is the single place identity is built. The Marmot server rebuilds
// it from the asset's type, provider and name, so every reference to an
// asset, including lineage to assets the Glue plugin owns, goes through here.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// taskName qualifies a workflow step with its workflow, which is what makes
// it unique across the account.
func taskName(workflow, node string) string {
	return workflow + "/" + node
}

// node is one step of a workflow: a job, a crawler or a trigger.
type node struct {
	Name     string
	Type     string
	UniqueID string
	Trigger  *types.Trigger
}

// workflowNodes returns the steps of a workflow. AWS reports them in the run
// graph; when the graph is missing, the workflow's own triggers still
// describe its shape, so they are used instead.
func workflowNodes(workflow types.Workflow, triggers []types.Trigger) []node {
	if workflow.Graph != nil && len(workflow.Graph.Nodes) > 0 {
		nodes := make([]node, 0, len(workflow.Graph.Nodes))
		for _, graphNode := range workflow.Graph.Nodes {
			name := safeStr(graphNode.Name)
			if name == "" {
				continue
			}
			n := node{
				Name:     name,
				Type:     nodeType(graphNode.Type),
				UniqueID: safeStr(graphNode.UniqueId),
			}
			if graphNode.TriggerDetails != nil {
				n.Trigger = graphNode.TriggerDetails.Trigger
			}
			nodes = append(nodes, n)
		}
		return nodes
	}

	nodes := make([]node, 0, len(triggers))
	for _, trigger := range triggers {
		name := safeStr(trigger.Name)
		if name == "" {
			continue
		}
		nodes = append(nodes, node{
			Name:     name,
			Type:     "trigger",
			UniqueID: safeStr(trigger.Id),
			Trigger:  &trigger,
		})
	}
	return nodes
}

// nodeType maps a graph node type to the lowercase form used in metadata.
func nodeType(t types.NodeType) string {
	return strings.ToLower(string(t))
}

// pipelineAsset builds the Pipeline asset for a workflow.
func (s *Source) pipelineAsset(ctx context.Context, workflow types.Workflow, nodes []node) pluginsdk.Asset {
	name := safeStr(workflow.Name)
	metadata := map[string]interface{}{
		"node_count":    len(nodes),
		"job_count":     countNodes(nodes, "job"),
		"crawler_count": countNodes(nodes, "crawler"),
		"trigger_count": countNodes(nodes, "trigger"),
	}

	if description := safeStr(workflow.Description); description != "" {
		metadata["description"] = description
	}
	if len(workflow.DefaultRunProperties) > 0 {
		metadata["default_run_properties"] = formatParameters(workflow.DefaultRunProperties)
	}
	if workflow.CreatedOn != nil {
		metadata["created_on"] = workflow.CreatedOn.Format(time.RFC3339)
	}
	if workflow.LastModifiedOn != nil {
		metadata["last_modified_on"] = workflow.LastModifiedOn.Format(time.RFC3339)
	}
	if workflow.MaxConcurrentRuns != nil {
		metadata["max_concurrent_runs"] = *workflow.MaxConcurrentRuns
	}
	if run := workflow.LastRun; run != nil {
		if id := safeStr(run.WorkflowRunId); id != "" {
			metadata["last_run_id"] = id
		}
		if run.Status != "" {
			metadata["last_run_status"] = string(run.Status)
		}
		if run.StartedOn != nil {
			metadata["last_run_started"] = run.StartedOn.Format(time.RFC3339)
		}
		if run.CompletedOn != nil {
			metadata["last_run_completed"] = run.CompletedOn.Format(time.RFC3339)
		}
		if stats := runStatistics(run.Statistics); stats != nil {
			metadata["last_run_statistics"] = formatStatistics(stats)
		}
	}
	if s.region != "" {
		metadata["region"] = s.region
	}
	url := workflowURL(s.region, name)
	if url != "" {
		metadata["url"] = url
	}
	maps.Copy(metadata, pluginsdk.ProcessAWSTags(s.config.TagsToMetadata, s.config.IncludeTags, s.fetchTags(ctx, "workflow", name)))

	var description *string
	if value, ok := metadata["description"].(string); ok {
		description = &value
	}

	mrnValue := assetMRN("Pipeline", name)
	asset := pluginsdk.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Pipeline",
		Providers:   []string{provider},
		Description: description,
		Metadata:    metadata,
		Tags:        pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
	if url != "" {
		asset.ExternalLinks = []pluginsdk.AssetExternalLink{{Name: "Open in AWS Console", URL: url}}
	}
	return asset
}

// taskAsset builds the Task asset for one workflow step.
func (s *Source) taskAsset(workflow string, n node, jobs map[string]types.Job) pluginsdk.Asset {
	name := taskName(workflow, n.Name)
	metadata := map[string]interface{}{
		"workflow":  workflow,
		"node_name": n.Name,
		"node_type": n.Type,
	}

	if n.UniqueID != "" {
		metadata["unique_id"] = n.UniqueID
	}

	switch n.Type {
	case "job":
		metadata["job"] = n.Name
		// The Glue plugin owns the Job asset; these two fields say what the
		// step runs without repeating the whole job definition.
		if job, ok := jobs[n.Name]; ok {
			if job.Command != nil {
				if command := safeStr(job.Command.Name); command != "" {
					metadata["job_type"] = command
				}
				if script := safeStr(job.Command.ScriptLocation); script != "" {
					metadata["job_script_location"] = script
				}
			}
		}
	case "crawler":
		metadata["crawler"] = n.Name
	}

	if trigger := n.Trigger; trigger != nil {
		if trigger.Type != "" {
			metadata["trigger_type"] = string(trigger.Type)
		}
		if schedule := safeStr(trigger.Schedule); schedule != "" {
			metadata["trigger_schedule"] = schedule
		}
		if trigger.State != "" {
			metadata["trigger_state"] = string(trigger.State)
		}
		if predicate := formatPredicate(trigger.Predicate); predicate != "" {
			metadata["trigger_predicate"] = predicate
		}
		if actions := formatActions(trigger.Actions); actions != "" {
			metadata["trigger_actions"] = actions
		}
	}

	var description *string
	if n.Trigger != nil {
		if value := safeStr(n.Trigger.Description); value != "" {
			description = &value
		}
	}

	mrnValue := assetMRN("Task", name)
	return pluginsdk.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Task",
		Providers:   []string{provider},
		Description: description,
		Metadata:    metadata,
		Tags:        pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// workflowLineage links a workflow to its steps, the steps to each other and
// each step to the Glue job or crawler it runs. Job and Crawler assets are
// owned by the Glue plugin, so only the edges are emitted here.
func workflowLineage(workflow string, nodes []node, graph *types.WorkflowGraph) []pluginsdk.LineageEdge {
	pipeline := assetMRN("Pipeline", workflow)
	var edges []pluginsdk.LineageEdge

	taskByID := make(map[string]string, len(nodes))
	for _, n := range nodes {
		task := assetMRN("Task", taskName(workflow, n.Name))
		if n.UniqueID != "" {
			taskByID[n.UniqueID] = task
		}

		edges = append(edges, pluginsdk.LineageEdge{Source: pipeline, Target: task, Type: "CONTAINS"})

		switch n.Type {
		case "job":
			edges = append(edges, pluginsdk.LineageEdge{Source: task, Target: assetMRN("Job", n.Name), Type: "DEPENDS_ON"})
		case "crawler":
			edges = append(edges, pluginsdk.LineageEdge{Source: task, Target: assetMRN("Crawler", n.Name), Type: "DEPENDS_ON"})
		}

		edges = append(edges, triggerLineage(task, n.Trigger)...)
	}

	if graph != nil {
		for _, edge := range graph.Edges {
			source, sourceFound := taskByID[safeStr(edge.SourceId)]
			target, targetFound := taskByID[safeStr(edge.DestinationId)]
			if !sourceFound || !targetFound {
				continue
			}
			edges = append(edges, pluginsdk.LineageEdge{Source: source, Target: target, Type: "DEPENDS_ON"})
		}
	}

	return edges
}

// triggerLineage links a trigger step to the jobs and crawlers it starts,
// and to the ones it waits for.
func triggerLineage(task string, trigger *types.Trigger) []pluginsdk.LineageEdge {
	if trigger == nil {
		return nil
	}

	var edges []pluginsdk.LineageEdge
	for _, action := range trigger.Actions {
		if target := resourceMRN(action.JobName, action.CrawlerName); target != "" {
			edges = append(edges, pluginsdk.LineageEdge{Source: task, Target: target, Type: "DEPENDS_ON"})
		}
	}

	if trigger.Predicate != nil {
		for _, condition := range trigger.Predicate.Conditions {
			if source := resourceMRN(condition.JobName, condition.CrawlerName); source != "" {
				edges = append(edges, pluginsdk.LineageEdge{Source: source, Target: task, Type: "DEPENDS_ON"})
			}
		}
	}

	return edges
}

// resourceMRN returns the MRN of the Glue job or crawler a trigger refers to.
func resourceMRN(jobName, crawlerName *string) string {
	if name := safeStr(jobName); name != "" {
		return assetMRN("Job", name)
	}
	if name := safeStr(crawlerName); name != "" {
		return assetMRN("Crawler", name)
	}
	return ""
}

// crawlerLineage links a crawler to the buckets it reads and the database it
// writes. The Crawler, Bucket and Database assets belong to other plugins.
func crawlerLineage(crawler types.Crawler) []pluginsdk.LineageEdge {
	name := safeStr(crawler.Name)
	if name == "" {
		return nil
	}

	crawlerMRN := assetMRN("Crawler", name)
	var edges []pluginsdk.LineageEdge

	if crawler.Targets != nil {
		for _, target := range crawler.Targets.S3Targets {
			bucket := bucketFromS3Path(safeStr(target.Path))
			if bucket == "" {
				continue
			}
			edges = append(edges, pluginsdk.LineageEdge{
				Source: mrn.New("Bucket", "S3", bucket),
				Target: crawlerMRN,
				Type:   "FEEDS",
			})
		}
	}

	if database := safeStr(crawler.DatabaseName); database != "" {
		edges = append(edges, pluginsdk.LineageEdge{
			Source: crawlerMRN,
			Target: assetMRN("Database", database),
			Type:   "PRODUCES",
		})
	}

	return edges
}

func countNodes(nodes []node, nodeType string) int {
	count := 0
	for _, n := range nodes {
		if n.Type == nodeType {
			count++
		}
	}
	return count
}

// formatParameters renders a string map as a stable, flat metadata value.
func formatParameters(params map[string]string) string {
	parts := make([]string, 0, len(params))
	for _, key := range slices.Sorted(maps.Keys(params)) {
		parts = append(parts, fmt.Sprintf("%s=%s", key, params[key]))
	}
	return strings.Join(parts, ", ")
}

// formatStatistics renders workflow run counters as a flat metadata value.
func formatStatistics(stats map[string]interface{}) string {
	parts := make([]string, 0, len(stats))
	for _, key := range slices.Sorted(maps.Keys(stats)) {
		parts = append(parts, fmt.Sprintf("%s=%v", key, stats[key]))
	}
	return strings.Join(parts, ", ")
}

// formatPredicate renders what a conditional trigger waits for.
func formatPredicate(predicate *types.Predicate) string {
	if predicate == nil || len(predicate.Conditions) == 0 {
		return ""
	}

	parts := make([]string, 0, len(predicate.Conditions))
	for _, condition := range predicate.Conditions {
		if name := safeStr(condition.JobName); name != "" {
			parts = append(parts, strings.TrimSpace(fmt.Sprintf("job %s %s", name, condition.State)))
			continue
		}
		if name := safeStr(condition.CrawlerName); name != "" {
			parts = append(parts, strings.TrimSpace(fmt.Sprintf("crawler %s %s", name, condition.CrawlState)))
		}
	}
	if len(parts) == 0 {
		return ""
	}

	logical := string(predicate.Logical)
	if logical == "" {
		logical = "AND"
	}
	return logical + ": " + strings.Join(parts, ", ")
}

// formatActions renders what a trigger starts.
func formatActions(actions []types.Action) string {
	parts := make([]string, 0, len(actions))
	for _, action := range actions {
		if name := safeStr(action.JobName); name != "" {
			parts = append(parts, "job "+name)
			continue
		}
		if name := safeStr(action.CrawlerName); name != "" {
			parts = append(parts, "crawler "+name)
		}
	}
	return strings.Join(parts, ", ")
}
