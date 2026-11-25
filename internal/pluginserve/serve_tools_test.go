package pluginserve

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/dataquery"
	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

// testToolManifest mirrors the shape of the Kubernetes plugin's scale and rollout_restart, which the policy example in the docs is written against.
func testToolManifest() []pluginsdk.ToolSpec {
	namespace := pluginsdk.ToolParam{Name: "namespace", Description: "Namespace", Type: pluginsdk.ParamString, Required: true, MaxLength: 63}
	return []pluginsdk.ToolSpec{
		{
			Name: "scale", Version: 1, Title: "Scale", Description: "Sets a deployment's replicas",
			Targets:       []pluginsdk.TargetKind{"test/deployment"},
			Reversibility: pluginsdk.ReversibilityReversible, Risk: pluginsdk.RiskBounded, Inverse: "scale",
			Params: []pluginsdk.ToolParam{
				namespace,
				{Name: "replicas", Description: "Replicas", Type: pluginsdk.ParamInt, Required: true, Min: 0, Max: 100},
			},
		},
		{
			Name: "rollout_restart", Version: 1, Title: "Restart", Description: "Restarts a deployment",
			Targets:       []pluginsdk.TargetKind{"test/deployment"},
			Reversibility: pluginsdk.ReversibilitySelfHealing, Risk: pluginsdk.RiskBounded,
			Params: []pluginsdk.ToolParam{namespace},
		},
		testReadSpec(),
	}
}

// testReadSpec is a read tool, as the Kubernetes plugin's workload_status is.
func testReadSpec() pluginsdk.ToolSpec {
	return pluginsdk.ToolSpec{
		Name: "read_status", Version: 1, Title: "Status", Description: "Reads a deployment's status",
		Targets:       []pluginsdk.TargetKind{"test/deployment"},
		Reversibility: pluginsdk.ReversibilityReadOnly, Risk: pluginsdk.RiskRead,
		Params: []pluginsdk.ToolParam{{Name: "namespace", Description: "Namespace", Type: pluginsdk.ParamString, Required: true, MaxLength: 63}},
	}
}

var testManifestDigest = pluginsdk.ManifestDigest(testToolManifest())

// fakePluginTools stands in for plugin.ToolHost over a plugin like the SDK's plugintest/testdata/toolplugin: it keeps a replica count with a version as the precondition, runs the gate exactly where the host does, and checks requests again as the SDK's Serve does inside a real plugin.
type fakePluginTools struct {
	mu       sync.Mutex
	specs    []pluginsdk.ToolSpec
	replicas int64
	version  int
	plans    []pluginsdk.ToolRequest
	executes []pluginsdk.ToolRequest
	// fail makes ExecuteTool fail after the gate, as a plugin whose target moved on would.
	fail error
}

func newFakePluginTools() *fakePluginTools {
	return &fakePluginTools{specs: testToolManifest(), replicas: 2}
}

func (f *fakePluginTools) ToolManifest(context.Context, string) ([]pluginsdk.ToolSpec, error) {
	return f.specs, nil
}

func (f *fakePluginTools) PlanTool(_ context.Context, _ string, _ map[string]any, req pluginsdk.ToolRequest, gate func([]pluginsdk.ToolSpec) error) (*pluginsdk.ToolPlan, error) {
	if err := gate(f.specs); err != nil {
		return nil, err
	}
	if _, err := pluginsdk.CheckToolRequest(f.specs, req, false); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.plans = append(f.plans, req)
	return &pluginsdk.ToolPlan{
		Target: "deployment/" + req.StringParam("namespace") + "/api", TargetKind: "test/deployment",
		Summary:      "scale from " + strconv.FormatInt(f.replicas, 10),
		Precondition: strconv.Itoa(f.version),
	}, nil
}

func (f *fakePluginTools) ExecuteTool(_ context.Context, _ string, _ map[string]any, req pluginsdk.ToolRequest, gate func([]pluginsdk.ToolSpec) error) (*pluginsdk.ToolResult, error) {
	if err := gate(f.specs); err != nil {
		return nil, err
	}
	if _, err := pluginsdk.CheckToolRequest(f.specs, req, true); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.executes = append(f.executes, req)
	if f.fail != nil {
		return nil, f.fail
	}
	if req.Tool == "read_status" {
		return &pluginsdk.ToolResult{
			Target: "deployment/" + req.StringParam("namespace") + "/api", TargetKind: "test/deployment", Summary: "read",
			Output: []byte(`{"replicas":` + strconv.FormatInt(f.replicas, 10) + `,"untrusted":true}`), Untrusted: true,
		}, nil
	}
	if req.Precondition != strconv.Itoa(f.version) {
		return nil, status.Error(codes.FailedPrecondition, "the deployment changed since it was planned")
	}
	previous := f.replicas
	if n, ok := req.IntParam("replicas"); ok {
		f.replicas = n
	}
	f.version++
	return &pluginsdk.ToolResult{
		Target: "deployment/" + req.StringParam("namespace") + "/api", TargetKind: "test/deployment",
		Summary: "scaled",
		Undo:    &pluginsdk.ToolRequest{Tool: "scale", Params: map[string]any{"namespace": req.StringParam("namespace"), "replicas": previous}},
	}, nil
}

func (f *fakePluginTools) calls() (plans, executes int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.plans), len(f.executes)
}

// toolServer is one serving process with tools configured, and what a test needs to drive it.
type toolServer struct {
	source    string
	principal auth.Principal
	runner    *fakePluginTools
	audit     *AuditLog
	auditPath string
}

// serveTools runs a serving process for a fresh source. policy is the policy file's body with SOURCE standing for the source's name, or empty for no policy.
func (h *harness) serveTools(hub *Hub, allow bool, policy string) *toolServer {
	h.t.Helper()
	a := &toolServer{source: newSourceName(), principal: servePrincipal(), runner: newFakePluginTools()}
	a.auditPath = filepath.Join(h.t.TempDir(), "audit.log")
	var err error
	a.audit, err = OpenAuditLog(a.auditPath)
	require.NoError(h.t, err)
	h.t.Cleanup(func() { _ = a.audit.Close() })
	var p *Policy
	if policy != "" {
		p, err = LoadPolicy(writePolicy(h.t, strings.ReplaceAll(policy, "SOURCE", a.source)))
		require.NoError(h.t, err)
	}
	h.serveWith(h.enroller(hub), hub, a.principal, sessionCredential, a.source, &fakeQuerier{rows: 1}, func(o *ServeOptions) {
		o.AllowTools, o.PluginTools, o.Policy, o.AuditLog = allow, a.runner, p, a.audit
	})
	return a
}

func (a *toolServer) call(tool string, params map[string]any) ToolCall {
	return ToolCall{
		Source:         a.source,
		ID:             uuid.NewString(),
		ManifestDigest: testManifestDigest,
		Request:        pluginsdk.ToolRequest{Tool: tool, Params: params, Identity: "user:alice", Origin: "task_run:7"},
	}
}

var scalePolicy = approvalsYAML() + `
sources:
  SOURCE:
    query: false
    tools:
      rollout_restart:
        origins: ["task_run:*"]
      read_status: {}
      scale:
        actors: ["user:alice", "task:*"]
        params:
          namespace: {values: [payments, checkout]}
          replicas: {min: 1, max: 10}
`

func TestServeToolsPlanAndExecuteEndToEnd(t *testing.T) {
	h := newHarness(t)
	connected, other := h.startHub(), h.startHub()
	a := h.serveTools(connected, true, scalePolicy)
	enrolment := h.sessionFor(connected, a.principal, a.source).enrolment.ID

	for name, hub := range map[string]*Hub{"from the replica holding the session": connected, "through a forwarding replica": other} {
		t.Run(name, func(t *testing.T) {
			call := a.call("scale", map[string]any{"namespace": "payments", "replicas": 5})
			plan, receipt, err := hub.PlanTool(h.ctx, call)
			require.NoError(t, err)
			require.NotNil(t, plan)
			assert.Equal(t, "deployment/payments/api", plan.Target)
			require.True(t, receipt.Verified(), receipt.Problem)
			assert.Equal(t, AuditToolPlan, receipt.Operation)
			assert.Equal(t, call.ID, receipt.ID)
			assert.Equal(t, "scale", receipt.Tool)
			assert.Equal(t, testManifestDigest, receipt.ManifestDigest)
			assert.Equal(t, enrolment, receipt.Enrolment)
			assert.NotEmpty(t, receipt.ParamsSHA256)

			call.ID = uuid.NewString()
			call.Request.Precondition = plan.Precondition
			call.Approval = approve(t, hub, call, 5*time.Minute, alice)
			result, receipt, err := hub.ExecuteTool(h.ctx, call)
			require.NoError(t, err)
			require.True(t, receipt.Verified(), receipt.Problem)
			assert.Equal(t, AuditToolExecute, receipt.Operation)
			require.Len(t, receipt.Approvers, 1)
			assert.Equal(t, "alice", receipt.Approvers[0].Name)
			assert.NotEmpty(t, receipt.ApprovalDigest)
			assert.Equal(t, plan.Precondition, receipt.Precondition)
			assert.Equal(t, "deployment/payments/api", receipt.Target)
			assert.Equal(t, "scaled", result.Summary)
			require.NotNil(t, result.Undo)
			assert.Equal(t, "scale", result.Undo.Tool)

			// Executing the same plan again, freshly approved, finds the target moved on: the plugin's failure comes back with its code and a signed receipt.
			call.ID = uuid.NewString()
			call.Approval = approve(t, hub, call, 5*time.Minute, alice)
			_, receipt, err = hub.ExecuteTool(h.ctx, call)
			assert.Equal(t, codes.FailedPrecondition, status.Code(err))
			require.True(t, receipt.Verified(), receipt.Problem)
			assert.Contains(t, receipt.Error, "changed since it was planned")
		})
	}

	_, executes := a.runner.calls()
	require.Equal(t, 4, executes)
	last := a.runner.executes[1]
	assert.Equal(t, "user:alice", last.Identity)
	assert.Equal(t, "task_run:7", last.Origin)
	assert.False(t, last.DryRun)
	assert.True(t, a.runner.plans[0].DryRun)

	n, err := VerifyAuditLog(a.auditPath)
	require.NoError(t, err)
	entries := readAuditLog(t, a.auditPath)
	require.Len(t, entries, n)
	var events []string
	for _, e := range entries[:5] {
		events = append(events, e.Event)
	}
	assert.Equal(t, []string{AuditToolPlan, AuditToolIntent, AuditToolExecute, AuditToolIntent, AuditToolExecute}, events, "every execute's intent is written before its outcome")
	assert.Equal(t, testManifestDigest, entries[1].ManifestDigest)
	assert.NotEmpty(t, entries[2].ReceiptSignature)
	assert.Empty(t, entries[2].StatementSHA256, "a tool has no statement")
}

func TestServeToolsRefusals(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()

	expectRefused := func(t *testing.T, a *toolServer, call ToolCall, contains string) {
		t.Helper()
		plansBefore, executesBefore := a.runner.calls()
		_, receipt, err := hub.PlanTool(h.ctx, call)
		assert.Equal(t, codes.PermissionDenied, status.Code(err), err)
		assert.ErrorContains(t, err, contains)
		assert.Nil(t, receipt, "nothing ran, so there is no receipt")

		call.Request.Precondition = "0"
		_, _, err = hub.ExecuteTool(h.ctx, call)
		assert.Equal(t, codes.PermissionDenied, status.Code(err), err)
		assert.ErrorContains(t, err, contains)

		plans, executes := a.runner.calls()
		assert.Equal(t, plansBefore, plans, "a refused plan never reaches the plugin")
		assert.Equal(t, executesBefore, executes, "a refused execute never reaches the plugin")
		entries := readAuditLog(t, a.auditPath)
		require.GreaterOrEqual(t, len(entries), 2)
		for _, e := range entries[len(entries)-2:] {
			assert.Equal(t, AuditRefused, e.Event)
			assert.Contains(t, e.Reason, contains)
		}
	}
	scale := map[string]any{"namespace": "payments", "replicas": 3}

	t.Run("without --allow-tools", func(t *testing.T) {
		a := h.serveTools(hub, false, scalePolicy)
		expectRefused(t, a, a.call("scale", scale), "tools are switched off on this runner")
	})

	t.Run("without a policy, even with --allow-tools", func(t *testing.T) {
		a := h.serveTools(hub, true, "")
		expectRefused(t, a, a.call("scale", scale), "with no policy every tool is refused")
	})

	t.Run("for a source the policy omits", func(t *testing.T) {
		a := h.serveTools(hub, true, "sources:\n  some-other-source:\n    tools:\n      scale: {}\n")
		expectRefused(t, a, a.call("scale", scale), "is not in the policy")
	})

	a := h.serveTools(hub, true, scalePolicy)
	for name, tc := range map[string]struct {
		call     func(ToolCall) ToolCall
		contains string
	}{
		"a tool the policy omits": {
			call: func(c ToolCall) ToolCall { c.Request.Tool = "delete_namespace"; return c }, contains: "is not allowed",
		},
		"a manifest digest other than the plugin's": {
			call: func(c ToolCall) ToolCall { c.ManifestDigest = pluginsdk.ManifestDigest(nil); return c }, contains: "the plugin's is " + testManifestDigest,
		},
		"a value the policy does not allow": {
			call: func(c ToolCall) ToolCall {
				c.Request.Params = map[string]any{"namespace": "kube-system", "replicas": 3}
				return c
			},
			contains: "not one of the values",
		},
		"a number above the policy's maximum, though the manifest allows it": {
			call: func(c ToolCall) ToolCall {
				c.Request.Params = map[string]any{"namespace": "payments", "replicas": 50}
				return c
			},
			contains: "outside the policy's range 1 to 10",
		},
		"an actor outside the tool's allow-list": {
			call: func(c ToolCall) ToolCall { c.Request.Identity = "user:mallory"; return c }, contains: `actor "user:mallory" may not run scale`,
		},
		"an origin outside the tool's allow-list": {
			call: func(c ToolCall) ToolCall {
				c.Request.Tool, c.Request.Origin = "rollout_restart", "editor"
				c.Request.Params = map[string]any{"namespace": "payments"}
				return c
			},
			contains: `origin "editor" may not run rollout_restart`,
		},
		"parameters the manifest refuses": {
			call: func(c ToolCall) ToolCall {
				c.Request.Tool, c.Request.Origin = "rollout_restart", "task_run:1"
				c.Request.Params = map[string]any{"namespace": "payments", "force": true}
				return c
			},
			contains: "is not a parameter of rollout_restart",
		},
	} {
		t.Run(name, func(t *testing.T) {
			expectRefused(t, a, tc.call(a.call("scale", scale)), tc.contains)
		})
	}

	t.Run("a manifest other than the one the policy pins", func(t *testing.T) {
		pinned := h.serveTools(hub, true, approvalsYAML()+"sources:\n  SOURCE:\n    manifest: sha256:"+strings.Repeat("0", 64)+"\n    tools:\n      scale: {}\n")
		expectRefused(t, pinned, pinned.call("scale", scale), "the policy pins sha256:"+strings.Repeat("0", 64))
	})

	t.Run("an execute without a precondition", func(t *testing.T) {
		_, _, err := hub.ExecuteTool(h.ctx, a.call("scale", scale))
		assert.Equal(t, codes.InvalidArgument, status.Code(err), "the hub refuses it before signing anything")

		// A Marmot that sent one anyway is refused by the serving process itself.
		s := h.sessionFor(hub, a.principal, a.source)
		fq, err := forwardedTool(pb.Operation_OPERATION_TOOL_PLAN, a.call("scale", scale), false)
		require.NoError(t, err)
		fq.Operation = pb.Operation_OPERATION_TOOL_EXECUTE
		signed, err := hub.signFor(s, fq)
		require.NoError(t, err)
		_, err = s.client.ExecuteTool(h.ctx, signed)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
		assert.ErrorContains(t, err, "precondition is required")
		_, executes := a.runner.calls()
		assert.Zero(t, executes)
	})

	t.Run("a plan envelope presented as an execute", func(t *testing.T) {
		s := h.sessionFor(hub, a.principal, a.source)
		call := a.call("scale", scale)
		call.Request.Precondition = "0"
		fq, err := forwardedTool(pb.Operation_OPERATION_TOOL_PLAN, call, false)
		require.NoError(t, err)
		signed, err := hub.signFor(s, fq)
		require.NoError(t, err)
		_, err = s.client.ExecuteTool(h.ctx, signed)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
		_, executes := a.runner.calls()
		assert.Zero(t, executes)
	})
}

// An execute whose intent cannot be written is never run: no audit, no tool.
func TestServeToolsFailClosedWhenTheAuditLogFails(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	a := h.serveTools(hub, true, scalePolicy)

	call := a.call("scale", map[string]any{"namespace": "payments", "replicas": 3})
	plan, _, err := hub.PlanTool(h.ctx, call)
	require.NoError(t, err)

	// Closing the file underneath the log makes every write fail, as a full or revoked disk would.
	require.NoError(t, a.audit.f.Close())
	call.Request.Precondition = plan.Precondition
	call.Approval = approve(t, hub, call, 5*time.Minute, alice)
	_, receipt, err := hub.ExecuteTool(h.ctx, call)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.ErrorContains(t, err, "the audit log could not be written")
	assert.Nil(t, receipt)
	_, executes := a.runner.calls()
	assert.Zero(t, executes, "the plugin is never called without its intent on disk")
}

func TestServeToolsRefusesToStartMisconfigured(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	start := func(t *testing.T, policy string, withAudit bool) error {
		t.Helper()
		source := newSourceName()
		opts := h.serveOptions(h.enroller(hub), servePrincipal(), sessionCredential, source, &fakeQuerier{})
		opts.AllowTools, opts.PluginTools = true, newFakePluginTools()
		if withAudit {
			auditLog, err := OpenAuditLog(filepath.Join(t.TempDir(), "audit.log"))
			require.NoError(t, err)
			t.Cleanup(func() { _ = auditLog.Close() })
			opts.AuditLog = auditLog
		}
		if policy != "" {
			p, err := LoadPolicy(writePolicy(t, strings.ReplaceAll(policy, "SOURCE", source)))
			require.NoError(t, err)
			opts.Policy = p
		}
		ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
		defer cancel()
		return Serve(ctx, opts)
	}

	assert.ErrorContains(t, start(t, scalePolicy, false), "--allow-tools needs --audit-log")
	assert.ErrorContains(t, start(t, "sources:\n  SOURCE:\n    tools:\n      scael: {}\n", true), `tool "scael" is not in the plugin's manifest`)
	assert.ErrorContains(t, start(t, "sources:\n  SOURCE:\n    tools:\n      scale:\n        params:\n          replica: {max: 3}\n", true), `has no parameter "replica"`)
}

// Forwarded tools take the same signed forwards as queries: a replica certificate alone cannot have a replica act.
func TestReplicaRefusesToolForwardsThatAreUnsignedOrReplayed(t *testing.T) {
	h := newHarness(t)
	connected, other := h.startHub(), h.startHub()
	a := h.serveTools(connected, true, scalePolicy)

	conn, err := grpc.NewClient(connected.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(other.clientTLS())))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	replica := pb.NewReplicaClient(conn)
	audience := h.keyring.replicaID(connected.replicaID)

	sign := func(op pb.Operation) *pb.SignedForward {
		call := a.call("scale", map[string]any{"namespace": "payments", "replicas": 3})
		call.Request.Precondition = "0"
		fq, err := forwardedTool(op, call, false)
		require.NoError(t, err)
		signed, err := h.keyring.signForward(fq, audience, time.Now())
		require.NoError(t, err)
		return signed
	}

	valid := sign(pb.Operation_OPERATION_TOOL_PLAN)
	resp, err := replica.PlanTool(h.ctx, valid)
	require.NoError(t, err, "control: a signed forward from a replica is answered")
	assert.NotNil(t, resp.Receipt)

	for name, tc := range map[string]struct {
		signed *pb.SignedForward
		rpc    func(context.Context, *pb.SignedForward, ...grpc.CallOption) (*pb.ToolResponse, error)
	}{
		"unsigned":                   {&pb.SignedForward{Forward: valid.Forward}, replica.PlanTool},
		"replayed":                   {valid, replica.PlanTool},
		"plan forward as an execute": {sign(pb.Operation_OPERATION_TOOL_PLAN), replica.ExecuteTool},
		"query forward as a tool": {func() *pb.SignedForward {
			fq, err := forwarded(pb.Operation_OPERATION_EXECUTE, a.source, "", "", pluginsdk.QueryRequest{Statement: "SELECT 1", MaxRows: 1})
			require.NoError(t, err)
			signed, err := h.keyring.signForward(fq, audience, time.Now())
			require.NoError(t, err)
			return signed
		}(), replica.ExecuteTool},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := tc.rpc(h.ctx, tc.signed)
			assert.Equal(t, codes.PermissionDenied, status.Code(err))
		})
	}
	_, executes := a.runner.calls()
	assert.Zero(t, executes)
}

// A receipt that does not cover the output that arrived is reported, and the output is not passed off as verified.
func TestServeToolReceiptMustCoverTheOutput(t *testing.T) {
	h := newHarness(t)
	hub := h.startHub()
	a := h.serveTools(hub, true, scalePolicy)
	owner := Owner{Type: string(a.principal.Type()), ID: a.principal.ID()}

	fq, err := forwardedTool(pb.Operation_OPERATION_TOOL_PLAN, a.call("scale", map[string]any{"namespace": "payments", "replicas": 3}), false)
	require.NoError(t, err)
	resp, s, err := hub.tool(h.ctx, fq, owner, false)
	require.NoError(t, err)
	exp := receiptExpectation{fq: fq, owner: owner, enrolmentID: s.enrolment.ID}
	require.True(t, hub.verifyToolReceipt(h.ctx, resp, exp).Verified())

	resp.OutputJson = []byte(`{"target":"deployment/kube-system/api","precondition":"0"}`)
	r := hub.verifyToolReceipt(h.ctx, resp, exp)
	assert.Equal(t, dataquery.ReceiptInvalid, r.Status)
	assert.Contains(t, r.Problem, "another output")

	resp.Receipt = nil
	assert.Equal(t, dataquery.ReceiptMissing, hub.verifyToolReceipt(h.ctx, resp, exp).Status)
}
