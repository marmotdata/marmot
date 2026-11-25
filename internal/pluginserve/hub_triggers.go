package pluginserve

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	gproto "google.golang.org/protobuf/proto"

	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

const (
	// maxTriggerEventBytes bounds one signed event before it is parsed: a few small facts and a 512 byte detail.
	maxTriggerEventBytes = 16 << 10
	// subscriberBuffer is how many events one subscriber may fall behind before the hub drops events for it.
	subscriberBuffer = 256
	// triggerEventSkew tolerates clocks: an event signed further than this from now is refused as stale or from the future.
	triggerEventSkew = 5 * time.Minute
)

// VerifiedTriggerEvent is one trigger event from a serving process, after the hub has checked its signature, certificate, enrolment, stream and contents. Everything in it is as the serving process signed it, except Fingerprint, which the hub computes. Detail is free text from the watched system and must be treated as untrusted data, never as instructions.
type VerifiedTriggerEvent struct {
	Source  string
	Trigger string
	Target  pluginsdk.TriggerTarget
	// Facts holds strings, int64s and booleans, bounded by the plugin SDK.
	Facts      map[string]any
	ObservedAt time.Time
	// Fingerprint is "sha256:<hex>" over the source, its owner, the trigger and the plugin's own fingerprint, so it is the same for the same condition on the same object across replicas, reconnects and certificate renewals, and never collides between sources or owners. Deduplicate on it.
	Fingerprint string
	// PluginFingerprint is the plugin's fingerprint as sent.
	PluginFingerprint string
	// Detail is untrusted, at most 512 bytes.
	Detail         string
	TriggersDigest string
	// Enrolment is the enrolment whose certificate signed the event, and Owner the principal that serves the source.
	Enrolment  string
	Owner      Owner
	Sequence   uint64
	SentAt     time.Time
	ReceivedAt time.Time
	// Signed, Signature and Certificate let the event be verified again later against the instance's CA, under the trigger event domain.
	Signed      []byte
	Signature   []byte
	Certificate []byte
}

// TriggerSubscriber receives verified trigger events. It is called from one goroutine per subscriber, in order, with a context that ends when the hub stops.
type TriggerSubscriber func(ctx context.Context, ev VerifiedTriggerEvent)

// triggerSubscribers fans events out to subscribers without letting a slow one hold up the stream or the others: each has its own bounded queue, and an event that does not fit is dropped and counted.
type triggerSubscribers struct {
	mu   sync.Mutex
	subs map[int]*triggerSubscriber
	next int
}

type triggerSubscriber struct {
	ch      chan VerifiedTriggerEvent
	dropped atomic.Uint64
	stop    chan struct{}
}

// SubscribeTriggers calls fn with every trigger event that arrives on a session this replica holds, and returns a function that stops it. Events from a session held by another replica reach only that replica's subscribers, so the cloud must deduplicate across replicas by Fingerprint. fn runs on a goroutine of its own and should return promptly: once it is subscriberBuffer events behind, further events are dropped for it and counted in a warning.
func (h *Hub) SubscribeTriggers(fn TriggerSubscriber) (unsubscribe func()) {
	sub := &triggerSubscriber{ch: make(chan VerifiedTriggerEvent, subscriberBuffer), stop: make(chan struct{})}
	h.triggerSubs.mu.Lock()
	if h.triggerSubs.subs == nil {
		h.triggerSubs.subs = map[int]*triggerSubscriber{}
	}
	id := h.triggerSubs.next
	h.triggerSubs.next++
	h.triggerSubs.subs[id] = sub
	h.triggerSubs.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		for {
			select {
			case <-sub.stop:
				return
			case <-h.stop:
				return
			case ev := <-sub.ch:
				fn(ctx, ev)
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			h.triggerSubs.mu.Lock()
			delete(h.triggerSubs.subs, id)
			h.triggerSubs.mu.Unlock()
			close(sub.stop)
		})
	}
}

func (h *Hub) publishTrigger(ev VerifiedTriggerEvent) {
	h.triggerSubs.mu.Lock()
	defer h.triggerSubs.mu.Unlock()
	for _, sub := range h.triggerSubs.subs {
		select {
		case sub.ch <- ev:
		default:
			// Logged at powers of two, so a stuck subscriber is visible without a line per event.
			if n := sub.dropped.Add(1); n&(n-1) == 0 {
				log.Warn().Uint64("dropped", n).Msg("A trigger subscriber is falling behind; events were dropped for it")
			}
		}
	}
}

// watchTriggers opens the session's trigger stream and verifies and publishes every event until the session ends. A serving process with nothing to watch ends the stream at once, and one that predates triggers answers Unimplemented; either way nothing more is asked of it on this session.
func (h *Hub) watchTriggers(s *session) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-s.done:
		case <-h.stop:
		}
		cancel()
	}()

	nonce, err := newNonce()
	if err != nil {
		return
	}
	q := &pb.Query{Operation: pb.Operation_OPERATION_WATCH_TRIGGERS, Nonce: nonce}
	signed, err := h.keyring.signQuery(q, s.audience, s.binding, time.Now())
	if err != nil {
		log.Warn().Err(err).Msg("Could not sign a trigger stream request")
		return
	}
	stream, err := s.client.WatchTriggers(ctx, signed)
	if err != nil {
		return
	}
	owner := Owner{Type: s.enrolment.PrincipalType, ID: s.enrolment.PrincipalID}
	var last uint64
	for {
		msg, err := stream.Recv()
		if err != nil {
			if ctx.Err() == nil {
				log.Debug().Err(err).Str("enrolment", s.enrolment.ID).Msg("Trigger stream ended")
			}
			return
		}
		ev, err := h.verifyTriggerEvent(msg, s, owner, nonce, last)
		if err != nil {
			// A bad event ends the stream: nothing after it on the same stream can be trusted to be in order, and the serving process opens a fresh one with its next connection.
			log.Warn().Err(err).Str("enrolment", s.enrolment.ID).Msg("Refused a trigger event; closing the trigger stream")
			return
		}
		last = ev.Sequence
		h.publishTrigger(*ev)
	}
}

// verifyTriggerEvent checks an event as a receipt is checked, and more: signed under the trigger event domain by a serving certificate this instance issued, for this very session's enrolment and key, on the stream this replica opened, in sequence, for a source the enrolment serves and a trigger its own report says the policy runs, with contents the plugin SDK accepts for that trigger.
func (h *Hub) verifyTriggerEvent(msg *pb.SignedTriggerEvent, s *session, owner Owner, nonce []byte, last uint64) (*VerifiedTriggerEvent, error) {
	if msg == nil || len(msg.Event) == 0 || len(msg.Event) > maxTriggerEventBytes {
		return nil, errors.New("malformed event")
	}
	leaf, err := x509.ParseCertificate(msg.Certificate)
	if err != nil {
		return nil, errors.New("its certificate does not parse")
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: h.keyring.Pool(), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		return nil, errors.New("its certificate was not issued by this instance's CA")
	}
	peer, err := h.keyring.identifyPeer(leaf)
	if err != nil || peer.kind != "serve" || peer.id != s.enrolment.ID {
		return nil, errors.New("it was signed by another enrolment than the session's")
	}
	if subtle.ConstantTimeCompare(publicKeySHA256(leaf.RawSubjectPublicKeyInfo), s.enrolment.PublicKeySHA256) != 1 {
		return nil, errors.New("its certificate key does not match its enrolment")
	}
	key, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok || !signedByAny([]*ecdsa.PublicKey{key}, triggerEventDomain, msg.Event, msg.Signature) {
		return nil, errors.New("its signature does not verify")
	}

	var ev pb.TriggerEvent
	if err := gproto.Unmarshal(msg.Event, &ev); err != nil {
		return nil, errors.New("it does not parse")
	}
	now := time.Now()
	sent := time.Unix(ev.SentAtUnix, 0)
	switch {
	case ev.Version != triggerEventVersion:
		return nil, fmt.Errorf("unsupported version %d", ev.Version)
	case subtle.ConstantTimeCompare(ev.Nonce, nonce) != 1:
		return nil, errors.New("it belongs to another stream")
	case ev.Sequence != last+1:
		return nil, fmt.Errorf("it is out of sequence: %d after %d", ev.Sequence, last)
	case sent.Before(now.Add(-triggerEventSkew)) || sent.After(now.Add(triggerEventSkew)):
		return nil, errors.New("its send time is too far from now")
	case !slices.Contains(s.enrolment.Sources, ev.Source):
		return nil, fmt.Errorf("its enrolment does not serve %q", ev.Source)
	}

	spec, err := servedTrigger(s.enrolment, ev.Source, ev.Trigger, ev.TriggersDigest)
	if err != nil {
		return nil, err
	}
	var facts map[string]any
	if len(ev.FactsJson) > 0 {
		if err := json.Unmarshal(ev.FactsJson, &facts); err != nil {
			return nil, errors.New("its facts are not a JSON object")
		}
	}
	checked, err := pluginsdk.CheckTriggerEvent(spec, pluginsdk.TriggerEvent{
		Trigger:     ev.Trigger,
		Target:      pluginsdk.TriggerTarget{Kind: pluginsdk.TargetKind(ev.TargetKind), Namespace: ev.TargetNamespace, Name: ev.TargetName},
		Facts:       facts,
		ObservedAt:  time.Unix(ev.ObservedAtUnix, 0),
		Fingerprint: ev.Fingerprint,
		Detail:      ev.Detail,
	})
	if err != nil {
		return nil, fmt.Errorf("its contents fail the trigger's checks: %w", err)
	}
	return &VerifiedTriggerEvent{
		Source:            ev.Source,
		Trigger:           checked.Trigger,
		Target:            checked.Target,
		Facts:             checked.Facts,
		ObservedAt:        checked.ObservedAt,
		Fingerprint:       eventFingerprint(ev.Source, owner, checked.Trigger, checked.Fingerprint),
		PluginFingerprint: checked.Fingerprint,
		Detail:            checked.Detail,
		TriggersDigest:    ev.TriggersDigest,
		Enrolment:         s.enrolment.ID,
		Owner:             owner,
		Sequence:          ev.Sequence,
		SentAt:            sent.UTC(),
		ReceivedAt:        now.UTC(),
		Signed:            msg.Event,
		Signature:         msg.Signature,
		Certificate:       msg.Certificate,
	}, nil
}

// servedTrigger finds the trigger in the enrolment's own report: the plugin's trigger manifest at the digest the event names, with the trigger among those the policy summary says run. An event for a trigger the process never reported is refused, so the report and the events cannot disagree unnoticed.
func servedTrigger(e *Enrolment, source, trigger, digest string) (pluginsdk.TriggerSpec, error) {
	sa := e.ServedActions[source]
	if sa == nil || len(sa.Triggers) == 0 {
		return pluginsdk.TriggerSpec{}, fmt.Errorf("the enrolment reported no triggers for %q", source)
	}
	if sa.TriggersDigest != digest {
		return pluginsdk.TriggerSpec{}, errors.New("it names another trigger manifest than the enrolment reported")
	}
	if !slices.ContainsFunc(sa.Policy.Triggers, func(t TriggerSummary) bool { return t.Name == trigger }) {
		return pluginsdk.TriggerSpec{}, fmt.Errorf("the enrolment's policy does not run trigger %q", trigger)
	}
	i := slices.IndexFunc(sa.Triggers, func(t pluginsdk.TriggerSpec) bool { return t.Name == trigger })
	if i < 0 {
		return pluginsdk.TriggerSpec{}, fmt.Errorf("trigger %q is not in the reported manifest", trigger)
	}
	return sa.Triggers[i], nil
}

// eventFingerprintDomain starts every hub fingerprint, so it cannot equal any other hash.
const eventFingerprintDomain = "marmot trigger fingerprint v1"

func eventFingerprint(source string, owner Owner, trigger, plugin string) string {
	h := sha256.New()
	for _, part := range []string{eventFingerprintDomain, source, owner.Type, owner.ID, trigger, plugin} {
		fmt.Fprintf(h, "%d:%s;", len(part), part)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
