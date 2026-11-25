package pluginserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// ServedTools is what a serving process reports at enrolment about one source's tools and triggers: the manifests its plugin process gave it, their digests as it computed them, and a summary of what its own policy allows. It is informative only. Marmot uses it to offer tools and show what they need, but the serving process enforces its own copy of the policy on every request, so a summary that is stale, wrong or forged can make Marmot ask for something that is refused, never make the process run it.
type ServedTools struct {
	Manifest       []pluginsdk.ToolSpec    `json:"manifest,omitempty"`
	ManifestDigest string                  `json:"manifest_digest,omitempty"`
	Triggers       []pluginsdk.TriggerSpec `json:"triggers,omitempty"`
	TriggersDigest string                  `json:"triggers_digest,omitempty"`
	Policy         PolicySummary           `json:"policy"`
}

// PolicySummary is what the serving process's policy allows for one source, without the approvers' keys.
type PolicySummary struct {
	// ToolsAllowed is --allow-tools: without it nothing below runs.
	ToolsAllowed bool `json:"tools_allowed"`
	// PinnedManifest is the manifest digest the policy pins, when it pins one.
	PinnedManifest string           `json:"pinned_manifest,omitempty"`
	Tools          []ToolSummary    `json:"tools,omitempty"`
	Triggers       []TriggerSummary `json:"triggers,omitempty"`
	// ApprovalTrust is how the process trusts approvals: "marmot" for Marmot-signed. Empty when the policy has no approvals section, in which case every write that needs approval is refused.
	ApprovalTrust string `json:"approval_trust,omitempty"`
	// ApprovalMaxAge is the longest an approval may run before it expires.
	ApprovalMaxAge string `json:"approval_max_age,omitempty"`
}

// ToolSummary is one tool the policy names on the source.
type ToolSummary struct {
	Name string `json:"name"`
	// Read is set for a tool that only reads, which never needs approval.
	Read bool `json:"read,omitempty"`
	// Approvals is how many distinct people must approve an execute.
	Approvals int `json:"approvals"`
	// Approvers narrows who may approve, by Marmot user id or name; empty means anyone Marmot names.
	Approvers []string `json:"approvers,omitempty"`
}

// TriggerSummary is one trigger the policy runs on the source.
type TriggerSummary struct {
	Name     string         `json:"name"`
	Params   map[string]any `json:"params,omitempty"`
	Cooldown string         `json:"cooldown"`
}

// maxServedToolsBytes bounds what one source's report may take in the enrolment and the database.
const maxServedToolsBytes = 256 << 10

// summarise builds the policy summary for source.
func (p *Policy) summarise(source string, allowTools bool, specs []pluginsdk.ToolSpec) PolicySummary {
	out := PolicySummary{ToolsAllowed: allowTools}
	if p == nil {
		return out
	}
	if p.Approvals != nil {
		out.ApprovalTrust = p.Approvals.Trust
		out.ApprovalMaxAge = p.Approvals.MaxAge.String()
	}
	sp := p.Sources[source]
	out.PinnedManifest = sp.Manifest
	for name, ap := range sp.Tools {
		s := ToolSummary{Name: name, Read: isReadTool(specs, name), Approvers: ap.Approvers}
		if !s.Read {
			s.Approvals = ap.need().required
		}
		out.Tools = append(out.Tools, s)
	}
	slices.SortFunc(out.Tools, func(a, b ToolSummary) int { return compareStrings(a.Name, b.Name) })
	for name, tp := range sp.Triggers {
		out.Triggers = append(out.Triggers, TriggerSummary{Name: name, Params: tp.Params, Cooldown: tp.Cooldown.String()})
	}
	slices.SortFunc(out.Triggers, func(a, b TriggerSummary) int { return compareStrings(a.Name, b.Name) })
	return out
}

func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// servedTools is what this process reports for src at enrolment, or nil when it serves no tools or triggers for it. A plugin that cannot be asked for its manifest is reported without one rather than failing the enrolment, since queries do not depend on it.
func servedToolsFor(ctx context.Context, opts ServeOptions, src LocalSource) *ServedTools {
	if !opts.AllowTools || opts.PluginTools == nil {
		return nil
	}
	sp := opts.Policy.sourcePolicy(src.Name)
	if len(sp.Tools) == 0 && len(sp.Triggers) == 0 {
		return nil
	}
	out := &ServedTools{}
	specs, err := opts.PluginTools.ToolManifest(ctx, src.PluginID)
	if err != nil {
		log.Warn().Err(err).Str("source", src.Name).Msg("Could not read the plugin's tool manifest to report it")
	} else {
		out.Manifest, out.ManifestDigest = specs, pluginsdk.ManifestDigest(specs)
	}
	if len(sp.Triggers) > 0 && opts.Watcher != nil {
		triggers, err := opts.Watcher.TriggerManifest(ctx, src.PluginID)
		if err != nil {
			log.Warn().Err(err).Str("source", src.Name).Msg("Could not read the plugin's trigger manifest to report it")
		} else {
			out.Triggers, out.TriggersDigest = triggers, pluginsdk.TriggersDigest(triggers)
		}
	}
	out.Policy = opts.Policy.summarise(src.Name, opts.AllowTools, specs)
	return out
}

func (p *Policy) sourcePolicy(source string) SourcePolicy {
	if p == nil {
		return SourcePolicy{}
	}
	return p.Sources[source]
}

// checkServedTools vets a report at enrolment. Marmot stores it to show people, so it must at least be what a plugin could have said: manifests the SDK accepts, digests that match them, and a bounded size. Anything else refuses the enrolment, so a serving process that sends nonsense is told at once.
func checkServedTools(name string, sa *ServedTools) error {
	if sa == nil {
		return nil
	}
	data, err := json.Marshal(sa)
	if err != nil || len(data) > maxServedToolsBytes {
		return fmt.Errorf("%w: source %q: its tools report is too large", ErrInvalidEnrolment, name)
	}
	if len(sa.Manifest) > 0 || sa.ManifestDigest != "" {
		if err := pluginsdk.ValidateManifest(sa.Manifest); err != nil {
			return fmt.Errorf("%w: source %q: tool manifest: %v", ErrInvalidEnrolment, name, err)
		}
		if pluginsdk.ManifestDigest(sa.Manifest) != sa.ManifestDigest {
			return fmt.Errorf("%w: source %q: the tool manifest does not match its digest", ErrInvalidEnrolment, name)
		}
	}
	if len(sa.Triggers) > 0 || sa.TriggersDigest != "" {
		if err := pluginsdk.ValidateTriggers(sa.Triggers); err != nil {
			return fmt.Errorf("%w: source %q: trigger manifest: %v", ErrInvalidEnrolment, name, err)
		}
		if pluginsdk.TriggersDigest(sa.Triggers) != sa.TriggersDigest {
			return fmt.Errorf("%w: source %q: the trigger manifest does not match its digest", ErrInvalidEnrolment, name)
		}
	}
	return nil
}

// ServedTools returns what the serving process for source last reported about its tools and triggers, and whether it is connected now to any replica. It reads the newest enrolment of the source's owner, preferring one with a live session. The report is informative only: see ServedTools. It returns ErrNotConnected when no enrolment of the source's owner has reported one.
func (h *Hub) ServedTools(ctx context.Context, source string) (*ServedTools, bool, error) {
	owner, err := h.owners(ctx, source)
	if err != nil {
		return nil, false, err
	}
	return h.store.servedTools(ctx, source, owner, sessionFreshness)
}

func (s *Store) servedTools(ctx context.Context, source string, owner Owner, freshness time.Duration) (*ServedTools, bool, error) {
	var data []byte
	var connected bool
	err := s.db.QueryRow(ctx,
		`SELECT e.served_tools -> $1,
		        EXISTS (SELECT 1 FROM plugin_serve_sessions s
		                 WHERE s.enrolment_id = e.id AND s.heartbeat_at > NOW() - $4::interval) AND e.expires_at > NOW() AS connected
		   FROM plugin_serve_enrolments e
		  WHERE $1 = ANY(e.sources) AND e.principal_type = $2 AND e.principal_id = $3
		    AND e.served_tools ? $1
		  ORDER BY connected DESC, e.created_at DESC
		  LIMIT 1`,
		source, owner.Type, owner.ID, freshness.String()).Scan(&data, &connected)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, ErrNotConnected
	}
	if err != nil {
		return nil, false, fmt.Errorf("reading served tools for %s: %w", source, err)
	}
	var out ServedTools
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, false, fmt.Errorf("decoding served tools for %s: %w", source, err)
	}
	return &out, connected, nil
}
