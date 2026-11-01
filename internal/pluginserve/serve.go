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
}

// pins are the CAs and query signing keys this process trusts. They are taken from its first enrolment and carried forward through Marmot's key rotations: a later enrolment is accepted only for the same instance and only if it still includes at least one CA and one signing key already trusted, so the trust can move to new keys Marmot announced in advance but can never be replaced outright.
type pins struct {
	instanceID string
	cas        map[string]bool
	keys       map[string]bool
}

// trustedBundle is an enrolment's CAs and signing keys, parsed.
type trustedBundle struct {
	pool *x509.CertPool
	cas  map[string]bool
	keys []*ecdsa.PublicKey
	raw  map[string]bool
}

func parseBundle(res *EnrolResult) (*trustedBundle, error) {
	b := &trustedBundle{pool: x509.NewCertPool(), cas: map[string]bool{}, raw: map[string]bool{}}
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
	return b, nil
}

// acceptBundle checks an enrolment's bundle against the pins and moves the pins on to it.
func acceptBundle(p **pins, instanceID string, b *trustedBundle) error {
	if *p != nil {
		current := *p
		if current.instanceID != instanceID || !overlaps(current.cas, b.cas) || !overlaps(current.keys, b.raw) {
			return errors.New("marmot's serving identity changed with no overlap with the one this process trusts; restart it to trust the new identity")
		}
	}
	*p = &pins{instanceID: instanceID, cas: b.cas, keys: b.raw}
	return nil
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
	var pinned *pins
	backoff := time.Second
	for {
		conn, err := connect(ctx, opts, &pinned)
		if err != nil {
			if ctx.Err() != nil {
				return nil
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

		if !serveUntilDropped(ctx, opts, &pinned, conn) {
			return nil
		}
	}
}

// serveUntilDropped serves on conn, replacing it with a freshly enrolled connection at each half-life, until the connection drops (true) or ctx ends (false). A failed renewal keeps the current connection and tries again at half the remaining lifetime.
func serveUntilDropped(ctx context.Context, opts ServeOptions, pinned **pins, conn *servingConn) bool {
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
			next, err := connect(ctx, opts, pinned)
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
func connect(ctx context.Context, opts ServeOptions, pinned **pins) (*servingConn, error) {
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
		in.Sources = append(in.Sources, EnrolSource{Name: s.Name, PluginID: s.PluginID, Config: s.DisplayConfig})
	}
	res, err := opts.Enrol(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("enrolling: %w", err)
	}
	bundle, err := parseBundle(res)
	if err != nil {
		return nil, err
	}
	if err := acceptBundle(pinned, res.InstanceID, bundle); err != nil {
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
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		RootCAs:    bundle.pool,
		// Replicas are recognised by a fixed name only this instance's CA issues, not by DNS, so this check holds behind any load balancer.
		ServerName:   ServerNameFor(res.InstanceID),
		Certificates: []tls.Certificate{{Certificate: [][]byte{certBlock.Bytes}, PrivateKey: key}},
	}

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
	server := grpc.NewServer(append(requirePeer(isReplicaOf(res.InstanceID)), grpc.Creds(establishedTLS{state: &state}))...)
	pb.RegisterPluginServer(server, &pluginService{
		verifier: NewVerifier(bundle.keys, res.InstanceID, audience, binding),
		sources:  sourcesByName(opts.Sources),
		querier:  opts.Querier,
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
}

func (p *pluginService) accept(signed *pb.SignedQuery, op pb.Operation) (*pb.Query, LocalSource, error) {
	q, err := p.verifier.Verify(signed, op)
	if err != nil {
		log.Warn().Err(err).Msg("Refused query")
		return nil, LocalSource{}, status.Error(codes.PermissionDenied, ErrRejected.Error())
	}
	src, ok := p.sources[q.Source]
	if !ok {
		return nil, LocalSource{}, status.Errorf(codes.NotFound, "this process does not serve %q", q.Source)
	}
	return q, src, nil
}

func (p *pluginService) PlanQuery(ctx context.Context, signed *pb.SignedQuery) (*pb.PlanQueryResponse, error) {
	q, src, err := p.accept(signed, pb.Operation_OPERATION_PLAN)
	if err != nil {
		return nil, err
	}
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

func (p *pluginService) ExecuteQuery(signed *pb.SignedQuery, out grpc.ServerStreamingServer[pb.QueryChunk]) error {
	q, src, err := p.accept(signed, pb.Operation_OPERATION_EXECUTE)
	if err != nil {
		return err
	}

	started := time.Now()
	statementHash := sha256.Sum256([]byte(q.Statement))
	audit := log.Info().Str("query_id", q.QueryId).Str("actor", q.Actor).Str("source", q.Source).
		Str("statement_sha256", hex.EncodeToString(statementHash[:])).Str("nonce", hex.EncodeToString(q.Nonce))

	rows, err := p.stream(out, src, q)
	audit.Int64("rows", rows).Dur("duration", time.Since(started)).AnErr("error", err).Msg("Served query")
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
