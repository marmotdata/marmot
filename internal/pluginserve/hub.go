package pluginserve

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	pb "github.com/marmotdata/marmot/internal/pluginserve/proto"
)

const (
	replicaCertLifetime = 24 * time.Hour
	sweepInterval       = 15 * time.Second
	// sessionFreshness is how recent another replica's heartbeat must be before queries are forwarded to it.
	sessionFreshness = 3 * sweepInterval
	handshakeTimeout = 10 * time.Second
	// maxResultBytes caps the rows Marmot accepts for one query from a serving process, so a misbehaving plugin cannot exhaust Marmot's memory.
	maxResultBytes = 256 << 20
)

// ErrNotConnected is returned when no serving process for a source is connected to any replica.
var ErrNotConnected = errors.New("no serving plugin is connected for this source")

// Owner is the principal that owns a served source.
type Owner struct {
	Type string
	ID   string
}

// OwnerLookup resolves the principal that owns a served source, or an error when the source is not served.
type OwnerLookup func(ctx context.Context, source string) (Owner, error)

// PrincipalChecker reports whether an enrolment's principal may still serve: it exists, is active, has not had its sessions revoked since enrolling and holds ingestion:serve.
type PrincipalChecker interface {
	MayServe(ctx context.Context, e *Enrolment) (bool, error)
}

// replicaCert is this replica's certificate and the key generation that issued it.
type replicaCert struct {
	tls        *tls.Certificate
	generation string
}

// HubConfig configures a replica's hub.
type HubConfig struct {
	ListenAddress string
	// ReplicaAddress is the host:port other replicas dial to forward queries here.
	ReplicaAddress string
}

// Hub is one replica's end of the serving connections: it accepts serving processes and other replicas on a mutual TLS listener and routes each query to a serving process connected here or to the replica that holds one.
type Hub struct {
	keyring   *Keyring
	store     *Store
	owners    OwnerLookup
	checker   PrincipalChecker
	cfg       HubConfig
	replicaID string

	cert          atomic.Pointer[replicaCert]
	listener      net.Listener
	replicaServer *grpc.Server
	replicaConns  *connListener

	mu         sync.Mutex
	sessions   map[string]*session
	peers      map[string]*grpc.ClientConn
	peersTrust string

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// session is one serving process connected to this replica.
type session struct {
	enrolment *Enrolment
	leaf      *x509.Certificate
	audience  string
	binding   []byte
	client    pb.PluginClient
	grpcConn  *grpc.ClientConn
	mux       *yamux.Session
	done      chan struct{}
	closeOnce sync.Once
}

func NewHub(keyring *Keyring, store *Store, owners OwnerLookup, checker PrincipalChecker, cfg HubConfig) *Hub {
	return &Hub{
		keyring:   keyring,
		store:     store,
		owners:    owners,
		checker:   checker,
		cfg:       cfg,
		replicaID: uuid.NewString(),
		sessions:  make(map[string]*session),
		peers:     make(map[string]*grpc.ClientConn),
		stop:      make(chan struct{}),
	}
}

// Start issues this replica's certificate and begins accepting connections.
func (h *Hub) Start() error {
	if err := h.renewReplicaCert(); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", h.cfg.ListenAddress)
	if err != nil {
		return fmt.Errorf("listening for serving plugins on %s: %w", h.cfg.ListenAddress, err)
	}
	h.listener = listener

	// Replica connections reach this server after the listener's mutual TLS handshake; gRPC is handed that connection's verified state, and every call is checked to come from a replica.
	h.replicaConns = newConnListener(listener.Addr())
	h.replicaServer = grpc.NewServer(append(requirePeer(h.isReplica), grpc.Creds(establishedTLS{}))...)
	pb.RegisterReplicaServer(h.replicaServer, &replicaService{hub: h})

	h.wg.Add(3)
	go func() { defer h.wg.Done(); _ = h.replicaServer.Serve(h.replicaConns) }()
	go func() { defer h.wg.Done(); h.acceptLoop() }()
	go func() { defer h.wg.Done(); h.sweepLoop() }()

	log.Info().Str("address", listener.Addr().String()).Msg("Accepting serving plugins")
	return nil
}

// Stop closes the listener and every connection.
func (h *Hub) Stop() {
	h.stopOnce.Do(func() {
		close(h.stop)
		if h.listener != nil {
			_ = h.listener.Close()
		}
		if h.replicaServer != nil {
			h.replicaServer.Stop()
		}
		h.mu.Lock()
		sessions := make([]*session, 0, len(h.sessions))
		for _, s := range h.sessions {
			sessions = append(sessions, s)
		}
		for _, c := range h.peers {
			_ = c.Close()
		}
		h.mu.Unlock()
		for _, s := range sessions {
			h.closeSession(s)
		}
	})
	h.wg.Wait()
}

// Addr returns the address the hub listens on.
func (h *Hub) Addr() net.Addr {
	return h.listener.Addr()
}

func (h *Hub) renewReplicaCert() error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generating replica key: %w", err)
	}
	der, generation, err := h.keyring.issueReplicaCert(&key.PublicKey, h.replicaID, time.Now().Add(replicaCertLifetime))
	if err != nil {
		return fmt.Errorf("issuing replica certificate: %w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return err
	}
	h.cert.Store(&replicaCert{tls: &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, generation: generation})
	return nil
}

func (h *Hub) currentCert() *tls.Certificate {
	return h.cert.Load().tls
}

// isReplica refuses a peer certificate that is not another replica of this instance.
func (h *Hub) isReplica(cert *x509.Certificate) error {
	peer, err := h.keyring.identifyPeer(cert)
	if err != nil {
		return err
	}
	if peer.kind != "replica" {
		return errors.New("not a replica")
	}
	return nil
}

// serverTLS is the listener's TLS configuration: TLS 1.3 only, a client certificate from this instance's CA required, and every peer checked in VerifyConnection. Session tickets are off so every connection is a full handshake with a fresh certificate check.
func (h *Hub) serverTLS() *tls.Config {
	return &tls.Config{
		MinVersion:             tls.VersionTLS13,
		SessionTicketsDisabled: true,
		GetCertificate:         func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return h.currentCert(), nil },
		ClientAuth:             tls.RequireAndVerifyClientCert,
		ClientCAs:              h.keyring.Pool(),
		NextProtos:             []string{"h2"},
		VerifyConnection:       h.verifyPeer,
	}
}

// verifyPeer admits another replica of this instance, or a serving process whose enrolment is live, matches the certificate's key and whose principal may still serve.
func (h *Hub) verifyPeer(cs tls.ConnectionState) error {
	if len(cs.VerifiedChains) == 0 {
		return errors.New("client certificate not verified")
	}
	leaf := cs.VerifiedChains[0][0]
	peer, err := h.keyring.identifyPeer(leaf)
	if err != nil {
		return err
	}
	if peer.kind == "replica" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
	defer cancel()
	enrolment, err := h.store.enrolment(ctx, peer.id)
	if err != nil {
		return errors.New("unknown enrolment")
	}
	if err := h.checkEnrolment(ctx, enrolment); err != nil {
		return err
	}
	if revoked, err := h.store.revokedCredentials(ctx, []string{enrolment.ID}); err != nil || revoked[enrolment.ID] {
		return errors.New("enrolment credential revoked")
	}
	if subtle.ConstantTimeCompare(publicKeySHA256(leaf.RawSubjectPublicKeyInfo), enrolment.PublicKeySHA256) != 1 {
		return errors.New("certificate key does not match its enrolment")
	}
	return nil
}

// checkEnrolment fails unless the enrolment is live and its principal may still serve.
func (h *Hub) checkEnrolment(ctx context.Context, e *Enrolment) error {
	if !e.active(time.Now()) {
		return errors.New("enrolment expired")
	}
	ok, err := h.checker.MayServe(ctx, e)
	if err != nil {
		return fmt.Errorf("checking principal: %w", err)
	}
	if !ok {
		return errors.New("principal may no longer serve")
	}
	return nil
}

func (h *Hub) acceptLoop() {
	for {
		conn, err := h.listener.Accept()
		if err != nil {
			select {
			case <-h.stop:
				return
			default:
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			log.Error().Err(err).Msg("Serve listener stopped")
			return
		}
		go h.handleConn(conn)
	}
}

func (h *Hub) handleConn(raw net.Conn) {
	conn := tls.Server(raw, h.serverTLS())
	ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
	err := conn.HandshakeContext(ctx)
	cancel()
	if err != nil {
		log.Debug().Err(err).Str("remote", raw.RemoteAddr().String()).Msg("Refused serve connection")
		_ = conn.Close()
		return
	}

	state := conn.ConnectionState()
	leaf := state.VerifiedChains[0][0]
	peer, err := h.keyring.identifyPeer(leaf)
	if err != nil {
		_ = conn.Close()
		return
	}
	if peer.kind == "replica" {
		h.replicaConns.push(conn)
		return
	}
	if err := h.openSession(conn, state, peer.id, leaf); err != nil {
		log.Warn().Err(err).Str("remote", raw.RemoteAddr().String()).Msg("Refused serving plugin")
		_ = conn.Close()
	}
}

// openSession takes a verified serving process's connection, opens the multiplexed gRPC channel Marmot calls it over and keeps it until the certificate expires, its principal loses the right to serve or the process goes away.
func (h *Hub) openSession(conn *tls.Conn, state tls.ConnectionState, enrolmentID string, leaf *x509.Certificate) error {
	ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
	defer cancel()
	enrolment, err := h.store.enrolment(ctx, enrolmentID)
	if err != nil {
		return err
	}
	binding, err := channelBinding(state)
	if err != nil {
		return err
	}

	mux, err := yamux.Client(conn, muxConfig())
	if err != nil {
		return fmt.Errorf("starting multiplexer: %w", err)
	}
	// Each gRPC stream is a yamux stream inside this mutually authenticated TLS 1.3 connection, whose verified state gRPC is handed rather than running a handshake of its own.
	grpcConn, err := grpc.NewClient("passthrough:///serving-plugin",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return mux.Open() }),
		grpc.WithTransportCredentials(establishedTLS{state: &state}),
	)
	if err != nil {
		_ = mux.Close()
		return fmt.Errorf("starting gRPC channel: %w", err)
	}

	s := &session{
		enrolment: enrolment,
		leaf:      leaf,
		audience:  h.keyring.ServeID(enrolment.ID),
		binding:   binding,
		client:    pb.NewPluginClient(grpcConn),
		grpcConn:  grpcConn,
		mux:       mux,
		done:      make(chan struct{}),
	}

	h.mu.Lock()
	if old, ok := h.sessions[enrolment.ID]; ok {
		h.mu.Unlock()
		h.closeSession(old)
		h.mu.Lock()
	}
	h.sessions[enrolment.ID] = s
	h.mu.Unlock()

	if err := h.store.upsertSession(ctx, enrolment.ID, h.replicaID, h.cfg.ReplicaAddress); err != nil {
		h.closeSession(s)
		return err
	}

	expiry := leaf.NotAfter
	if enrolment.ExpiresAt.Before(expiry) {
		expiry = enrolment.ExpiresAt
	}
	timer := time.AfterFunc(time.Until(expiry), func() { h.closeSession(s) })
	go func() {
		select {
		case <-mux.CloseChan():
		case <-s.done:
		}
		timer.Stop()
		h.closeSession(s)
	}()

	log.Info().Str("enrolment", enrolment.ID).Str("principal", enrolment.AuditSubject).Strs("sources", enrolment.Sources).Msg("Serving plugin connected")
	return nil
}

func (h *Hub) closeSession(s *session) {
	s.closeOnce.Do(func() {
		close(s.done)
		h.mu.Lock()
		if h.sessions[s.enrolment.ID] == s {
			delete(h.sessions, s.enrolment.ID)
		}
		h.mu.Unlock()
		_ = s.grpcConn.Close()
		_ = s.mux.Close()

		ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
		defer cancel()
		if err := h.store.deleteSession(ctx, s.enrolment.ID, h.replicaID); err != nil {
			log.Warn().Err(err).Msg("Failed to delete serve session")
		}
		log.Info().Str("enrolment", s.enrolment.ID).Msg("Serving plugin disconnected")
	})
}

// sweepLoop keeps this replica in step with the database: it picks up key rotations, renews its certificate when the active keys change or it reaches half-life, closes any connection that should no longer be open, heartbeats the rest and prunes old enrolments.
func (h *Hub) sweepLoop() {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-h.stop:
			return
		case <-ticker.C:
			h.sweep()
		}
	}
}

func (h *Hub) sweep() {
	ctx, cancel := context.WithTimeout(context.Background(), sweepInterval)
	defer cancel()

	if err := h.keyring.Refresh(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to refresh serving keys")
	}
	if cert := h.cert.Load(); cert.generation != h.keyring.activeID() || time.Until(cert.tls.Leaf.NotAfter) < replicaCertLifetime/2 {
		if err := h.renewReplicaCert(); err != nil {
			log.Error().Err(err).Msg("Failed to renew replica certificate")
		}
	}
	h.dropPeersOnTrustChange()

	h.mu.Lock()
	sessions := make([]*session, 0, len(h.sessions))
	ids := make([]string, 0, len(h.sessions))
	for _, s := range h.sessions {
		sessions = append(sessions, s)
		ids = append(ids, s.enrolment.ID)
	}
	h.mu.Unlock()
	if len(sessions) == 0 {
		h.prune(ctx)
		return
	}

	// While the database is unreachable nothing can be verified, but nothing can be queried either: queries need the database to find a source's owner. Sessions are kept rather than all dropped at once, and each still ends when its short-lived certificate does.
	revoked, err := h.store.revokedCredentials(ctx, ids)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check serving credentials")
		return
	}

	pool := h.keyring.Pool()
	live := make([]string, 0, len(sessions))
	for _, s := range sessions {
		if err := h.stillValid(ctx, s, pool, revoked[s.enrolment.ID]); err != nil {
			log.Info().Err(err).Str("enrolment", s.enrolment.ID).Str("principal", s.enrolment.AuditSubject).Msg("Closing serving plugin")
			h.closeSession(s)
			continue
		}
		live = append(live, s.enrolment.ID)
	}
	if len(live) > 0 {
		if err := h.store.heartbeat(ctx, h.replicaID, live); err != nil {
			log.Warn().Err(err).Msg("Failed to heartbeat serve sessions")
		}
	}
	h.prune(ctx)
}

// stillValid fails when a session's certificate no longer chains to a trusted key generation, its API key was revoked or its principal may no longer serve.
func (h *Hub) stillValid(ctx context.Context, s *session, pool *x509.CertPool, credentialRevoked bool) error {
	if _, err := s.leaf.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		return fmt.Errorf("certificate no longer trusted: %w", err)
	}
	if credentialRevoked {
		return errors.New("enrolment credential revoked")
	}
	return h.checkEnrolment(ctx, s.enrolment)
}

// dropPeersOnTrustChange closes connections to other replicas when the trusted keys change, so the next forward dials with the current trust.
func (h *Hub) dropPeersOnTrustChange() {
	version := h.keyring.trustVersion()
	h.mu.Lock()
	defer h.mu.Unlock()
	if version == h.peersTrust {
		return
	}
	for address, conn := range h.peers {
		_ = conn.Close()
		delete(h.peers, address)
	}
	h.peersTrust = version
}

func (h *Hub) prune(ctx context.Context) {
	if err := h.store.pruneEnrolments(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to prune serve enrolments")
	}
}

// localSession returns a connected session that serves source for its owner.
func (h *Hub) localSession(source string, owner Owner) *session {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for _, s := range h.sessions {
		e := s.enrolment
		if e.PrincipalType == owner.Type && e.PrincipalID == owner.ID && e.active(now) && slices.Contains(e.Sources, source) {
			return s
		}
	}
	return nil
}

// PlanQuery asks the serving plugin for source what a statement would touch.
func (h *Hub) PlanQuery(ctx context.Context, source string, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	resp, err := h.plan(ctx, forwarded(pb.Operation_OPERATION_PLAN, source, "", req), true)
	if err != nil {
		return nil, err
	}
	return decodePlan(resp.PlanJson)
}

// ExecuteQuery runs a statement on the serving plugin for source. queryID is the query's id in Marmot's log, which the serving process records so both audit trails can be matched.
func (h *Hub) ExecuteQuery(ctx context.Context, source, queryID string, req pluginsdk.QueryRequest) (pluginsdk.QueryStream, error) {
	ctx, cancel := context.WithCancel(ctx)
	stream, err := h.execute(ctx, forwarded(pb.Operation_OPERATION_EXECUTE, source, queryID, req), true)
	if err != nil {
		cancel()
		return nil, err
	}
	return &resultStream{stream: stream, cancel: cancel}, nil
}

func forwarded(op pb.Operation, source, queryID string, req pluginsdk.QueryRequest) *pb.ForwardedQuery {
	return &pb.ForwardedQuery{
		Operation: op,
		Source:    source,
		Statement: req.Statement,
		MaxRows:   req.MaxRows,
		Actor:     req.Identity,
		QueryId:   queryID,
	}
}

// plan sends a plan to a local session, or to a replica holding one when mayForward is set. Forwarded queries are never forwarded again, so they cannot loop between replicas.
func (h *Hub) plan(ctx context.Context, fq *pb.ForwardedQuery, mayForward bool) (*pb.PlanQueryResponse, error) {
	owner, err := h.owners(ctx, fq.Source)
	if err != nil {
		return nil, err
	}
	if s := h.localSession(fq.Source, owner); s != nil {
		signed, err := h.signFor(s, fq)
		if err != nil {
			return nil, err
		}
		return s.client.PlanQuery(ctx, signed)
	}
	if !mayForward {
		return nil, status.Error(codes.Unavailable, ErrNotConnected.Error())
	}
	peer, err := h.peerFor(ctx, fq.Source, owner)
	if err != nil {
		return nil, err
	}
	return pb.NewReplicaClient(peer).PlanQuery(ctx, fq)
}

func (h *Hub) execute(ctx context.Context, fq *pb.ForwardedQuery, mayForward bool) (grpc.ServerStreamingClient[pb.QueryChunk], error) {
	owner, err := h.owners(ctx, fq.Source)
	if err != nil {
		return nil, err
	}
	if s := h.localSession(fq.Source, owner); s != nil {
		signed, err := h.signFor(s, fq)
		if err != nil {
			return nil, err
		}
		return s.client.ExecuteQuery(ctx, signed)
	}
	if !mayForward {
		return nil, status.Error(codes.Unavailable, ErrNotConnected.Error())
	}
	peer, err := h.peerFor(ctx, fq.Source, owner)
	if err != nil {
		return nil, err
	}
	return pb.NewReplicaClient(peer).ExecuteQuery(ctx, fq)
}

// signFor turns a query into a Query signed for, and bound to, one session's connection.
func (h *Hub) signFor(s *session, fq *pb.ForwardedQuery) (*pb.SignedQuery, error) {
	return h.keyring.signQuery(&pb.Query{
		Operation: fq.Operation,
		Source:    fq.Source,
		Statement: fq.Statement,
		MaxRows:   fq.MaxRows,
		Actor:     fq.Actor,
		QueryId:   fq.QueryId,
	}, s.audience, s.binding, time.Now())
}

// peerFor returns a mutual TLS gRPC client to a replica holding a session for source.
func (h *Hub) peerFor(ctx context.Context, source string, owner Owner) (*grpc.ClientConn, error) {
	addresses, err := h.store.remoteReplicas(ctx, source, owner.Type, owner.ID, h.replicaID, sessionFreshness)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, ErrNotConnected
	}
	address := addresses[0]

	h.mu.Lock()
	defer h.mu.Unlock()
	if conn, ok := h.peers[address]; ok {
		return conn, nil
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(h.clientTLS())))
	if err != nil {
		return nil, fmt.Errorf("connecting to replica %s: %w", address, err)
	}
	h.peers[address] = conn
	return conn, nil
}

// clientTLS is how a replica dials another: TLS 1.3, the peer verified against this instance's CA under the fixed replica server name, and this replica's own certificate presented.
func (h *Hub) clientTLS() *tls.Config {
	return &tls.Config{
		MinVersion:           tls.VersionTLS13,
		RootCAs:              h.keyring.Pool(),
		ServerName:           h.keyring.ServerName(),
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return h.currentCert(), nil },
	}
}

// replicaService answers queries forwarded by other replicas, only ever from sessions held here.
type replicaService struct {
	pb.UnimplementedReplicaServer
	hub *Hub
}

func (r *replicaService) PlanQuery(ctx context.Context, fq *pb.ForwardedQuery) (*pb.PlanQueryResponse, error) {
	return r.hub.plan(ctx, fq, false)
}

func (r *replicaService) ExecuteQuery(fq *pb.ForwardedQuery, out grpc.ServerStreamingServer[pb.QueryChunk]) error {
	in, err := r.hub.execute(out.Context(), fq, false)
	if err != nil {
		return err
	}
	var total int
	for {
		chunk, err := in.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if total += len(chunk.ChunkJson); total > maxResultBytes {
			return status.Error(codes.ResourceExhausted, "query result too large")
		}
		if err := out.Send(chunk); err != nil {
			return err
		}
	}
}

// resultStream decodes chunks from a serving plugin and enforces the result size cap.
type resultStream struct {
	stream grpc.ServerStreamingClient[pb.QueryChunk]
	cancel context.CancelFunc
	total  int
}

func (r *resultStream) Recv() (*pluginsdk.QueryResultChunk, error) {
	msg, err := r.stream.Recv()
	if err != nil {
		return nil, err
	}
	if r.total += len(msg.ChunkJson); r.total > maxResultBytes {
		return nil, status.Error(codes.ResourceExhausted, "query result too large")
	}
	var chunk pluginsdk.QueryResultChunk
	if err := json.Unmarshal(msg.ChunkJson, &chunk); err != nil {
		return nil, fmt.Errorf("decoding result chunk: %w", err)
	}
	return &chunk, nil
}

func (r *resultStream) Close() error {
	r.cancel()
	return nil
}

func decodePlan(data []byte) (*pluginsdk.QueryPlan, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var plan pluginsdk.QueryPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("decoding plan: %w", err)
	}
	return &plan, nil
}

// muxConfig keeps yamux's defaults but refuses to queue streams the other side opens: Marmot only ever opens streams towards a serving process, never accepts them.
func muxConfig() *yamux.Config {
	cfg := yamux.DefaultConfig()
	cfg.AcceptBacklog = 1
	cfg.LogOutput = io.Discard
	return cfg
}

// connListener hands already-authenticated replica connections to the replica gRPC server.
type connListener struct {
	addr   net.Addr
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newConnListener(addr net.Addr) *connListener {
	return &connListener{addr: addr, conns: make(chan net.Conn), closed: make(chan struct{})}
}

func (l *connListener) push(c net.Conn) {
	select {
	case l.conns <- c:
	case <-l.closed:
		_ = c.Close()
	}
}

func (l *connListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *connListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *connListener) Addr() net.Addr { return l.addr }
