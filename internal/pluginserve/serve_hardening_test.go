package pluginserve

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	gproto "google.golang.org/protobuf/proto"

	"github.com/marmotdata/marmot/internal/core/dataquery"
	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

func receiptOf(t *testing.T, stream pluginsdk.QueryStream) *dataquery.RunnerReceipt {
	t.Helper()
	rs, ok := stream.(dataquery.ReceiptStream)
	require.True(t, ok, "a served stream carries a receipt")
	return rs.Receipt()
}

func TestServeRefusesAMarmotWhoseCAFingerprintDiffers(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal := servePrincipal()
	source := newSourceName()
	opts := h.serveOptions(h.enroller(hub), principal, sessionCredential, source, &fakeQuerier{rows: 1})
	opts.CAFingerprints = []string{CAFingerprint([]byte("not this instance's CA"))}

	ctx, cancel := context.WithTimeout(h.ctx, 10*time.Second)
	defer cancel()
	err := Serve(ctx, opts)
	assert.ErrorIs(t, err, ErrCAFingerprintMismatch, "the process gives up rather than trusting an unexpected CA")
	assert.Nil(t, h.sessionFor(hub, principal, source))
}

func TestServeAcceptsAMarmotWhoseCAFingerprintMatches(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	source := newSourceName()
	fingerprint := h.keyring.Generations()[0].CAFingerprint
	h.serveWith(h.enroller(hub), hub, servePrincipal(), sessionCredential, source, &fakeQuerier{rows: 1}, func(o *ServeOptions) {
		o.CAFingerprints = []string{CAFingerprint([]byte("an old CA")), fingerprint}
	})
	assert.NoError(t, h.planWorks(hub, source))
}

func TestServePolicy(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	source := newSourceName()
	policy, err := LoadPolicy(writePolicy(t, `
sources:
  `+source+`:
    max_rows: 5
    actors: ["user:alice", "task:*"]
`))
	require.NoError(t, err)
	querier := &fakeQuerier{rows: 100}
	h.serveWith(h.enroller(hub), hub, servePrincipal(), sessionCredential, source, querier, func(o *ServeOptions) { o.Policy = policy })

	t.Run("lowers the signed row limit", func(t *testing.T) {
		stream, err := hub.ExecuteQuery(h.ctx, source, "", "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 50, Identity: "user:alice"})
		require.NoError(t, err)
		assert.Equal(t, 5, collect(t, stream))
		assert.Equal(t, int64(5), receiptOf(t, stream).MaxRows, "the receipt records the limit the process applied")
	})

	t.Run("allows an actor matching a glob", func(t *testing.T) {
		_, err := hub.PlanQuery(h.ctx, source, "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 1, Identity: "task:nightly"})
		assert.NoError(t, err)
	})

	t.Run("refuses an actor outside the allow-list, for plan and execute", func(t *testing.T) {
		_, err := hub.PlanQuery(h.ctx, source, "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 1, Identity: "user:mallory"})
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
		assert.ErrorContains(t, err, "local policy policy.yaml")

		stream, err := hub.ExecuteQuery(h.ctx, source, "", "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 1, Identity: "user:mallory"})
		if err == nil {
			_, err = stream.Recv()
		}
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})
}

// Once a policy exists, a source it does not name is refused even though the process serves it.
func TestServePolicyDeniesSourcesItOmits(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	source := newSourceName()
	policy, err := LoadPolicy(writePolicy(t, "sources:\n  some-other-source: {}\n"))
	require.NoError(t, err)
	querier := &fakeQuerier{rows: 1}
	h.serveWith(h.enroller(hub), hub, servePrincipal(), sessionCredential, source, querier, func(o *ServeOptions) { o.Policy = policy })

	err = h.planWorks(hub, source)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.ErrorContains(t, err, "is not in the policy")
	querier.mu.Lock()
	assert.Empty(t, querier.reqs, "a refused query never reaches the plugin")
	querier.mu.Unlock()
}

func TestServeReceiptsVerifyEndToEnd(t *testing.T) {
	h := newHarness(t)
	connected, other := h.startHub(), h.startHub()
	principal := servePrincipal()
	source := newSourceName()
	h.serve(connected, principal, source, &fakeQuerier{rows: 25})
	enrolment := h.sessionFor(connected, principal, source).enrolment.ID

	for name, hub := range map[string]*Hub{"from the replica holding the session": connected, "through a forwarding replica": other} {
		t.Run(name, func(t *testing.T) {
			queryID := uuid.NewString()
			stream, err := hub.ExecuteQuery(h.ctx, source, queryID, "task_run:42", pluginsdk.QueryRequest{Statement: "SELECT * FROM orders", MaxRows: 100, Identity: "user:alice"})
			require.NoError(t, err)
			assert.Equal(t, 25, collect(t, stream), "the receipt is not handed out as rows")

			r := receiptOf(t, stream)
			require.NotNil(t, r)
			assert.Equal(t, dataquery.ReceiptVerified, r.Status, r.Problem)
			assert.Equal(t, queryID, r.QueryID)
			assert.Equal(t, "task_run:42", r.Origin)
			assert.Equal(t, "user:alice", r.Actor)
			assert.Equal(t, int64(25), r.Rows)
			assert.Equal(t, enrolment, r.Enrolment)
			assert.NotEmpty(t, r.Certificate)
		})
	}
}

func TestServeReceiptOfAFailedQuery(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	source := newSourceName()
	h.serve(hub, servePrincipal(), source, &failingQuerier{})

	stream, err := hub.ExecuteQuery(h.ctx, source, uuid.NewString(), "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 10})
	require.NoError(t, err)
	_, err = stream.Recv()
	assert.ErrorContains(t, err, "relation does not exist")
	r := receiptOf(t, stream)
	require.NotNil(t, r)
	assert.Equal(t, dataquery.ReceiptVerified, r.Status, r.Problem)
	assert.Contains(t, r.Error, "relation does not exist")
}

// A receipt changed on its way to Marmot is kept and marked invalid, never dropped or taken as verified.
func TestServeTamperedReceiptIsRecordedInvalid(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	source := newSourceName()
	h.serve(hub, servePrincipal(), source, &fakeQuerier{rows: 3})

	for name, tamper := range map[string]func(*pb.SignedReceipt){
		"rows changed": func(r *pb.SignedReceipt) {
			var receipt pb.Receipt
			require.NoError(t, gproto.Unmarshal(r.Receipt, &receipt))
			receipt.Rows = 1000
			data, err := gproto.Marshal(&receipt)
			require.NoError(t, err)
			r.Receipt = data
		},
		"signature altered":           func(r *pb.SignedReceipt) { r.Signature[len(r.Signature)-1] ^= 0xff },
		"certificate from another CA": func(r *pb.SignedReceipt) { r.Certificate = h.rogueServeCert() },
	} {
		t.Run(name, func(t *testing.T) {
			stream, err := hub.ExecuteQuery(h.ctx, source, uuid.NewString(), "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 10})
			require.NoError(t, err)
			rs := stream.(*resultStream)
			rs.stream = &tamperingStream{ServerStreamingClient: rs.stream, tamper: tamper}
			assert.Equal(t, 3, collect(t, stream))
			r := receiptOf(t, stream)
			require.NotNil(t, r)
			assert.Equal(t, dataquery.ReceiptInvalid, r.Status)
			assert.NotEmpty(t, r.Problem)
		})
	}
}

// A serving process sends a receipt only when Marmot asks for one, so a Marmot that predates receipts never reads one as rows.
func TestServeSendsNoReceiptUnasked(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	principal := servePrincipal()
	source := newSourceName()
	h.serve(hub, principal, source, &fakeQuerier{rows: 2})
	s := h.sessionFor(hub, principal, source)

	fq, err := forwarded(pb.Operation_OPERATION_EXECUTE, source, "", "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 10})
	require.NoError(t, err)
	signed, err := h.keyring.signQuery(&pb.Query{Operation: fq.Operation, Source: fq.Source, Statement: fq.Statement, MaxRows: fq.MaxRows}, s.audience, s.binding, time.Now())
	require.NoError(t, err)
	stream, err := s.client.ExecuteQuery(h.ctx, signed)
	require.NoError(t, err)
	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		assert.Nil(t, chunk.Receipt)
	}
}

func TestServeAuditLog(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	source := newSourceName()
	path := filepath.Join(t.TempDir(), "audit.log")
	auditLog, err := OpenAuditLog(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = auditLog.Close() })
	policy, err := LoadPolicy(writePolicy(t, "sources:\n  "+source+":\n    actors: [\"user:alice\"]\n"))
	require.NoError(t, err)
	h.serveWith(h.enroller(hub), hub, servePrincipal(), sessionCredential, source, &fakeQuerier{rows: 4}, func(o *ServeOptions) {
		o.Policy, o.AuditLog = policy, auditLog
	})

	_, err = hub.PlanQuery(h.ctx, source, "task_run:1", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 10, Identity: "user:alice"})
	require.NoError(t, err)
	queryID := uuid.NewString()
	stream, err := hub.ExecuteQuery(h.ctx, source, queryID, "task_run:1", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 10, Identity: "user:alice"})
	require.NoError(t, err)
	collect(t, stream)
	_, err = hub.PlanQuery(h.ctx, source, "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 10, Identity: "user:mallory"})
	require.Error(t, err)

	n, err := VerifyAuditLog(path)
	require.NoError(t, err)
	require.Equal(t, 3, n)

	entries := readAuditLog(t, path)
	assert.Equal(t, AuditPlan, entries[0].Event)
	assert.Equal(t, "task_run:1", entries[0].Origin)
	assert.Equal(t, AuditExecute, entries[1].Event)
	assert.Equal(t, queryID, entries[1].QueryID)
	assert.Equal(t, int64(4), entries[1].Rows)
	assert.NotEmpty(t, entries[1].ReceiptSignature)
	assert.NotEmpty(t, entries[1].StatementSHA256)
	assert.Equal(t, AuditRefused, entries[2].Event)
	assert.Equal(t, "refused", entries[2].Policy)
	assert.Equal(t, "user:mallory", entries[2].Actor)
}

// Forwards between replicas must be signed for the replica receiving them, unexpired and used once, even from a peer holding a valid replica certificate.
func TestReplicaRefusesForwardsThatAreUnsignedExpiredOrReplayed(t *testing.T) {
	h := newHarness(t)
	connected, other := h.startHub(), h.startHub()
	source := newSourceName()
	h.serve(connected, servePrincipal(), source, &fakeQuerier{rows: 1})

	conn, err := grpc.NewClient(connected.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(other.clientTLS())))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	replica := pb.NewReplicaClient(conn)

	sign := func(audience string, now time.Time) *pb.SignedForward {
		fq, err := forwarded(pb.Operation_OPERATION_PLAN, source, "", "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 1})
		require.NoError(t, err)
		signed, err := h.keyring.signForward(fq, audience, now)
		require.NoError(t, err)
		return signed
	}
	audience := h.keyring.replicaID(connected.replicaID)

	valid := sign(audience, time.Now())
	_, err = replica.PlanQuery(h.ctx, valid)
	require.NoError(t, err, "control: a signed forward from a replica is answered")

	for name, signed := range map[string]*pb.SignedForward{
		"unsigned":            {Forward: valid.Forward},
		"expired":             sign(audience, time.Now().Add(-2*time.Minute)),
		"replayed":            valid,
		"for another replica": sign(h.keyring.replicaID(other.replicaID), time.Now()),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := replica.PlanQuery(h.ctx, signed)
			assert.Equal(t, codes.PermissionDenied, status.Code(err))
		})
	}

	t.Run("plan used as execute", func(t *testing.T) {
		stream, err := replica.ExecuteQuery(h.ctx, sign(audience, time.Now()))
		if err == nil {
			_, err = stream.Recv()
		}
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})
}

type failingQuerier struct{ fakeQuerier }

func (f *failingQuerier) ExecuteQuery(context.Context, string, map[string]any, pluginsdk.QueryRequest) (pluginsdk.QueryStream, error) {
	return nil, errors.New("relation does not exist")
}

// tamperingStream stands in for something between the serving process and the replica that reads its stream, changing the receipt on the way.
type tamperingStream struct {
	grpc.ServerStreamingClient[pb.QueryChunk]
	tamper func(*pb.SignedReceipt)
}

func (s *tamperingStream) Recv() (*pb.QueryChunk, error) {
	chunk, err := s.ServerStreamingClient.Recv()
	if err == nil && chunk.Receipt != nil {
		s.tamper(chunk.Receipt)
	}
	return chunk, err
}

// rogueServeCert issues a serving certificate naming this instance from another CA, for a receipt that claims to be this instance's but chains elsewhere.
func (h *harness) rogueServeCert() []byte {
	h.t.Helper()
	rogue := newTestKeyring(h.t)
	rogue.InstanceID = h.keyring.InstanceID
	csr, err := ParseCSR(enrolInput(h.t, nil, "x").CSR)
	require.NoError(h.t, err)
	der, err := rogue.IssueServeCert(csr, uuid.NewString(), time.Now().Add(time.Hour))
	require.NoError(h.t, err)
	return der
}

func readAuditLog(t *testing.T, path string) []AuditEntry {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	var entries []AuditEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e AuditEntry
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &e))
		entries = append(entries, e)
	}
	return entries
}
