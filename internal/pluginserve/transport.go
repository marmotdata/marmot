package pluginserve

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// establishedTLS is gRPC transport security for connections that are already TLS 1.3 with verified certificates on both sides: replica connections handed over after the listener's handshake, and the streams multiplexed inside a serving connection. It adds no second handshake. It refuses anything that is not such a connection and reports the connection's real TLS state to gRPC, so gRPC applies its privacy-and-integrity rules and handlers can see the verified peer with peer.FromContext.
type establishedTLS struct {
	// state is the outer connection's state for streams multiplexed inside it; when nil, the connection itself must be a completed *tls.Conn.
	state *tls.ConnectionState
}

func (c establishedTLS) handshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	state := c.state
	if state == nil {
		tlsConn, ok := conn.(*tls.Conn)
		if !ok {
			return nil, nil, errors.New("connection is not TLS")
		}
		s := tlsConn.ConnectionState()
		state = &s
	}
	if !state.HandshakeComplete || state.Version != tls.VersionTLS13 || len(state.VerifiedChains) == 0 {
		return nil, nil, errors.New("connection is not mutually authenticated TLS 1.3")
	}
	return conn, credentials.TLSInfo{
		State:          *state,
		CommonAuthInfo: credentials.CommonAuthInfo{SecurityLevel: credentials.PrivacyAndIntegrity},
	}, nil
}

func (c establishedTLS) ClientHandshake(_ context.Context, _ string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return c.handshake(conn)
}

func (c establishedTLS) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return c.handshake(conn)
}

func (c establishedTLS) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{SecurityProtocol: "tls"}
}

func (c establishedTLS) Clone() credentials.TransportCredentials { return c }

// OverrideServerName is part of the interface but meaningless here: the peer was verified before gRPC saw the connection.
func (c establishedTLS) OverrideServerName(string) error { return nil }

// peerCertificate returns the verified leaf certificate of the caller of an RPC.
func peerCertificate(ctx context.Context) (*x509.Certificate, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("no peer")
	}
	info, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(info.State.VerifiedChains) == 0 {
		return nil, errors.New("peer is not authenticated")
	}
	return info.State.VerifiedChains[0][0], nil
}

// requirePeer returns server options that run check on the verified peer certificate of every call, on top of the check made when the connection was accepted.
func requirePeer(check func(*x509.Certificate) error) []grpc.ServerOption {
	unary, stream := peerInterceptors(check)
	return []grpc.ServerOption{grpc.UnaryInterceptor(unary), grpc.StreamInterceptor(stream)}
}

func peerInterceptors(check func(*x509.Certificate) error) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	authorise := func(ctx context.Context) error {
		cert, err := peerCertificate(ctx)
		if err == nil {
			err = check(cert)
		}
		if err != nil {
			return status.Error(codes.PermissionDenied, "peer not permitted")
		}
		return nil
	}
	unary := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := authorise(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
	stream := func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := authorise(ss.Context()); err != nil {
			return err
		}
		return handler(srv, ss)
	}
	return unary, stream
}

// isReplicaOf reports an error unless cert is a replica certificate of the given instance: it carries the instance's fixed replica name.
func isReplicaOf(instanceID string) func(*x509.Certificate) error {
	return func(cert *x509.Certificate) error {
		if !slices.Contains(cert.DNSNames, ServerNameFor(instanceID)) {
			return errors.New("not a replica of this instance")
		}
		return nil
	}
}
