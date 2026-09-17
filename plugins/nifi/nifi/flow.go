package nifi

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// maxDepth bounds the group walk. NiFi flows are trees, so this only
// guards against a pathological API answer.
const maxDepth = 100

// group is one process group with everything discovery needs from it.
// The starting group's component comes from its own endpoint, a child's
// from the parent's flow listing; both carry the same fields.
type group struct {
	component processGroupComponent
	flow      flowContents
	name      string
	parent    *group
}

// topicUsage records which tasks publish to and consume from a Kafka
// topic, so the Topic asset can list them.
type topicUsage struct {
	producers []string
	consumers []string
}

// discovery accumulates the assets and edges of one run.
type discovery struct {
	config  *Config
	client  *client
	version string
	now     time.Time

	groups    []*group
	groupMRNs map[string]string
	taskMRNs  map[string]string
	topics    map[string]*topicUsage
	services  map[string]*controllerServiceComponent
	edges     map[string]struct{}

	assets  []pluginsdk.Asset
	lineage []pluginsdk.LineageEdge
}

func newDiscovery(config *Config, c *client) *discovery {
	return &discovery{
		config:    config,
		client:    c,
		now:       time.Now(),
		groupMRNs: make(map[string]string),
		taskMRNs:  make(map[string]string),
		topics:    make(map[string]*topicUsage),
		services:  make(map[string]*controllerServiceComponent),
		edges:     make(map[string]struct{}),
	}
}

// run fetches the whole group tree first and only then builds assets,
// because a connection between two groups is listed under their parent
// and the tasks it links are only named once both groups are known.
func (d *discovery) run(ctx context.Context) error {
	version, err := d.client.version(ctx)
	if err != nil {
		return fmt.Errorf("reading NiFi version: %w", err)
	}
	d.version = version

	start := d.config.RootProcessGroup
	if start == "" {
		start = "root"
	}

	entity, err := d.client.processGroup(ctx, start)
	if err != nil {
		return fmt.Errorf("reading process group %s: %w", start, err)
	}
	flow, err := d.client.processGroupFlow(ctx, start)
	if err != nil {
		return fmt.Errorf("reading process group flow %s: %w", start, err)
	}

	root := &group{
		component: entity.Component,
		flow:      flow.Flow,
		name:      breadcrumbPath(flow.Breadcrumb),
	}
	d.groups = append(d.groups, root)
	d.walkChildren(ctx, root, 1)

	log.Debug().Int("groups", len(d.groups)).Str("version", d.version).Msg("Fetched process groups")

	for _, g := range d.groups {
		d.emitGroup(ctx, g)
	}
	for _, g := range d.groups {
		d.emitConnections(g)
	}
	d.emitTopics()

	return nil
}

// walkChildren fetches every child group's flow, depth first. A child
// that fails to load is skipped with its subtree; the rest of the flow
// is still worth cataloguing.
func (d *discovery) walkChildren(ctx context.Context, parent *group, depth int) {
	if depth > maxDepth {
		log.Warn().Str("group", parent.name).Int("depth", depth).Msg("Process group nesting too deep, not descending further")
		return
	}

	names := uniqueNames(groupNames(parent.flow.ProcessGroups))

	for _, child := range parent.flow.ProcessGroups {
		flow, err := d.client.processGroupFlow(ctx, child.ID)
		if err != nil {
			log.Warn().Err(err).Str("group", child.Component.Name).Msg("Failed to read process group, skipping it")
			continue
		}

		g := &group{
			component: child.Component,
			flow:      flow.Flow,
			name:      parent.name + "/" + names[child.ID],
			parent:    parent,
		}
		d.groups = append(d.groups, g)
		d.walkChildren(ctx, g, depth+1)
	}
}

// emitGroup builds the group's Pipeline, one Task per processor (and per
// port when enabled), the CONTAINS edges between them and the data
// lineage of each processor.
func (d *discovery) emitGroup(ctx context.Context, g *group) {
	pipelineMRN := assetMRN("Pipeline", g.name)
	d.groupMRNs[g.component.ID] = pipelineMRN

	metadata := map[string]any{
		"id":                g.component.ID,
		"path":              g.name,
		"running_count":     g.component.RunningCount,
		"stopped_count":     g.component.StoppedCount,
		"invalid_count":     g.component.InvalidCount,
		"disabled_count":    g.component.DisabledCount,
		"processor_count":   len(g.flow.Processors),
		"connection_count":  len(g.flow.Connections),
		"input_port_count":  len(g.flow.InputPorts),
		"output_port_count": len(g.flow.OutputPorts),
		"url":               d.groupURL(g.component.ID),
	}
	putIf(metadata, "parent_id", g.component.ParentGroupID)
	putIf(metadata, "comments", g.component.Comments)
	putIf(metadata, "nifi_version", d.version)
	if g.component.ParameterContext != nil {
		putIf(metadata, "parameter_context", g.component.ParameterContext.Component.Name)
	}

	d.assets = append(d.assets, d.newAsset("Pipeline", g.name, g.component.Comments, metadata, d.groupURL(g.component.ID)))

	if g.parent != nil {
		d.link(d.groupMRNs[g.parent.component.ID], pipelineMRN, "CONTAINS")
	}

	// Processors and ports share one namespace within the group, so a
	// port named like a processor gets the same disambiguation.
	var members []namedID
	if d.config.IncludeProcessors {
		for _, p := range g.flow.Processors {
			members = append(members, namedID{p.ID, p.Component.Name})
		}
	}
	if d.config.IncludePorts {
		for _, p := range g.flow.InputPorts {
			members = append(members, namedID{p.ID, p.Component.Name})
		}
		for _, p := range g.flow.OutputPorts {
			members = append(members, namedID{p.ID, p.Component.Name})
		}
	}
	names := uniqueNames(members)

	if d.config.IncludeProcessors {
		for _, p := range g.flow.Processors {
			taskName := g.name + "/" + names[p.ID]
			taskMRN := d.emitProcessor(g, p, taskName)
			d.link(pipelineMRN, taskMRN, "CONTAINS")

			if d.config.DiscoverLineage {
				d.dataLineage(ctx, p.Component, taskMRN, taskName)
			}
		}
	}

	if d.config.IncludePorts {
		for _, p := range append(append([]portEntity{}, g.flow.InputPorts...), g.flow.OutputPorts...) {
			taskMRN := d.emitPort(g, p, g.name+"/"+names[p.ID])
			d.link(pipelineMRN, taskMRN, "CONTAINS")
		}
	}
}

func (d *discovery) emitProcessor(g *group, p processorEntity, taskName string) string {
	c := p.Component
	taskMRN := assetMRN("Task", taskName)
	d.taskMRNs[p.ID] = taskMRN

	metadata := map[string]any{
		"id":        c.ID,
		"group_id":  g.component.ID,
		"pipeline":  g.name,
		"type":      simpleTypeName(c.Type),
		"type_full": c.Type,
		"state":     processorState(c),
		"url":       d.componentURL(g.component.ID, "processors", c.ID),
	}
	putIf(metadata, "scheduling_period", c.Config.SchedulingPeriod)
	putIf(metadata, "scheduling_strategy", c.Config.SchedulingStrategy)
	putIf(metadata, "comments", c.Config.Comments)
	if props := publicProperties(c.Config); len(props) > 0 {
		metadata["properties"] = props
	}
	if rels := relationshipNames(c.Relationships); len(rels) > 0 {
		metadata["relationships"] = rels
	}

	d.assets = append(d.assets, d.newAsset("Task", taskName, c.Config.Comments, metadata, d.componentURL(g.component.ID, "processors", c.ID)))
	return taskMRN
}

func (d *discovery) emitPort(g *group, p portEntity, taskName string) string {
	c := p.Component
	taskMRN := assetMRN("Task", taskName)
	d.taskMRNs[p.ID] = taskMRN

	metadata := map[string]any{
		"id":        c.ID,
		"group_id":  g.component.ID,
		"pipeline":  g.name,
		"port_type": c.Type,
		"state":     c.State,
		"url":       d.componentURL(g.component.ID, portKind(c.Type), c.ID),
	}
	putIf(metadata, "comments", c.Comments)

	d.assets = append(d.assets, d.newAsset("Task", taskName, c.Comments, metadata, d.componentURL(g.component.ID, portKind(c.Type), c.ID)))
	return taskMRN
}

// emitConnections turns the group's connections into DEPENDS_ON edges.
// Two tasks this run created are linked directly; that covers processor
// to processor, and connections through ports once ports are tasks. A
// connection from one group's output port into another group's input
// port also links the two pipelines, which is the only view of that
// hand-off when ports are left out.
func (d *discovery) emitConnections(g *group) {
	for _, conn := range g.flow.Connections {
		src, dst := conn.Component.Source, conn.Component.Destination

		if srcMRN, ok := d.taskMRNs[src.ID]; ok {
			if dstMRN, ok := d.taskMRNs[dst.ID]; ok {
				d.link(srcMRN, dstMRN, "DEPENDS_ON")
			}
		}

		if src.Type == "OUTPUT_PORT" && dst.Type == "INPUT_PORT" && src.GroupID != dst.GroupID {
			srcMRN, srcOK := d.groupMRNs[src.GroupID]
			dstMRN, dstOK := d.groupMRNs[dst.GroupID]
			if srcOK && dstOK {
				d.link(srcMRN, dstMRN, "DEPENDS_ON")
			}
		}
	}
}

// emitTopics creates one Topic per Kafka topic seen this run. The
// identity is the one the Kafka plugin uses, so a topic it already
// catalogued gains NiFi as a second source instead of a duplicate.
func (d *discovery) emitTopics() {
	names := make([]string, 0, len(d.topics))
	for name := range d.topics {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		usage := d.topics[name]
		metadata := map[string]any{"topic_name": name}
		if len(usage.producers) > 0 {
			metadata["producers"] = usage.producers
		}
		if len(usage.consumers) > 0 {
			metadata["consumers"] = usage.consumers
		}

		topic := name
		mrnValue := nativeMRN("Topic", "Kafka", name)
		d.assets = append(d.assets, pluginsdk.Asset{
			Name:      &topic,
			MRN:       &mrnValue,
			Type:      "Topic",
			Providers: []string{"Kafka"},
			Metadata:  metadata,
			Tags:      pluginsdk.InterpolateTags(d.config.Tags, metadata),
			Sources: []pluginsdk.AssetSource{{
				Name:       provider,
				LastSyncAt: d.now,
				Properties: metadata,
				Priority:   1,
			}},
		})
	}
}

func (d *discovery) newAsset(assetType, name, description string, metadata map[string]any, link string) pluginsdk.Asset {
	assetName := name
	mrnValue := assetMRN(assetType, name)

	asset := pluginsdk.Asset{
		Name:      &assetName,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Tags:      pluginsdk.InterpolateTags(d.config.Tags, metadata),
		ExternalLinks: []pluginsdk.AssetExternalLink{{
			Name: "Open in NiFi",
			URL:  link,
		}},
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: d.now,
			Properties: metadata,
			Priority:   1,
		}},
	}
	if description != "" {
		asset.Description = &description
	}
	return asset
}

// link records an edge once, however many connections express it.
func (d *discovery) link(source, target, edgeType string) {
	key := source + "|" + target + "|" + edgeType
	if _, seen := d.edges[key]; seen {
		return
	}
	d.edges[key] = struct{}{}
	d.lineage = append(d.lineage, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
}

// groupURL links to a group on the canvas. NiFi 2 replaced the UI and
// its query-string deep links with hash routes, so the shape depends on
// the release found at /flow/about.
func (d *discovery) groupURL(groupID string) string {
	if d.legacyUI() {
		return d.config.Host + "/nifi/?processGroupId=" + url.QueryEscape(groupID)
	}
	return d.config.Host + "/nifi/#/process-groups/" + url.PathEscape(groupID)
}

// componentURL links to one component selected on its group's canvas.
// kind is the NiFi 2 route segment: processors, input-ports or
// output-ports.
func (d *discovery) componentURL(groupID, kind, componentID string) string {
	if d.legacyUI() {
		return d.groupURL(groupID) + "&componentIds=" + url.QueryEscape(componentID)
	}
	return d.groupURL(groupID) + "/" + kind + "/" + url.PathEscape(componentID)
}

func (d *discovery) legacyUI() bool {
	return strings.HasPrefix(d.version, "1.")
}

// portKind is the route segment for a port of the given type.
func portKind(portType string) string {
	if portType == "INPUT_PORT" {
		return "input-ports"
	}
	return "output-ports"
}

// breadcrumbPath joins the breadcrumb chain from the root down, so a
// group is named the same whether discovery started at the root or at
// the group itself.
func breadcrumbPath(b breadcrumbEntity) string {
	var names []string
	for cur := &b; cur != nil; cur = cur.ParentBreadcrumb {
		names = append([]string{cur.Breadcrumb.Name}, names...)
	}
	return strings.Join(names, "/")
}

type namedID struct {
	id   string
	name string
}

func groupNames(groups []processGroupEntity) []namedID {
	out := make([]namedID, 0, len(groups))
	for _, g := range groups {
		out = append(out, namedID{g.ID, g.Component.Name})
	}
	return out
}

// uniqueNames maps each id to its name, with the id appended when a
// sibling shares the name. NiFi allows duplicates; the catalog does not.
func uniqueNames(items []namedID) map[string]string {
	counts := make(map[string]int, len(items))
	for _, item := range items {
		counts[item.name]++
	}

	names := make(map[string]string, len(items))
	for _, item := range items {
		if counts[item.name] > 1 {
			names[item.id] = fmt.Sprintf("%s (%s)", item.name, item.id)
		} else {
			names[item.id] = item.name
		}
	}
	return names
}

// processorState folds validity into the scheduled state: a stopped
// processor that cannot start is more usefully reported as INVALID.
func processorState(c processorComponent) string {
	if c.State != "RUNNING" && c.ValidationStatus == "INVALID" {
		return "INVALID"
	}
	return c.State
}

// simpleTypeName is the Java class name without its package, which is
// what the NiFi UI shows.
func simpleTypeName(fullType string) string {
	return fullType[strings.LastIndex(fullType, ".")+1:]
}

// maskedValue replaces every sensitive property value.
const maskedValue = "****"

// publicProperties returns the set properties with sensitive ones
// masked. NiFi already hides those values before they leave the server,
// but the descriptor is the authority, not the value.
func publicProperties(config processorConfig) map[string]any {
	out := make(map[string]any)
	for name, value := range config.Properties {
		if value == nil || *value == "" {
			continue
		}
		if config.Descriptors[name].Sensitive {
			out[name] = maskedValue
			continue
		}
		out[name] = *value
	}
	return out
}

// propertyValue returns a non-sensitive property's value, or "" when it
// is unset, sensitive, or an expression NiFi resolves at run time.
func propertyValue(config processorConfig, names ...string) string {
	for _, name := range names {
		value, ok := config.Properties[name]
		if !ok || value == nil || config.Descriptors[name].Sensitive {
			continue
		}
		v := strings.TrimSpace(*value)
		if v == "" || strings.Contains(v, "${") || strings.Contains(v, "#{") {
			continue
		}
		return v
	}
	return ""
}

func relationshipNames(rels []relationship) []string {
	names := make([]string, 0, len(rels))
	for _, r := range rels {
		names = append(names, r.Name)
	}
	sort.Strings(names)
	return names
}

func putIf(metadata map[string]any, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}
