package dataquery

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// EdgeTypeDataQuery is the observed lineage type from a table to an agent that read it through Marmot.
const EdgeTypeDataQuery = "DATA_QUERY"

// AgentAssetType matches the type the agent SDK integrations register, so an agent catalogued by its SDK and one seen querying data are the same asset.
const AgentAssetType = "Agent"

// ObservedLineage records runtime-observed lineage edges. It is satisfied by the lineage service.
type ObservedLineage interface {
	BatchObservedLineage(ctx context.Context, edges []lineage.ObservedEdge) error
}

// consumerRecorder catalogs the service accounts that read data through Marmot as Agent assets and records an observed lineage edge from each table they read, so agents sit in the catalog and its lineage like any other consumer. Each edge is written at most once per interval per replica because data_queries already keeps the exact counts, and a nil lineage makes record a no-op.
type consumerRecorder struct {
	assets   asset.Service
	lineage  ObservedLineage
	interval time.Duration

	mu       sync.Mutex
	agents   map[string]string
	lastEdge map[string]time.Time
}

// consumerEdgeLimit bounds the in-memory edge throttle so a long-lived replica with many agents cannot grow it without limit.
const consumerEdgeLimit = 10000

func newConsumerRecorder(assets asset.Service, lin ObservedLineage, interval time.Duration) *consumerRecorder {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	return &consumerRecorder{
		assets:   assets,
		lineage:  lin,
		interval: interval,
		agents:   make(map[string]string),
		lastEdge: make(map[string]time.Time),
	}
}

// record catalogs the principal and its reads in the background. People are not catalogued, only service accounts, and whole-source fallback resources are skipped because they are not assets.
func (c *consumerRecorder) record(principal auth.Principal, mrns []string) {
	if c == nil || c.lineage == nil || c.assets == nil || principal == nil || principal.Type() != auth.PrincipalTypeServiceAccount {
		return
	}

	c.mu.Lock()
	now := time.Now()
	if len(c.lastEdge) > consumerEdgeLimit {
		for key, at := range c.lastEdge {
			if now.Sub(at) >= c.interval {
				delete(c.lastEdge, key)
			}
		}
	}
	var due []string
	for _, m := range mrns {
		if m == "" || isSourceMRN(m) {
			continue
		}
		key := principal.ID() + "\x00" + m
		if last, ok := c.lastEdge[key]; ok && now.Sub(last) < c.interval {
			continue
		}
		c.lastEdge[key] = now
		due = append(due, m)
	}
	c.mu.Unlock()
	if len(due) == 0 {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		agentMRN, err := c.ensureAgent(ctx, principal)
		if err != nil {
			log.Warn().Err(err).Str("principal", principal.AuditSubject()).Msg("Could not catalog an agent")
			return
		}
		edges := make([]lineage.ObservedEdge, 0, len(due))
		for _, m := range due {
			edges = append(edges, lineage.ObservedEdge{Source: m, Target: agentMRN, Type: EdgeTypeDataQuery})
		}
		if err := c.lineage.BatchObservedLineage(ctx, edges); err != nil {
			log.Warn().Err(err).Str("agent", agentMRN).Msg("Could not record agent lineage")
		}
	}()
}

// ensureAgent finds the principal's Agent asset, reusing one an SDK integration registered under the same name, and creates it otherwise.
func (c *consumerRecorder) ensureAgent(ctx context.Context, principal auth.Principal) (string, error) {
	c.mu.Lock()
	cached, ok := c.agents[principal.ID()]
	c.mu.Unlock()
	if ok {
		return cached, nil
	}

	name := principal.DisplayName()
	agentMRN := mrn.New(AgentAssetType, "Marmot", name)
	if existing, err := c.assets.GetByTypeAndName(ctx, AgentAssetType, name); err == nil && existing != nil && existing.MRN != nil {
		agentMRN = *existing.MRN
	} else {
		description := "Service account that reads data through Marmot."
		_, err := c.assets.Create(ctx, asset.CreateInput{
			Name:        &name,
			MRN:         &agentMRN,
			Type:        AgentAssetType,
			Providers:   []string{"Marmot"},
			Description: &description,
			Metadata:    map[string]interface{}{"service_account_id": principal.ID()},
			CreatedBy:   "query",
		})
		if err != nil && !errors.Is(err, asset.ErrAlreadyExists) {
			return "", err
		}
	}

	c.mu.Lock()
	c.agents[principal.ID()] = agentMRN
	c.mu.Unlock()
	return agentMRN, nil
}
