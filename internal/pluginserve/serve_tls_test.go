package pluginserve

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serveHandshake runs a serving process's TLS handshake against a server presenting chain, and reports the SNI the server saw.
func serveHandshake(t *testing.T, roots *x509.CertPool, instanceID, address string, chain [][]byte, key *ecdsa.PrivateKey) (string, error) {
	t.Helper()
	// Loopback TCP rather than net.Pipe: an unbuffered pipe deadlocks when the client writes its alert while the server is still writing its flight.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	clientConn, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer clientConn.Close()
	serverConn, err := ln.Accept()
	require.NoError(t, err)
	defer serverConn.Close()

	sni := make(chan string, 1)
	server := tls.Server(serverConn, &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{{Certificate: chain, PrivateKey: key}},
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			sni <- hello.ServerName
			return nil, nil
		},
	})
	go func() { _ = server.Handshake() }()

	client := tls.Client(clientConn, serveClientTLS(roots, instanceID, address, tls.Certificate{}))
	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	err = client.Handshake()
	return <-sni, err
}

// The serve address's host goes out as SNI for routing, while the certificate is still held to this instance's CA and replica name.
func TestServeClientTLSRoutesByHostAndVerifiesReplicaName(t *testing.T) {
	k := newTestKeyring(t)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	replica, _, err := k.issueReplicaCert(&key.PublicKey, "r1", time.Now().Add(time.Hour))
	require.NoError(t, err)

	sni, err := serveHandshake(t, k.Pool(), k.InstanceID, "acme.serve.example.com:8443", [][]byte{replica}, key)
	require.NoError(t, err, "control: this instance's replica certificate is accepted")
	assert.Equal(t, "acme.serve.example.com", sni)

	t.Run("an IP address sends no SNI", func(t *testing.T) {
		sni, err := serveHandshake(t, k.Pool(), k.InstanceID, "10.0.0.1:8443", [][]byte{replica}, key)
		require.NoError(t, err)
		assert.Empty(t, sni)
	})

	t.Run("another instance's replica", func(t *testing.T) {
		other := newTestKeyring(t)
		cert, _, err := other.issueReplicaCert(&key.PublicKey, "r1", time.Now().Add(time.Hour))
		require.NoError(t, err)
		_, err = serveHandshake(t, k.Pool(), k.InstanceID, "acme.serve.example.com:8443", [][]byte{cert}, key)
		assert.Error(t, err, "signed by a CA this process does not trust")
		_, err = serveHandshake(t, other.Pool(), k.InstanceID, "acme.serve.example.com:8443", [][]byte{cert}, key)
		assert.Error(t, err, "trusted CA, but named for another instance")
	})

	t.Run("a serving process's certificate", func(t *testing.T) {
		block, _ := pem.Decode([]byte(csrPEM(t, key, &x509.CertificateRequest{})))
		csr, err := x509.ParseCertificateRequest(block.Bytes)
		require.NoError(t, err)
		cert, err := k.IssueServeCert(csr, "e1", time.Now().Add(time.Hour))
		require.NoError(t, err)
		_, err = serveHandshake(t, k.Pool(), k.InstanceID, "acme.serve.example.com:8443", [][]byte{cert}, key)
		assert.Error(t, err, "a client-only certificate cannot stand in for Marmot")
	})

	t.Run("a certificate named for the SNI host", func(t *testing.T) {
		self := &x509.Certificate{
			SerialNumber: big.NewInt(1),
			DNSNames:     []string{"acme.serve.example.com"},
			NotBefore:    time.Now().Add(-time.Minute),
			NotAfter:     time.Now().Add(time.Hour),
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}
		der, err := x509.CreateCertificate(rand.Reader, self, self, &key.PublicKey, key)
		require.NoError(t, err)
		_, err = serveHandshake(t, k.Pool(), k.InstanceID, "acme.serve.example.com:8443", [][]byte{der}, key)
		assert.Error(t, err, "matching the routing host is not enough")
	})
}
