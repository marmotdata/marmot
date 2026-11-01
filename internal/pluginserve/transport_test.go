package pluginserve

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func verifiedState(leaf *x509.Certificate) *tls.ConnectionState {
	return &tls.ConnectionState{
		Version:           tls.VersionTLS13,
		HandshakeComplete: true,
		VerifiedChains:    [][]*x509.Certificate{{leaf}},
	}
}

// The credentials only ever vouch for a completed, mutually verified TLS 1.3 connection.
func TestEstablishedTLSRefusesAnythingElse(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	_, _, err := establishedTLS{}.ServerHandshake(a)
	assert.Error(t, err, "a plain connection is not TLS")

	for name, state := range map[string]*tls.ConnectionState{
		"TLS 1.2":           {Version: tls.VersionTLS12, HandshakeComplete: true, VerifiedChains: [][]*x509.Certificate{{{}}}},
		"incomplete":        {Version: tls.VersionTLS13, VerifiedChains: [][]*x509.Certificate{{{}}}},
		"no verified chain": {Version: tls.VersionTLS13, HandshakeComplete: true},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := establishedTLS{state: state}.ClientHandshake(context.Background(), "", a)
			assert.Error(t, err)
		})
	}

	_, info, err := establishedTLS{state: verifiedState(&x509.Certificate{})}.ServerHandshake(a)
	require.NoError(t, err)
	assert.Equal(t, credentials.PrivacyAndIntegrity, info.(credentials.TLSInfo).SecurityLevel)
}

func callWithPeer(t *testing.T, check func(*x509.Certificate) error, leaf *x509.Certificate) error {
	t.Helper()
	ctx := context.Background()
	if leaf != nil {
		ctx = peer.NewContext(ctx, &peer.Peer{AuthInfo: credentials.TLSInfo{State: *verifiedState(leaf)}})
	}
	unary, _ := peerInterceptors(check)
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) { return "ok", nil })
	return err
}

// Every call is checked against the verified peer, not just the connection.
func TestRequirePeerChecksEveryCall(t *testing.T) {
	k := newTestKeyring(t)
	opts := isReplicaOf(k.InstanceID)

	replica := &x509.Certificate{DNSNames: []string{k.ServerName()}}
	assert.NoError(t, callWithPeer(t, opts, replica))

	for name, leaf := range map[string]*x509.Certificate{
		"no peer":          nil,
		"serving process":  {},
		"another instance": {DNSNames: []string{ServerNameFor("11111111-1111-1111-1111-111111111111")}},
	} {
		t.Run(name, func(t *testing.T) {
			err := callWithPeer(t, opts, leaf)
			assert.Equal(t, codes.PermissionDenied, status.Code(err))
		})
	}
}
