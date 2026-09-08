// Package kafkaconnect discovers connectors, their tasks and the topics
// they move data through from a Kafka Connect cluster, and links each
// connector to the tables, collections and buckets it reads or writes.
package kafkaconnect

import (
	"context"
	"errors"
	"fmt"
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
	VerifySSL bool   `json:"verify_ssl" label:"Verify SSL" description:"Whether to verify the TLS certificate of the Connect REST endpoint" default:"true"`

	IncludeTasks    bool `json:"include_tasks" description:"Whether to discover connector tasks as Task assets" default:"true"`
	IncludeTopics   bool `json:"include_topics" description:"Whether to discover the topics connectors read and write as Topic assets" default:"true"`
	IncludeConfig   bool `json:"include_config" description:"Whether to store each connector's config in its metadata, with credentials masked" default:"true"`
	DiscoverLineage bool `json:"discover_lineage" description:"Whether to link connectors to the topics and datasets they move data between" default:"true"`
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
		ID:          "kafkaconnect",
		Name:        "Apache Kafka Connect",
		Description: "Discover connectors, tasks and topics from Kafka Connect clusters with lineage to the systems they move data between",
		Icon:        "kafka",
		Category:    "orchestration",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
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
		Assets:  d.assets,
		Lineage: d.lineage,
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
			d.useTopic(topic).producers = append(d.useTopic(topic).producers, name)
		}
	case "sink":
		for _, topic := range topics {
			d.useTopic(topic).consumers = append(d.useTopic(topic).consumers, name)
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
				d.link(datasetMRN(ds), pipelineMRN, "FEEDS")
			}
		}
		for _, topic := range topics {
			d.link(pipelineMRN, topicMRN(topic), "PRODUCES")
		}
	case "sink":
		for _, topic := range topics {
			d.link(topicMRN(topic), pipelineMRN, "FEEDS")
		}
		if known && r.targets != nil {
			// A sink sees the topic name after its own transforms, and
			// names tables from that.
			routed := make([]string, 0, len(topics))
			for _, topic := range topics {
				routed = append(routed, route(routers, topic))
			}
			for _, ds := range r.targets(cfg, routed) {
				d.link(pipelineMRN, datasetMRN(ds), "PRODUCES")
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

func (d *discovery) useTopic(topic string) *topicUsage {
	usage, ok := d.topics[topic]
	if !ok {
		usage = &topicUsage{}
		d.topics[topic] = usage
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

	mrnValue := assetMRN("Pipeline", name)
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
	name := taskName(connectorName, task.ID)

	metadata := map[string]any{
		"task_id":   task.ID,
		"state":     task.State,
		"worker_id": task.WorkerID,
		"connector": connectorName,
	}
	if task.State == "FAILED" {
		putIf(metadata, "error", trimTrace(task.Trace))
	}

	mrnValue := assetMRN("Task", name)
	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Task",
		Providers: []string{provider},
		Metadata:  metadata,
		Tags:      pluginsdk.InterpolateTags(d.config.Tags, metadata),
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
	names := make([]string, 0, len(d.topics))
	for name := range d.topics {
		names = append(names, name)
	}
	sort.Strings(names)

	assets := make([]pluginsdk.Asset, 0, len(names))
	for _, name := range names {
		usage := d.topics[name]
		metadata := map[string]any{"topic_name": name}
		if len(usage.producers) > 0 {
			metadata["producers"] = dedupe(usage.producers)
		}
		if len(usage.consumers) > 0 {
			metadata["consumers"] = dedupe(usage.consumers)
		}

		topic := name
		mrnValue := topicMRN(name)
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

// assetMRN is the single place a Kafka Connect MRN is built, for the
// Pipeline and Task assets this plugin owns.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// topicMRN builds the identity the Kafka plugin gives a topic.
func topicMRN(topic string) string {
	return mrn.New("Topic", "Kafka", topic)
}

// datasetMRN builds the identity the owning plugin gives a table,
// collection or bucket.
func datasetMRN(ds dataset) string {
	return mrn.New(ds.Type, ds.Provider, ds.Name)
}
