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

// maxActionOutputBytes caps a plan or result from a serving process. Both describe one change to one object, so this is generous.
const maxActionOutputBytes = 1 << 20

// ActionRunner plans and executes plugin actions on served sources. Implemented by Hub, which only ever sends them to a serving process over its signed connection: Marmot never runs a plugin's actions itself, and whatever it asks for, the serving process acts only within its own policy. Nothing calls it yet; approvals and the API that will are still to come.
type ActionRunner interface {
	PlanAction(ctx context.Context, call ActionCall) (*pluginsdk.ActionPlan, *ActionReceipt, error)
	ExecuteAction(ctx context.Context, call ActionCall) (*pluginsdk.ActionResult, *ActionReceipt, error)
}

var _ ActionRunner = (*Hub)(nil)

// ActionCall is one action to plan or execute on a served source.
type ActionCall struct {
	Source string
	// ID is the action's id in Marmot's log, when it keeps one, which the serving process records so both trails can be matched.
	ID string
	// ManifestDigest is the digest of the manifest the caller built the request against, from pluginsdk.ManifestDigest. The serving process refuses the call unless its plugin's manifest has exactly that digest.
	ManifestDigest string
	// Request is the action, its parameters, the acting principal's audit subject as Identity, the Origin and, for an execute, the Precondition from its plan. DryRun is ignored: the operation decides it.
	Request pluginsdk.ActionRequest
	// Approval is the people's approval of an execute of a write, signed with SignApproval for the call exactly as it is sent here. The serving process verifies Marmot's approval signature over a digest it computes from the request and counts the distinct approvers; a write without one is refused unless its policy says approvals: 0. Leave it nil for plans and reads, which refuse one.
	Approval *Approval
}

// ErrReceiptNotVerified is returned beside the plan or result and the receipt when an action's receipt did not verify. For an execute the action may well have run, so the result is still returned, but the receipt says why it cannot be relied on.
var ErrReceiptNotVerified = errors.New("the serving plugin's receipt did not verify")

// PlanAction asks the serving plugin for call's source what the action would change, without changing anything.
func (h *Hub) PlanAction(ctx context.Context, call ActionCall) (*pluginsdk.ActionPlan, *ActionReceipt, error) {
	data, receipt, err := h.runAction(ctx, pb.Operation_OPERATION_ACTION_PLAN, call)
	if data == nil {
		return nil, receipt, err
	}
	var plan pluginsdk.ActionPlan
	if derr := json.Unmarshal(data, &plan); derr != nil {
		return nil, receipt, fmt.Errorf("decoding action plan: %w", derr)
	}
	return &plan, receipt, err
}

// ExecuteAction asks the serving plugin for call's source to run the action. The serving process writes its intent to its audit log before the plugin acts, and refuses when it cannot.
func (h *Hub) ExecuteAction(ctx context.Context, call ActionCall) (*pluginsdk.ActionResult, *ActionReceipt, error) {
	data, receipt, err := h.runAction(ctx, pb.Operation_OPERATION_ACTION_EXECUTE, call)
	if data == nil {
		return nil, receipt, err
	}
	var result pluginsdk.ActionResult
	if derr := json.Unmarshal(data, &result); derr != nil {
		return nil, receipt, fmt.Errorf("decoding action result: %w", derr)
	}
	return &result, receipt, err
}

// runAction routes an action like a query and verifies the receipt that comes back. It returns the output only when the plugin succeeded, and the receipt whenever one arrived, including for a plugin's failure, which comes back as its gRPC code.
func (h *Hub) runAction(ctx context.Context, op pb.Operation, call ActionCall) ([]byte, *ActionReceipt, error) {
	read := false
	if op == pb.Operation_OPERATION_ACTION_EXECUTE && call.Request.Precondition == "" {
		// Only a read executes without a plan. The enrolment's report says which actions read; it is informative, and the serving process refuses a write without a precondition whatever the report says.
		read = h.reportedRead(ctx, call.Source, call.Request.Action, call.ManifestDigest)
	}
	fq, err := forwardedAction(op, call, read)
	if err != nil {
		return nil, nil, err
	}
	owner, err := h.owners(ctx, call.Source)
	if err != nil {
		return nil, nil, err
	}
	resp, s, err := h.action(ctx, fq, owner, true)
	if err != nil {
		return nil, nil, err
	}
	if len(resp.OutputJson) > maxActionOutputBytes {
		return nil, nil, status.Error(codes.ResourceExhausted, "action output too large")
	}
	exp := receiptExpectation{fq: fq, owner: owner}
	if s != nil {
		exp.enrolmentID = s.enrolment.ID
	}
	receipt := h.verifyActionReceipt(ctx, resp, exp)

	if resp.Code != uint32(codes.OK) {
		// The message is the receipt's, which is signed when the receipt verified.
		msg := receipt.Error
		if msg == "" {
			msg = "the action failed in the serving plugin"
		}
		return nil, receipt, status.Error(codes.Code(resp.Code), msg)
	}
	if !receipt.Verified() {
		return resp.OutputJson, receipt, fmt.Errorf("%w: %s", ErrReceiptNotVerified, receipt.Problem)
	}
	return resp.OutputJson, receipt, nil
}

// forwardedAction builds an action as it travels between replicas, refusing early what every serving process would refuse anyway. The serving process checks everything again against its plugin's manifest and its own policy.
func forwardedAction(op pb.Operation, call ActionCall, read bool) (*pb.ForwardedQuery, error) {
	req := call.Request
	switch {
	case req.Action == "":
		return nil, status.Error(codes.InvalidArgument, "an action is required")
	case call.ManifestDigest == "":
		return nil, status.Error(codes.InvalidArgument, "the manifest digest the action was built against is required")
	case op == pb.Operation_OPERATION_ACTION_EXECUTE && req.Precondition == "" && !read:
		return nil, status.Error(codes.InvalidArgument, "an execute needs the precondition from its plan")
	}
	var params []byte
	if len(req.Params) > 0 {
		data, err := json.Marshal(req.Params)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "encoding the action's parameters: %v", err)
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
		Action:         req.Action,
		ParamsJson:     params,
		Precondition:   req.Precondition,
		ManifestDigest: call.ManifestDigest,
	}, nil
}

// reportedRead reports whether the source's enrolment report declares action a read in the manifest with digest.
func (h *Hub) reportedRead(ctx context.Context, source, action, digest string) bool {
	served, _, err := h.ServedActions(ctx, source)
	if err != nil || served.ManifestDigest != digest {
		return false
	}
	return isReadAction(served.Manifest, action)
}

// action sends an action to a local session, or to a replica holding one when mayForward is set, and returns the session it went to, or nil when it was forwarded. As with queries, a forwarded action is never forwarded again.
func (h *Hub) action(ctx context.Context, fq *pb.ForwardedQuery, owner Owner, mayForward bool) (*pb.ActionResponse, *session, error) {
	if s := h.localSession(fq.Source, owner); s != nil {
		signed, err := h.signFor(s, fq)
		if err != nil {
			return nil, nil, err
		}
		var resp *pb.ActionResponse
		if fq.Operation == pb.Operation_OPERATION_ACTION_EXECUTE {
			resp, err = s.client.ExecuteAction(ctx, signed)
		} else {
			resp, err = s.client.PlanAction(ctx, signed)
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
	var resp *pb.ActionResponse
	if fq.Operation == pb.Operation_OPERATION_ACTION_EXECUTE {
		resp, err = client.ExecuteAction(ctx, signed)
	} else {
		resp, err = client.PlanAction(ctx, signed)
	}
	return resp, nil, err
}

func (r *replicaService) PlanAction(ctx context.Context, signed *pb.SignedForward) (*pb.ActionResponse, error) {
	return r.relayAction(ctx, signed, pb.Operation_OPERATION_ACTION_PLAN)
}

func (r *replicaService) ExecuteAction(ctx context.Context, signed *pb.SignedForward) (*pb.ActionResponse, error) {
	return r.relayAction(ctx, signed, pb.Operation_OPERATION_ACTION_EXECUTE)
}

// relayAction answers a forwarded action from a session held here and returns the serving process's response untouched, receipt and all, so the replica that took the action verifies the receipt against what it asked for.
func (r *replicaService) relayAction(ctx context.Context, signed *pb.SignedForward, op pb.Operation) (*pb.ActionResponse, error) {
	fq, owner, err := r.accept(ctx, signed, op)
	if err != nil {
		return nil, err
	}
	resp, _, err := r.hub.action(ctx, fq, owner, false)
	return resp, err
}
