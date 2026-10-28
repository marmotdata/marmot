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

// signQuery fills in the envelope fields of q for the connection identified by audience and binding, serialises it and signs the bytes with the active signing key.
func (k *Keyring) signQuery(q *pb.Query, audience string, binding []byte, now time.Time) (*pb.SignedQuery, error) {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	q.Version = envelopeVersion
	q.Issuer = k.InstanceID
	q.Audience = audience
	q.IssuedAtUnix = now.Unix()
	q.ExpiresAtUnix = now.Add(envelopeLifetime).Unix()
	q.Nonce = nonce
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

	mu   sync.Mutex
	seen map[string]time.Time
}

// NewVerifier builds a Verifier for one connection.
func NewVerifier(signingKeys []*ecdsa.PublicKey, issuer, audience string, binding []byte) *Verifier {
	return &Verifier{
		signingKeys: signingKeys,
		issuer:      issuer,
		audience:    audience,
		binding:     binding,
		now:         time.Now,
		seen:        make(map[string]time.Time),
	}
}

// ErrRejected wraps every reason a signed query is refused; the detail is for the serving process's log, not for Marmot.
var ErrRejected = errors.New("query rejected")

// Verify checks a signed query for op and returns it. The signature is checked before the bytes are parsed, so nothing unauthenticated is ever interpreted.
func (v *Verifier) Verify(signed *pb.SignedQuery, op pb.Operation) (*pb.Query, error) {
	if signed == nil || len(signed.Query) == 0 || len(signed.Query) > maxEnvelopeBytes {
		return nil, fmt.Errorf("%w: malformed", ErrRejected)
	}
	if !v.signedByPinnedKey(signed) {
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

	if !v.remember(string(q.Nonce), expires.Add(envelopeSkew), now) {
		return nil, fmt.Errorf("%w: replayed", ErrRejected)
	}
	return &q, nil
}

func (v *Verifier) signedByPinnedKey(signed *pb.SignedQuery) bool {
	digest := sha256.Sum256(signed.Query)
	for _, key := range v.signingKeys {
		if ecdsa.VerifyASN1(key, digest[:], signed.Signature) {
			return true
		}
	}
	return false
}

// remember records a nonce until forgetAt and reports whether it was new. Expired nonces are dropped as it goes, so the cache holds at most one minute or so of queries.
func (v *Verifier) remember(nonce string, forgetAt, now time.Time) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	for n, t := range v.seen {
		if now.After(t) {
			delete(v.seen, n)
		}
	}
	if _, ok := v.seen[nonce]; ok {
		return false
	}
	v.seen[nonce] = forgetAt
	return true
}
