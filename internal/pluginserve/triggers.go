package pluginserve

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

// Triggers run in the serving process, never in Marmot, and only as its own policy says. Each one the policy names is watched for the life of the process with the parameters the policy gives; Marmot never names a trigger, sends a parameter or an expression, and the WatchTriggers request that opens a connection's stream is refused if it tries. The process checks every event against the plugin's trigger manifest, drops repeats within the trigger's cooldown, rate limits each source, signs what is left with the connection's key and audits it, then streams it to Marmot, which only ever receives.

// TriggerWatcher runs triggers on local plugin processes. Implemented by plugin.ActionHost. gate is run against the trigger manifest of the process that will watch, before the watch starts.
type TriggerWatcher interface {
	TriggerManifest(ctx context.Context, pluginID string) ([]pluginsdk.TriggerSpec, error)
	WatchTrigger(ctx context.Context, pluginID string, config map[string]any, trigger string, params map[string]any, gate func([]pluginsdk.TriggerSpec) error) (pluginsdk.TriggerStream, error)
}

// triggerEventDomain prefixes the bytes a trigger event's signature covers, apart from every receipt's.
var triggerEventDomain = []byte("marmot serve trigger event v1\x00")

const (
	triggerEventVersion = 1
	// triggerRate and triggerBurst limit each source to 30 events a minute, so a plugin or a cluster in turmoil cannot flood Marmot.
	triggerRate  = 30
	triggerBurst = 30
	// triggerBuffer bounds the events held while no stream is open, as between a connection dropping and the next one.
	triggerBuffer = 64
	// maxDedupe bounds the fingerprints remembered for cooldowns; the oldest are forgotten first.
	maxDedupe = 10000
	// triggerStartGrace is how long Serve waits for each watch to fail at once, as one with a parameter the plugin cannot use does, before it starts serving.
	triggerStartGrace = 3 * time.Second
	// dropAuditInterval spaces the lines that sum dropped events.
	dropAuditInterval = time.Minute
	triggerMaxBackoff = time.Minute
)

// processState is what outlives each connection: connections are replaced at every certificate renewal, and neither spent approvals nor trigger watches may reset with them.
type processState struct {
	spent    *spentApprovals
	triggers *triggerDispatcher
}

// startProcess checks the action and trigger setup, reads spent approvals back from the audit log and starts the policy's triggers. Not being able to read the audit log back is fatal with actions on: without it a restart could make spent approvals usable again.
func startProcess(ctx context.Context, opts ServeOptions) (*processState, error) {
	if err := checkActionSetup(ctx, opts); err != nil {
		return nil, err
	}
	state := &processState{spent: newSpentApprovals(), triggers: newTriggerDispatcher(opts.AuditLog)}
	if opts.AllowActions && opts.AuditLog != nil {
		now := time.Now()
		entries, err := opts.AuditLog.Since(now.Add(-maxApprovalAge - 2*envelopeSkew))
		if err != nil {
			return nil, fmt.Errorf("reading spent approvals back from the audit log: %w", err)
		}
		if n := state.spent.load(entries, now); n > 0 {
			log.Info().Int("approvals", n).Msg("Approvals spent before the restart stay spent")
		}
	}
	if err := startTriggers(ctx, opts, state.triggers); err != nil {
		return nil, err
	}
	return state, nil
}

// startTriggers starts a watch for each trigger the policy names, and refuses to start the process when one fails at once with InvalidArgument, which is a parameter or expression the plugin cannot use and only the operator can fix.
func startTriggers(ctx context.Context, opts ServeOptions, d *triggerDispatcher) error {
	if !opts.AllowActions || opts.Policy == nil {
		return nil
	}
	var failed []error
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, src := range opts.Sources {
		for name, tp := range opts.Policy.Sources[src.Name].Triggers {
			w := &triggerWatch{source: src, name: name, policy: tp, watcher: opts.Watcher, dispatcher: d, audit: opts.AuditLog}
			early := make(chan error, 1)
			wg.Add(1)
			go func() {
				defer wg.Done()
				select {
				case err := <-early:
					mu.Lock()
					failed = append(failed, fmt.Errorf("trigger %q on %q: %w", name, src.Name, err))
					mu.Unlock()
				case <-time.After(triggerStartGrace):
				case <-ctx.Done():
				}
			}()
			go w.run(ctx, early)
		}
	}
	wg.Wait()
	if len(failed) > 0 {
		return errors.Join(failed...)
	}
	return nil
}

// triggerWatch is one trigger running on one source, restarted with backoff whenever its plugin process ends it.
type triggerWatch struct {
	source     LocalSource
	name       string
	policy     TriggerPolicy
	watcher    TriggerWatcher
	dispatcher *triggerDispatcher
	audit      *AuditLog
}

func (w *triggerWatch) run(ctx context.Context, early chan<- error) {
	backoff := time.Second
	first := true
	for ctx.Err() == nil {
		started := time.Now()
		err := w.once(ctx)
		if ctx.Err() != nil {
			return
		}
		if first && status.Code(err) == codes.InvalidArgument && time.Since(started) < triggerStartGrace {
			early <- err
			return
		}
		first = false
		log.Warn().Err(err).Str("source", w.source.Name).Str("trigger", w.name).Dur("retry_in", backoff).Msg("Trigger watch ended")
		if !sleep(ctx, jitter(backoff)) {
			return
		}
		if time.Since(started) > triggerMaxBackoff {
			backoff = time.Second
		} else {
			backoff = min(backoff*2, triggerMaxBackoff)
		}
	}
}

// once runs the watch until its stream ends. The gate checks the trigger and the policy's parameters against the manifest of the very process that will watch, and every event is checked against that same trigger spec.
func (w *triggerWatch) once(ctx context.Context) error {
	if w.watcher == nil {
		return status.Error(codes.InvalidArgument, "nothing in this process can run triggers")
	}
	var spec pluginsdk.TriggerSpec
	var digest string
	params := yamlParams(w.policy.Params)
	gate := func(specs []pluginsdk.TriggerSpec) error {
		i := slices.IndexFunc(specs, func(s pluginsdk.TriggerSpec) bool { return s.Name == w.name })
		if i < 0 {
			return status.Errorf(codes.InvalidArgument, "trigger %q is not in the plugin's manifest", w.name)
		}
		if err := pluginsdk.ValidateTriggerParams(specs[i], params); err != nil {
			return status.Errorf(codes.InvalidArgument, "the policy's parameters for %s: %v", w.name, err)
		}
		spec, digest = specs[i], pluginsdk.TriggersDigest(specs)
		return nil
	}
	stream, err := w.watcher.WatchTrigger(ctx, w.source.PluginID, w.source.Config, w.name, params, gate)
	if err != nil {
		w.refused(err)
		return err
	}
	defer func() { _ = stream.Close() }()
	for {
		ev, err := stream.Recv()
		if err != nil {
			if status.Code(err) == codes.InvalidArgument {
				w.refused(err)
			}
			return err
		}
		w.dispatcher.offer(w.source.Name, spec, digest, w.policy.Cooldown, *ev)
	}
}

func (w *triggerWatch) refused(err error) {
	if w.audit == nil || status.Code(err) != codes.InvalidArgument {
		return
	}
	if aerr := w.audit.Append(auditEntryFor(AuditEntry{Event: AuditRefused, Operation: "watch_trigger", Policy: "allowed", Source: w.source.Name, Trigger: w.name, Reason: status.Convert(err).Message()}, nil)); aerr != nil {
		log.Error().Err(aerr).Msg("Failed to write the audit log")
	}
}

// pendingEvent is a checked event waiting for a stream.
type pendingEvent struct {
	source string
	digest string
	ev     pluginsdk.TriggerEvent
}

// triggerDispatcher takes every watch's events and hands them to the newest WatchTriggers stream. There is one per process, because each connection is short-lived and a cooldown or rate limit that reset with it would not limit anything.
type triggerDispatcher struct {
	audit *AuditLog
	now   func() time.Time

	mu       sync.Mutex
	pending  []pendingEvent
	lastSent map[string]time.Time
	buckets  map[string]*tokenBucket
	drops    map[string]*dropCount
	// stream is the newest WatchTriggers stream's wake channel; events go only there, so the same event is never sent on two connections.
	stream chan struct{}
}

type dropCount struct {
	n       int64
	audited time.Time
	reason  string
}

func newTriggerDispatcher(audit *AuditLog) *triggerDispatcher {
	return &triggerDispatcher{
		audit:    audit,
		now:      time.Now,
		lastSent: map[string]time.Time{},
		buckets:  map[string]*tokenBucket{},
		drops:    map[string]*dropCount{},
	}
}

// offer checks an event from a watch and queues it unless it repeats one sent within cooldown or its source is over its rate.
func (d *triggerDispatcher) offer(source string, spec pluginsdk.TriggerSpec, digest string, cooldown time.Duration, ev pluginsdk.TriggerEvent) {
	checked, err := pluginsdk.CheckTriggerEvent(spec, ev)
	if err != nil {
		log.Warn().Err(err).Str("source", source).Str("trigger", spec.Name).Msg("Dropped a trigger event the plugin should not have sent")
		d.dropped(source, "the event failed the trigger's checks: "+err.Error())
		return
	}
	now := d.now()
	key := source + "\x00" + checked.Trigger + "\x00" + checked.Fingerprint

	d.mu.Lock()
	defer d.mu.Unlock()
	if last, ok := d.lastSent[key]; ok && now.Sub(last) < cooldown {
		return
	}
	b := d.buckets[source]
	if b == nil {
		b = &tokenBucket{tokens: triggerBurst, last: now}
		d.buckets[source] = b
	}
	if !b.take(now) {
		d.droppedLocked(source, "over the rate limit of 30 events a minute", now)
		return
	}
	d.forgetOld(now)
	d.lastSent[key] = now
	if len(d.pending) >= triggerBuffer {
		d.droppedLocked(d.pending[0].source, "no stream to Marmot was open and the buffer was full", now)
		d.pending = d.pending[1:]
	}
	d.pending = append(d.pending, pendingEvent{source: source, digest: digest, ev: checked})
	if d.stream != nil {
		select {
		case d.stream <- struct{}{}:
		default:
		}
	}
}

// forgetOld keeps the cooldown map bounded: entries past the longest cooldown can never suppress anything, and past the cap the oldest go first.
func (d *triggerDispatcher) forgetOld(now time.Time) {
	if len(d.lastSent) < maxDedupe {
		return
	}
	for k, t := range d.lastSent {
		if now.Sub(t) > maxTriggerCooldown {
			delete(d.lastSent, k)
		}
	}
	for len(d.lastSent) >= maxDedupe {
		var oldest string
		var at time.Time
		for k, t := range d.lastSent {
			if oldest == "" || t.Before(at) {
				oldest, at = k, t
			}
		}
		delete(d.lastSent, oldest)
	}
}

func (d *triggerDispatcher) dropped(source, reason string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.droppedLocked(source, reason, d.now())
}

// droppedLocked counts a drop and writes the count to the audit log at most once a minute per source.
func (d *triggerDispatcher) droppedLocked(source, reason string, now time.Time) {
	c := d.drops[source]
	if c == nil {
		c = &dropCount{}
		d.drops[source] = c
	}
	c.n++
	c.reason = reason
	if now.Sub(c.audited) < dropAuditInterval {
		return
	}
	log.Warn().Str("source", source).Int64("count", c.n).Str("reason", reason).Msg("Dropped trigger events")
	if d.audit != nil {
		if err := d.audit.Append(auditEntryFor(AuditEntry{Event: AuditTriggerDropped, Policy: "allowed", Source: source, Count: c.n, Reason: reason}, nil)); err != nil {
			log.Error().Err(err).Msg("Failed to write the audit log")
		}
	}
	c.n, c.audited = 0, now
}

// attach makes a stream the one events go to and returns its wake channel. A stream attached later replaces it, and the earlier one ends.
func (d *triggerDispatcher) attach() chan struct{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	wake := make(chan struct{}, 1)
	if d.stream != nil {
		close(d.stream)
	}
	d.stream = wake
	if len(d.pending) > 0 {
		wake <- struct{}{}
	}
	return wake
}

func (d *triggerDispatcher) detach(wake chan struct{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stream == wake {
		close(wake)
		d.stream = nil
	}
}

// take removes the pending events, for the attached stream to send.
func (d *triggerDispatcher) take() []pendingEvent {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := d.pending
	d.pending = nil
	return out
}

// requeue puts back events a stream could not send, ahead of newer ones.
func (d *triggerDispatcher) requeue(events []pendingEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pending = append(events, d.pending...)
	for len(d.pending) > triggerBuffer {
		d.droppedLocked(d.pending[0].source, "no stream to Marmot was open and the buffer was full", d.now())
		d.pending = d.pending[1:]
	}
}

// tokenBucket is a plain token bucket refilled at triggerRate a minute.
type tokenBucket struct {
	tokens float64
	last   time.Time
}

func (b *tokenBucket) take(now time.Time) bool {
	b.tokens = min(float64(triggerBurst), b.tokens+now.Sub(b.last).Minutes()*triggerRate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// WatchTriggers streams the process's trigger events to Marmot on this connection until it closes, or until a newer connection's stream takes over. The request only opens the stream: it is refused if it names a source, a statement, an action, parameters or an approval, since nothing Marmot sends may choose what is watched.
func (p *pluginService) WatchTriggers(signed *pb.SignedQuery, out grpc.ServerStreamingServer[pb.SignedTriggerEvent]) error {
	op := pb.Operation_OPERATION_WATCH_TRIGGERS
	q, err := p.verifier.Verify(signed, op)
	if err != nil {
		log.Warn().Err(err).Msg("Refused trigger stream")
		p.record(AuditEntry{Event: AuditRefused, Operation: "watch_triggers", Reason: err.Error()}, nil)
		return status.Error(codes.PermissionDenied, ErrRejected.Error())
	}
	if q.Source != "" || q.Statement != "" || q.Action != "" || len(q.ParamsJson) > 0 || q.Precondition != "" || len(q.Approval) > 0 || q.MaxRows != 0 {
		reason := "a trigger stream request carries nothing but its envelope: what is watched is this process's decision"
		p.record(AuditEntry{Event: AuditRefused, Operation: "watch_triggers", Reason: reason}, q)
		return status.Error(codes.PermissionDenied, "refused by the serving process: "+reason)
	}
	if !p.actions || p.triggers == nil {
		// Nothing runs, so the stream ends at once rather than idling.
		return nil
	}

	wake := p.triggers.attach()
	defer p.triggers.detach(wake)
	var sequence uint64
	for {
		select {
		case <-out.Context().Done():
			return nil
		case _, open := <-wake:
			if !open {
				return nil
			}
		}
		events := p.triggers.take()
		for i, pe := range events {
			sequence++
			signedEvent, msg, err := p.signTriggerEvent(q, pe, sequence)
			if err != nil {
				log.Error().Err(err).Msg("Failed to sign a trigger event")
				p.triggers.dropped(pe.source, "the event could not be signed")
				continue
			}
			if err := out.Send(signedEvent); err != nil {
				// Marmot went away; the events not sent wait for the next stream.
				p.triggers.requeue(events[i:])
				return err
			}
			p.record(AuditEntry{
				Event: AuditTrigger, Policy: "allowed", Source: msg.Source, Trigger: msg.Trigger, Fingerprint: msg.Fingerprint,
				Target: string(pe.ev.Target.Kind) + "/" + pe.ev.Target.Namespace + "/" + pe.ev.Target.Name, Sequence: sequence,
				Nonce: hex.EncodeToString(q.Nonce), ReceiptSignature: hex.EncodeToString(signedEvent.Signature),
			}, nil)
		}
	}
}

func (p *pluginService) signTriggerEvent(q *pb.Query, pe pendingEvent, sequence uint64) (*pb.SignedTriggerEvent, *pb.TriggerEvent, error) {
	facts, err := json.Marshal(pe.ev.Facts)
	if err != nil {
		return nil, nil, err
	}
	msg := &pb.TriggerEvent{
		Version:         triggerEventVersion,
		Source:          pe.source,
		Trigger:         pe.ev.Trigger,
		TargetKind:      string(pe.ev.Target.Kind),
		TargetNamespace: pe.ev.Target.Namespace,
		TargetName:      pe.ev.Target.Name,
		FactsJson:       facts,
		ObservedAtUnix:  pe.ev.ObservedAt.Unix(),
		Fingerprint:     pe.ev.Fingerprint,
		Detail:          pe.ev.Detail,
		Nonce:           q.Nonce,
		Sequence:        sequence,
		TriggersDigest:  pe.digest,
		SentAtUnix:      time.Now().Unix(),
	}
	sr, err := p.signer.signUnder(triggerEventDomain, msg)
	if err != nil {
		return nil, nil, err
	}
	return &pb.SignedTriggerEvent{Event: sr.Receipt, Signature: sr.Signature, Certificate: sr.Certificate}, msg, nil
}
