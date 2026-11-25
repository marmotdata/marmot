package pluginserve

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	gproto "google.golang.org/protobuf/proto"

	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

const (
	envelopeVersion = 1
	// envelopeLifetime bounds how long a signed query is valid, and so how long a serving process must remember its nonce.
	envelopeLifetime = 60 * time.Second
	// envelopeSkew tolerates clock drift between Marmot and a serving process.
	envelopeSkew = 30 * time.Second
	nonceSize    = 16
	// maxEnvelopeBytes caps a signed query before it is parsed; statements are text, so this is generous.
	maxEnvelopeBytes = 1 << 20
)

// exporterLabel is the RFC 9266 tls-exporter channel binding label. With TLS 1.3, which both ends require, an exporter value is unique to its connection, so a query bound to one connection is refused on any other.
const exporterLabel = "EXPORTER-Channel-Binding"

// channelBinding returns the RFC 9266 tls-exporter value of a TLS 1.3 connection.
func channelBinding(state tls.ConnectionState) ([]byte, error) {
	if state.Version != tls.VersionTLS13 {
		return nil, errors.New("channel binding requires TLS 1.3")
	}
	return state.ExportKeyingMaterial(exporterLabel, nil, 32)
}

func newNonce() ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	return nonce, nil
}

// signQuery fills in the envelope fields of q for the connection identified by audience and binding, serialises it and signs the bytes with the active signing key. A nonce already set on q is kept: it is the one the replica that took the query chose, and the serving process's receipt echoes it back there.
func (k *Keyring) signQuery(q *pb.Query, audience string, binding []byte, now time.Time) (*pb.SignedQuery, error) {
	if len(q.Nonce) == 0 {
		nonce, err := newNonce()
		if err != nil {
			return nil, err
		}
		q.Nonce = nonce
	}
	q.Version = envelopeVersion
	q.Issuer = k.InstanceID
	q.Audience = audience
	q.IssuedAtUnix = now.Unix()
	q.ExpiresAtUnix = now.Add(envelopeLifetime).Unix()
	q.ChannelBinding = binding

	data, err := gproto.Marshal(q)
	if err != nil {
		return nil, fmt.Errorf("encoding query: %w", err)
	}
	signature, err := k.Sign(data)
	if err != nil {
		return nil, fmt.Errorf("signing query: %w", err)
	}
	return &pb.SignedQuery{Query: data, Signature: signature}, nil
}

// Verifier checks signed queries on the serving side. One Verifier serves one connection: it knows the identity Marmot gave that connection's certificate and the connection's channel binding, and remembers nonces until their queries expire. It accepts any of the signing keys pinned for the connection, so queries keep verifying while Marmot rotates keys.
type Verifier struct {
	signingKeys []*ecdsa.PublicKey
	issuer      string
	audience    string
	binding     []byte
	now         func() time.Time
	seen        nonceCache
}

// NewVerifier builds a Verifier for one connection.
func NewVerifier(signingKeys []*ecdsa.PublicKey, issuer, audience string, binding []byte) *Verifier {
	return &Verifier{
		signingKeys: signingKeys,
		issuer:      issuer,
		audience:    audience,
		binding:     binding,
		now:         time.Now,
		seen:        nonceCache{seen: make(map[string]time.Time)},
	}
}

// ErrRejected wraps every reason a signed query is refused; the detail is for the serving process's log, not for Marmot.
var ErrRejected = errors.New("query rejected")

// Verify checks a signed query for op and returns it. The signature is checked before the bytes are parsed, so nothing unauthenticated is ever interpreted.
func (v *Verifier) Verify(signed *pb.SignedQuery, op pb.Operation) (*pb.Query, error) {
	if signed == nil || len(signed.Query) == 0 || len(signed.Query) > maxEnvelopeBytes {
		return nil, fmt.Errorf("%w: malformed", ErrRejected)
	}
	if !signedByAny(v.signingKeys, nil, signed.Query, signed.Signature) {
		return nil, fmt.Errorf("%w: bad signature", ErrRejected)
	}

	var q pb.Query
	if err := gproto.Unmarshal(signed.Query, &q); err != nil {
		return nil, fmt.Errorf("%w: malformed", ErrRejected)
	}

	now := v.now()
	issued, expires := time.Unix(q.IssuedAtUnix, 0), time.Unix(q.ExpiresAtUnix, 0)
	switch {
	case q.Version != envelopeVersion:
		return nil, fmt.Errorf("%w: unsupported version %d", ErrRejected, q.Version)
	case q.Issuer != v.issuer:
		return nil, fmt.Errorf("%w: wrong issuer", ErrRejected)
	case q.Audience != v.audience:
		return nil, fmt.Errorf("%w: wrong audience", ErrRejected)
	case q.Operation != op:
		return nil, fmt.Errorf("%w: wrong operation", ErrRejected)
	case subtle.ConstantTimeCompare(q.ChannelBinding, v.binding) != 1:
		return nil, fmt.Errorf("%w: bound to another connection", ErrRejected)
	case expires.Sub(issued) > envelopeLifetime || expires.Before(issued):
		return nil, fmt.Errorf("%w: lifetime too long", ErrRejected)
	case now.Before(issued.Add(-envelopeSkew)):
		return nil, fmt.Errorf("%w: issued in the future", ErrRejected)
	case now.After(expires.Add(envelopeSkew)):
		return nil, fmt.Errorf("%w: expired", ErrRejected)
	case len(q.Nonce) != nonceSize:
		return nil, fmt.Errorf("%w: malformed nonce", ErrRejected)
	}

	if !v.seen.remember(string(q.Nonce), expires.Add(envelopeSkew), now) {
		return nil, fmt.Errorf("%w: replayed", ErrRejected)
	}
	return &q, nil
}

// signedByAny reports whether signature is one of keys' over domain followed by message. Each kind of signed message has its own domain, or none for a Query, so a signature made for one can never be passed off as another.
func signedByAny(keys []*ecdsa.PublicKey, domain, message, signature []byte) bool {
	h := sha256.New()
	h.Write(domain)
	h.Write(message)
	digest := h.Sum(nil)
	for _, key := range keys {
		if ecdsa.VerifyASN1(key, digest, signature) {
			return true
		}
	}
	return false
}

// nonceCache remembers nonces until the messages carrying them expire.
type nonceCache struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

// remember records a nonce until forgetAt and reports whether it was new. Expired nonces are dropped as it goes, so the cache holds at most a minute or two of messages.
func (c *nonceCache) remember(nonce string, forgetAt, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for n, t := range c.seen {
		if now.After(t) {
			delete(c.seen, n)
		}
	}
	if _, ok := c.seen[nonce]; ok {
		return false
	}
	c.seen[nonce] = forgetAt
	return true
}

const (
	forwardVersion = 1
	// forwardLifetime bounds a signed forward. A forward is sent the moment it is signed, to a replica in the same deployment, so it needs far less than a query's minute.
	forwardLifetime = 15 * time.Second
)

// forwardDomain prefixes the bytes a forward's signature covers. Forwards are signed with the same key as queries, and without it a forward's signature could be presented to a serving process as a query's.
var forwardDomain = []byte("marmot serve forward v1\x00")

// signForward signs a query for forwarding to the replica whose SPIFFE ID is audience. The forward's nonce is the one the query will carry to the serving process, so it is set before this is called.
func (k *Keyring) signForward(fq *pb.ForwardedQuery, audience string, now time.Time) (*pb.SignedForward, error) {
	if len(fq.Nonce) != nonceSize {
		return nil, errors.New("forward has no nonce")
	}
	fq = gproto.CloneOf(fq)
	fq.Version = forwardVersion
	fq.Issuer = k.InstanceID
	fq.Audience = audience
	fq.IssuedAtUnix = now.Unix()
	fq.ExpiresAtUnix = now.Add(forwardLifetime).Unix()

	data, err := gproto.Marshal(fq)
	if err != nil {
		return nil, fmt.Errorf("encoding forward: %w", err)
	}
	signature, err := k.Sign(append(append([]byte{}, forwardDomain...), data...))
	if err != nil {
		return nil, fmt.Errorf("signing forward: %w", err)
	}
	return &pb.SignedForward{Forward: data, Signature: signature}, nil
}

// forwardVerifier checks forwards on the receiving replica: signed by one of the instance's trusted signing keys, addressed to this replica, unexpired and never seen before. Mutual TLS already says the sender holds a replica certificate; the signature also needs the signing key, which a stolen replica certificate alone does not give.
type forwardVerifier struct {
	keyring  *Keyring
	audience string
	now      func() time.Time
	seen     nonceCache
}

func newForwardVerifier(k *Keyring, audience string) *forwardVerifier {
	return &forwardVerifier{keyring: k, audience: audience, now: time.Now, seen: nonceCache{seen: make(map[string]time.Time)}}
}

func (v *forwardVerifier) verify(signed *pb.SignedForward) (*pb.ForwardedQuery, error) {
	if signed == nil || len(signed.Forward) == 0 || len(signed.Forward) > maxEnvelopeBytes {
		return nil, fmt.Errorf("%w: malformed forward", ErrRejected)
	}
	if !signedByAny(v.keyring.signingKeys(), forwardDomain, signed.Forward, signed.Signature) {
		return nil, fmt.Errorf("%w: bad forward signature", ErrRejected)
	}
	var fq pb.ForwardedQuery
	if err := gproto.Unmarshal(signed.Forward, &fq); err != nil {
		return nil, fmt.Errorf("%w: malformed forward", ErrRejected)
	}

	now := v.now()
	issued, expires := time.Unix(fq.IssuedAtUnix, 0), time.Unix(fq.ExpiresAtUnix, 0)
	switch {
	case fq.Version != forwardVersion:
		return nil, fmt.Errorf("%w: unsupported forward version %d", ErrRejected, fq.Version)
	case fq.Issuer != v.keyring.InstanceID:
		return nil, fmt.Errorf("%w: forward from another instance", ErrRejected)
	case fq.Audience != v.audience:
		return nil, fmt.Errorf("%w: forward for another replica", ErrRejected)
	case expires.Sub(issued) > forwardLifetime || expires.Before(issued):
		return nil, fmt.Errorf("%w: forward lifetime too long", ErrRejected)
	case now.Before(issued.Add(-envelopeSkew)):
		return nil, fmt.Errorf("%w: forward issued in the future", ErrRejected)
	case now.After(expires.Add(envelopeSkew)):
		return nil, fmt.Errorf("%w: forward expired", ErrRejected)
	case len(fq.Nonce) != nonceSize:
		return nil, fmt.Errorf("%w: malformed forward nonce", ErrRejected)
	}
	if !v.seen.remember(string(fq.Nonce), expires.Add(envelopeSkew), now) {
		return nil, fmt.Errorf("%w: forward replayed", ErrRejected)
	}
	return &fq, nil
}
