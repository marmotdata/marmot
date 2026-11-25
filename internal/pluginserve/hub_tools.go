package pluginserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

// maxToolOutputBytes caps a plan or result from a serving process. Both describe one change to one object, so this is generous.
const maxToolOutputBytes = 1 << 20

// ToolRunner plans and executes plugin tools on served sources. Implemented by Hub, which only ever sends them to a serving process over its signed connection: Marmot never runs a plugin's tools itself, and whatever it asks for, the serving process acts only within its own policy. Nothing calls it yet; approvals and the API that will are still to come.
type ToolRunner interface {
	PlanTool(ctx context.Context, call ToolCall) (*pluginsdk.ToolPlan, *ToolReceipt, error)
	ExecuteTool(ctx context.Context, call ToolCall) (*pluginsdk.ToolResult, *ToolReceipt, error)
}

var _ ToolRunner = (*Hub)(nil)

// ToolCall is one tool to plan or execute on a served source.
type ToolCall struct {
	Source string
	// ID is the tool's id in Marmot's log, when it keeps one, which the serving process records so both trails can be matched.
	ID string
	// ManifestDigest is the digest of the manifest the caller built the request against, from pluginsdk.ManifestDigest. The serving process refuses the call unless its plugin's manifest has exactly that digest.
	ManifestDigest string
	// Request is the tool, its parameters, the acting principal's audit subject as Identity, the Origin and, for an execute, the Precondition from its plan. DryRun is ignored: the operation decides it.
	Request pluginsdk.ToolRequest
	// Approval is the people's approval of an execute of a write, signed with SignApproval for the call exactly as it is sent here. The serving process verifies Marmot's approval signature over a digest it computes from the request and counts the distinct approvers; a write without one is refused unless its policy says approvals: 0. Leave it nil for plans and reads, which refuse one.
	Approval *Approval
}

// ErrReceiptNotVerified is returned beside the plan or result and the receipt when a tool's receipt did not verify. For an execute the tool may well have run, so the result is still returned, but the receipt says why it cannot be relied on.
var ErrReceiptNotVerified = errors.New("the serving plugin's receipt did not verify")

// PlanTool asks the serving plugin for call's source what the tool would change, without changing anything.
func (h *Hub) PlanTool(ctx context.Context, call ToolCall) (*pluginsdk.ToolPlan, *ToolReceipt, error) {
	data, receipt, err := h.runTool(ctx, pb.Operation_OPERATION_TOOL_PLAN, call)
	if data == nil {
		return nil, receipt, err
	}
	var plan pluginsdk.ToolPlan
	if derr := json.Unmarshal(data, &plan); derr != nil {
		return nil, receipt, fmt.Errorf("decoding tool plan: %w", derr)
	}
	return &plan, receipt, err
}

// ExecuteTool asks the serving plugin for call's source to run the tool. The serving process writes its intent to its audit log before the plugin acts, and refuses when it cannot.
func (h *Hub) ExecuteTool(ctx context.Context, call ToolCall) (*pluginsdk.ToolResult, *ToolReceipt, error) {
	data, receipt, err := h.runTool(ctx, pb.Operation_OPERATION_TOOL_EXECUTE, call)
	if data == nil {
		return nil, receipt, err
	}
	var result pluginsdk.ToolResult
	if derr := json.Unmarshal(data, &result); derr != nil {
		return nil, receipt, fmt.Errorf("decoding tool result: %w", derr)
	}
	return &result, receipt, err
}

// runTool routes a tool like a query and verifies the receipt that comes back. It returns the output only when the plugin succeeded, and the receipt whenever one arrived, including for a plugin's failure, which comes back as its gRPC code.
func (h *Hub) runTool(ctx context.Context, op pb.Operation, call ToolCall) ([]byte, *ToolReceipt, error) {
	read := false
	if op == pb.Operation_OPERATION_TOOL_EXECUTE && call.Request.Precondition == "" {
		// Only a read executes without a plan. The enrolment's report says which tools read; it is informative, and the serving process refuses a write without a precondition whatever the report says.
		read = h.reportedRead(ctx, call.Source, call.Request.Tool, call.ManifestDigest)
	}
	fq, err := forwardedTool(op, call, read)
	if err != nil {
		return nil, nil, err
	}
	owner, err := h.owners(ctx, call.Source)
	if err != nil {
		return nil, nil, err
	}
	resp, s, err := h.tool(ctx, fq, owner, true)
	if err != nil {
		return nil, nil, err
	}
	if len(resp.OutputJson) > maxToolOutputBytes {
		return nil, nil, status.Error(codes.ResourceExhausted, "tool output too large")
	}
	exp := receiptExpectation{fq: fq, owner: owner}
	if s != nil {
		exp.enrolmentID = s.enrolment.ID
	}
	receipt := h.verifyToolReceipt(ctx, resp, exp)

	if resp.Code != uint32(codes.OK) {
		// The message is the receipt's, which is signed when the receipt verified.
		msg := receipt.Error
		if msg == "" {
			msg = "the tool failed in the serving plugin"
		}
		return nil, receipt, status.Error(codes.Code(resp.Code), msg)
	}
	if !receipt.Verified() {
		return resp.OutputJson, receipt, fmt.Errorf("%w: %s", ErrReceiptNotVerified, receipt.Problem)
	}
	return resp.OutputJson, receipt, nil
}

// forwardedTool builds a tool as it travels between replicas, refusing early what every serving process would refuse anyway. The serving process checks everything again against its plugin's manifest and its own policy.
func forwardedTool(op pb.Operation, call ToolCall, read bool) (*pb.ForwardedQuery, error) {
	req := call.Request
	switch {
	case req.Tool == "":
		return nil, status.Error(codes.InvalidArgument, "a tool is required")
	case call.ManifestDigest == "":
		return nil, status.Error(codes.InvalidArgument, "the manifest digest the tool was built against is required")
	case op == pb.Operation_OPERATION_TOOL_EXECUTE && req.Precondition == "" && !read:
		return nil, status.Error(codes.InvalidArgument, "an execute needs the precondition from its plan")
	}
	var params []byte
	if len(req.Params) > 0 {
		data, err := json.Marshal(req.Params)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "encoding the tool's parameters: %v", err)
		}
		params = data
	}
	approval, err := encodeApproval(call.Approval)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	nonce, err := newNonce()
	if err != nil {
		return nil, err
	}
	return &pb.ForwardedQuery{
		Approval:       approval,
		Operation:      op,
		Source:         call.Source,
		Actor:          req.Identity,
		QueryId:        call.ID,
		Origin:         req.Origin,
		Nonce:          nonce,
		Tool:           req.Tool,
		ParamsJson:     params,
		Precondition:   req.Precondition,
		ManifestDigest: call.ManifestDigest,
	}, nil
}

// reportedRead reports whether the source's enrolment report declares tool a read in the manifest with digest.
func (h *Hub) reportedRead(ctx context.Context, source, tool, digest string) bool {
	served, _, err := h.ServedTools(ctx, source)
	if err != nil || served.ManifestDigest != digest {
		return false
	}
	return isReadTool(served.Manifest, tool)
}

// tool sends a tool to a local session, or to a replica holding one when mayForward is set, and returns the session it went to, or nil when it was forwarded. As with queries, a forwarded tool is never forwarded again.
func (h *Hub) tool(ctx context.Context, fq *pb.ForwardedQuery, owner Owner, mayForward bool) (*pb.ToolResponse, *session, error) {
	if s := h.localSession(fq.Source, owner); s != nil {
		signed, err := h.signFor(s, fq)
		if err != nil {
			return nil, nil, err
		}
		var resp *pb.ToolResponse
		if fq.Operation == pb.Operation_OPERATION_TOOL_EXECUTE {
			resp, err = s.client.ExecuteTool(ctx, signed)
		} else {
			resp, err = s.client.PlanTool(ctx, signed)
		}
		return resp, s, err
	}
	if !mayForward {
		return nil, nil, status.Error(codes.Unavailable, ErrNotConnected.Error())
	}
	peer, signed, err := h.forwardTo(ctx, fq, owner)
	if err != nil {
		return nil, nil, err
	}
	client := pb.NewReplicaClient(peer)
	var resp *pb.ToolResponse
	if fq.Operation == pb.Operation_OPERATION_TOOL_EXECUTE {
		resp, err = client.ExecuteTool(ctx, signed)
	} else {
		resp, err = client.PlanTool(ctx, signed)
	}
	return resp, nil, err
}

func (r *replicaService) PlanTool(ctx context.Context, signed *pb.SignedForward) (*pb.ToolResponse, error) {
	return r.relayTool(ctx, signed, pb.Operation_OPERATION_TOOL_PLAN)
}

func (r *replicaService) ExecuteTool(ctx context.Context, signed *pb.SignedForward) (*pb.ToolResponse, error) {
	return r.relayTool(ctx, signed, pb.Operation_OPERATION_TOOL_EXECUTE)
}

// relayTool answers a forwarded tool from a session held here and returns the serving process's response untouched, receipt and all, so the replica that took the tool verifies the receipt against what it asked for.
func (r *replicaService) relayTool(ctx context.Context, signed *pb.SignedForward, op pb.Operation) (*pb.ToolResponse, error) {
	fq, owner, err := r.accept(ctx, signed, op)
	if err != nil {
		return nil, err
	}
	resp, _, err := r.hub.tool(ctx, fq, owner, false)
	return resp, err
}
