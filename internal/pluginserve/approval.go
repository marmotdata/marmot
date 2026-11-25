package pluginserve

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"time"

	gproto "google.golang.org/protobuf/proto"

	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

// Approvals say which people agreed to a write. Today they are Marmot-signed: Marmot collects the approvals, enforcing who may approve and the two-person rule, and signs the result with an approval key of its own, which the serving process pins from enrolment like the query signing keys. The process recomputes what was approved from the request it is about to run and re-checks the count of distinct approvers, so a stolen query signing key cannot approve anything, and any approval can be spent only once, before it expires, on the very action and state it names. A Marmot taken over with its approval key can approve on people's behalf; trust through the company's own identity provider is planned to close that.

// ApprovalDigestVersion versions ApprovalDigest's encoding. A change to the encoding is a new version, so an approval made under one can never verify under another.
const ApprovalDigestVersion = 1

// approvalDomain starts every approval digest, so it can never equal a hash of anything else.
var approvalDomain = []byte("marmot action approval\x00")

// ApprovalSubject is everything an approval covers: the exact action a person is shown and agrees to. Marmot signs the digest of it; the serving process computes it again from the request it received, and only that one is ever checked.
type ApprovalSubject struct {
	Source string
	Action string
	// Params is the action's parameters as a JSON object, or empty for none. It is canonicalised before hashing, so any encoding of the same scalars gives the same digest.
	Params []byte
	// Precondition is the token from the plan the person reviewed.
	Precondition   string
	ManifestDigest string
	Origin         string
	// Actor is the audit subject the action will run as, so an approval for a task's action cannot be spent under another name.
	Actor string
	// ActionID is the action's id in Marmot's log.
	ActionID  string
	ExpiresAt time.Time
}

// ApprovalSubjectFor is the subject of an ActionCall, encoding its parameters exactly as the hub does when it sends them.
func ApprovalSubjectFor(call ActionCall, expiresAt time.Time) (ApprovalSubject, error) {
	var params []byte
	if len(call.Request.Params) > 0 {
		data, err := json.Marshal(call.Request.Params)
		if err != nil {
			return ApprovalSubject{}, fmt.Errorf("encoding the action's parameters: %w", err)
		}
		params = data
	}
	return ApprovalSubject{
		Source:         call.Source,
		Action:         call.Request.Action,
		Params:         params,
		Precondition:   call.Request.Precondition,
		ManifestDigest: call.ManifestDigest,
		Origin:         call.Request.Origin,
		Actor:          call.Request.Identity,
		ActionID:       call.ID,
		ExpiresAt:      expiresAt,
	}, nil
}

// ApprovalDigest returns the 32 bytes an approval is about, which Marmot's approval signature covers.
//
// Version 1 is SHA-256 over "marmot action approval\x00", then the version as a big-endian uint32, then source, action, canonical params, precondition, manifest digest, origin, actor and action id, each as a big-endian uint32 length followed by its bytes, then expires_at as big-endian int64 unix seconds. Canonical params are the JSON object with keys sorted, whole numbers in decimal, and strings and booleans as encoding/json writes them; nothing else is a valid parameter, so anything else fails. Length prefixes make the encoding unambiguous: no two subjects share a digest by moving bytes between fields.
func ApprovalDigest(s ApprovalSubject) ([]byte, error) {
	params, err := canonicalParams(s.Params)
	if err != nil {
		return nil, err
	}
	if s.ExpiresAt.IsZero() {
		return nil, errors.New("an approval needs an expiry")
	}
	h := sha256.New()
	h.Write(approvalDomain)
	_ = binary.Write(h, binary.BigEndian, uint32(ApprovalDigestVersion))
	for _, field := range [][]byte{
		[]byte(s.Source), []byte(s.Action), params, []byte(s.Precondition),
		[]byte(s.ManifestDigest), []byte(s.Origin), []byte(s.Actor), []byte(s.ActionID),
	} {
		_ = binary.Write(h, binary.BigEndian, uint32(len(field))) //nolint:gosec // G115: every field is bounded far below 4 GiB by maxApprovalBytes and the query limits
		h.Write(field)
	}
	_ = binary.Write(h, binary.BigEndian, s.ExpiresAt.Unix())
	return h.Sum(nil), nil
}

// canonicalParams re-encodes an action's parameters so the hub's encoding and the serving process's agree whatever decoder sat between them. Parameters are scalars only, so a nested value, null or a fraction is refused rather than given some canonical form.
func canonicalParams(data []byte) ([]byte, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return []byte("{}"), nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return nil, errors.New("the action's parameters are not a JSON object")
	}
	if dec.More() {
		return nil, errors.New("the action's parameters are followed by more JSON")
	}
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		switch x := v.(type) {
		case string, bool:
			out[k] = x
		case json.Number:
			n, err := strconv.ParseInt(string(x), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parameter %s is not a whole number", k)
			}
			out[k] = n
		default:
			return nil, fmt.Errorf("parameter %s is not a string, whole number or boolean", k)
		}
	}
	// encoding/json writes map keys sorted.
	return json.Marshal(out)
}

// ApprovalTrustMarmot is the one approval trust mode today: Marmot signs, with its approval key, which people approved. The serving process trusts that signature, not the people directly, so a Marmot taken over can approve on anyone's behalf; what it cannot do is spend an approval on any other action, twice, late, or with fewer people than the policy asks for. Trust through the company's own identity provider (oidc) is planned.
const ApprovalTrustMarmot = "marmot"

// approvalSignatureDomain starts every approval signature, so the approval key's signature over anything else can never be an approval. The approval key is also its own key, never the query signing key, so nothing signed as a query can be one either.
var approvalSignatureDomain = []byte("marmot action approval signature v1\x00")

const (
	// maxApprovalAge is the most a policy may set max_age to.
	maxApprovalAge = time.Hour
	// defaultApprovalAge applies when the policy sets no max_age.
	defaultApprovalAge = 15 * time.Minute
	// maxApprovalBytes bounds a serialised Approval before it is parsed.
	maxApprovalBytes = 16 << 10
	// maxApprovalApprovers bounds the people one approval names.
	maxApprovalApprovers = 8
	maxApproverIDLength  = 64
	maxApproverNameBytes = 255
	maxApprovalSig       = 128
)

// Approval is Marmot's statement that people approved one action, as the hub sends it: when it expires, who approved and Marmot's signature over both and the action's digest. Build it with Hub.SignApproval.
type Approval struct {
	ExpiresAt time.Time  `json:"expires_at"`
	Approvers []Approver `json:"approvers"`
	Signature []byte     `json:"signature"`
}

// Approver is one person who approved, as Marmot identifies them.
type Approver struct {
	UserID     string    `json:"user_id"`
	Name       string    `json:"name"`
	ApprovedAt time.Time `json:"approved_at"`
}

// ApprovalSignedMessage is what the approval key signs: the domain, the action's ApprovalDigest (which covers its expiry), then the number of approvers as a big-endian uint32 and each approver as its user id and name, each a big-endian uint32 length and its bytes, and when it approved as big-endian int64 unix seconds.
func ApprovalSignedMessage(digest []byte, approvers []Approver) []byte {
	var b bytes.Buffer
	b.Write(approvalSignatureDomain)
	b.Write(digest)
	_ = binary.Write(&b, binary.BigEndian, uint32(len(approvers))) //nolint:gosec // G115: checkApprovers bounds approvers to maxApprovalApprovers
	for _, a := range approvers {
		for _, field := range []string{a.UserID, a.Name} {
			_ = binary.Write(&b, binary.BigEndian, uint32(len(field))) //nolint:gosec // G115: ids and names are bounded by checkApprovers
			b.WriteString(field)
		}
		_ = binary.Write(&b, binary.BigEndian, a.ApprovedAt.Unix())
	}
	return b.Bytes()
}

// checkApprovers bounds an approval's approvers: a few, each with a printable id and name, no person twice.
func checkApprovers(approvers []Approver) error {
	if len(approvers) == 0 || len(approvers) > maxApprovalApprovers {
		return fmt.Errorf("an approval names between 1 and %d approvers", maxApprovalApprovers)
	}
	seen := map[string]bool{}
	for i, a := range approvers {
		if a.UserID == "" || len(a.UserID) > maxApproverIDLength || !printable(a.UserID) {
			return fmt.Errorf("approver %d has no valid user id", i+1)
		}
		if len(a.Name) > maxApproverNameBytes || !printable(a.Name) {
			return fmt.Errorf("approver %d has an invalid name", i+1)
		}
		if seen[a.UserID] {
			return fmt.Errorf("approver %s is named twice", a.UserID)
		}
		seen[a.UserID] = true
	}
	return nil
}

func printable(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// SignApproval signs approval for call as it will be sent, with the active approval key. It refuses an approval the serving process would refuse for its shape, so a bad one fails here rather than at the runner.
func (h *Hub) SignApproval(call ActionCall, approval *Approval) error {
	if approval == nil {
		return errors.New("no approval to sign")
	}
	if err := checkApprovers(approval.Approvers); err != nil {
		return err
	}
	subject, err := ApprovalSubjectFor(call, approval.ExpiresAt)
	if err != nil {
		return err
	}
	digest, err := ApprovalDigest(subject)
	if err != nil {
		return err
	}
	sig, err := h.keyring.SignApproval(ApprovalSignedMessage(digest, approval.Approvers))
	if err != nil {
		return fmt.Errorf("signing the approval: %w", err)
	}
	approval.Signature = sig
	return nil
}

// encodeApproval serialises an approval for the signed Query, refusing early what every serving process would refuse.
func encodeApproval(a *Approval) ([]byte, error) {
	if a == nil {
		return nil, nil
	}
	if err := checkApprovers(a.Approvers); err != nil {
		return nil, err
	}
	if len(a.Signature) == 0 || len(a.Signature) > maxApprovalSig {
		return nil, errors.New("the approval is not signed")
	}
	msg := &pb.Approval{Version: ApprovalDigestVersion, ExpiresAtUnix: a.ExpiresAt.Unix(), Signature: a.Signature}
	for _, ap := range a.Approvers {
		msg.Approvers = append(msg.Approvers, &pb.Approver{UserId: ap.UserID, Name: ap.Name, ApprovedAtUnix: ap.ApprovedAt.Unix()})
	}
	data, err := gproto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	if len(data) > maxApprovalBytes {
		return nil, errors.New("the approval is too large")
	}
	return data, nil
}

// ApprovalsPolicy is how the serving process trusts approvals of its writes.
type ApprovalsPolicy struct {
	// Trust is how approvals are verified. "marmot", the only mode today, trusts approvals Marmot signed with its approval key, pinned from enrolment like the query signing keys. "oidc", approvals signed by the company's own identity provider, is planned.
	Trust string `yaml:"trust"`
	// MaxAge refuses an approval that expires further ahead than this, at most an hour; 15 minutes when unset.
	MaxAge time.Duration `yaml:"max_age"`
	// KeyFingerprint, when set, is the only approval key accepted: sha256:<hex> of its SubjectPublicKeyInfo, as `marmot admin serve-keys` shows it. Without it any approval key pinned from enrolment is.
	KeyFingerprint string `yaml:"key_fingerprint"`
}

// load checks the approvals section, so a mistake fails at start rather than refusing the first approval.
func (ap *ApprovalsPolicy) load() error {
	switch ap.Trust {
	case ApprovalTrustMarmot:
	case "":
		return errors.New("approvals: trust is required; the only value accepted today is marmot (Marmot-signed approvals)")
	case "oidc":
		return errors.New("approvals: trust: oidc is planned and not supported yet; use trust: marmot")
	default:
		return fmt.Errorf("approvals: trust %q is not supported; the only value accepted today is marmot", ap.Trust)
	}
	if ap.MaxAge == 0 {
		ap.MaxAge = defaultApprovalAge
	}
	if ap.MaxAge < 0 || ap.MaxAge > maxApprovalAge {
		return fmt.Errorf("approvals: max_age must be more than zero and at most %s", maxApprovalAge)
	}
	if ap.KeyFingerprint != "" {
		fp, err := ParseCAFingerprint(ap.KeyFingerprint)
		if err != nil {
			return errors.New("approvals: key_fingerprint must be sha256: followed by 64 hex digits")
		}
		ap.KeyFingerprint = fp
	}
	return nil
}

// Describe is how the startup log and the docs name the trust mode, so nobody mistakes it for stronger protection than it is.
func (ap *ApprovalsPolicy) Describe() string {
	if ap == nil {
		return "none: every write that needs approval is refused"
	}
	return "Marmot-signed"
}

// approvalNeed is how many distinct people an action needs and which of them count.
type approvalNeed struct {
	required int
	// approvers narrows who counts, by user id or name; empty means anyone Marmot names.
	approvers []string
}

// verifiedApproval is an approval that passed every check, ready to be claimed and recorded.
type verifiedApproval struct {
	digest    []byte
	expiresAt time.Time
	approvers []*pb.Approver
}

func (v *verifiedApproval) names() []string {
	if v == nil {
		return nil
	}
	names := make([]string, 0, len(v.approvers))
	for _, a := range v.approvers {
		names = append(names, a.Name+" ("+a.UserId+")")
	}
	return names
}

// ApprovalKeyFingerprint is the form an approval key is pinned in with approvals.key_fingerprint: the SHA-256 of its SubjectPublicKeyInfo, as sha256:<hex>.
func ApprovalKeyFingerprint(pub *ecdsa.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return ""
	}
	return CAFingerprint(der)
}

// verifyApproval checks an approval against the approval keys this process pinned and a digest computed from the request itself: the signature, the expiry and the policy's max_age, and enough distinct approvers that the action's own narrowing admits.
func (ap *ApprovalsPolicy) verifyApproval(data []byte, subject ApprovalSubject, need approvalNeed, keys []*ecdsa.PublicKey, now time.Time) (*verifiedApproval, error) {
	if len(data) == 0 {
		return nil, errors.New("the action needs an approval and carries none")
	}
	if ap == nil {
		return nil, errors.New("no approvers configured: the policy has no approvals section")
	}
	if len(data) > maxApprovalBytes {
		return nil, errors.New("the approval is too large")
	}
	var msg pb.Approval
	if err := gproto.Unmarshal(data, &msg); err != nil {
		return nil, errors.New("the approval does not parse")
	}
	if msg.Version != ApprovalDigestVersion {
		return nil, fmt.Errorf("approval version %d is not supported", msg.Version)
	}
	expires := time.Unix(msg.ExpiresAtUnix, 0)
	switch {
	case now.After(expires):
		return nil, errors.New("the approval has expired")
	case expires.Sub(now) > ap.MaxAge:
		return nil, fmt.Errorf("the approval expires more than the policy's max_age of %s ahead", ap.MaxAge)
	}
	approvers := make([]Approver, 0, len(msg.Approvers))
	for _, a := range msg.Approvers {
		approvers = append(approvers, Approver{UserID: a.UserId, Name: a.Name, ApprovedAt: time.Unix(a.ApprovedAtUnix, 0)})
	}
	if err := checkApprovers(approvers); err != nil {
		return nil, err
	}
	subject.ExpiresAt = expires
	digest, err := ApprovalDigest(subject)
	if err != nil {
		return nil, err
	}
	trusted := keys
	if ap.KeyFingerprint != "" {
		trusted = nil
		for _, k := range keys {
			if ApprovalKeyFingerprint(k) == ap.KeyFingerprint {
				trusted = append(trusted, k)
			}
		}
	}
	if len(trusted) == 0 {
		return nil, errors.New("no approval key is pinned that the policy accepts")
	}
	if len(msg.Signature) == 0 || len(msg.Signature) > maxApprovalSig ||
		!signedByAny(trusted, nil, ApprovalSignedMessage(digest, approvers), msg.Signature) {
		// A signature over another digest is an approval of something else, or not Marmot's approval key at all.
		return nil, errors.New("it is not signed by Marmot's approval key for this action")
	}
	counted := 0
	for _, a := range approvers {
		if a.ApprovedAt.After(expires) {
			return nil, fmt.Errorf("approver %s approved after the approval expired", a.UserID)
		}
		if len(need.approvers) == 0 || slices.Contains(need.approvers, a.UserID) || slices.Contains(need.approvers, a.Name) {
			counted++
		}
	}
	if counted < need.required {
		return nil, fmt.Errorf("the action needs %d distinct approvers the policy admits and the approval has %d", need.required, counted)
	}
	out := &verifiedApproval{digest: digest, expiresAt: expires}
	for _, a := range msg.Approvers {
		out.approvers = append(out.approvers, &pb.Approver{UserId: a.UserId, Name: a.Name, ApprovedAtUnix: a.ApprovedAtUnix})
	}
	return out, nil
}

// spentApprovals remembers every approval digest accepted until it expires, so an approval runs at most once. The audit log holds the same digests, and a restart reads them back, so restarting the process does not make an approval usable again.
type spentApprovals struct {
	mu    sync.Mutex
	spent map[string]time.Time
}

func newSpentApprovals() *spentApprovals {
	return &spentApprovals{spent: map[string]time.Time{}}
}

// claim records digest as spent until expires and reports whether it was not spent already. Expired entries are dropped as it goes, since an expired approval is refused anyway.
func (s *spentApprovals) claim(digest []byte, expires, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for d, t := range s.spent {
		if now.After(t.Add(envelopeSkew)) {
			delete(s.spent, d)
		}
	}
	key := hex.EncodeToString(digest)
	if _, ok := s.spent[key]; ok {
		return false
	}
	s.spent[key] = expires
	return true
}

// load records the digests of approvals the audit log says were spent and have not yet expired.
func (s *spentApprovals) load(entries []AuditEntry, now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, e := range entries {
		if e.ApprovalDigest == "" || e.ApprovalExpiresAt == nil || now.After(e.ApprovalExpiresAt.Add(envelopeSkew)) {
			continue
		}
		s.spent[e.ApprovalDigest] = *e.ApprovalExpiresAt
		n++
	}
	return n
}
