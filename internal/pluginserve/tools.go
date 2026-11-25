package pluginserve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

// Tools change the systems a plugin catalogues, so a serving process treats them as Marmot's requests rather than its instructions. Marmot signs what it wants; the process runs it only when its own operator switched tools on, named the tool in the local policy and the request fits every rule there, all checked here against the plugin's own manifest. Nothing Marmot sends can widen any of it, and every refusal is a PermissionDenied with a line in the audit log.

// toolReceiptDomain prefixes the bytes a tool receipt's signature covers, apart from a query receipt's, so neither can be passed off as the other.
var toolReceiptDomain = []byte("marmot serve tool receipt v1\x00")

func isTool(op pb.Operation) bool {
	return op == pb.Operation_OPERATION_TOOL_PLAN || op == pb.Operation_OPERATION_TOOL_EXECUTE
}

// checkToolSetup runs when Serve starts. With tools switched on it requires an audit log, since a tool only runs once its intent is written there, and checks the policy against each plugin's manifest, so a misspelt tool or parameter fails here rather than refusing the first request. The digest of each manifest is logged, ready to pin in the policy.
func checkToolSetup(ctx context.Context, opts ServeOptions) error {
	if !opts.AllowTools {
		return nil
	}
	if opts.AuditLog == nil {
		return errors.New("--allow-tools needs --audit-log: no tool runs unless its intent is written down first")
	}
	if opts.PluginTools == nil {
		return errors.New("tools are switched on but nothing can run them")
	}
	if opts.Policy == nil {
		log.Warn().Msg("Tools are switched on but there is no --policy, so every tool will be refused")
		return nil
	}
	for _, src := range opts.Sources {
		sp := opts.Policy.Sources[src.Name]
		if len(sp.Triggers) > 0 {
			if opts.Watcher == nil {
				return fmt.Errorf("source %q: the policy names triggers but nothing in this process can run them", src.Name)
			}
			triggers, err := opts.Watcher.TriggerManifest(ctx, src.PluginID)
			if err != nil {
				return fmt.Errorf("source %q: %w", src.Name, err)
			}
			if err := opts.Policy.CheckTriggers(src.Name, triggers); err != nil {
				return err
			}
			log.Info().Str("source", src.Name).Str("triggers", pluginsdk.TriggersDigest(triggers)).Msg("Running the triggers the policy names")
		}
		if len(sp.Tools) == 0 {
			continue
		}
		specs, err := opts.PluginTools.ToolManifest(ctx, src.PluginID)
		if err != nil {
			return fmt.Errorf("source %q: %w", src.Name, err)
		}
		if err := opts.Policy.CheckManifest(src.Name, specs); err != nil {
			return err
		}
		if opts.Policy.Approvals == nil {
			for name, ap := range sp.Tools {
				if !isReadTool(specs, name) && ap.need().required > 0 {
					return fmt.Errorf("source %q: tool %q is a write that needs approval, and the policy has no approvals section: add approvals: {trust: marmot}", src.Name, name)
				}
			}
		}
		log.Info().Str("source", src.Name).Str("manifest", pluginsdk.ManifestDigest(specs)).Msg("Serving tools the policy names")
	}
	return nil
}

func (p *pluginService) PlanTool(ctx context.Context, signed *pb.SignedQuery) (*pb.ToolResponse, error) {
	return p.serveTool(ctx, signed, pb.Operation_OPERATION_TOOL_PLAN)
}

func (p *pluginService) ExecuteTool(ctx context.Context, signed *pb.SignedQuery) (*pb.ToolResponse, error) {
	return p.serveTool(ctx, signed, pb.Operation_OPERATION_TOOL_EXECUTE)
}

// toolRefusal is a reason the process will not act, kept apart from the plugin's own failures so it is never mistaken for one.
type toolRefusal struct {
	reason string
	// policy is set when the local policy refused, so the refusal names it.
	policy bool
}

func (r *toolRefusal) Error() string { return r.reason }

// refuse audits a refusal and returns the PermissionDenied that tells Marmot.
func (p *pluginService) refuse(op pb.Operation, q *pb.Query, digest string, r *toolRefusal) error {
	entry := AuditEntry{Event: AuditRefused, Operation: operationName(op), Reason: r.reason, ManifestDigest: digest}
	if r.policy {
		entry.Policy = "refused"
	}
	p.record(entry, q)
	ev := log.Warn().Str("operation", operationName(op)).Str("reason", r.reason)
	if q != nil {
		ev = ev.Str("source", q.Source).Str("tool", q.Tool).Str("actor", q.Actor).Str("origin", q.Origin)
	}
	ev.Msg("Refused tool")
	if r.policy {
		return status.Errorf(codes.PermissionDenied, "refused by the serving process's local policy %s: %s", p.policy.Name(), r.reason)
	}
	return status.Errorf(codes.PermissionDenied, "refused by the serving process: %s", r.reason)
}

// serveTool is every tool's path through the process, in the order its checks are made: the envelope for this exact operation, a source this process serves, --allow-tools, a policy naming the source and the tool, then, against the manifest of the plugin process that will act, the manifest digest Marmot signed and any the policy pins, the actor, origin and parameter rules, the SDK's own request check, and a precondition for an execute. An execute's intent is written to the audit log after all of that and before the plugin is called, and a failed write refuses it.
func (p *pluginService) serveTool(ctx context.Context, signed *pb.SignedQuery, op pb.Operation) (*pb.ToolResponse, error) {
	q, err := p.verifier.Verify(signed, op)
	if err != nil {
		log.Warn().Err(err).Msg("Refused tool")
		p.record(AuditEntry{Event: AuditRefused, Operation: operationName(op), Reason: err.Error()}, nil)
		return nil, status.Error(codes.PermissionDenied, ErrRejected.Error())
	}
	src, ok := p.sources[q.Source]
	if !ok {
		return nil, p.refuse(op, q, "", &toolRefusal{reason: fmt.Sprintf("this process does not serve %q", q.Source)})
	}
	allowTools := p.tools && p.runner != nil
	sp, ap, err := p.policy.toolPolicy(allowTools, q.Source, q.Tool)
	if err != nil {
		return nil, p.refuse(op, q, "", &toolRefusal{reason: err.Error(), policy: p.policy != nil && !errors.Is(err, errToolsOff)})
	}
	params, err := decodeParams(q.ParamsJson)
	if err != nil {
		return nil, p.refuse(op, q, "", &toolRefusal{reason: err.Error()})
	}

	execute := op == pb.Operation_OPERATION_TOOL_EXECUTE
	req := pluginsdk.ToolRequest{
		Tool:         q.Tool,
		Params:       params,
		Identity:     q.Actor,
		Origin:       q.Origin,
		Precondition: q.Precondition,
		DryRun:       !execute,
	}

	// gate runs inside the PluginTools, between acquiring the plugin process and calling it, so every check below is made against that process's manifest. reached records that the plugin was called, which is what decides between a refusal and a receipt.
	var digest string
	var refusal *toolRefusal
	var approval *verifiedApproval
	read := false
	reached := false
	gate := func(specs []pluginsdk.ToolSpec) error {
		digest = pluginsdk.ManifestDigest(specs)
		if refusal = checkTool(q, sp, ap, req, specs, digest, execute); refusal != nil {
			return refusal
		}
		read = isReadTool(specs, q.Tool)
		if approval, refusal = p.checkApproval(q, ap, digest, read, execute); refusal != nil {
			return refusal
		}
		// A read changes nothing, so like a plan it is audited after it ran, with no intent before it.
		if execute && !read {
			if err := p.recordIntent(q, digest, approval); err != nil {
				log.Error().Err(err).Msg("Failed to write a tool's intent to the audit log")
				refusal = &toolRefusal{reason: "the audit log could not be written, so the tool was not run"}
				return refusal
			}
		}
		reached = true
		return nil
	}

	started := time.Now()
	var output any
	if execute {
		output, err = p.runner.ExecuteTool(ctx, src.PluginID, src.Config, req, gate)
	} else {
		output, err = p.runner.PlanTool(ctx, src.PluginID, src.Config, req, gate)
	}
	if !reached {
		if refusal == nil {
			// The plugin could not be started, does not act or has a manifest the SDK refuses: nothing ran, and Marmot is told so as a refusal.
			refusal = &toolRefusal{reason: err.Error()}
		}
		return nil, p.refuse(op, q, digest, refusal)
	}
	return p.answerTool(op, q, digest, output, err, started, read, approval)
}

func isReadTool(specs []pluginsdk.ToolSpec, tool string) bool {
	for _, s := range specs {
		if s.Name == tool {
			return s.ReadOnly()
		}
	}
	return false
}

// checkApproval is the approvals step of the gate, made once everything else about the request has passed. A read and a plan take no approval and are refused if they carry one, since nothing honest attaches one there. An execute of a write needs as many distinct approvers as its policy says, named in an approval signed by Marmot's pinned approval key over a digest computed from this very request; the approval is then claimed, so it can run once, before the intent is written. A claimed approval stays spent even when the intent cannot be written, which fails closed: a person approves again rather than an approval ever running twice.
func (p *pluginService) checkApproval(q *pb.Query, ap ToolPolicy, digest string, read, execute bool) (*verifiedApproval, *toolRefusal) {
	if read || !execute {
		if len(q.Approval) > 0 {
			what := "a plan"
			if read {
				what = "a read"
			}
			return nil, &toolRefusal{reason: what + " takes no approval, and this request carries one", policy: true}
		}
		return nil, nil
	}
	need := ap.need()
	if need.required == 0 && len(q.Approval) == 0 {
		return nil, nil
	}
	approvals := p.policy.approvals()
	subject := ApprovalSubject{
		Source: q.Source, Tool: q.Tool, Params: q.ParamsJson, Precondition: q.Precondition,
		// The digest of the manifest this process computed, which checkTool has already required to equal the one Marmot signed.
		ManifestDigest: digest, Origin: q.Origin, Actor: q.Actor, ToolID: q.QueryId,
	}
	now := time.Now()
	verified, err := approvals.verifyApproval(q.Approval, subject, need, p.approvalKeys, now)
	if err != nil {
		return nil, &toolRefusal{reason: "approval refused: " + err.Error(), policy: true}
	}
	if !p.spent.claim(verified.digest, verified.expiresAt, now) {
		return nil, &toolRefusal{reason: "approval refused: it has already been used", policy: true}
	}
	return verified, nil
}

// checkTool is the part of the gate that needs the manifest. It returns nil when the request may reach the plugin.
func checkTool(q *pb.Query, sp SourcePolicy, ap ToolPolicy, req pluginsdk.ToolRequest, specs []pluginsdk.ToolSpec, digest string, execute bool) *toolRefusal {
	if q.ManifestDigest != digest {
		return &toolRefusal{reason: fmt.Sprintf("Marmot asked against manifest %q, and the plugin's is %s", q.ManifestDigest, digest)}
	}
	if err := sp.allowTool(ap, q, req.Params, specs, digest); err != nil {
		return &toolRefusal{reason: err.Error(), policy: true}
	}
	if _, err := pluginsdk.CheckToolRequest(specs, req, execute); err != nil {
		return &toolRefusal{reason: err.Error()}
	}
	// CheckToolRequest already requires one for an execute of a write and refuses to plan a read; this states both rules where the process's own checks are read, so neither can be lost to a change in the SDK. A read changes nothing a precondition could bind, so it is the one execute without a plan.
	read := isReadTool(specs, q.Tool)
	if !execute && read {
		return &toolRefusal{reason: q.Tool + " only reads: execute it directly, there is nothing to plan"}
	}
	if execute && !read && q.Precondition == "" {
		return &toolRefusal{reason: "an execute needs the precondition from its plan"}
	}
	return nil
}

// recordIntent writes an execute's intent before the plugin is called. Unlike every other line it fails the call when it cannot be written: a tool that changes a system must never run unrecorded.
func (p *pluginService) recordIntent(q *pb.Query, digest string, approval *verifiedApproval) error {
	if p.audit == nil {
		return errors.New("no audit log")
	}
	return p.audit.Append(auditEntryFor(withApproval(AuditEntry{Event: AuditToolIntent, Policy: "allowed", ManifestDigest: digest}, approval), q))
}

// withApproval records an approval on an audit entry. The intent line carries its digest and expiry, which is what a restart reads back to keep it spent.
func withApproval(e AuditEntry, approval *verifiedApproval) AuditEntry {
	if approval == nil {
		return e
	}
	expires := approval.expiresAt.UTC()
	e.ApprovalDigest, e.ApprovalExpiresAt, e.Approvers = hex.EncodeToString(approval.digest), &expires, approval.names()
	return e
}

// answerTool signs the receipt for a tool that reached the plugin, whether it succeeded or not, audits the outcome and builds the response. The plugin's failure travels in the receipt and the response's code rather than as a gRPC error, so Marmot gets the signed account of a failed execute too.
func (p *pluginService) answerTool(op pb.Operation, q *pb.Query, digest string, output any, callErr error, started time.Time, read bool, approval *verifiedApproval) (*pb.ToolResponse, error) {
	var outputJSON []byte
	var target, summary string
	if callErr == nil {
		switch o := output.(type) {
		case *pluginsdk.ToolPlan:
			if o == nil {
				callErr = errors.New("the plugin returned no plan")
			} else {
				target, summary = o.Target, o.Summary
			}
		case *pluginsdk.ToolResult:
			if o == nil {
				callErr = errors.New("the plugin returned no result")
			} else {
				target, summary = o.Target, o.Summary
			}
		}
	}
	if callErr == nil {
		data, err := json.Marshal(output)
		switch {
		case err != nil:
			callErr = fmt.Errorf("encoding the plugin's output: %w", err)
		case read && len(data) > pluginsdk.MaxReadOutputBytes:
			// The SDK refuses this in the plugin too; a plugin built against an older SDK is held to it here.
			callErr = status.Errorf(codes.ResourceExhausted, "the read returned %d bytes, above the %d a read may return", len(data), pluginsdk.MaxReadOutputBytes)
		default:
			outputJSON = data
		}
	}

	finished := time.Now()
	outputHash := sha256.Sum256(outputJSON)
	receipt := &pb.ToolReceipt{
		Version:        receiptVersion,
		Operation:      op,
		Id:             q.QueryId,
		Origin:         q.Origin,
		Source:         q.Source,
		Actor:          q.Actor,
		Tool:           q.Tool,
		ParamsSha256:   sha256Sum(q.ParamsJson),
		Precondition:   q.Precondition,
		ManifestDigest: digest,
		Nonce:          q.Nonce,
		Target:         target,
		Summary:        summary,
		OutputSha256:   outputHash[:],
		DurationMs:     finished.Sub(started).Milliseconds(),
		FinishedAtUnix: finished.Unix(),
	}
	if approval != nil {
		receipt.ApprovalDigest, receipt.Approvers, receipt.ApprovalTrust = approval.digest, approval.approvers, ApprovalTrustMarmot
	}
	resp := &pb.ToolResponse{OutputJson: outputJSON}
	if callErr != nil {
		st := status.Convert(callErr)
		receipt.Error = st.Message()
		resp.Code = uint32(st.Code())
		if resp.Code == uint32(codes.OK) {
			resp.Code = uint32(codes.Unknown)
		}
	}

	event := AuditToolPlan
	switch {
	case op == pb.Operation_OPERATION_TOOL_EXECUTE && read:
		event = AuditToolRead
	case op == pb.Operation_OPERATION_TOOL_EXECUTE:
		event = AuditToolExecute
	}
	entry := withApproval(AuditEntry{Event: event, Policy: "allowed", ManifestDigest: digest, Target: target, Summary: summary, DurationMS: receipt.DurationMs, Error: receipt.Error}, approval)
	// The outcome line names the approvers but not the digest, which the intent already holds; a restart reads spent approvals from intents.
	entry.ApprovalDigest, entry.ApprovalExpiresAt = "", nil
	signedReceipt, err := p.signer.signTool(receipt)
	if err != nil {
		// For an execute the tool has already run, so the outcome is still audited before Marmot is told the receipt is missing.
		log.Error().Err(err).Str("tool", q.Tool).Msg("Failed to sign tool receipt")
		entry.Error = errors.Join(callErr, fmt.Errorf("signing the receipt: %w", err)).Error()
		p.record(entry, q)
		return nil, status.Error(codes.Internal, "the tool's receipt could not be signed")
	}
	entry.ReceiptSignature = hex.EncodeToString(signedReceipt.Signature)
	resp.Receipt = signedReceipt

	log.Info().Str("operation", operationName(op)).Str("id", q.QueryId).Str("origin", q.Origin).Str("actor", q.Actor).Str("source", q.Source).
		Str("tool", q.Tool).Str("target", target).Dur("duration", finished.Sub(started)).AnErr("error", callErr).Msg("Served tool")
	p.record(entry, q)
	return resp, nil
}

// decodeParams reads the signed parameters, which must be a JSON object or absent.
func decodeParams(data []byte) (map[string]any, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var params map[string]any
	if err := json.Unmarshal(data, &params); err != nil {
		return nil, errors.New("the tool's parameters are not a JSON object")
	}
	return params, nil
}

func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
