package pluginserve

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gproto "google.golang.org/protobuf/proto"

	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

const testAudience = "spiffe://instance/serve/enrolment"

func testIdentity(t *testing.T) *Keyring {
	t.Helper()
	return newTestKeyring(t)
}

func testBinding() []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func signedPlan(t *testing.T, id *Keyring, now time.Time) *pb.SignedQuery {
	t.Helper()
	signed, err := id.signQuery(&pb.Query{Operation: pb.Operation_OPERATION_PLAN, Source: "warehouse", Statement: "SELECT 1", MaxRows: 10, Actor: "user:alice"}, testAudience, testBinding(), now)
	require.NoError(t, err)
	return signed
}

func verifierFor(id *Keyring) *Verifier {
	return NewVerifier([]*ecdsa.PublicKey{id.gens[0].signingPub}, id.InstanceID, testAudience, testBinding())
}

func TestVerifyAcceptsSignedQuery(t *testing.T) {
	id := testIdentity(t)
	q, err := verifierFor(id).Verify(signedPlan(t, id, time.Now()), pb.Operation_OPERATION_PLAN)
	require.NoError(t, err)
	assert.Equal(t, "SELECT 1", q.Statement)
	assert.Equal(t, "user:alice", q.Actor)
}

func TestVerifyRejects(t *testing.T) {
	id := testIdentity(t)
	now := time.Now()

	// resign re-serialises a tampered query under id's key, to show each check holds even for validly signed bytes.
	resign := func(mutate func(*pb.Query)) *pb.SignedQuery {
		var q pb.Query
		require.NoError(t, gproto.Unmarshal(signedPlan(t, id, now).Query, &q))
		mutate(&q)
		data, err := gproto.Marshal(&q)
		require.NoError(t, err)
		signature, err := id.Sign(data)
		require.NoError(t, err)
		return &pb.SignedQuery{Query: data, Signature: signature}
	}

	tests := []struct {
		name   string
		signed *pb.SignedQuery
		op     pb.Operation
	}{
		{"nil", nil, pb.Operation_OPERATION_PLAN},
		{"empty", &pb.SignedQuery{}, pb.Operation_OPERATION_PLAN},
		{"oversized", &pb.SignedQuery{Query: make([]byte, maxEnvelopeBytes+1)}, pb.Operation_OPERATION_PLAN},
		{"garbage signed with a valid key", func() *pb.SignedQuery {
			data := []byte{0xff, 0xff, 0xff}
			signature, err := id.Sign(data)
			require.NoError(t, err)
			return &pb.SignedQuery{Query: data, Signature: signature}
		}(), pb.Operation_OPERATION_PLAN},
		{"tampered statement", func() *pb.SignedQuery {
			s := signedPlan(t, id, now)
			var q pb.Query
			require.NoError(t, gproto.Unmarshal(s.Query, &q))
			q.Statement = "DROP TABLE orders"
			s.Query, _ = gproto.Marshal(&q)
			return s
		}(), pb.Operation_OPERATION_PLAN},
		{"signed by another key", signedPlan(t, testIdentity(t), now), pb.Operation_OPERATION_PLAN},
		{"plan used as execute", signedPlan(t, id, now), pb.Operation_OPERATION_EXECUTE},
		{"wrong issuer", resign(func(q *pb.Query) { q.Issuer = "other-instance" }), pb.Operation_OPERATION_PLAN},
		{"wrong audience", resign(func(q *pb.Query) { q.Audience = "spiffe://instance/serve/someone-else" }), pb.Operation_OPERATION_PLAN},
		{"bound to another connection", resign(func(q *pb.Query) { q.ChannelBinding = make([]byte, 32) }), pb.Operation_OPERATION_PLAN},
		{"no channel binding", resign(func(q *pb.Query) { q.ChannelBinding = nil }), pb.Operation_OPERATION_PLAN},
		{"unknown version", resign(func(q *pb.Query) { q.Version = 2 }), pb.Operation_OPERATION_PLAN},
		{"lifetime too long", resign(func(q *pb.Query) { q.ExpiresAtUnix = q.IssuedAtUnix + 3600 }), pb.Operation_OPERATION_PLAN},
		{"expires before issued", resign(func(q *pb.Query) { q.ExpiresAtUnix = q.IssuedAtUnix - 1 }), pb.Operation_OPERATION_PLAN},
		{"short nonce", resign(func(q *pb.Query) { q.Nonce = []byte{1} }), pb.Operation_OPERATION_PLAN},
		{"expired", signedPlan(t, id, now.Add(-5*time.Minute)), pb.Operation_OPERATION_PLAN},
		{"issued in the future", signedPlan(t, id, now.Add(5*time.Minute)), pb.Operation_OPERATION_PLAN},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := verifierFor(id).Verify(tt.signed, tt.op)
			assert.ErrorIs(t, err, ErrRejected)
		})
	}
}

func TestVerifyRejectsReplay(t *testing.T) {
	id := testIdentity(t)
	v := verifierFor(id)
	signed := signedPlan(t, id, time.Now())

	_, err := v.Verify(signed, pb.Operation_OPERATION_PLAN)
	require.NoError(t, err)
	_, err = v.Verify(signed, pb.Operation_OPERATION_PLAN)
	assert.ErrorIs(t, err, ErrRejected)
}

func TestVerifyToleratesSmallClockSkew(t *testing.T) {
	id := testIdentity(t)
	_, err := verifierFor(id).Verify(signedPlan(t, id, time.Now().Add(20*time.Second)), pb.Operation_OPERATION_PLAN)
	assert.NoError(t, err)
}

func TestVerifierForgetsExpiredNonces(t *testing.T) {
	id := testIdentity(t)
	v := verifierFor(id)
	start := time.Now()
	for i := 0; i < 50; i++ {
		_, err := v.Verify(signedPlan(t, id, start), pb.Operation_OPERATION_PLAN)
		require.NoError(t, err)
	}

	v.now = func() time.Time { return start.Add(envelopeLifetime + envelopeSkew + time.Second) }
	_, err := v.Verify(signedPlan(t, id, v.now()), pb.Operation_OPERATION_PLAN)
	require.NoError(t, err)
	assert.Len(t, v.seen, 1, "nonces past their query's expiry are dropped")
}

// While Marmot rotates keys a connection pins both, and a query signed by either verifies.
func TestVerifyAcceptsAnyPinnedKey(t *testing.T) {
	old, next := testIdentity(t), testIdentity(t)
	next.InstanceID = old.InstanceID
	v := NewVerifier([]*ecdsa.PublicKey{old.gens[0].signingPub, next.gens[0].signingPub}, old.InstanceID, testAudience, testBinding())

	_, err := v.Verify(signedPlan(t, old, time.Now()), pb.Operation_OPERATION_PLAN)
	assert.NoError(t, err)
	_, err = v.Verify(signedPlan(t, next, time.Now()), pb.Operation_OPERATION_PLAN)
	assert.NoError(t, err)

	stranger := testIdentity(t)
	stranger.InstanceID = old.InstanceID
	_, err = v.Verify(signedPlan(t, stranger, time.Now()), pb.Operation_OPERATION_PLAN)
	assert.ErrorIs(t, err, ErrRejected)
}
