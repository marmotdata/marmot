package pluginserve

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"time"

	gproto "google.golang.org/protobuf/proto"

	"github.com/marmotdata/marmot/internal/core/dataquery"
	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

const receiptVersion = 1

// receiptDomain prefixes the bytes a receipt's signature covers, so nothing else signed with a serving process's connection key, such as its TLS handshake, can be passed off as a receipt.
var receiptDomain = []byte("marmot serve receipt v1\x00")

// receiptSigner is a serving connection's key and certificate, which sign its receipts. The key never leaves the process, and the certificate binds it to the enrolment, so a receipt proves which enrolment served the query.
type receiptSigner struct {
	key  *ecdsa.PrivateKey
	leaf []byte
}

func (s receiptSigner) sign(r *pb.Receipt) (*pb.SignedReceipt, error) {
	return s.signUnder(receiptDomain, r)
}

// signTool signs a tool's receipt under its own domain.
func (s receiptSigner) signTool(r *pb.ToolReceipt) (*pb.SignedReceipt, error) {
	return s.signUnder(toolReceiptDomain, r)
}

func (s receiptSigner) signUnder(domain []byte, r gproto.Message) (*pb.SignedReceipt, error) {
	data, err := gproto.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("encoding receipt: %w", err)
	}
	digest := sha256.Sum256(append(append([]byte{}, domain...), data...))
	signature, err := ecdsa.SignASN1(rand.Reader, s.key, digest[:])
	if err != nil {
		return nil, fmt.Errorf("signing receipt: %w", err)
	}
	return &pb.SignedReceipt{Receipt: data, Signature: signature, Certificate: s.leaf}, nil
}

// receiptExpectation is what the replica that took a query asked for, which the receipt must match. When the query went out on a session held here, enrolmentID names it; when it was forwarded, the enrolment is looked up and must serve source for owner.
type receiptExpectation struct {
	fq          *pb.ForwardedQuery
	enrolmentID string
	owner       Owner
}

// verifyReceipt checks a receipt and returns it for the query log, as verified or as invalid with the reason. Invalid receipts are kept with whatever they claimed, so a forged or mangled one is visible in the log rather than dropped.
func (h *Hub) verifyReceipt(ctx context.Context, signed *pb.SignedReceipt, exp receiptExpectation, rows int64) *dataquery.RunnerReceipt {
	out := &dataquery.RunnerReceipt{
		Status:      dataquery.ReceiptInvalid,
		Signed:      signed.Receipt,
		Signature:   signed.Signature,
		Certificate: signed.Certificate,
	}
	var r pb.Receipt
	parseErr := gproto.Unmarshal(signed.Receipt, &r)
	if parseErr == nil {
		fillReceipt(out, &r)
	}
	if err := h.checkReceipt(ctx, signed, &r, parseErr, exp, rows, out); err != nil {
		out.Problem = err.Error()
		return out
	}
	out.Status = dataquery.ReceiptVerified
	return out
}

func (h *Hub) checkReceipt(ctx context.Context, signed *pb.SignedReceipt, r *pb.Receipt, parseErr error, exp receiptExpectation, rows int64, out *dataquery.RunnerReceipt) error {
	enrolment, err := h.checkReceiptSigner(ctx, signed, receiptDomain, exp)
	out.Enrolment = enrolment
	if err != nil {
		return err
	}
	if parseErr != nil {
		return errors.New("it does not parse")
	}

	fq := exp.fq
	statement := sha256.Sum256([]byte(fq.Statement))
	switch {
	case r.Version != receiptVersion:
		return fmt.Errorf("unsupported version %d", r.Version)
	case subtle.ConstantTimeCompare(r.Nonce, fq.Nonce) != 1:
		return errors.New("it is for another query: the nonce differs")
	case !bytes.Equal(r.StatementSha256, statement[:]):
		return errors.New("it names another statement")
	case r.QueryId != fq.QueryId, r.Source != fq.Source, r.Actor != fq.Actor, r.Origin != fq.Origin:
		return errors.New("its query id, source, actor or origin differ from the query's")
	case r.Rows != rows:
		return fmt.Errorf("it counts %d rows where %d arrived", r.Rows, rows)
	}
	return nil
}

// checkReceiptSigner checks that a receipt was signed under domain by a serving certificate this instance issued, for an enrolment that may answer exp, and returns that enrolment's id, which is known once the certificate is identified even when a later check fails.
func (h *Hub) checkReceiptSigner(ctx context.Context, signed *pb.SignedReceipt, domain []byte, exp receiptExpectation) (string, error) {
	leaf, err := x509.ParseCertificate(signed.Certificate)
	if err != nil {
		return "", errors.New("its certificate does not parse")
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: h.keyring.Pool(), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		return "", errors.New("its certificate was not issued by this instance's CA")
	}
	peer, err := h.keyring.identifyPeer(leaf)
	if err != nil || peer.kind != "serve" {
		return "", errors.New("its certificate is not a serving plugin's")
	}
	if err := h.checkReceiptEnrolment(ctx, peer.id, leaf, exp); err != nil {
		return peer.id, err
	}
	key, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok || !signedByAny([]*ecdsa.PublicKey{key}, domain, signed.Receipt, signed.Signature) {
		return peer.id, errors.New("its signature does not verify")
	}
	return peer.id, nil
}

// checkReceiptEnrolment ties the certificate that signed a receipt to the enrolment the query was sent to, or, for a forwarded query, to an enrolment that serves the source for its owner.
func (h *Hub) checkReceiptEnrolment(ctx context.Context, enrolmentID string, leaf *x509.Certificate, exp receiptExpectation) error {
	if exp.enrolmentID != "" && enrolmentID != exp.enrolmentID {
		return errors.New("it was signed by another enrolment than the one the query was sent to")
	}
	e, err := h.store.enrolment(ctx, enrolmentID)
	if err != nil {
		return errors.New("its enrolment is unknown")
	}
	if e.PrincipalType != exp.owner.Type || e.PrincipalID != exp.owner.ID || !slices.Contains(e.Sources, exp.fq.Source) {
		return errors.New("its enrolment does not serve this source for its owner")
	}
	if subtle.ConstantTimeCompare(publicKeySHA256(leaf.RawSubjectPublicKeyInfo), e.PublicKeySHA256) != 1 {
		return errors.New("its certificate key does not match its enrolment")
	}
	return nil
}

func fillReceipt(out *dataquery.RunnerReceipt, r *pb.Receipt) {
	out.QueryID = r.QueryId
	out.Origin = r.Origin
	out.Source = r.Source
	out.Actor = r.Actor
	out.StatementSHA256 = hex.EncodeToString(r.StatementSha256)
	out.Nonce = hex.EncodeToString(r.Nonce)
	out.Rows = r.Rows
	out.MaxRows = r.MaxRows
	out.DurationMS = r.DurationMs
	out.Error = r.Error
	if r.FinishedAtUnix != 0 {
		finished := time.Unix(r.FinishedAtUnix, 0).UTC()
		out.FinishedAt = &finished
	}
}

// ToolReceipt is a serving plugin's signed account of one tool it planned or executed, as Marmot checked it. Status is dataquery's: verified, invalid with the reason in Problem, or missing. It carries the signed bytes, signature and certificate so it can be verified again later against the instance's CA.
type ToolReceipt struct {
	Status    string `json:"status"`
	Problem   string `json:"problem,omitempty"`
	Enrolment string `json:"enrolment,omitempty"`
	// Operation is tool_plan or tool_execute.
	Operation      string     `json:"operation,omitempty"`
	ID             string     `json:"id,omitempty"`
	Origin         string     `json:"origin,omitempty"`
	Source         string     `json:"source,omitempty"`
	Actor          string     `json:"actor,omitempty"`
	Tool           string     `json:"tool,omitempty"`
	ParamsSHA256   string     `json:"params_sha256,omitempty"`
	Precondition   string     `json:"precondition,omitempty"`
	ManifestDigest string     `json:"manifest_digest,omitempty"`
	Nonce          string     `json:"nonce,omitempty"`
	Target         string     `json:"target,omitempty"`
	Summary        string     `json:"summary,omitempty"`
	Error          string     `json:"error,omitempty"`
	OutputSHA256   string     `json:"output_sha256,omitempty"`
	DurationMS     int64      `json:"duration_ms"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	// ApprovalDigest is the hex ApprovalDigest the serving process verified the approval over, Approvers who approved, as the approval named them, and ApprovalTrust how it trusted them ("marmot": Marmot-signed); all empty for a tool that needed and carried no approval.
	ApprovalDigest string            `json:"approval_digest,omitempty"`
	Approvers      []ReceiptApprover `json:"approvers,omitempty"`
	ApprovalTrust  string            `json:"approval_trust,omitempty"`
	Signed         []byte            `json:"signed,omitempty"`
	Signature      []byte            `json:"signature,omitempty"`
	Certificate    []byte            `json:"certificate,omitempty"`
}

// ReceiptApprover is one person the approval named.
type ReceiptApprover struct {
	UserID     string    `json:"user_id"`
	Name       string    `json:"name"`
	ApprovedAt time.Time `json:"approved_at"`
}

// Verified reports whether the receipt checked out.
func (r *ToolReceipt) Verified() bool {
	return r != nil && r.Status == dataquery.ReceiptVerified
}

// verifyToolReceipt checks the receipt on a tool's response against what this replica asked for, as verifyReceipt does for a query, and also that it covers the very output and failure the response carries.
func (h *Hub) verifyToolReceipt(ctx context.Context, resp *pb.ToolResponse, exp receiptExpectation) *ToolReceipt {
	signed := resp.Receipt
	if signed == nil {
		return &ToolReceipt{Status: dataquery.ReceiptMissing, Problem: "the serving plugin sent no receipt"}
	}
	out := &ToolReceipt{Status: dataquery.ReceiptInvalid, Signed: signed.Receipt, Signature: signed.Signature, Certificate: signed.Certificate}
	var r pb.ToolReceipt
	parseErr := gproto.Unmarshal(signed.Receipt, &r)
	if parseErr == nil {
		fillToolReceipt(out, &r)
	}
	enrolment, err := h.checkReceiptSigner(ctx, signed, toolReceiptDomain, exp)
	out.Enrolment = enrolment
	if err == nil && parseErr != nil {
		err = errors.New("it does not parse")
	}
	if err == nil {
		err = checkToolReceipt(&r, resp, exp.fq)
	}
	if err != nil {
		out.Problem = err.Error()
		return out
	}
	out.Status = dataquery.ReceiptVerified
	return out
}

func checkToolReceipt(r *pb.ToolReceipt, resp *pb.ToolResponse, fq *pb.ForwardedQuery) error {
	params := sha256.Sum256(fq.ParamsJson)
	output := sha256.Sum256(resp.OutputJson)
	switch {
	case r.Version != receiptVersion:
		return fmt.Errorf("unsupported version %d", r.Version)
	case r.Operation != fq.Operation:
		return errors.New("it is for another operation")
	case subtle.ConstantTimeCompare(r.Nonce, fq.Nonce) != 1:
		return errors.New("it is for another tool: the nonce differs")
	case r.Id != fq.QueryId, r.Source != fq.Source, r.Actor != fq.Actor, r.Origin != fq.Origin, r.Tool != fq.Tool:
		return errors.New("its id, source, actor, origin or tool differ from the request's")
	case !bytes.Equal(r.ParamsSha256, params[:]):
		return errors.New("it names other parameters")
	case r.Precondition != fq.Precondition:
		return errors.New("it names another precondition")
	case r.ManifestDigest != fq.ManifestDigest:
		return errors.New("it names another manifest")
	case !bytes.Equal(r.OutputSha256, output[:]):
		return errors.New("it covers another output than the one that arrived")
	case (r.Error != "") != (resp.Code != 0):
		return errors.New("it disagrees with the response about whether the plugin failed")
	}
	return checkReceiptApproval(r, fq)
}

// checkReceiptApproval checks the receipt names the approval this replica sent, by the digest it computes itself, or none when it sent none.
func checkReceiptApproval(r *pb.ToolReceipt, fq *pb.ForwardedQuery) error {
	if len(fq.Approval) == 0 {
		if len(r.ApprovalDigest) > 0 || len(r.Approvers) > 0 || r.ApprovalTrust != "" {
			return errors.New("it names an approval the request did not carry")
		}
		return nil
	}
	if len(r.ApprovalDigest) == 0 {
		// An approval on a write the policy lets run with approvals: 0 is still verified, so an execute that carried one always names it.
		return errors.New("it names no approval, though the request carried one")
	}
	var a pb.Approval
	if err := gproto.Unmarshal(fq.Approval, &a); err != nil {
		return errors.New("the request's approval does not parse")
	}
	want, err := ApprovalDigest(ApprovalSubject{
		Source: fq.Source, Tool: fq.Tool, Params: fq.ParamsJson, Precondition: fq.Precondition,
		ManifestDigest: fq.ManifestDigest, Origin: fq.Origin, Actor: fq.Actor, ToolID: fq.QueryId,
		ExpiresAt: time.Unix(a.ExpiresAtUnix, 0),
	})
	if err != nil || subtle.ConstantTimeCompare(want, r.ApprovalDigest) != 1 {
		return errors.New("it names another approval than the one sent")
	}
	if r.ApprovalTrust != ApprovalTrustMarmot {
		return fmt.Errorf("it trusted the approval as %q, not as Marmot-signed", r.ApprovalTrust)
	}
	// The receipt must name exactly the people the approval named, so the record of who approved is the runner's as well as Marmot's.
	if len(r.Approvers) != len(a.Approvers) {
		return errors.New("it names other approvers than the approval")
	}
	for i, ap := range r.Approvers {
		if ap.UserId != a.Approvers[i].UserId || ap.Name != a.Approvers[i].Name || ap.ApprovedAtUnix != a.Approvers[i].ApprovedAtUnix {
			return errors.New("it names other approvers than the approval")
		}
	}
	return nil
}

func fillToolReceipt(out *ToolReceipt, r *pb.ToolReceipt) {
	out.Operation = operationName(r.Operation)
	out.ID = r.Id
	out.Origin = r.Origin
	out.Source = r.Source
	out.Actor = r.Actor
	out.Tool = r.Tool
	out.ParamsSHA256 = hex.EncodeToString(r.ParamsSha256)
	out.Precondition = r.Precondition
	out.ManifestDigest = r.ManifestDigest
	out.Nonce = hex.EncodeToString(r.Nonce)
	out.Target = r.Target
	out.Summary = r.Summary
	out.Error = r.Error
	out.OutputSHA256 = hex.EncodeToString(r.OutputSha256)
	out.DurationMS = r.DurationMs
	if r.FinishedAtUnix != 0 {
		finished := time.Unix(r.FinishedAtUnix, 0).UTC()
		out.FinishedAt = &finished
	}
	if len(r.ApprovalDigest) > 0 {
		out.ApprovalDigest = hex.EncodeToString(r.ApprovalDigest)
	}
	for _, a := range r.Approvers {
		out.Approvers = append(out.Approvers, ReceiptApprover{UserID: a.UserId, Name: a.Name, ApprovedAt: time.Unix(a.ApprovedAtUnix, 0).UTC()})
	}
	out.ApprovalTrust = r.ApprovalTrust
}
