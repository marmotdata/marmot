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

// ServedActions is what a serving process reports at enrolment about one source's actions and triggers: the manifests its plugin process gave it, their digests as it computed them, and a summary of what its own policy allows. It is informative only. Marmot uses it to offer actions and show what they need, but the serving process enforces its own copy of the policy on every request, so a summary that is stale, wrong or forged can make Marmot ask for something that is refused, never make the process run it.
type ServedActions struct {
	Manifest       []pluginsdk.ActionSpec  `json:"manifest,omitempty"`
	ManifestDigest string                  `json:"manifest_digest,omitempty"`
	Triggers       []pluginsdk.TriggerSpec `json:"triggers,omitempty"`
	TriggersDigest string                  `json:"triggers_digest,omitempty"`
	Policy         PolicySummary           `json:"policy"`
}

// PolicySummary is what the serving process's policy allows for one source, without the approvers' keys.
type PolicySummary struct {
	// ActionsAllowed is --allow-actions: without it nothing below runs.
	ActionsAllowed bool `json:"actions_allowed"`
	// PinnedManifest is the manifest digest the policy pins, when it pins one.
	PinnedManifest string           `json:"pinned_manifest,omitempty"`
	Actions        []ActionSummary  `json:"actions,omitempty"`
	Triggers       []TriggerSummary `json:"triggers,omitempty"`
	// ApprovalTrust is how the process trusts approvals: "marmot" for Marmot-signed. Empty when the policy has no approvals section, in which case every write that needs approval is refused.
	ApprovalTrust string `json:"approval_trust,omitempty"`
	// ApprovalMaxAge is the longest an approval may run before it expires.
	ApprovalMaxAge string `json:"approval_max_age,omitempty"`
}

// ActionSummary is one action the policy names on the source.
type ActionSummary struct {
	Name string `json:"name"`
	// Read is set for an action that only reads, which never needs approval.
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

// maxServedActionsBytes bounds what one source's report may take in the enrolment and the database.
const maxServedActionsBytes = 256 << 10

// summarise builds the policy summary for source.
func (p *Policy) summarise(source string, allowActions bool, specs []pluginsdk.ActionSpec) PolicySummary {
	out := PolicySummary{ActionsAllowed: allowActions}
	if p == nil {
		return out
	}
	if p.Approvals != nil {
		out.ApprovalTrust = p.Approvals.Trust
		out.ApprovalMaxAge = p.Approvals.MaxAge.String()
	}
	sp := p.Sources[source]
	out.PinnedManifest = sp.Manifest
	for name, ap := range sp.Actions {
		s := ActionSummary{Name: name, Read: isReadAction(specs, name), Approvers: ap.Approvers}
		if !s.Read {
			s.Approvals = ap.need().required
		}
		out.Actions = append(out.Actions, s)
	}
	slices.SortFunc(out.Actions, func(a, b ActionSummary) int { return compareStrings(a.Name, b.Name) })
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

// servedActions is what this process reports for src at enrolment, or nil when it serves no actions or triggers for it. A plugin that cannot be asked for its manifest is reported without one rather than failing the enrolment, since queries do not depend on it.
func servedActionsFor(ctx context.Context, opts ServeOptions, src LocalSource) *ServedActions {
	if !opts.AllowActions || opts.Actor == nil {
		return nil
	}
	sp := opts.Policy.sourcePolicy(src.Name)
	if len(sp.Actions) == 0 && len(sp.Triggers) == 0 {
		return nil
	}
	out := &ServedActions{}
	specs, err := opts.Actor.ActionManifest(ctx, src.PluginID)
	if err != nil {
		log.Warn().Err(err).Str("source", src.Name).Msg("Could not read the plugin's action manifest to report it")
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
	out.Policy = opts.Policy.summarise(src.Name, opts.AllowActions, specs)
	return out
}

func (p *Policy) sourcePolicy(source string) SourcePolicy {
	if p == nil {
		return SourcePolicy{}
	}
	return p.Sources[source]
}

// checkServedActions vets a report at enrolment. Marmot stores it to show people, so it must at least be what a plugin could have said: manifests the SDK accepts, digests that match them, and a bounded size. Anything else refuses the enrolment, so a serving process that sends nonsense is told at once.
func checkServedActions(name string, sa *ServedActions) error {
	if sa == nil {
		return nil
	}
	data, err := json.Marshal(sa)
	if err != nil || len(data) > maxServedActionsBytes {
		return fmt.Errorf("%w: source %q: its actions report is too large", ErrInvalidEnrolment, name)
	}
	if len(sa.Manifest) > 0 || sa.ManifestDigest != "" {
		if err := pluginsdk.ValidateManifest(sa.Manifest); err != nil {
			return fmt.Errorf("%w: source %q: action manifest: %v", ErrInvalidEnrolment, name, err)
		}
		if pluginsdk.ManifestDigest(sa.Manifest) != sa.ManifestDigest {
			return fmt.Errorf("%w: source %q: the action manifest does not match its digest", ErrInvalidEnrolment, name)
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

// ServedActions returns what the serving process for source last reported about its actions and triggers, and whether it is connected now to any replica. It reads the newest enrolment of the source's owner, preferring one with a live session. The report is informative only: see ServedActions. It returns ErrNotConnected when no enrolment of the source's owner has reported one.
func (h *Hub) ServedActions(ctx context.Context, source string) (*ServedActions, bool, error) {
	owner, err := h.owners(ctx, source)
	if err != nil {
		return nil, false, err
	}
	return h.store.servedActions(ctx, source, owner, sessionFreshness)
}

func (s *Store) servedActions(ctx context.Context, source string, owner Owner, freshness time.Duration) (*ServedActions, bool, error) {
	var data []byte
	var connected bool
	err := s.db.QueryRow(ctx,
		`SELECT e.served_actions -> $1,
		        EXISTS (SELECT 1 FROM plugin_serve_sessions s
		                 WHERE s.enrolment_id = e.id AND s.heartbeat_at > NOW() - $4::interval) AND e.expires_at > NOW() AS connected
		   FROM plugin_serve_enrolments e
		  WHERE $1 = ANY(e.sources) AND e.principal_type = $2 AND e.principal_id = $3
		    AND e.served_actions ? $1
		  ORDER BY connected DESC, e.created_at DESC
		  LIMIT 1`,
		source, owner.Type, owner.ID, freshness.String()).Scan(&data, &connected)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, ErrNotConnected
	}
	if err != nil {
		return nil, false, fmt.Errorf("reading served actions for %s: %w", source, err)
	}
	var out ServedActions
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, false, fmt.Errorf("decoding served actions for %s: %w", source, err)
	}
	return &out, connected, nil
}
