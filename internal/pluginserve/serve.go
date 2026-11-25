package pluginserve

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/yamux"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

const (
	dialTimeout      = 15 * time.Second
	maxBackoff       = time.Minute
	minRenewInterval = 5 * time.Second
)

// LocalSource is a source this process serves, with its real config; that config never leaves the process.
type LocalSource struct {
	Name     string
	PluginID string
	Config   map[string]any
	// DisplayConfig is the masked config sent to Marmot for reference.
	DisplayConfig map[string]any
}

// Querier runs queries on local plugin processes. Implemented by plugin.InstanceManager.
type Querier interface {
	PlanQuery(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error)
	ExecuteQuery(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.QueryRequest) (pluginsdk.QueryStream, error)
}

// EnrolFunc sends an enrolment to Marmot's API with the process's ordinary Marmot credential.
type EnrolFunc func(ctx context.Context, in EnrolInput) (*EnrolResult, error)

// ServeOptions configures a serving process.
type ServeOptions struct {
	// Host is the Marmot host name, dialled on the enrolment's port when Marmot does not name an address.
	Host    string
	Sources []LocalSource
	Enrol   EnrolFunc
	Querier Querier
	// CAFingerprints, normalised by ParseCAFingerprint, are CAs the first enrolment must include one of before anything it returns is trusted. Without them the process trusts whatever its first enrolment over HTTPS returns, and logs the fingerprints so they can be passed next time.
	CAFingerprints []string
	// Policy, when set, is the process's own policy, applied to every query after Marmot's signature is checked.
	Policy *Policy
	// AuditLog, when set, receives a hash-chained line for every query served or refused.
	AuditLog *AuditLog
	// AllowTools switches on serving tools, which are refused outright without it. It is only the first of the runner's gates: each tool must also be named by Policy, and Serve refuses to start with it unless AuditLog is set, since no tool runs without its intent written down first.
	AllowTools bool
	// PluginTools runs tools on local plugin processes. Only needed with AllowTools; implemented by plugin.ToolHost.
	PluginTools PluginTools
	// Watcher runs the triggers the policy names on local plugin processes. Triggers run only with AllowTools, like tools; implemented by plugin.ToolHost.
	Watcher TriggerWatcher
}

// PluginTools runs tools on local plugin processes. Implemented by plugin.ToolHost. gate is run against the manifest of the process that will act, after it is started and before the plugin is called; when gate fails the plugin is not called and its error is returned as it was.
type PluginTools interface {
	ToolManifest(ctx context.Context, pluginID string) ([]pluginsdk.ToolSpec, error)
	PlanTool(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.ToolRequest, gate func([]pluginsdk.ToolSpec) error) (*pluginsdk.ToolPlan, error)
	ExecuteTool(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.ToolRequest, gate func([]pluginsdk.ToolSpec) error) (*pluginsdk.ToolResult, error)
}

// ErrCAFingerprintMismatch is returned when the first enrolment's CAs include none the operator named. Serve gives up rather than retrying, because only the operator can tell whether the fingerprint or the Marmot it reached is wrong.
var ErrCAFingerprintMismatch = errors.New("none of the CAs Marmot returned matches --ca-fingerprint; compare with `marmot admin serve-keys` and check MARMOT_HOST names the Marmot you meant")

// pins are the CAs and query signing keys this process trusts. They are taken from its first enrolment and carried forward through Marmot's key rotations: a later enrolment is accepted only for the same instance and only if it still includes at least one CA and one signing key already trusted, so the trust can move to new keys Marmot announced in advance but can never be replaced outright.
type pins struct {
	instanceID string
	cas        map[string]bool
	keys       map[string]bool
	// approvalKeys are pinned the same way, apart from the signing keys so neither can stand in for the other. Once any is pinned, a later enrolment must keep one of them.
	approvalKeys map[string]bool
}

// trustedBundle is an enrolment's CAs and signing keys, parsed.
type trustedBundle struct {
	pool         *x509.CertPool
	cas          map[string]bool
	fingerprints []string
	keys         []*ecdsa.PublicKey
	raw          map[string]bool
	approvalKeys []*ecdsa.PublicKey
	approvalRaw  map[string]bool
}

func parseBundle(res *EnrolResult) (*trustedBundle, error) {
	b := &trustedBundle{pool: x509.NewCertPool(), cas: map[string]bool{}, raw: map[string]bool{}, approvalRaw: map[string]bool{}}
	for rest := []byte(res.CABundle); ; {
		var block *pem.Block
		if block, rest = pem.Decode(rest); block == nil {
			break
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil || !cert.IsCA {
			return nil, errors.New("enrolment returned an invalid CA")
		}
		b.pool.AddCert(cert)
		b.cas[string(cert.Raw)] = true
		b.fingerprints = append(b.fingerprints, CAFingerprint(cert.Raw))
	}
	for _, keyPEM := range res.SigningKeys {
		block, _ := pem.Decode([]byte(keyPEM))
		if block == nil {
			return nil, errors.New("enrolment returned an invalid signing key")
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		key, ok := parsed.(*ecdsa.PublicKey)
		if err != nil || !ok || key.Curve != elliptic.P256() {
			return nil, errors.New("enrolment returned an invalid signing key")
		}
		b.keys = append(b.keys, key)
		b.raw[string(block.Bytes)] = true
	}
	if len(b.cas) == 0 || len(b.keys) == 0 {
		return nil, errors.New("enrolment returned no CA or signing key")
	}
	for _, keyPEM := range res.ApprovalKeys {
		block, _ := pem.Decode([]byte(keyPEM))
		if block == nil {
			return nil, errors.New("enrolment returned an invalid approval key")
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		key, ok := parsed.(*ecdsa.PublicKey)
		if err != nil || !ok || key.Curve != elliptic.P256() {
			return nil, errors.New("enrolment returned an invalid approval key")
		}
		if b.raw[string(block.Bytes)] {
			return nil, errors.New("enrolment returned a signing key as an approval key")
		}
		b.approvalKeys = append(b.approvalKeys, key)
		b.approvalRaw[string(block.Bytes)] = true
	}
	return b, nil
}

// serveClientTLS is how a serving process dials Marmot's serve port. The address's host goes out as SNI so a shared TLS passthrough proxy can route the connection to the right instance, but the certificate is checked against the fixed replica name only this instance's CA issues, never against the host, so the check holds behind any load balancer or proxy.
func serveClientTLS(roots *x509.CertPool, instanceID, address string, cert tls.Certificate) *tls.Config {
	sni, _, err := net.SplitHostPort(address)
	if err != nil {
		sni = address
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		ServerName:   sni,
		Certificates: []tls.Certificate{cert},
		// ServerName now names the route rather than the peer, so the default check, which matches the certificate to it, is replaced by VerifyConnection below.
		InsecureSkipVerify: true, //nolint:gosec // verified in VerifyConnection against roots and the replica name
		VerifyConnection: func(cs tls.ConnectionState) error {
			_, err := verifyReplicaChain(roots, instanceID, cs.PeerCertificates)
			return err
		},
	}
}

// verifyReplicaChain verifies the certificates a Marmot replica presented: issued by one of roots, for server use, under this instance's replica name.
func verifyReplicaChain(roots *x509.CertPool, instanceID string, certs []*x509.Certificate) ([][]*x509.Certificate, error) {
	if len(certs) == 0 {
		return nil, errors.New("Marmot presented no certificate")
	}
	intermediates := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}
	chains, err := certs[0].Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		DNSName:       ServerNameFor(instanceID),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	if err != nil {
		return nil, fmt.Errorf("verifying Marmot's serve certificate: %w", err)
	}
	return chains, nil
}

// acceptBundle checks an enrolment's bundle against the pins and moves the pins on to it. The first bundle has nothing to overlap with, so it is checked against the operator's fingerprints when there are any.
func acceptBundle(p **pins, instanceID string, b *trustedBundle, fingerprints []string) error {
	if *p != nil {
		current := *p
		if current.instanceID != instanceID || !overlaps(current.cas, b.cas) || !overlaps(current.keys, b.raw) {
			return errors.New("marmot's serving identity changed with no overlap with the one this process trusts; restart it to trust the new identity")
		}
		// Approval keys move on only through an overlapping rotation, like the signing keys, and once pinned can never be dropped: an enrolment without one, then one with a new key, would otherwise replace them outright.
		if len(current.approvalKeys) > 0 && !overlaps(current.approvalKeys, b.approvalRaw) {
			return errors.New("marmot's approval key was replaced with no overlap with the one this process trusts; restart it to trust the new key")
		}
		if len(current.approvalKeys) == 0 && len(b.approvalRaw) > 0 {
			logApprovalKeys("Pinning Marmot's approval keys, which it did not have before", b.approvalKeys)
		}
	} else {
		if err := checkFirstBundle(b, fingerprints); err != nil {
			return err
		}
		if len(b.approvalKeys) > 0 {
			logApprovalKeys("Trusting Marmot's approval keys from the first enrolment; pin one with approvals.key_fingerprint to check it", b.approvalKeys)
		}
	}
	*p = &pins{instanceID: instanceID, cas: b.cas, keys: b.raw, approvalKeys: b.approvalRaw}
	return nil
}

func logApprovalKeys(msg string, keys []*ecdsa.PublicKey) {
	fps := make([]string, 0, len(keys))
	for _, k := range keys {
		fps = append(fps, ApprovalKeyFingerprint(k))
	}
	log.Info().Strs("approval_key_fingerprints", fps).Msg(msg)
}

// checkFirstBundle is the only point where trust is taken rather than carried forward, so it is where the operator's fingerprints apply, and where, without them, the operator is shown what was trusted.
func checkFirstBundle(b *trustedBundle, fingerprints []string) error {
	if len(fingerprints) == 0 {
		log.Info().Strs("ca_fingerprints", b.fingerprints).Msg("Trusting Marmot's serving CAs from the first enrolment; pass one with --ca-fingerprint to check it before trusting it")
		return nil
	}
	for _, fp := range b.fingerprints {
		if slices.Contains(fingerprints, fp) {
			log.Info().Str("ca_fingerprint", fp).Msg("Marmot's serving CA matches --ca-fingerprint")
			return nil
		}
	}
	return fmt.Errorf("%w (Marmot returned %s)", ErrCAFingerprintMismatch, strings.Join(b.fingerprints, ", "))
}

func overlaps(a, b map[string]bool) bool {
	for k := range b {
		if a[k] {
			return true
		}
	}
	return false
}

// Serve enrols, connects to Marmot and answers queries until ctx is cancelled. Each certificate is replaced at half-life by enrolling again; a dropped connection is retried with capped exponential backoff.
func Serve(ctx context.Context, opts ServeOptions) error {
	if unknown := opts.Policy.unknownSources(opts.Sources); len(unknown) > 0 {
		log.Warn().Strs("sources", unknown).Str("policy", opts.Policy.Name()).Msg("The policy names sources this process does not serve")
	}
	if opts.Policy != nil {
		log.Info().Str("policy", opts.Policy.Name()).Msg("Applying a local policy; sources it does not name are refused")
	}
	if opts.AllowTools {
		// Said at start and on every receipt, so nobody takes a Marmot-signed approval for a person's own signature.
		log.Info().Str("approvals", opts.Policy.approvals().Describe()).Msg("Approvals of writes: " + opts.Policy.approvals().Describe())
	}
	state, err := startProcess(ctx, opts)
	if err != nil {
		return err
	}

	var pinned *pins
	backoff := time.Second
	for {
		conn, err := connect(ctx, opts, state, &pinned)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, ErrCAFingerprintMismatch) {
				return err
			}
			log.Warn().Err(err).Dur("retry_in", backoff).Msg("Could not connect to Marmot")
			if !sleep(ctx, jitter(backoff)) {
				return nil
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}
		backoff = time.Second
		log.Info().Strs("sources", sourceNames(opts.Sources)).Msg("Serving queries")

		if !serveUntilDropped(ctx, opts, state, &pinned, conn) {
			return nil
		}
	}
}

// serveUntilDropped serves on conn, replacing it with a freshly enrolled connection at each half-life, until the connection drops (true) or ctx ends (false). A failed renewal keeps the current connection and tries again at half the remaining lifetime.
func serveUntilDropped(ctx context.Context, opts ServeOptions, state *processState, pinned **pins, conn *servingConn) bool {
	for {
		renewIn := max(time.Until(conn.expiresAt)/2, minRenewInterval)
		select {
		case <-ctx.Done():
			conn.close()
			return false
		case <-conn.mux.CloseChan():
			log.Warn().Msg("Connection to Marmot closed, reconnecting")
			conn.close()
			return true
		case <-time.After(renewIn):
			next, err := connect(ctx, opts, state, pinned)
			if err != nil {
				log.Warn().Err(err).Msg("Could not renew certificate, will retry")
				continue
			}
			// The new connection is up before the old one closes, so queries keep flowing; queries already running on the old one finish first.
			conn.drainAndClose()
			conn = next
		}
	}
}

// servingConn is one live connection to a Marmot replica.
type servingConn struct {
	mux       *yamux.Session
	server    *grpc.Server
	expiresAt time.Time
}

func (c *servingConn) close() {
	c.server.Stop()
	_ = c.mux.Close()
}

// drainAndClose lets in-flight queries finish before closing, bounded by the certificate's expiry when Marmot closes the connection anyway.
func (c *servingConn) drainAndClose() {
	go func() {
		c.server.GracefulStop()
		_ = c.mux.Close()
	}()
}

// connect enrols a fresh key and opens a mutual TLS connection to Marmot with it.
func connect(ctx context.Context, opts ServeOptions, process *processState, pinned **pins) (*servingConn, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating key: %w", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	if err != nil {
		return nil, fmt.Errorf("creating csr: %w", err)
	}

	in := EnrolInput{CSR: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}))}
	for _, s := range opts.Sources {
		in.Sources = append(in.Sources, EnrolSource{Name: s.Name, PluginID: s.PluginID, Config: s.DisplayConfig, Tools: servedToolsFor(ctx, opts, s)})
	}
	res, err := opts.Enrol(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("enrolling: %w", err)
	}
	bundle, err := parseBundle(res)
	if err != nil {
		return nil, err
	}
	if err := acceptBundle(pinned, res.InstanceID, bundle, opts.CAFingerprints); err != nil {
		return nil, err
	}

	certBlock, _ := pem.Decode([]byte(res.Certificate))
	if certBlock == nil {
		return nil, errors.New("enrolment returned no certificate")
	}

	address := res.Address
	if address == "" {
		address = net.JoinHostPort(opts.Host, res.Port)
	}
	tlsConfig := serveClientTLS(bundle.pool, res.InstanceID, address, tls.Certificate{Certificate: [][]byte{certBlock.Bytes}, PrivateKey: key})

	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: dialTimeout}, Config: tlsConfig}
	raw, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", address, err)
	}
	conn := raw.(*tls.Conn)

	binding, err := channelBinding(conn.ConnectionState())
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	mux, err := yamux.Server(conn, serveMuxConfig())
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("starting multiplexer: %w", err)
	}

	audience := "spiffe://" + res.InstanceID + "/serve/" + res.EnrolmentID
	// The gRPC server runs on yamux streams inside the mutual TLS connection just made. gRPC is handed that connection's verified state rather than running a handshake of its own, and every call is checked to come from a replica of the Marmot this process enrolled with.
	state := conn.ConnectionState()
	// The handshake verified the peer in VerifyConnection, which Go does not record, so the chains are rebuilt the same way for gRPC and the per-call checks to see.
	state.VerifiedChains, err = verifyReplicaChain(bundle.pool, res.InstanceID, state.PeerCertificates)
	if err != nil {
		_ = mux.Close()
		return nil, err
	}
	server := grpc.NewServer(append(requirePeer(isReplicaOf(res.InstanceID)), grpc.Creds(establishedTLS{state: &state}))...)
	pb.RegisterPluginServer(server, &pluginService{
		verifier:     NewVerifier(bundle.keys, res.InstanceID, audience, binding),
		approvalKeys: bundle.approvalKeys,
		sources:      sourcesByName(opts.Sources),
		querier:      opts.Querier,
		policy:       opts.Policy,
		audit:        opts.AuditLog,
		signer:       receiptSigner{key: key, leaf: certBlock.Bytes},
		runner:       opts.PluginTools,
		tools:        opts.AllowTools,
		spent:        process.spent,
		triggers:     process.triggers,
	})
	go func() { _ = server.Serve(mux) }()

	return &servingConn{mux: mux, server: server, expiresAt: res.ExpiresAt}, nil
}

func serveMuxConfig() *yamux.Config {
	cfg := yamux.DefaultConfig()
	cfg.LogOutput = io.Discard
	return cfg
}

// pluginService answers Marmot's signed queries from local plugin processes.
type pluginService struct {
	pb.UnimplementedPluginServer
	verifier *Verifier
	sources  map[string]LocalSource
	querier  Querier
	policy   *Policy
	audit    *AuditLog
	signer   receiptSigner
	runner   PluginTools
	// tools is --allow-tools.
	tools bool
	// approvalKeys are the approval keys pinned from this connection's enrolment.
	approvalKeys []*ecdsa.PublicKey
	// spent is the process's record of approvals already used, shared by every connection, since a connection lasts only until the next renewal.
	spent *spentApprovals
	// triggers hands this connection's WatchTriggers stream the process's trigger events.
	triggers *triggerDispatcher
}

// accept verifies a signed query, checks this process serves its source and applies the local policy, lowering the query's row limit when the policy says so. Every refusal is logged and audited.
func (p *pluginService) accept(signed *pb.SignedQuery, op pb.Operation) (*pb.Query, LocalSource, policyDecision, error) {
	q, err := p.verifier.Verify(signed, op)
	if err != nil {
		log.Warn().Err(err).Msg("Refused query")
		p.record(AuditEntry{Event: AuditRefused, Operation: operationName(op), Reason: err.Error()}, nil)
		return nil, LocalSource{}, policyDecision{}, status.Error(codes.PermissionDenied, ErrRejected.Error())
	}
	src, ok := p.sources[q.Source]
	if !ok {
		p.record(AuditEntry{Event: AuditRefused, Operation: operationName(op), Reason: "source not served"}, q)
		return nil, LocalSource{}, policyDecision{}, status.Errorf(codes.NotFound, "this process does not serve %q", q.Source)
	}
	decision := p.policy.decide(q)
	if !decision.allowed {
		log.Warn().Str("query_id", q.QueryId).Str("actor", q.Actor).Str("source", q.Source).Str("reason", decision.reason).Str("policy", p.policy.Name()).Msg("Local policy refused query")
		p.record(AuditEntry{Event: AuditRefused, Operation: operationName(op), Policy: decision.String(), Reason: decision.reason}, q)
		return nil, LocalSource{}, decision, status.Errorf(codes.PermissionDenied, "refused by the serving process's local policy %s: %s", p.policy.Name(), decision.reason)
	}
	q.MaxRows = decision.maxRows
	return q, src, decision, nil
}

// record appends to the audit log, when there is one, filling in what the query says. A failed write is logged rather than failing the query: the query has already run or been refused by then. A tool's intent is the exception, written with recordIntent, which fails the tool instead.
func (p *pluginService) record(e AuditEntry, q *pb.Query) {
	if p.audit == nil {
		return
	}
	if err := p.audit.Append(auditEntryFor(e, q)); err != nil {
		log.Error().Err(err).Msg("Failed to write the audit log")
	}
}

// auditEntryFor fills in what the query or tool says.
func auditEntryFor(e AuditEntry, q *pb.Query) AuditEntry {
	// A query refused before the policy was reached, for a bad signature or a source not served here, was not decided by it.
	if e.Policy == "" {
		e.Policy = "not applied"
	}
	if q != nil {
		e.QueryID, e.Origin, e.Source, e.Actor, e.Nonce = q.QueryId, q.Origin, q.Source, q.Actor, hex.EncodeToString(q.Nonce)
		if isTool(q.Operation) {
			e.Tool, e.ParamsSHA256, e.Precondition = q.Tool, sha256Hex(q.ParamsJson), q.Precondition
			// The digest the process computed is kept when the entry has one; otherwise it is the one Marmot asked against.
			if e.ManifestDigest == "" {
				e.ManifestDigest = q.ManifestDigest
			}
		} else {
			e.StatementSHA256, e.MaxRows = sha256Hex([]byte(q.Statement)), q.MaxRows
		}
	}
	e.Time = time.Now().UTC()
	return e
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func operationName(op pb.Operation) string {
	switch op {
	case pb.Operation_OPERATION_PLAN:
		return AuditPlan
	case pb.Operation_OPERATION_EXECUTE:
		return AuditExecute
	case pb.Operation_OPERATION_TOOL_PLAN:
		return AuditToolPlan
	case pb.Operation_OPERATION_TOOL_EXECUTE:
		return AuditToolExecute
	}
	return ""
}

func policyLabel(p *Policy, d policyDecision) string {
	if p == nil {
		return "none"
	}
	return d.String()
}

func (p *pluginService) PlanQuery(ctx context.Context, signed *pb.SignedQuery) (*pb.PlanQueryResponse, error) {
	q, src, decision, err := p.accept(signed, pb.Operation_OPERATION_PLAN)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	resp, err := p.plan(ctx, src, q)
	entry := AuditEntry{Event: AuditPlan, Policy: policyLabel(p.policy, decision), Reason: decision.reason, DurationMS: time.Since(started).Milliseconds()}
	if err != nil {
		entry.Error = err.Error()
	}
	p.record(entry, q)
	return resp, err
}

func (p *pluginService) plan(ctx context.Context, src LocalSource, q *pb.Query) (*pb.PlanQueryResponse, error) {
	plan, err := p.querier.PlanQuery(ctx, src.PluginID, src.Config, requestFor(q))
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return &pb.PlanQueryResponse{}, nil
	}
	data, err := json.Marshal(plan)
	if err != nil {
		return nil, status.Error(codes.Internal, "encoding plan")
	}
	return &pb.PlanQueryResponse{PlanJson: data}, nil
}

// ExecuteQuery runs a query and ends the stream with a signed receipt, whether the query succeeded or not, when Marmot asked for one.
func (p *pluginService) ExecuteQuery(signed *pb.SignedQuery, out grpc.ServerStreamingServer[pb.QueryChunk]) error {
	q, src, decision, err := p.accept(signed, pb.Operation_OPERATION_EXECUTE)
	if err != nil {
		return err
	}

	started := time.Now()
	statementHash := sha256.Sum256([]byte(q.Statement))
	audit := log.Info().Str("query_id", q.QueryId).Str("origin", q.Origin).Str("actor", q.Actor).Str("source", q.Source).
		Str("statement_sha256", hex.EncodeToString(statementHash[:])).Str("nonce", hex.EncodeToString(q.Nonce))

	rows, err := p.stream(out, src, q)
	finished := time.Now()
	receipt := &pb.Receipt{
		Version:         receiptVersion,
		QueryId:         q.QueryId,
		Origin:          q.Origin,
		Source:          q.Source,
		Actor:           q.Actor,
		StatementSha256: statementHash[:],
		Nonce:           q.Nonce,
		Rows:            rows,
		DurationMs:      finished.Sub(started).Milliseconds(),
		FinishedAtUnix:  finished.Unix(),
		MaxRows:         q.MaxRows,
	}
	if err != nil {
		receipt.Error = err.Error()
	}

	entry := AuditEntry{Event: AuditExecute, Policy: policyLabel(p.policy, decision), Reason: decision.reason, Rows: rows, DurationMS: receipt.DurationMs, Error: receipt.Error}
	if q.ReceiptWanted {
		if signedReceipt, serr := p.signer.sign(receipt); serr != nil {
			log.Error().Err(serr).Str("query_id", q.QueryId).Msg("Failed to sign receipt")
		} else {
			entry.ReceiptSignature = hex.EncodeToString(signedReceipt.Signature)
			// A send that fails means Marmot has gone away, and it records the receipt as missing.
			_ = out.Send(&pb.QueryChunk{Receipt: signedReceipt})
		}
	}
	audit.Int64("rows", rows).Dur("duration", finished.Sub(started)).AnErr("error", err).Msg("Served query")
	p.record(entry, q)
	return err
}

// stream runs the query and sends its chunks, stopping at the signed row limit and the result size cap whatever the plugin produces.
func (p *pluginService) stream(out grpc.ServerStreamingServer[pb.QueryChunk], src LocalSource, q *pb.Query) (int64, error) {
	stream, err := p.querier.ExecuteQuery(out.Context(), src.PluginID, src.Config, requestFor(q))
	if err != nil {
		return 0, err
	}
	defer func() { _ = stream.Close() }()

	var rows int64
	var total int
	for rows < q.MaxRows {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return rows, err
		}
		if remaining := q.MaxRows - rows; int64(len(chunk.Rows)) > remaining {
			chunk.Rows = chunk.Rows[:remaining]
		}
		data, err := json.Marshal(chunk)
		if err != nil {
			return rows, status.Error(codes.Internal, "encoding result")
		}
		if total += len(data); total > maxResultBytes {
			return rows, status.Error(codes.ResourceExhausted, "query result too large")
		}
		if err := out.Send(&pb.QueryChunk{ChunkJson: data}); err != nil {
			return rows, err
		}
		rows += int64(len(chunk.Rows))
	}
	return rows, nil
}

func requestFor(q *pb.Query) pluginsdk.QueryRequest {
	return pluginsdk.QueryRequest{Statement: q.Statement, MaxRows: q.MaxRows, Identity: q.Actor}
}

func sourcesByName(sources []LocalSource) map[string]LocalSource {
	m := make(map[string]LocalSource, len(sources))
	for _, s := range sources {
		m[s.Name] = s
	}
	return m
}

func sourceNames(sources []LocalSource) []string {
	names := make([]string, 0, len(sources))
	for _, s := range sources {
		names = append(names, s.Name)
	}
	return names
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// jitter spreads reconnects between half and all of d, so serving processes that lost Marmot together do not return in lockstep.
func jitter(d time.Duration) time.Duration {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(d/2)+1))
	if err != nil {
		return d
	}
	return d/2 + time.Duration(n.Int64())
}
