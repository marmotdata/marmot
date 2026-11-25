package pluginserve

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"gopkg.in/yaml.v3"

	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

// Policy is a serving process's own say over what it runs, on top of everything Marmot decided and signed. Marmot authorises every query before signing it; the policy lets the people who run the serving process narrow that further without trusting Marmot to, so a compromised or misconfigured Marmot still cannot make the process run what its operators ruled out.
//
// Once a policy is given, a source it does not name is refused: an operator who writes a policy has said what may run, and a source added to the pipeline later should not slip past it.
type Policy struct {
	// Approvals says how approvals of writes are trusted: trust: marmot, the only mode today. Without it every write is refused, apart from an action the policy explicitly lets run with approvals: 0.
	Approvals *ApprovalsPolicy        `yaml:"approvals"`
	Sources   map[string]SourcePolicy `yaml:"sources"`
	// name is the policy file's base name, which refusals quote so whoever reads them in Marmot knows which local policy said no without learning the host's paths.
	name string
}

// SourcePolicy is the policy for one served source. New per-source rules are added here as further keys; strict decoding means an older serving process refuses a policy that uses them rather than ignoring a rule it does not know.
type SourcePolicy struct {
	// Query, when false, refuses every plan and execute for the source. Unset means true. It says nothing about actions, which are refused unless listed under Actions.
	Query *bool `yaml:"query"`
	// MaxRows, when set, is a ceiling on rows returned: a signed row limit above it is lowered to it.
	MaxRows int64 `yaml:"max_rows"`
	// Actors is an allow-list of globs over the signed actor, such as user:alice or task:*, where * matches any run of characters. Empty means any actor. It applies to the source's actions as well as its queries.
	Actors []string `yaml:"actors"`
	// Origins is an allow-list of globs over the signed origin, such as task_run:*. Tasks run as their own principal, so the origin is how a serving process tells task traffic from a person's. Empty means any origin, including none. Like Actors it applies to actions too.
	Origins []string `yaml:"origins"`
	// Manifest, when set, pins the plugin's action manifest by its digest, sha256:<hex>: an action is refused unless the plugin's manifest, as this process computes it, has exactly that digest, so a plugin upgrade that changes any action or bound stops actions until the operator has looked at it.
	Manifest string `yaml:"manifest"`
	// Actions names each action that may run on the source. An action it does not name is refused, and with no Actions none is allowed: there is no default allow for writes.
	Actions map[string]ActionPolicy `yaml:"actions"`
	// Triggers names each trigger this process runs on the source, with its parameters. They come from here alone: Marmot never names a trigger, sends a parameter or an expression, it only receives the events. A trigger not named here never runs.
	Triggers map[string]TriggerPolicy `yaml:"triggers"`
}

// TriggerPolicy is one trigger the process runs, and how often its events may repeat.
type TriggerPolicy struct {
	// Params are the trigger's parameters, checked against the plugin's trigger manifest at start.
	Params map[string]any `yaml:"params"`
	// Cooldown suppresses an event whose fingerprint was sent less than this long ago, from one minute to a day; 30 minutes when unset.
	Cooldown time.Duration `yaml:"cooldown"`
}

const (
	defaultTriggerCooldown = 30 * time.Minute
	minTriggerCooldown     = time.Minute
	maxTriggerCooldown     = 24 * time.Hour
)

// ActionPolicy is the policy for one action on a source. Every rule only narrows what the plugin's manifest allows.
type ActionPolicy struct {
	// Actors and Origins are allow-lists like the source's, checked as well as them.
	Actors  []string `yaml:"actors"`
	Origins []string `yaml:"origins"`
	// Params narrows the manifest's bounds on named parameters. A constrained parameter must be given, so leaving it out cannot fall back to a plugin default the policy never saw.
	Params map[string]ParamPolicy `yaml:"params"`
	// Approvals is how many distinct people must approve each execute: 1 when unset, at most 2. 0 is allowed only for a risk 1, reversible action, and must be written out. A read never takes approvals.
	Approvals *int `yaml:"approvals"`
	// Approvers narrows who may approve the action, by Marmot user id or name. Only approvers it lists count towards Approvals.
	Approvers []string `yaml:"approvers"`
}

// maxRequiredApprovals is the most approvals an action may require.
const maxRequiredApprovals = 2

// need is how many approvals the action needs, with the default applied.
func (ap ActionPolicy) need() approvalNeed {
	n := 1
	if ap.Approvals != nil {
		n = *ap.Approvals
	}
	return approvalNeed{required: n, approvers: ap.Approvers}
}

// ParamPolicy narrows one action parameter. Min and Max apply to int parameters; Values lists the only values allowed and must match the parameter's type.
type ParamPolicy struct {
	Min    *int64 `yaml:"min"`
	Max    *int64 `yaml:"max"`
	Values []any  `yaml:"values"`
}

// LoadPolicy reads a policy file, refusing keys it does not know so a typo cannot quietly loosen it.
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: the path is the operator's --policy flag
	if err != nil {
		return nil, fmt.Errorf("reading policy: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	p := &Policy{name: filepath.Base(path)}
	if err := dec.Decode(p); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parsing policy %s: %w", path, err)
	}
	if p.Sources == nil {
		p.Sources = map[string]SourcePolicy{}
	}
	if p.Approvals != nil {
		if err := p.Approvals.load(); err != nil {
			return nil, fmt.Errorf("policy %s: %w", path, err)
		}
	}
	for name, sp := range p.Sources {
		if sp.MaxRows < 0 {
			return nil, fmt.Errorf("policy %s: source %q: max_rows must be positive", path, name)
		}
		if err := checkPatterns(sp.Actors, sp.Origins); err != nil {
			return nil, fmt.Errorf("policy %s: source %q: %w", path, name, err)
		}
		if sp.Manifest != "" && !manifestDigestPattern.MatchString(sp.Manifest) {
			return nil, fmt.Errorf("policy %s: source %q: manifest must be sha256: followed by 64 lower case hex digits", path, name)
		}
		for action, ap := range sp.Actions {
			if err := ap.check(); err != nil {
				return nil, fmt.Errorf("policy %s: source %q: action %q: %w", path, name, action, err)
			}
			if err := ap.checkApprovers(p.Approvals); err != nil {
				return nil, fmt.Errorf("policy %s: source %q: action %q: %w", path, name, action, err)
			}
		}
		for trigger, tp := range sp.Triggers {
			if tp.Cooldown == 0 {
				tp.Cooldown = defaultTriggerCooldown
			}
			if tp.Cooldown < minTriggerCooldown || tp.Cooldown > maxTriggerCooldown {
				return nil, fmt.Errorf("policy %s: source %q: trigger %q: cooldown must be from %s to %s", path, name, trigger, minTriggerCooldown, maxTriggerCooldown)
			}
			sp.Triggers[trigger] = tp
		}
	}
	return p, nil
}

// manifestDigestPattern is the shape of pluginsdk.ManifestDigest.
var manifestDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func checkPatterns(actors, origins []string) error {
	for _, a := range actors {
		if strings.TrimSpace(a) == "" {
			return errors.New("an actor pattern is empty")
		}
	}
	for _, o := range origins {
		if strings.TrimSpace(o) == "" {
			return errors.New("an origin pattern is empty")
		}
	}
	return nil
}

// checkApprovers checks an action's approval rules. That each count can ever be met is checked here, so a policy that could never approve anything fails at start; whether the action may run with none is checked against the manifest.
func (ap ActionPolicy) checkApprovers(approvals *ApprovalsPolicy) error {
	need := ap.need()
	if need.required < 0 || need.required > maxRequiredApprovals {
		return fmt.Errorf("approvals must be 0, 1 or 2")
	}
	if len(ap.Approvers) > 0 && approvals == nil {
		return errors.New("approvers are named but the policy has no approvals section")
	}
	for _, a := range ap.Approvers {
		if a == "" || len(a) > maxApproverNameBytes || !printable(a) {
			return fmt.Errorf("approver %q is not a user id or name", a)
		}
	}
	if len(ap.Approvers) > 0 && len(ap.Approvers) < need.required {
		return fmt.Errorf("needs %d approvers and only %d may approve it", need.required, len(ap.Approvers))
	}
	return nil
}

// approvals is the policy's approvals section, nil for no policy or none.
func (p *Policy) approvals() *ApprovalsPolicy {
	if p == nil {
		return nil
	}
	return p.Approvals
}

// check is what can be checked of an action's policy without the plugin's manifest. CheckManifest checks the rest once the manifest is known.
func (ap ActionPolicy) check() error {
	if err := checkPatterns(ap.Actors, ap.Origins); err != nil {
		return err
	}
	for name, pp := range ap.Params {
		if pp.Min == nil && pp.Max == nil && len(pp.Values) == 0 {
			return fmt.Errorf("parameter %q: give min, max or values", name)
		}
		if pp.Min != nil && pp.Max != nil && *pp.Min > *pp.Max {
			return fmt.Errorf("parameter %q: min is above max", name)
		}
		if (pp.Min != nil || pp.Max != nil) && len(pp.Values) > 0 {
			return fmt.Errorf("parameter %q: give min and max or values, not both", name)
		}
		for _, v := range pp.Values {
			switch v.(type) {
			case string, int, bool:
			default:
				return fmt.Errorf("parameter %q: values must be strings, whole numbers or booleans", name)
			}
		}
	}
	return nil
}

// CheckManifest checks the policy's actions on source against the plugin's manifest: every action it names is in the manifest, every parameter it constrains is declared by that action, and each constraint suits the parameter's type. The serving process runs it at start, when it first learns the manifest, so a misspelt action or parameter is an error there rather than a refusal at the first request; requests are checked against the manifest again, since the plugin can change while the process runs.
func (p *Policy) CheckManifest(source string, specs []pluginsdk.ActionSpec) error {
	if p == nil {
		return nil
	}
	for action, ap := range p.Sources[source].Actions {
		if err := ap.checkAgainst(action, specs); err != nil {
			return fmt.Errorf("policy %s: source %q: %w", p.name, source, err)
		}
	}
	return nil
}

func (ap ActionPolicy) checkAgainst(action string, specs []pluginsdk.ActionSpec) error {
	i := slices.IndexFunc(specs, func(s pluginsdk.ActionSpec) bool { return s.Name == action })
	if i < 0 {
		return fmt.Errorf("action %q is not in the plugin's manifest", action)
	}
	if err := ap.approvalsSuit(specs[i]); err != nil {
		return fmt.Errorf("action %q: %w", action, err)
	}
	for name, pp := range ap.Params {
		j := slices.IndexFunc(specs[i].Params, func(a pluginsdk.ActionParam) bool { return a.Name == name })
		if j < 0 {
			return fmt.Errorf("action %q has no parameter %q", action, name)
		}
		if err := pp.suits(specs[i].Params[j]); err != nil {
			return fmt.Errorf("action %q: parameter %q: %w", action, name, err)
		}
	}
	return nil
}

// approvalsSuit checks an action's approval rules against what the action is. A read changes nothing and takes no approval, so rules for one are a mistake; going without approval is for an action that is both the lowest risk and undoable, and only when the policy says so.
func (ap ActionPolicy) approvalsSuit(spec pluginsdk.ActionSpec) error {
	if spec.ReadOnly() {
		if ap.Approvals != nil || len(ap.Approvers) > 0 {
			return errors.New("it only reads, and reads take no approvals")
		}
		return nil
	}
	if ap.need().required == 0 && (spec.Risk != pluginsdk.RiskBounded || spec.Reversibility != pluginsdk.ReversibilityReversible) {
		return fmt.Errorf("approvals: 0 is only allowed for a risk 1, reversible action, and it is risk %d, %s", spec.Risk, spec.Reversibility)
	}
	return nil
}

// CheckTriggers checks the policy's triggers on source against the plugin's trigger manifest: each is declared and its parameters validate. The serving process runs it at start, so a misspelt trigger or parameter is an error there.
func (p *Policy) CheckTriggers(source string, specs []pluginsdk.TriggerSpec) error {
	if p == nil {
		return nil
	}
	for name, tp := range p.Sources[source].Triggers {
		i := slices.IndexFunc(specs, func(s pluginsdk.TriggerSpec) bool { return s.Name == name })
		if i < 0 {
			return fmt.Errorf("policy %s: source %q: trigger %q is not in the plugin's manifest", p.name, source, name)
		}
		if err := pluginsdk.ValidateTriggerParams(specs[i], yamlParams(tp.Params)); err != nil {
			return fmt.Errorf("policy %s: source %q: trigger %q: %w", p.name, source, name, err)
		}
	}
	return nil
}

// yamlParams turns YAML's decoded values into the shapes JSON would give, which is what the SDK checks: whole numbers as int64 and lists as []any.
func yamlParams(params map[string]any) map[string]any {
	out := make(map[string]any, len(params))
	for k, v := range params {
		switch x := v.(type) {
		case int:
			out[k] = int64(x)
		default:
			out[k] = x
		}
	}
	return out
}

// suits fails when a constraint cannot apply to the parameter it names, such as a range on a string or a value the manifest's enum does not hold, which would otherwise refuse every request or never match.
func (pp ParamPolicy) suits(param pluginsdk.ActionParam) error {
	if (pp.Min != nil || pp.Max != nil) && param.Type != pluginsdk.ParamInt {
		return fmt.Errorf("min and max apply to int parameters, and it is %s", param.Type)
	}
	for _, v := range pp.Values {
		ok := false
		switch param.Type {
		case pluginsdk.ParamString:
			_, ok = v.(string)
		case pluginsdk.ParamEnum:
			s, isString := v.(string)
			ok = isString && slices.Contains(param.Enum, s)
		case pluginsdk.ParamInt:
			_, ok = v.(int)
		case pluginsdk.ParamBool:
			_, ok = v.(bool)
		}
		if !ok {
			return fmt.Errorf("value %v does not suit a %s parameter", v, param.Type)
		}
	}
	return nil
}

// Name is the policy file's base name.
func (p *Policy) Name() string { return p.name }

// policyDecision is what the policy said about one query.
type policyDecision struct {
	allowed bool
	// reason says why a query was refused, or what the policy changed about one it allowed.
	reason  string
	maxRows int64
}

// String is the decision as the audit log records it.
func (d policyDecision) String() string {
	switch {
	case !d.allowed:
		return "refused"
	case d.reason != "":
		return "allowed, " + d.reason
	default:
		return "allowed"
	}
}

// decide applies the policy to a verified query. A nil policy allows everything, which is how a process with no --policy behaves.
func (p *Policy) decide(q *pb.Query) policyDecision {
	if p == nil {
		return policyDecision{allowed: true, maxRows: q.MaxRows}
	}
	sp, ok := p.Sources[q.Source]
	if !ok {
		return policyDecision{reason: fmt.Sprintf("source %q is not in the policy", q.Source)}
	}
	if sp.Query != nil && !*sp.Query {
		return policyDecision{reason: fmt.Sprintf("queries on %q are switched off", q.Source)}
	}
	if len(sp.Actors) > 0 && !matchesAny(sp.Actors, q.Actor) {
		return policyDecision{reason: fmt.Sprintf("actor %q may not query %q", q.Actor, q.Source)}
	}
	if len(sp.Origins) > 0 && !matchesAny(sp.Origins, q.Origin) {
		return policyDecision{reason: fmt.Sprintf("origin %q may not query %q", q.Origin, q.Source)}
	}
	d := policyDecision{allowed: true, maxRows: q.MaxRows}
	if sp.MaxRows > 0 && q.MaxRows > sp.MaxRows {
		d.maxRows = sp.MaxRows
		d.reason = fmt.Sprintf("max_rows lowered from %d to %d", q.MaxRows, sp.MaxRows)
	}
	return d
}

// actionPolicy returns the policy for an action on a source, failing unless actions are switched on with --allow-actions, a policy exists and it names both. Switching actions on takes the flag as well as a policy naming each action, so neither a policy file copied from elsewhere nor the flag alone is enough. A nil policy refuses: unlike queries, no action runs without an operator having named it.
func (p *Policy) actionPolicy(allowActions bool, source, action string) (SourcePolicy, ActionPolicy, error) {
	if !allowActions {
		return SourcePolicy{}, ActionPolicy{}, errActionsOff
	}
	if p == nil {
		return SourcePolicy{}, ActionPolicy{}, errActionsWithoutPolicy
	}
	sp, ok := p.Sources[source]
	if !ok {
		return SourcePolicy{}, ActionPolicy{}, fmt.Errorf("source %q is not in the policy", source)
	}
	ap, ok := sp.Actions[action]
	if !ok {
		return SourcePolicy{}, ActionPolicy{}, fmt.Errorf("action %q is not allowed on %q by the policy", action, source)
	}
	return sp, ap, nil
}

var (
	errActionsOff           = errors.New("actions are switched off on this runner")
	errActionsWithoutPolicy = errors.New("actions need a --policy naming each one; with no policy every action is refused")
)

// allowAction applies the rules that need the plugin's manifest: the manifest pin, then the actor and origin allow-lists, then the parameter constraints, which are checked against the manifest again because the plugin may have changed since the process started.
func (sp SourcePolicy) allowAction(ap ActionPolicy, q *pb.Query, params map[string]any, specs []pluginsdk.ActionSpec, digest string) error {
	if sp.Manifest != "" && sp.Manifest != digest {
		return fmt.Errorf("the plugin's manifest is %s, and the policy pins %s", digest, sp.Manifest)
	}
	for _, rule := range []struct {
		patterns    []string
		value, what string
	}{
		{sp.Actors, q.Actor, "actor"},
		{ap.Actors, q.Actor, "actor"},
		{sp.Origins, q.Origin, "origin"},
		{ap.Origins, q.Origin, "origin"},
	} {
		if len(rule.patterns) > 0 && !matchesAny(rule.patterns, rule.value) {
			return fmt.Errorf("%s %q may not run %s on %q", rule.what, rule.value, q.Action, q.Source)
		}
	}
	if err := ap.checkAgainst(q.Action, specs); err != nil {
		return fmt.Errorf("the policy no longer fits the plugin: %w", err)
	}
	// Sorted so a refusal names the same parameter every time.
	names := make([]string, 0, len(ap.Params))
	for name := range ap.Params {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := ap.Params[name].allows(name, params); err != nil {
			return err
		}
	}
	return nil
}

// allows checks one parameter against its constraint. Values are compared after the same JSON round trip the plugin sees, so 3 in the policy matches 3 however the request encoded it, and a fraction never matches a whole number.
func (pp ParamPolicy) allows(name string, params map[string]any) error {
	v, ok := params[name]
	if !ok {
		return fmt.Errorf("parameter %s is constrained by the policy, so it must be given", name)
	}
	req := pluginsdk.ActionRequest{Params: params}
	if pp.Min != nil || pp.Max != nil {
		n, isInt := req.IntParam(name)
		if !isInt {
			return fmt.Errorf("parameter %s must be a whole number", name)
		}
		if (pp.Min != nil && n < *pp.Min) || (pp.Max != nil && n > *pp.Max) {
			return fmt.Errorf("parameter %s is %d, outside the policy's %s", name, n, pp.rangeString())
		}
	}
	if len(pp.Values) == 0 {
		return nil
	}
	for _, allowed := range pp.Values {
		switch a := allowed.(type) {
		case int:
			if n, isInt := req.IntParam(name); isInt && n == int64(a) {
				return nil
			}
		default:
			if v == allowed {
				return nil
			}
		}
	}
	return fmt.Errorf("parameter %s is not one of the values the policy allows", name)
}

func (pp ParamPolicy) rangeString() string {
	switch {
	case pp.Min != nil && pp.Max != nil:
		return fmt.Sprintf("range %d to %d", *pp.Min, *pp.Max)
	case pp.Min != nil:
		return fmt.Sprintf("minimum %d", *pp.Min)
	default:
		return fmt.Sprintf("maximum %d", *pp.Max)
	}
}

// unknownSources lists the sources the policy names that this process does not serve. They are not an error, since one policy file may cover several pipelines, but they are worth a warning: a misspelt source name means the real one is refused.
func (p *Policy) unknownSources(served []LocalSource) []string {
	if p == nil {
		return nil
	}
	names := map[string]bool{}
	for _, s := range served {
		names[s.Name] = true
	}
	var unknown []string
	for name := range p.Sources {
		if !names[name] {
			unknown = append(unknown, name)
		}
	}
	return unknown
}

func matchesAny(patterns []string, s string) bool {
	for _, p := range patterns {
		if globMatch(p, s) {
			return true
		}
	}
	return false
}

// globMatch matches s against a pattern in which * stands for any run of characters, including none, and every other character for itself. path.Match is not used because its * stops at a slash and its [ and \ are special, neither of which an operator writing actor names expects.
func globMatch(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	last := parts[len(parts)-1]
	for _, part := range parts[1 : len(parts)-1] {
		i := strings.Index(s, part)
		if i < 0 {
			return false
		}
		s = s[i+len(part):]
	}
	return len(s) >= len(last) && strings.HasSuffix(s, last)
}
