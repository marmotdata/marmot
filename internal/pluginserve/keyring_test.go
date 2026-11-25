package pluginserve

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcrypto "github.com/marmotdata/marmot/internal/crypto"
)

func testEncryptor(t *testing.T) *mcrypto.Encryptor {
	t.Helper()
	key, err := mcrypto.GenerateKey()
	require.NoError(t, err)
	enc, err := mcrypto.NewEncryptor(key)
	require.NoError(t, err)
	return enc
}

// newTestKeyring builds a keyring with one active, database-style generation and no store, for tests that never touch the database.
func newTestKeyring(t *testing.T) *Keyring {
	t.Helper()
	k := &Keyring{InstanceID: uuid.NewString(), encryptor: testEncryptor(t), now: time.Now}
	record, err := k.newGeneration(context.Background(), KeyConfig{}, time.Now().Add(-time.Second))
	require.NoError(t, err)
	record.ID = uuid.NewString()
	g, err := k.openGeneration(context.Background(), record)
	require.NoError(t, err)
	k.gens = []*generation{g}
	return k
}

func csrPEM(t *testing.T, key any, template *x509.CertificateRequest) string {
	t.Helper()
	der, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
}

func TestGenerationKeepsKeysEncrypted(t *testing.T) {
	k := &Keyring{InstanceID: uuid.NewString(), encryptor: testEncryptor(t), now: time.Now}
	record, err := k.newGeneration(context.Background(), KeyConfig{}, time.Now())
	require.NoError(t, err)

	require.NotNil(t, record.CAKey)
	require.NotNil(t, record.SigningKey)
	assert.True(t, mcrypto.LooksEncrypted(*record.CAKey))
	assert.True(t, mcrypto.LooksEncrypted(*record.SigningKey))
	assert.Nil(t, record.CAKeyURI)

	other := &Keyring{InstanceID: k.InstanceID, encryptor: testEncryptor(t), now: time.Now}
	_, err = other.openGeneration(context.Background(), record)
	assert.Error(t, err, "the keys are useless without the server encryption key")
}

// A generation whose stored certificate or public key does not match its private keys is refused, so a tampered row cannot slip in a foreign CA or verification key.
func TestOpenGenerationRefusesMismatchedKeys(t *testing.T) {
	k := &Keyring{InstanceID: uuid.NewString(), encryptor: testEncryptor(t), now: time.Now}
	a, err := k.newGeneration(context.Background(), KeyConfig{}, time.Now())
	require.NoError(t, err)
	b, err := k.newGeneration(context.Background(), KeyConfig{}, time.Now())
	require.NoError(t, err)

	swappedCert := *a
	swappedCert.CACert = b.CACert
	_, err = k.openGeneration(context.Background(), &swappedCert)
	assert.Error(t, err)

	swappedPub := *a
	swappedPub.SigningPublicKey = b.SigningPublicKey
	_, err = k.openGeneration(context.Background(), &swappedPub)
	assert.Error(t, err)

	swappedApproval := *a
	swappedApproval.ApprovalPublicKey = b.ApprovalPublicKey
	_, err = k.openGeneration(context.Background(), &swappedApproval)
	assert.ErrorContains(t, err, "approval public key does not match")

	// An approval key that is the signing key would let signing a query approve; the stored row is refused.
	sameKey := *a
	sameKey.ApprovalKey, sameKey.ApprovalPublicKey = a.SigningKey, &a.SigningPublicKey
	_, err = k.openGeneration(context.Background(), &sameKey)
	assert.ErrorContains(t, err, "the approval key is the signing key")
}

func TestNewGenerationNeedsEveryKMSKeyOrNone(t *testing.T) {
	k := &Keyring{InstanceID: uuid.NewString(), encryptor: testEncryptor(t), now: time.Now}
	_, err := k.newGeneration(context.Background(), KeyConfig{CAKeyURI: "hashivault://ca"}, time.Now())
	assert.Error(t, err)
	_, err = k.newGeneration(context.Background(), KeyConfig{CAKeyURI: "hashivault://ca", SigningKeyURI: "hashivault://sign"}, time.Now())
	assert.ErrorContains(t, err, "approval key")
	_, err = k.newGeneration(context.Background(), KeyConfig{CAKeyURI: "hashivault://ca", SigningKeyURI: "hashivault://sign", ApprovalKeyURI: "hashivault://sign"}, time.Now())
	assert.ErrorContains(t, err, "a key of its own")
}

// The approval key is a key of its own: generated with the others, encrypted like them, published beside them and never the signing key.
func TestGenerationHasASeparateApprovalKey(t *testing.T) {
	k := newTestKeyring(t)
	g := k.gens[0]
	require.NotNil(t, g.approvalKey)
	assert.False(t, g.approvalPub.Equal(g.signingPub))
	_, signing, approval := k.Bundle()
	require.Len(t, approval, 1)
	assert.NotEqual(t, signing[0], approval[0])

	msg := []byte("an approval")
	sig, err := k.SignApproval(msg)
	require.NoError(t, err)
	assert.True(t, signedByAny([]*ecdsa.PublicKey{g.approvalPub}, nil, msg, sig))
	assert.False(t, signedByAny([]*ecdsa.PublicKey{g.signingPub}, nil, msg, sig), "the signing key cannot verify an approval")

	g.approvalKey, g.approvalPub = nil, nil
	_, err = k.SignApproval(msg)
	assert.ErrorIs(t, err, ErrNoApprovalKey)
}

// Approval keys are pinned like signing keys: taken from the first enrolment, carried through overlapping rotations, and never replaced or dropped outright.
func TestApprovalKeysArePinned(t *testing.T) {
	k := newTestKeyring(t)
	cas, signing, approval := k.Bundle()
	bundle := func(approvalKeys []string) *trustedBundle {
		b, err := parseBundle(&EnrolResult{CABundle: cas, SigningKeys: signing, ApprovalKeys: approvalKeys})
		require.NoError(t, err)
		return b
	}
	var p *pins
	require.NoError(t, acceptBundle(&p, k.InstanceID, bundle(approval), nil))

	other := newTestKeyring(t)
	_, _, foreign := other.Bundle()
	assert.ErrorContains(t, acceptBundle(&p, k.InstanceID, bundle(foreign), nil), "approval key was replaced with no overlap")
	assert.ErrorContains(t, acceptBundle(&p, k.InstanceID, bundle(nil), nil), "approval key was replaced", "an enrolment cannot drop the pinned approval keys")
	require.NoError(t, acceptBundle(&p, k.InstanceID, bundle(append(approval, foreign...)), nil), "an overlapping rotation moves on")
	require.NoError(t, acceptBundle(&p, k.InstanceID, bundle(foreign), nil), "and then the old key can go")

	_, err := parseBundle(&EnrolResult{CABundle: cas, SigningKeys: signing, ApprovalKeys: signing})
	assert.ErrorContains(t, err, "a signing key as an approval key")
}

func TestParseCSR(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	_, err = ParseCSR(csrPEM(t, ecKey, &x509.CertificateRequest{}))
	assert.NoError(t, err)
	_, err = ParseCSR(csrPEM(t, edKey, &x509.CertificateRequest{}))
	assert.NoError(t, err)

	t.Run("rejects other key types", func(t *testing.T) {
		p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		require.NoError(t, err)
		_, err = ParseCSR(csrPEM(t, p384, &x509.CertificateRequest{}))
		assert.Error(t, err)

		rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		_, err = ParseCSR(csrPEM(t, rsaKey, &x509.CertificateRequest{}))
		assert.Error(t, err)
	})

	t.Run("rejects a forged signature", func(t *testing.T) {
		block, _ := pem.Decode([]byte(csrPEM(t, ecKey, &x509.CertificateRequest{})))
		block.Bytes[len(block.Bytes)-3] ^= 0xff
		_, err := ParseCSR(string(pem.EncodeToMemory(block)))
		assert.Error(t, err)
	})

	t.Run("rejects other PEM and garbage", func(t *testing.T) {
		_, err := ParseCSR("not pem")
		assert.Error(t, err)
		_, err = ParseCSR(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{1, 2, 3}})))
		assert.Error(t, err)
	})
}

// A serving process chooses only its key: everything it asks for in the CSR is ignored.
func TestIssueServeCertIgnoresRequestedNames(t *testing.T) {
	k := newTestKeyring(t)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	evil, _ := url.Parse("spiffe://" + k.InstanceID + "/replica/11111111-1111-1111-1111-111111111111")
	csr, err := ParseCSR(csrPEM(t, key, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: "admin"},
		DNSNames: []string{k.ServerName()},
		URIs:     []*url.URL{evil},
	}))
	require.NoError(t, err)

	enrolmentID := "22222222-2222-2222-2222-222222222222"
	der, err := k.IssueServeCert(csr, enrolmentID, time.Now().Add(CertLifetime))
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	assert.Empty(t, cert.DNSNames)
	require.Len(t, cert.URIs, 1)
	assert.Equal(t, k.ServeID(enrolmentID), cert.URIs[0].String())
	assert.Equal(t, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, cert.ExtKeyUsage, "a serving process can never act as a server")
	assert.False(t, cert.IsCA)

	peer, err := k.identifyPeer(cert)
	require.NoError(t, err)
	assert.Equal(t, peerIdentity{kind: "serve", id: enrolmentID}, peer)
}

// Even someone holding the CA key cannot mint a certificate that verifies for another instance, a real host name or an IP address.
func TestCAIsNameConstrained(t *testing.T) {
	k := newTestKeyring(t)
	g, err := k.active()
	require.NoError(t, err)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	mint := func(template *x509.Certificate) error {
		template.SerialNumber = mustSerial()
		template.NotBefore = time.Now().Add(-time.Minute)
		template.NotAfter = time.Now().Add(time.Hour)
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
		der, err := x509.CreateCertificate(rand.Reader, template, g.caCert, &key.PublicKey, g.caKey)
		require.NoError(t, err)
		cert, err := x509.ParseCertificate(der)
		require.NoError(t, err)
		_, err = cert.Verify(x509.VerifyOptions{Roots: k.Pool(), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}})
		return err
	}
	uri := func(s string) []*url.URL { u, _ := url.Parse(s); return []*url.URL{u} }

	assert.NoError(t, mint(&x509.Certificate{URIs: uri("spiffe://" + k.InstanceID + "/serve/x")}), "control")
	assert.NoError(t, mint(&x509.Certificate{DNSNames: []string{k.ServerName()}}), "control")

	assert.Error(t, mint(&x509.Certificate{URIs: uri("spiffe://" + uuid.NewString() + "/serve/x")}))
	assert.Error(t, mint(&x509.Certificate{DNSNames: []string{"marmot.example.com"}}))
	assert.Error(t, mint(&x509.Certificate{IPAddresses: []net.IP{net.ParseIP("10.0.0.1")}}))
	assert.Error(t, mint(&x509.Certificate{IPAddresses: []net.IP{net.ParseIP("::1")}}))
}

func TestIdentifyPeerRejects(t *testing.T) {
	k := newTestKeyring(t)
	uri := func(s string) []*url.URL {
		u, err := url.Parse(s)
		require.NoError(t, err)
		return []*url.URL{u}
	}
	enrolment := "33333333-3333-3333-3333-333333333333"

	for name, uris := range map[string][]*url.URL{
		"no SPIFFE ID":     nil,
		"two SPIFFE IDs":   append(uri(k.ServeID(enrolment)), uri(k.ServeID(enrolment))...),
		"another instance": uri("spiffe://44444444-4444-4444-4444-444444444444/serve/" + enrolment),
		"other scheme":     uri("https://" + k.InstanceID + "/serve/" + enrolment),
		"unknown kind":     uri("spiffe://" + k.InstanceID + "/admin/" + enrolment),
		"non-uuid id":      uri("spiffe://" + k.InstanceID + "/serve/../replica"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := k.identifyPeer(&x509.Certificate{URIs: uris})
			assert.Error(t, err)
		})
	}
}

// Activation and retirement follow the clock: a published generation is trusted but unused until it activates, and is dropped once it retires.
func TestKeyringGenerationsFollowTheClock(t *testing.T) {
	k := newTestKeyring(t)
	old := k.gens[0]
	record, err := k.newGeneration(context.Background(), KeyConfig{}, time.Now().Add(RotationOverlap))
	require.NoError(t, err)
	record.ID = uuid.NewString()
	next, err := k.openGeneration(context.Background(), record)
	require.NoError(t, err)
	retires := time.Now().Add(2 * RotationOverlap)
	old.retiresAt = &retires
	k.gens = append(k.gens, next)

	gens, active := k.trusted()
	assert.Len(t, gens, 2, "the next generation is published before it is used")
	assert.Same(t, old, active)
	cas, keys, approvals := k.Bundle()
	assert.Len(t, keys, 2)
	assert.Len(t, approvals, 2)
	assert.Equal(t, 2, len(pemBlocks(cas)))

	k.now = func() time.Time { return time.Now().Add(RotationOverlap + time.Second) }
	_, active = k.trusted()
	assert.Same(t, next, active)

	k.now = func() time.Time { return time.Now().Add(2*RotationOverlap + time.Second) }
	gens, active = k.trusted()
	assert.Equal(t, []*generation{next}, gens, "the old generation is no longer trusted once retired")
	assert.Same(t, next, active)
}

func pemBlocks(s string) []*pem.Block {
	var blocks []*pem.Block
	for rest := []byte(s); ; {
		var b *pem.Block
		if b, rest = pem.Decode(rest); b == nil {
			return blocks
		}
		blocks = append(blocks, b)
	}
}
