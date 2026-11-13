// Package kafkaconnect discovers connectors, their tasks and the topics
// they move data through from a Kafka Connect cluster, and links each
// connector to the tables, collections and buckets it reads or writes.
package kafkaconnect

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

const provider = "Kafka Connect"

// Config for the Kafka Connect plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host      string `json:"host" description:"Kafka Connect REST URL (e.g. http://connect:8083)" validate:"required,url"`
	Username  string `json:"username,omitempty" description:"Username for basic authentication"`
	Password  string `json:"password,omitempty" description:"Password for basic authentication" sensitive:"true"`
	VerifySSL bool   `json:"verify_ssl" group:"Authentication" label:"Verify SSL" description:"Whether to verify the TLS certificate of the Connect REST endpoint" default:"true"`

	IncludeTasks    bool `json:"include_tasks" group:"Discovery" description:"Whether to discover connector tasks as Task assets" default:"true"`
	IncludeTopics   bool `json:"include_topics" group:"Discovery" description:"Whether to discover the topics connectors read and write as Topic assets" default:"true"`
	IncludeConfig   bool `json:"include_config" group:"Discovery" description:"Whether to store each connector's config in its metadata, with credentials masked" default:"true"`
	DiscoverLineage bool `json:"discover_lineage" group:"Discovery" description:"Whether to link connectors to the topics and datasets they move data between" default:"true"`

	BootstrapServers string                       `json:"bootstrap_servers,omitempty" group:"Discovery" description:"Bootstrap servers of the Kafka cluster behind this Connect cluster, for example broker:9092. Lets Marmot match the topics to the right Kafka zone"`
	ConnectorMap     map[string]map[string]string `json:"connector_map,omitempty" group:"Advanced" description:"Marmot zone of the systems a connector reads or writes, as connector name to provider to zone, for example orders-sink: {PostgreSQL: prod-eu}. Takes precedence over the address in the connector's config"`
}

// Example configuration for the plugin
var _ = `
host: "http://connect.internal:8083"
username: "marmot"
password: "connect_secure_pass"
verify_ssl: true
include_tasks: true
include_topics: true
include_config: true
discover_lineage: true
tags:
  - "kafka-connect"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:            "kafkaconnect",
		MarmotVersion: ">=0.13.0",
		Name:          "Apache Kafka Connect",
		Description:   "Discover connectors, tasks and topics from Kafka Connect clusters with lineage to the systems they move data between",
		Icon:          "kafka",
		Category:      "orchestration",
		Status:        "experimental",
		Features:      []string{"Assets", "Lineage"},
		ConfigSpec:    pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(KafkaConnectPipelineFields{}, "Pipeline",
				"The metadata fields the plugin emits for a connector's Pipeline asset."),
			pluginsdk.AssetSchemaOf(KafkaConnectTaskFields{}, "Task",
				"The metadata fields the plugin emits for a Task asset."),
			pluginsdk.AssetSchemaOf(KafkaConnectTopicFields{}, "Topic",
				"The metadata fields the plugin emits for a Topic asset."),
		},
	}
}

// Source implements the Kafka Connect plugin.
type Source struct {
	config *Config
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)

	config.Host = strings.TrimRight(strings.TrimSpace(config.Host), "/")

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	// The url rule accepts any scheme, so "connect:8083" would pass and
	// then fail at the first request with a less helpful error.
	if parsed, err := url.Parse(config.Host); err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("host must be an http or https URL")
	}

	if config.Password != "" && config.Username == "" {
		return nil, fmt.Errorf("username is required when password is set")
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers connectors, tasks and topics from Kafka Connect.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	client := newClient(s.config.Host, s.config.Username, s.config.Password, s.config.VerifySSL)

	info, err := client.serverInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("connecting to Kafka Connect: %w", err)
	}
	log.Debug().Str("version", info.Version).Str("kafka_cluster_id", info.KafkaClusterID).Msg("Connected to Kafka Connect")

	connectors, err := client.connectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing connectors: %w", err)
	}
	log.Debug().Int("count", len(connectors)).Msg("Found connectors")

	d := &discovery{
		config:   s.config,
		client:   client,
		server:   info,
		versions: make(map[string]string),
		topics:   make(map[string]*topicUsage),
	}

	plugins, err := client.connectorPlugins(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list connector plugins, connector versions will be missing")
	}
	for _, p := range plugins {
		d.versions[p.Class] = p.Version
	}

	for _, conn := range connectors {
		d.discoverConnector(ctx, conn)
	}

	if s.config.IncludeTopics {
		d.assets = append(d.assets, d.topicAssets()...)
	}

	log.Info().
		Int("assets", len(d.assets)).
		Int("lineages", len(d.lineage)).
		Msg("Kafka Connect discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:       d.assets,
		Lineage:      d.lineage,
		ZoneBindings: zoneBindings(s.config),
	}, nil
}

// discovery accumulates one run's output.
type discovery struct {
	config   *Config
	client   *client
	server   *serverInfo
	versions map[string]string

	// topics collects every topic seen across connectors, so one Topic
	// asset is emitted per topic however many connectors use it.
	topics map[string]*topicUsage

	// topicTrackingOff is set once the worker says it cannot answer the
	// active topics endpoint, so the remaining connectors skip it.
	topicTrackingOff bool

	assets  []pluginsdk.Asset
	lineage []pluginsdk.LineageEdge
}

type topicUsage struct {
	name      string
	mrn       string
	producers []string
	consumers []string
}

func (d *discovery) discoverConnector(ctx context.Context, conn connector) {
	name := conn.Info.Name
	cfg := connectorConfig(conn.Info.config())
	class := cfg.class()
	connectorType := strings.ToLower(conn.Info.Type)
	if connectorType == "" {
		connectorType = strings.ToLower(conn.Status.Type)
	}

	routers := regexRouters(cfg)
	topics := d.topicsFor(ctx, name, cfg, connectorType, routers)

	pipeline := d.pipelineAsset(conn, cfg, class, connectorType, topics)
	d.assets = append(d.assets, pipeline)
	pipelineMRN := *pipeline.MRN

	if d.config.IncludeTasks {
		for _, task := range conn.Status.Tasks {
			taskAsset := d.taskAsset(name, task)
			d.assets = append(d.assets, taskAsset)
			d.link(pipelineMRN, *taskAsset.MRN, "CONTAINS")
		}
	}

	switch connectorType {
	case "source":
		for _, topic := range topics {
			usage := d.useTopic(name, cfg, topic)
			usage.producers = append(usage.producers, name)
		}
	case "sink":
		for _, topic := range topics {
			usage := d.useTopic(name, cfg, topic)
			usage.consumers = append(usage.consumers, name)
		}
	}

	if !d.config.DiscoverLineage {
		return
	}

	r, known := resolverFor(class)
	if !known {
		log.Debug().Str("connector", name).Str("class", class).Msg("No dataset resolver for connector class, linking topics only")
	}

	switch connectorType {
	case "source":
		if known && r.sources != nil {
			for _, ds := range r.sources(cfg) {
				d.link(d.datasetMRN(name, ds), pipelineMRN, "FEEDS")
			}
		}
		for _, topic := range topics {
			d.link(pipelineMRN, d.topicMRN(name, cfg, topic), "PRODUCES")
		}
	case "sink":
		for _, topic := range topics {
			d.link(d.topicMRN(name, cfg, topic), pipelineMRN, "FEEDS")
		}
		if known && r.targets != nil {
			// A sink sees the topic name after its own transforms, and
			// names tables from that.
			routed := make([]string, 0, len(topics))
			for _, topic := range topics {
				routed = append(routed, route(routers, topic))
			}
			for _, ds := range r.targets(cfg, routed) {
				d.link(pipelineMRN, d.datasetMRN(name, ds), "PRODUCES")
			}
		}
	}
}

// topicsFor returns the topics a connector reads or writes. The worker's
// own record of active topics (KIP-558) is used when it has one; a
// connector that has not moved a record yet, or a worker without topic
// tracking, falls back to what the config says.
func (d *discovery) topicsFor(ctx context.Context, name string, cfg connectorConfig, connectorType string, routers []regexRouter) []string {
	var topics []string

	if !d.topicTrackingOff {
		active, err := d.client.connectorTopics(ctx, name)
		switch {
		case errors.Is(err, errTopicTrackingUnavailable):
			log.Debug().Err(err).Msg("Active topics endpoint unavailable, deriving topics from config for the rest of the run")
			d.topicTrackingOff = true
		case err != nil:
			log.Warn().Err(err).Str("connector", name).Msg("Failed to read active topics, deriving topics from config")
		default:
			topics = active
		}
	}

	if len(topics) == 0 {
		topics = configTopics(cfg, connectorType, routers)
	}

	var kept []string
	for _, topic := range topics {
		if !isInternalTopic(cfg, topic) {
			kept = append(kept, topic)
		}
	}
	return dedupe(kept)
}

// configTopics derives a connector's topics from its config: the keys
// connectors name a topic under directly, and for source connectors the
// naming rule of its family, run through its RegexRouter transforms.
func configTopics(cfg connectorConfig, connectorType string, routers []regexRouter) []string {
	topics := cfg.list("topics")
	topics = append(topics, cfg.get("topic"), cfg.get("kafka.topic"))

	if connectorType == "source" {
		usesRouter, staticTopic := outboxRoute(cfg)
		switch {
		case usesRouter:
			// The outbox router names topics from record fields, so only
			// a fixed route can be known from the config.
			topics = append(topics, staticTopic)
		default:
			if r, ok := resolverFor(cfg.class()); ok && r.topics != nil {
				topics = append(topics, r.topics(cfg)...)
			}
		}
		for i, topic := range topics {
			topics[i] = route(routers, topic)
		}
	}

	// A source connector such as MirrorMaker lists the topics it mirrors
	// as patterns, which are not topic names.
	var named []string
	for _, topic := range topics {
		if validTopicName(topic) {
			named = append(named, topic)
		}
	}
	return dedupe(named)
}

// validTopicName reports whether a string is a legal Kafka topic name:
// up to 249 characters of letters, digits, dots, underscores and hyphens.
func validTopicName(topic string) bool {
	if topic == "" || len(topic) > 249 || topic == "." || topic == ".." {
		return false
	}
	for _, r := range topic {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// isInternalTopic reports whether a topic is one a connector keeps for
// itself rather than data: Debezium's schema change, transaction and
// heartbeat topics, schema history and a sink's dead letter queue.
func isInternalTopic(cfg connectorConfig, topic string) bool {
	if prefix := debeziumPrefix(cfg); prefix != "" {
		if topic == prefix || topic == prefix+".transaction" {
			return true
		}
	}
	heartbeat := cfg.get("heartbeat.topics.prefix")
	if heartbeat == "" {
		heartbeat = "__debezium-heartbeat"
	}
	if strings.HasPrefix(topic, heartbeat+".") {
		return true
	}
	for _, key := range []string{
		"schema.history.internal.kafka.topic",
		"database.history.kafka.topic",
		"errors.deadletterqueue.topic.name",
	} {
		if v := cfg.get(key); v != "" && v == topic {
			return true
		}
	}
	return false
}

func (d *discovery) useTopic(connectorName string, cfg connectorConfig, topic string) *topicUsage {
	key := d.topicMRN(connectorName, cfg, topic)
	usage, ok := d.topics[key]
	if !ok {
		usage = &topicUsage{name: topic, mrn: key}
		d.topics[key] = usage
	}
	return usage
}

func (d *discovery) link(source, target, edgeType string) {
	for _, existing := range d.lineage {
		if existing.Source == source && existing.Target == target && existing.Type == edgeType {
			return
		}
	}
	d.lineage = append(d.lineage, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
}

// maxTraceLength bounds the stack trace stored on a failed connector or
// task so a Java stack does not swamp the asset's metadata.
const maxTraceLength = 500

func trimTrace(trace string) string {
	trace = strings.TrimSpace(trace)
	if len(trace) > maxTraceLength {
		return trace[:maxTraceLength]
	}
	return trace
}

func (d *discovery) pipelineAsset(conn connector, cfg connectorConfig, class, connectorType string, topics []string) pluginsdk.Asset {
	name := conn.Info.Name

	metadata := map[string]any{
		"connector_class": class,
		"connector_type":  connectorType,
		"state":           conn.Status.Connector.State,
		"worker_id":       conn.Status.Connector.WorkerID,
		"task_count":      len(conn.Status.Tasks),
		"url":             d.config.Host + "/connectors/" + url.PathEscape(name),
	}
	if d.server != nil {
		putIf(metadata, "connect_version", d.server.Version)
		putIf(metadata, "kafka_cluster_id", d.server.KafkaClusterID)
	}
	putIf(metadata, "plugin_version", d.versions[class])
	putIf(metadata, "description", cfg.get("description"))

	if len(conn.Status.Tasks) > 0 {
		states := make(map[string]any, len(conn.Status.Tasks))
		for _, task := range conn.Status.Tasks {
			states[strconv.Itoa(task.ID)] = task.State
		}
		metadata["task_states"] = states
	}
	if len(topics) > 0 {
		metadata["topics"] = topics
	}
	if d.config.IncludeConfig && len(cfg) > 0 {
		metadata["config"] = sanitiseConfig(cfg)
	}

	// The connector stays RUNNING when only one of its tasks has died, so
	// the first failed task's trace is the error worth surfacing then.
	if conn.Status.Connector.State == "FAILED" {
		putIf(metadata, "error", trimTrace(conn.Status.Connector.Trace))
	} else {
		for _, task := range conn.Status.Tasks {
			if task.State == "FAILED" && task.Trace != "" {
				putIf(metadata, "error", trimTrace(task.Trace))
				break
			}
		}
	}

	var description *string
	if desc := cfg.get("description"); desc != "" {
		description = &desc
	}

	mrnValue := d.config.AssetMRN("Pipeline", provider, name)
	return pluginsdk.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Pipeline",
		Providers:   []string{provider},
		Description: description,
		Metadata:    metadata,
		Tags:        pluginsdk.InterpolateTags(d.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func (d *discovery) taskAsset(connectorName string, task taskStatus) pluginsdk.Asset {
	name := fmt.Sprintf("task-%d", task.ID)

	metadata := map[string]any{
		"task_id":   task.ID,
		"state":     task.State,
		"worker_id": task.WorkerID,
		"connector": connectorName,
	}
	if task.State == "FAILED" {
		putIf(metadata, "error", trimTrace(task.Trace))
	}

	mrnValue := d.config.AssetMRN("Task", provider, connectorName, name)
	var previous []string
	if old := mrn.New("Task", provider, taskName(connectorName, task.ID)); old != mrnValue {
		previous = []string{old}
	}
	return pluginsdk.Asset{
		Name:         &name,
		MRN:          &mrnValue,
		PreviousMRNs: previous,
		Type:         "Task",
		Providers:    []string{provider},
		Metadata:     metadata,
		Tags:         pluginsdk.InterpolateTags(d.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// topicAssets emits one Topic per topic seen this run. The identity is
// the one the Kafka plugin uses, so a topic it already catalogued gains
// this plugin as a second source instead of a duplicate asset.
func (d *discovery) topicAssets() []pluginsdk.Asset {
	keys := make([]string, 0, len(d.topics))
	for key := range d.topics {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	assets := make([]pluginsdk.Asset, 0, len(keys))
	for _, key := range keys {
		usage := d.topics[key]
		name := usage.name
		metadata := map[string]any{"topic_name": name}
		if len(usage.producers) > 0 {
			metadata["producers"] = dedupe(usage.producers)
		}
		if len(usage.consumers) > 0 {
			metadata["consumers"] = dedupe(usage.consumers)
		}

		topic := name
		mrnValue := usage.mrn
		assets = append(assets, pluginsdk.Asset{
			Name:      &topic,
			MRN:       &mrnValue,
			Type:      "Topic",
			Providers: []string{"Kafka"},
			Metadata:  metadata,
			Tags:      pluginsdk.InterpolateTags(d.config.Tags, metadata),
			Sources: []pluginsdk.AssetSource{{
				Name:       provider,
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})
	}
	return assets
}

func putIf(metadata map[string]any, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}

// taskName is how a task is named: its connector and its numeric id,
// which is the only identity Connect gives a task.
func taskName(connectorName string, id int) string {
	return fmt.Sprintf("%s.task-%d", connectorName, id)
}

// reference builds the MRN of an asset another plugin owns. A zone mapped for the connector and provider in connector_map wins, then the address the connector was configured with as a host hint, then no zone.
func (d *discovery) reference(connectorName, assetType, otherProvider, host string, path ...string) string {
	m := mrn.Asset(assetType, otherProvider, path...)
	if zone := d.mappedZone(connectorName, otherProvider); zone != "" {
		return m.WithZone(zone).String()
	}
	return m.WithHost(host).String()
}

// mappedZone looks a connector and provider up in connector_map, ignoring case so the map can be written the way people say the names.
func (d *discovery) mappedZone(connectorName, otherProvider string) string {
	for name, providers := range d.config.ConnectorMap {
		if !strings.EqualFold(name, connectorName) {
			continue
		}
		for p, zone := range providers {
			if strings.EqualFold(p, otherProvider) {
				return strings.TrimSpace(zone)
			}
		}
	}
	return ""
}

// topicMRN builds the identity the Kafka plugin gives a topic. The connector's own bootstrap override, or else the Connect cluster's, tells which Kafka it is.
func (d *discovery) topicMRN(connectorName string, cfg connectorConfig, topic string) string {
	servers := cfg.get("producer.override.bootstrap.servers", "consumer.override.bootstrap.servers", "admin.override.bootstrap.servers", "bootstrap.servers")
	if servers == "" {
		servers = d.config.BootstrapServers
	}
	host := ""
	if first := splitList(servers); len(first) > 0 {
		host = hostPort(first[0], "", "9092")
	}
	return d.reference(connectorName, "Topic", "Kafka", host, topic)
}

// datasetMRN builds the identity the owning plugin gives a table, collection or bucket.
func (d *discovery) datasetMRN(connectorName string, ds dataset) string {
	return d.reference(connectorName, ds.Type, ds.Provider, ds.Host, ds.Path...)
}

// zoneBindings tells Marmot which zone the Connect worker this run connected to is.
func zoneBindings(cfg *Config) []pluginsdk.ZoneBinding {
	u, err := url.Parse(cfg.Host)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	return []pluginsdk.ZoneBinding{{Provider: provider, Host: net.JoinHostPort(strings.ToLower(u.Hostname()), port), Zone: cfg.Zone}}
}
