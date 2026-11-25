package pluginserve

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gproto "google.golang.org/protobuf/proto"

	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

// testApprover is a person as Marmot names them in an approval.
type testApprover struct{ id, name string }

var (
	alice = testApprover{id: "6f1c1c1e-0000-4000-8000-00000000a11c", name: "alice"}
	bob   = testApprover{id: "6f1c1c1e-0000-4000-8000-0000000000b0", name: "bob"}
	carol = testApprover{id: "6f1c1c1e-0000-4000-8000-00000000ca01", name: "carol"}
)

// approvalsYAML is the approvals section the test policies use.
func approvalsYAML() string {
	return "approvals:\n  trust: marmot\n  max_age: 15m\n"
}

func approversOf(people []testApprover) []Approver {
	out := make([]Approver, 0, len(people))
	for _, p := range people {
		out = append(out, Approver{UserID: p.id, Name: p.name, ApprovedAt: time.Now().Add(-time.Minute).Truncate(time.Second)})
	}
	return out
}

// approve has Marmot approve call as it will be sent, on behalf of people, expiring in ttl, signed by hub's approval key as the tasks service does.
func approve(t *testing.T, hub *Hub, call ActionCall, ttl time.Duration, people ...testApprover) *Approval {
	t.Helper()
	a := &Approval{ExpiresAt: time.Now().Add(ttl), Approvers: approversOf(people)}
	require.NoError(t, hub.SignApproval(call, a))
	return a
}

// signWith signs an approval with any key, skipping the hub's own checks, for the refusal tests: what a forger or a buggy Marmot could send.
func signWith(t *testing.T, key *ecdsa.PrivateKey, subject ApprovalSubject, expires time.Time, approvers []Approver) []byte {
	t.Helper()
	subject.ExpiresAt = expires
	digest, err := ApprovalDigest(subject)
	require.NoError(t, err)
	h := sha256.Sum256(ApprovalSignedMessage(digest, approvers))
	sig, err := ecdsa.SignASN1(rand.Reader, key, h[:])
	require.NoError(t, err)
	msg := &pb.Approval{Version: ApprovalDigestVersion, ExpiresAtUnix: expires.Unix(), Signature: sig}
	for _, a := range approvers {
		msg.Approvers = append(msg.Approvers, &pb.Approver{UserId: a.UserID, Name: a.Name, ApprovedAtUnix: a.ApprovedAt.Unix()})
	}
	data, err := gproto.Marshal(msg)
	require.NoError(t, err)
	return data
}

func newP256(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return k
}

func loadApprovalsPolicy(t *testing.T, body string) *Policy {
	t.Helper()
	p, err := LoadPolicy(writePolicy(t, body))
	require.NoError(t, err)
	return p
}

func testSubject() ApprovalSubject {
	return ApprovalSubject{
		Source: "prod", Action: "scale", Params: []byte(`{"replicas":3,"namespace":"payments"}`), Precondition: "41",
		ManifestDigest: "sha256:" + strings.Repeat("a", 64), Origin: "task_run:7", Actor: "task:7", ActionID: "id-1",
	}
}

func TestApprovalDigestIsCanonicalAndCoversEveryField(t *testing.T) {
	s := testSubject()
	s.ExpiresAt = time.Unix(1_900_000_000, 0)
	base, err := ApprovalDigest(s)
	require.NoError(t, err)

	same := s
	same.Params = []byte(` { "namespace" : "payments", "replicas" : 3 } `)
	got, err := ApprovalDigest(same)
	require.NoError(t, err)
	assert.Equal(t, base, got, "the same parameters in any encoding approve the same thing")

	for name, mutate := range map[string]func(*ApprovalSubject){
		"source":       func(s *ApprovalSubject) { s.Source = "staging" },
		"action":       func(s *ApprovalSubject) { s.Action = "rollout_restart" },
		"params":       func(s *ApprovalSubject) { s.Params = []byte(`{"replicas":4,"namespace":"payments"}`) },
		"precondition": func(s *ApprovalSubject) { s.Precondition = "42" },
		"manifest":     func(s *ApprovalSubject) { s.ManifestDigest = "sha256:" + strings.Repeat("b", 64) },
		"origin":       func(s *ApprovalSubject) { s.Origin = "task_run:8" },
		"actor":        func(s *ApprovalSubject) { s.Actor = "user:mallory" },
		"action id":    func(s *ApprovalSubject) { s.ActionID = "id-2" },
		"expiry":       func(s *ApprovalSubject) { s.ExpiresAt = s.ExpiresAt.Add(time.Second) },
		// Length prefixes keep bytes from moving between fields unnoticed.
		"a boundary": func(s *ApprovalSubject) { s.Source, s.Action = "prods", "cale" },
	} {
		changed := s
		mutate(&changed)
		d, err := ApprovalDigest(changed)
		require.NoError(t, err)
		assert.NotEqual(t, base, d, name)
	}

	for _, params := range []string{`{"a":1.5}`, `{"a":{"b":1}}`, `{"a":null}`, `[1]`, `{"a":1} {}`} {
		bad := s
		bad.Params = []byte(params)
		_, err := ApprovalDigest(bad)
		assert.Error(t, err, params)
	}
}

func TestVerifyApproval(t *testing.T) {
	p := loadApprovalsPolicy(t, approvalsYAML()+"sources: {}\n")
	ap := p.Approvals
	subject := testSubject()
	now := time.Now()
	expires := now.Add(10 * time.Minute)
	marmot, other := newP256(t), newP256(t)
	keys := []*ecdsa.PublicKey{&marmot.PublicKey}
	one := approvalNeed{required: 1}
	signed := func(people ...testApprover) []byte { return signWith(t, marmot, subject, expires, approversOf(people)) }

	t.Run("a valid approval", func(t *testing.T) {
		v, err := ap.verifyApproval(signed(alice), subject, one, keys, now)
		require.NoError(t, err)
		assert.Equal(t, []string{"alice (" + alice.id + ")"}, v.names())
		s := subject
		s.ExpiresAt = time.Unix(expires.Unix(), 0)
		d, _ := ApprovalDigest(s)
		assert.Equal(t, d, v.digest)
	})
	t.Run("two distinct people", func(t *testing.T) {
		v, err := ap.verifyApproval(signed(alice, bob), subject, approvalNeed{required: 2}, keys, now)
		require.NoError(t, err)
		assert.Len(t, v.approvers, 2)
	})
	t.Run("narrowed approvers, by id or by name", func(t *testing.T) {
		_, err := ap.verifyApproval(signed(alice, bob), subject, approvalNeed{required: 2, approvers: []string{alice.id, "bob"}}, keys, now)
		require.NoError(t, err)
	})
	t.Run("a pinned key fingerprint", func(t *testing.T) {
		pinned := *ap
		pinned.KeyFingerprint = ApprovalKeyFingerprint(&marmot.PublicKey)
		_, err := pinned.verifyApproval(signed(alice), subject, one, []*ecdsa.PublicKey{&other.PublicKey, &marmot.PublicKey}, now)
		require.NoError(t, err)
		pinned.KeyFingerprint = ApprovalKeyFingerprint(&other.PublicKey)
		_, err = pinned.verifyApproval(signed(alice), subject, one, keys, now)
		assert.ErrorContains(t, err, "no approval key is pinned that the policy accepts")
	})

	another := subject
	another.Params = []byte(`{"replicas":50,"namespace":"payments"}`)
	tampered := func() []byte {
		var msg pb.Approval
		require.NoError(t, gproto.Unmarshal(signed(alice), &msg))
		msg.Approvers[0].UserId = carol.id
		data, _ := gproto.Marshal(&msg)
		return data
	}()
	refused := map[string]struct {
		data     []byte
		need     approvalNeed
		subject  *ApprovalSubject
		keys     []*ecdsa.PublicKey
		contains string
	}{
		"expired":        {data: signWith(t, marmot, subject, now.Add(-time.Second), approversOf([]testApprover{alice})), contains: "expired"},
		"beyond max_age": {data: signWith(t, marmot, subject, now.Add(time.Hour), approversOf([]testApprover{alice})), contains: "max_age"},
		"the wrong key":  {data: signWith(t, other, subject, expires, approversOf([]testApprover{alice})), contains: "not signed by Marmot's approval key"},
		// The query signing key, or any other, is not the approval key even if Marmot holds it.
		"no approval key pinned": {data: signed(alice), keys: []*ecdsa.PublicKey{}, contains: "no approval key is pinned"},
		"another action":         {data: signed(alice), subject: &another, contains: "not signed by Marmot's approval key for this action"},
		"an approver swapped":    {data: tampered, contains: "not signed by Marmot's approval key"},
		"too few distinct approvers": {
			data: signed(alice), need: approvalNeed{required: 2}, contains: "needs 2 distinct approvers the policy admits and the approval has 1",
		},
		"the same person twice": {
			data: signWith(t, marmot, subject, expires, []Approver{{UserID: alice.id, Name: "alice", ApprovedAt: now}, {UserID: alice.id, Name: "alice again", ApprovedAt: now}}),
			need: approvalNeed{required: 2}, contains: "named twice",
		},
		"not among the action's approvers": {data: signed(alice), need: approvalNeed{required: 1, approvers: []string{"bob"}}, contains: "the approval has 0"},
		"approved after it expired": {
			data: signWith(t, marmot, subject, expires, []Approver{{UserID: alice.id, Name: "alice", ApprovedAt: expires.Add(time.Minute)}}), contains: "after the approval expired",
		},
		"none at all": {data: nil, contains: "carries none"},
		"a bad signature": {data: func() []byte {
			var msg pb.Approval
			require.NoError(t, gproto.Unmarshal(signed(alice), &msg))
			msg.Signature[len(msg.Signature)-1] ^= 1
			data, _ := gproto.Marshal(&msg)
			return data
		}(), contains: "not signed"},
	}
	for name, tc := range refused {
		t.Run("refuses "+name, func(t *testing.T) {
			need := tc.need
			if need.required == 0 && need.approvers == nil {
				need = one
			}
			s := subject
			if tc.subject != nil {
				s = *tc.subject
			}
			k := keys
			if tc.keys != nil {
				k = tc.keys
			}
			_, err := ap.verifyApproval(tc.data, s, need, k, now)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.contains)
		})
	}

	t.Run("refuses with no approvals section", func(t *testing.T) {
		var none *ApprovalsPolicy
		_, err := none.verifyApproval(signed(alice), subject, one, keys, now)
		assert.ErrorContains(t, err, "no approvers configured")
	})

	t.Run("refuses an unparseable or versioned approval", func(t *testing.T) {
		_, err := ap.verifyApproval([]byte{0xff, 0xff}, subject, one, keys, now)
		assert.Error(t, err)
		data, _ := gproto.Marshal(&pb.Approval{Version: 2, ExpiresAtUnix: expires.Unix()})
		_, err = ap.verifyApproval(data, subject, one, keys, now)
		assert.ErrorContains(t, err, "version")
	})
}

func TestApprovalSignatureIsDomainSeparated(t *testing.T) {
	d := make([]byte, 32)
	msg := ApprovalSignedMessage(d, []Approver{{UserID: "a", Name: "b", ApprovedAt: time.Unix(1, 0)}})
	assert.True(t, bytes.HasPrefix(msg, approvalSignatureDomain))
	// Length prefixes keep bytes from moving between an id and a name unnoticed.
	assert.NotEqual(t, msg, ApprovalSignedMessage(d, []Approver{{UserID: "ab", Name: "", ApprovedAt: time.Unix(1, 0)}}))
}

func TestApprovalsPolicyLoading(t *testing.T) {
	for name, tc := range map[string]struct {
		body, contains string
	}{
		"no trust":                    {"approvals:\n  max_age: 15m\n", "trust is required"},
		"oidc, planned":               {"approvals:\n  trust: oidc\n", "planned"},
		"an unknown trust":            {"approvals:\n  trust: passkeys\n", "only value accepted today is marmot"},
		"max_age above an hour":       {"approvals:\n  trust: marmot\n  max_age: 2h\n", "max_age"},
		"a bad key fingerprint":       {"approvals:\n  trust: marmot\n  key_fingerprint: abc\n", "key_fingerprint"},
		"an unknown key":              {approvalsYAML() + "  extra: 1\n", "extra"},
		"approvals above two":         {approvalsYAML() + "sources:\n  s:\n    actions:\n      scale: {approvals: 3}\n", "0, 1 or 2"},
		"more approvals than people":  {approvalsYAML() + "sources:\n  s:\n    actions:\n      scale: {approvals: 2, approvers: [alice]}\n", "only 1 may approve"},
		"approvers without a section": {"sources:\n  s:\n    actions:\n      scale: {approvers: [alice]}\n", "no approvals section"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadPolicy(writePolicy(t, tc.body))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.contains)
		})
	}

	p, err := LoadPolicy(writePolicy(t, "approvals:\n  trust: marmot\n  key_fingerprint: SHA256:"+strings.Repeat("AB", 32)+"\n"))
	require.NoError(t, err)
	assert.Equal(t, defaultApprovalAge, p.Approvals.MaxAge)
	assert.Equal(t, "sha256:"+strings.Repeat("ab", 32), p.Approvals.KeyFingerprint)
	assert.Equal(t, "Marmot-signed", p.Approvals.Describe())
}

func TestApprovalsAgainstTheManifest(t *testing.T) {
	specs := append(testActionManifest(), testReadSpec())
	check := func(body string) error {
		p := loadApprovalsPolicy(t, approvalsYAML()+body)
		return p.CheckManifest("s", specs)
	}
	assert.NoError(t, check("sources:\n  s:\n    actions:\n      scale: {approvals: 0}\n"), "scale is risk 1 and reversible here")
	assert.ErrorContains(t, check("sources:\n  s:\n    actions:\n      rollout_restart: {approvals: 0}\n"), "only allowed for a risk 1, reversible action")
	assert.ErrorContains(t, check("sources:\n  s:\n    actions:\n      read_status: {approvals: 1}\n"), "reads take no approvals")
	assert.NoError(t, check("sources:\n  s:\n    actions:\n      read_status: {}\n"))
}

// A spent approval stays spent across a restart: the intent line carries its digest and the process reads it back.
func TestSpentApprovalsSurviveARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	log1, err := OpenAuditLog(path)
	require.NoError(t, err)
	digest := []byte("0123456789abcdef0123456789abcdef")
	expires := time.Now().Add(10 * time.Minute)
	require.NoError(t, log1.Append(withApproval(AuditEntry{Event: AuditActionIntent, Time: time.Now().UTC()}, &verifiedApproval{digest: digest, expiresAt: expires, approvers: []*pb.Approver{{UserId: alice.id, Name: "alice"}}})))
	// Lines enough to push the intent back across a read chunk.
	for range 2000 {
		require.NoError(t, log1.Append(AuditEntry{Event: AuditPlan, Time: time.Now().UTC(), Reason: strings.Repeat("x", 40)}))
	}
	require.NoError(t, log1.Close())

	log2, err := OpenAuditLog(path)
	require.NoError(t, err)
	defer func() { _ = log2.Close() }()
	entries, err := log2.Since(time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Len(t, entries, 2001)
	spent := newSpentApprovals()
	assert.Equal(t, 1, spent.load(entries, time.Now()))
	assert.False(t, spent.claim(digest, expires, time.Now()), "the approval spent before the restart is refused after it")
	assert.True(t, spent.claim([]byte("another digest of thirty-two by"), expires, time.Now()))

	old, err := log2.Since(time.Now().Add(time.Minute))
	require.NoError(t, err)
	assert.Empty(t, old, "reading stops at the first line older than asked for")
}
