// Package pluginserve lets plugins that run outside Marmot (`marmot ingest --serve`) answer data queries. A serving process dials out to Marmot over mutual TLS, so it never listens on a port; Marmot then sends each query down that connection, signed and bound to it.
package pluginserve

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sigstore/sigstore/pkg/signature/kms"
	// KMS providers register themselves with sigstore's KMS package, which resolves a key URI to a crypto.Signer whose private key never leaves the KMS.
	_ "github.com/sigstore/sigstore/pkg/signature/kms/aws"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/azure"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/gcp"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/hashivault"

	mcrypto "github.com/marmotdata/marmot/internal/crypto"
)

// ErrKMSKeysRequired is returned when a rotation would move keys out of a KMS into the database.
var ErrKMSKeysRequired = errors.New("the serving keys are held in a KMS; name new KMS keys to rotate to")

// ErrEncryptionRequired is returned when serving is enabled without a server encryption key, which protects keys stored in the database.
var ErrEncryptionRequired = errors.New("serving plugins requires MARMOT_SERVER_ENCRYPTION_KEY")

const (
	caValidity = 10 * 365 * 24 * time.Hour
	// clockSkew backdates certificates so a peer whose clock runs slightly behind still accepts them.
	clockSkew = time.Minute
	// RotationOverlap is how long a rotation publishes new keys before using them, and keeps old keys trusted after: longer than a serving certificate lives, so every connected process has enrolled at least once with both and nothing issued under the old keys is still live when they go.
	RotationOverlap = 2 * CertLifetime
)

// KeyConfig names KMS keys for a generation, as sigstore KMS URIs (awskms://, gcpkms://, azurekms://, hashivault://). Both must be ECDSA P-256 keys. When empty, Marmot generates the keys and stores them encrypted with the server encryption key.
type KeyConfig struct {
	CAKeyURI      string
	SigningKeyURI string
}

// generation is one CA and query signing key pair.
type generation struct {
	id          string
	caCert      *x509.Certificate
	caKey       crypto.Signer
	signingKey  crypto.Signer
	signingPub  *ecdsa.PublicKey
	activatesAt time.Time
	retiresAt   *time.Time
}

func (g *generation) trustedAt(now time.Time) bool {
	return g.retiresAt == nil || now.Before(*g.retiresAt)
}

// Keyring is this instance's identity and every trusted generation of its CA and signing keys, shared by all replicas through the database. Replicas refresh it as they sweep, so a rotation started on one reaches the others within seconds.
type Keyring struct {
	InstanceID string
	store      *Store
	encryptor  *mcrypto.Encryptor
	now        func() time.Time

	mu   sync.RWMutex
	gens []*generation
}

// LoadKeyring reads the instance identity and its key generations, creating the first generation from cfg on first use. Replicas starting together serialise on the database, so exactly one first generation is made.
func LoadKeyring(ctx context.Context, store *Store, encryptor *mcrypto.Encryptor, cfg KeyConfig) (*Keyring, error) {
	if encryptor == nil {
		return nil, ErrEncryptionRequired
	}
	instanceID, err := store.instanceID(ctx)
	if err != nil {
		return nil, err
	}

	k := &Keyring{InstanceID: instanceID, store: store, encryptor: encryptor, now: time.Now}
	if err := store.withKeyLock(ctx, func(ctx context.Context, tx keyTx) error {
		exists, err := tx.anyGeneration(ctx)
		if err != nil || exists {
			return err
		}
		record, err := k.newGeneration(ctx, cfg, k.now())
		if err != nil {
			return err
		}
		return tx.insertGeneration(ctx, record)
	}); err != nil {
		return nil, err
	}
	if err := k.Refresh(ctx); err != nil {
		return nil, err
	}
	return k, nil
}

// Refresh reloads the trusted generations, reusing the signers of ones already loaded.
func (k *Keyring) Refresh(ctx context.Context) error {
	records, err := k.store.trustedGenerations(ctx)
	if err != nil {
		return err
	}

	k.mu.RLock()
	loaded := make(map[string]*generation, len(k.gens))
	for _, g := range k.gens {
		loaded[g.id] = g
	}
	k.mu.RUnlock()

	gens := make([]*generation, 0, len(records))
	for _, r := range records {
		if g, ok := loaded[r.ID]; ok {
			// Only the schedule can change on a stored generation; copy rather than mutate one readers may hold.
			updated := *g
			updated.activatesAt, updated.retiresAt = r.ActivatesAt, r.RetiresAt
			gens = append(gens, &updated)
			continue
		}
		g, err := k.openGeneration(ctx, r)
		if err != nil {
			return fmt.Errorf("opening key generation %s: %w", r.ID, err)
		}
		gens = append(gens, g)
	}
	if len(gens) == 0 {
		return errors.New("no trusted key generation")
	}

	k.mu.Lock()
	k.gens = gens
	k.mu.Unlock()
	return nil
}

// Rotate starts a new generation of keys from cfg. A graceful rotation publishes them now, starts using them after RotationOverlap and retires the current keys RotationOverlap after that, so connected serving processes move over without noticing. An immediate rotation, for when a key may be compromised, switches at once and retires every current key; serving processes must then be restarted, because they refuse a new identity that shares nothing with the one they trusted.
func (k *Keyring) Rotate(ctx context.Context, cfg KeyConfig, immediate bool) error {
	if cfg.CAKeyURI == "" && cfg.SigningKeyURI == "" && k.UsesKMS() {
		return ErrKMSKeysRequired
	}
	now := k.now()
	activates, retires := now.Add(RotationOverlap), now.Add(2*RotationOverlap)
	if immediate {
		activates, retires = now, now
	}

	err := k.store.withKeyLock(ctx, func(ctx context.Context, tx keyTx) error {
		record, err := k.newGeneration(ctx, cfg, activates)
		if err != nil {
			return err
		}
		if err := tx.retireGenerations(ctx, retires); err != nil {
			return err
		}
		return tx.insertGeneration(ctx, record)
	})
	if err != nil {
		return err
	}
	return k.Refresh(ctx)
}

// newGeneration creates the keys for a generation, or connects to the KMS keys cfg names, and issues the generation's CA certificate.
func (k *Keyring) newGeneration(ctx context.Context, cfg KeyConfig, activates time.Time) (*generationRecord, error) {
	if (cfg.CAKeyURI == "") != (cfg.SigningKeyURI == "") {
		return nil, errors.New("set both a CA key and a signing key URI, or neither")
	}

	record := &generationRecord{ActivatesAt: activates}
	var caKey, signingKey crypto.Signer
	if cfg.CAKeyURI != "" {
		var err error
		if caKey, err = kmsSigner(ctx, cfg.CAKeyURI); err != nil {
			return nil, fmt.Errorf("CA key: %w", err)
		}
		if signingKey, err = kmsSigner(ctx, cfg.SigningKeyURI); err != nil {
			return nil, fmt.Errorf("signing key: %w", err)
		}
		record.CAKeyURI, record.SigningKeyURI = &cfg.CAKeyURI, &cfg.SigningKeyURI
	} else {
		generated := make([]*ecdsa.PrivateKey, 2)
		sealed := make([]*string, 2)
		for i := range generated {
			key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				return nil, fmt.Errorf("generating key: %w", err)
			}
			der, err := x509.MarshalPKCS8PrivateKey(key)
			if err != nil {
				return nil, fmt.Errorf("encoding key: %w", err)
			}
			enc, err := k.encryptor.Encrypt(der)
			if err != nil {
				return nil, fmt.Errorf("encrypting key: %w", err)
			}
			generated[i], sealed[i] = key, &enc
		}
		caKey, signingKey = generated[0], generated[1]
		record.CAKey, record.SigningKey = sealed[0], sealed[1]
	}

	signingPub, err := p256Public(signingKey)
	if err != nil {
		return nil, fmt.Errorf("signing key: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(signingPub)
	if err != nil {
		return nil, err
	}
	record.SigningPublicKey = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	caDER, err := k.createCA(caKey)
	if err != nil {
		return nil, err
	}
	record.CACert = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}))
	return record, nil
}

// createCA self-signs a CA certificate whose name constraints confine it to this instance: it can only vouch for this instance's SPIFFE IDs and replica name, and never for IP addresses, so even a stolen CA key cannot mint a certificate anything else would accept.
func (k *Keyring) createCA(key crypto.Signer) ([]byte, error) {
	if _, err := p256Public(key); err != nil {
		return nil, fmt.Errorf("CA key: %w", err)
	}
	now := k.now()
	_, allIPv4, _ := net.ParseCIDR("0.0.0.0/0")
	_, allIPv6, _ := net.ParseCIDR("::/0")
	template := &x509.Certificate{
		SerialNumber:                mustSerial(),
		Subject:                     pkix.Name{Organization: []string{"Marmot"}, CommonName: "Marmot plugin serving CA " + k.InstanceID},
		NotBefore:                   now.Add(-clockSkew),
		NotAfter:                    now.Add(caValidity),
		KeyUsage:                    x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid:       true,
		IsCA:                        true,
		MaxPathLenZero:              true,
		PermittedDNSDomainsCritical: true,
		PermittedDNSDomains:         []string{ServerNameFor(k.InstanceID)},
		PermittedURIDomains:         []string{k.InstanceID},
		ExcludedIPRanges:            []*net.IPNet{allIPv4, allIPv6},
		URIs:                        []*url.URL{{Scheme: "spiffe", Host: k.InstanceID}},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		return nil, fmt.Errorf("creating CA certificate: %w", err)
	}
	return der, nil
}

// openGeneration loads a stored generation's signers: decrypting keys kept in the database, or connecting to KMS keys.
func (k *Keyring) openGeneration(ctx context.Context, r *generationRecord) (*generation, error) {
	block, _ := pem.Decode([]byte(r.CACert))
	if block == nil {
		return nil, errors.New("decoding CA certificate")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing CA certificate: %w", err)
	}

	caKey, err := k.openKey(ctx, r.CAKey, r.CAKeyURI)
	if err != nil {
		return nil, fmt.Errorf("CA key: %w", err)
	}
	signingKey, err := k.openKey(ctx, r.SigningKey, r.SigningKeyURI)
	if err != nil {
		return nil, fmt.Errorf("signing key: %w", err)
	}
	signingPub, err := p256Public(signingKey)
	if err != nil {
		return nil, err
	}

	// The stored certificate and public key must belong to the keys that sign, or the database was tampered with.
	if caPub, ok := caCert.PublicKey.(*ecdsa.PublicKey); !ok || !caPub.Equal(caKey.Public()) {
		return nil, errors.New("CA certificate does not match its key")
	}
	pubBlock, _ := pem.Decode([]byte(r.SigningPublicKey))
	if pubBlock == nil {
		return nil, errors.New("decoding signing public key")
	}
	storedPub, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil || !signingPub.Equal(storedPub) {
		return nil, errors.New("signing public key does not match its key")
	}

	return &generation{
		id:          r.ID,
		caCert:      caCert,
		caKey:       caKey,
		signingKey:  signingKey,
		signingPub:  signingPub,
		activatesAt: r.ActivatesAt,
		retiresAt:   r.RetiresAt,
	}, nil
}

func (k *Keyring) openKey(ctx context.Context, sealed, uri *string) (crypto.Signer, error) {
	if uri != nil {
		return kmsSigner(ctx, *uri)
	}
	if sealed == nil {
		return nil, errors.New("no key")
	}
	der, err := k.encryptor.Decrypt(*sealed)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parsing: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an ECDSA key")
	}
	return key, nil
}

// kmsSigner connects to a KMS key by sigstore URI and checks it is an ECDSA P-256 key, the one algorithm every supported KMS offers.
func kmsSigner(ctx context.Context, uri string) (crypto.Signer, error) {
	sv, err := kms.Get(ctx, uri, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("connecting to KMS key: %w", err)
	}
	signer, _, err := sv.CryptoSigner(ctx, func(error) {})
	if err != nil {
		return nil, fmt.Errorf("KMS signer: %w", err)
	}
	if _, err := p256Public(signer); err != nil {
		return nil, err
	}
	return signer, nil
}

func p256Public(key crypto.Signer) (*ecdsa.PublicKey, error) {
	pub, ok := key.Public().(*ecdsa.PublicKey)
	if !ok || pub.Curve != elliptic.P256() {
		return nil, errors.New("key must be ECDSA P-256")
	}
	return pub, nil
}

// trusted returns the generations trusted now, and the one in use: the latest whose activation has passed.
func (k *Keyring) trusted() (gens []*generation, active *generation) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	now := k.now()
	for _, g := range k.gens {
		if !g.trustedAt(now) {
			continue
		}
		gens = append(gens, g)
		if !g.activatesAt.After(now) && (active == nil || g.activatesAt.After(active.activatesAt)) {
			active = g
		}
	}
	return gens, active
}

func (k *Keyring) active() (*generation, error) {
	_, active := k.trusted()
	if active == nil {
		return nil, errors.New("no active key generation")
	}
	return active, nil
}

// Pool returns the trusted CA certificates.
func (k *Keyring) Pool() *x509.CertPool {
	gens, _ := k.trusted()
	pool := x509.NewCertPool()
	for _, g := range gens {
		pool.AddCert(g.caCert)
	}
	return pool
}

// Bundle returns the trusted CA certificates and query signing keys, as PEM, for a serving process to pin.
func (k *Keyring) Bundle() (cas string, signingKeys []string) {
	gens, _ := k.trusted()
	var b strings.Builder
	for _, g := range gens {
		b.Write(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: g.caCert.Raw}))
		der, err := x509.MarshalPKIXPublicKey(g.signingPub)
		if err != nil {
			continue
		}
		signingKeys = append(signingKeys, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})))
	}
	return b.String(), signingKeys
}

// Sign signs a serialised Query with the active signing key.
func (k *Keyring) Sign(message []byte) ([]byte, error) {
	g, err := k.active()
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(message)
	return g.signingKey.Sign(rand.Reader, digest[:], crypto.SHA256)
}

// ServerName is the name serving processes and replicas verify a replica's certificate against. It is fixed per instance rather than a real host name, so a replica is recognised by the CA that issued its certificate wherever it is reached from, behind any load balancer.
func (k *Keyring) ServerName() string {
	return ServerNameFor(k.InstanceID)
}

// ServerNameFor returns the replica server name for an instance; .invalid is reserved by RFC 2606 so the name can never resolve to a real host.
func ServerNameFor(instanceID string) string {
	return "replica." + instanceID + ".marmot.invalid"
}

// ServeID returns the SPIFFE ID Marmot puts in a serving process's certificate.
func (k *Keyring) ServeID(enrolmentID string) string {
	return "spiffe://" + k.InstanceID + "/serve/" + enrolmentID
}

func (k *Keyring) replicaID(replica string) string {
	return "spiffe://" + k.InstanceID + "/replica/" + replica
}

// IssueServeCert signs a client certificate for a serving process from its CSR. Only the CSR's public key is used: the subject, names and lifetime are Marmot's, so a serving process cannot ask for anything beyond its enrolment.
func (k *Keyring) IssueServeCert(csr *x509.CertificateRequest, enrolmentID string, notAfter time.Time) ([]byte, error) {
	spiffeID, err := url.Parse(k.ServeID(enrolmentID))
	if err != nil {
		return nil, err
	}
	return k.issue(&x509.Certificate{
		Subject:     pkix.Name{CommonName: enrolmentID},
		NotAfter:    notAfter,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		URIs:        []*url.URL{spiffeID},
	}, csr.PublicKey)
}

// issueReplicaCert signs a certificate a replica presents both as a server, to serving processes and other replicas, and as a client, when it forwards a query. It reports which generation issued it, so the replica can renew when the active generation changes.
func (k *Keyring) issueReplicaCert(pub crypto.PublicKey, replica string, notAfter time.Time) ([]byte, string, error) {
	spiffeID, err := url.Parse(k.replicaID(replica))
	if err != nil {
		return nil, "", err
	}
	g, err := k.active()
	if err != nil {
		return nil, "", err
	}
	der, err := k.issue(&x509.Certificate{
		Subject:     pkix.Name{CommonName: replica},
		NotAfter:    notAfter,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:    []string{k.ServerName()},
		URIs:        []*url.URL{spiffeID},
	}, pub)
	return der, g.id, err
}

func (k *Keyring) issue(template *x509.Certificate, pub crypto.PublicKey) ([]byte, error) {
	g, err := k.active()
	if err != nil {
		return nil, err
	}
	template.SerialNumber = mustSerial()
	template.NotBefore = k.now().Add(-clockSkew)
	template.KeyUsage = x509.KeyUsageDigitalSignature
	if template.NotAfter.After(g.caCert.NotAfter) {
		template.NotAfter = g.caCert.NotAfter
	}
	return x509.CreateCertificate(rand.Reader, template, g.caCert, pub, g.caKey)
}

// ParseCSR decodes a PEM certificate request and checks it is signed by an ECDSA P-256 or Ed25519 key, proving the requester holds the private key.
func ParseCSR(csrPEM string) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("csr is not a PEM certificate request")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing csr: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("csr signature: %w", err)
	}
	switch key := csr.PublicKey.(type) {
	case *ecdsa.PublicKey:
		if key.Curve != elliptic.P256() {
			return nil, fmt.Errorf("csr key must be ECDSA P-256 or Ed25519")
		}
	case ed25519.PublicKey:
	default:
		return nil, fmt.Errorf("csr key must be ECDSA P-256 or Ed25519")
	}
	return csr, nil
}

// publicKeySHA256 fingerprints a certificate's public key, binding an enrolment to the key it was issued for.
func publicKeySHA256(spki []byte) []byte {
	sum := sha256.Sum256(spki)
	return sum[:]
}

// peerIdentity is what a verified peer certificate says the peer is.
type peerIdentity struct {
	kind string // "serve" or "replica"
	id   string // enrolment or replica id
}

// identifyPeer reads the single SPIFFE URI in a certificate this instance's CA issued.
func (k *Keyring) identifyPeer(cert *x509.Certificate) (peerIdentity, error) {
	if len(cert.URIs) != 1 {
		return peerIdentity{}, fmt.Errorf("certificate must carry exactly one SPIFFE ID")
	}
	u := cert.URIs[0]
	if u.Scheme != "spiffe" || u.Host != k.InstanceID {
		return peerIdentity{}, fmt.Errorf("certificate is for another Marmot instance")
	}
	kind, rest, ok := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/")
	if !ok || (kind != "serve" && kind != "replica") {
		return peerIdentity{}, fmt.Errorf("unrecognised SPIFFE ID")
	}
	if _, err := uuid.Parse(rest); err != nil {
		return peerIdentity{}, fmt.Errorf("unrecognised SPIFFE ID")
	}
	return peerIdentity{kind: kind, id: rest}, nil
}

// mustSerial returns a random 128-bit certificate serial number, as RFC 5280 and CA/Browser Forum practice recommend.
func mustSerial() *big.Int {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		panic(fmt.Sprintf("reading random serial: %v", err))
	}
	return serial
}

// activeID returns the id of the generation in use, or "" when none is.
func (k *Keyring) activeID() string {
	_, active := k.trusted()
	if active == nil {
		return ""
	}
	return active.id
}

// trustVersion identifies the set of trusted generations, changing whenever one is added or retired.
func (k *Keyring) trustVersion() string {
	gens, _ := k.trusted()
	ids := make([]string, len(gens))
	for i, g := range gens {
		ids[i] = g.id
	}
	return strings.Join(ids, ",")
}

// GenerationStatus describes one trusted key generation.
type GenerationStatus struct {
	ID          string     `json:"id"`
	Storage     string     `json:"storage"`
	Active      bool       `json:"active"`
	ActivatesAt time.Time  `json:"activates_at"`
	RetiresAt   *time.Time `json:"retires_at,omitempty"`
} // @name ServeKeyGeneration

// Generations lists the trusted key generations, oldest first.
func (k *Keyring) Generations() []GenerationStatus {
	gens, active := k.trusted()
	out := make([]GenerationStatus, 0, len(gens))
	for _, g := range gens {
		storage := "database"
		if _, local := g.caKey.(*ecdsa.PrivateKey); !local {
			storage = "kms"
		}
		out = append(out, GenerationStatus{ID: g.id, Storage: storage, Active: g == active, ActivatesAt: g.activatesAt, RetiresAt: g.retiresAt})
	}
	return out
}

// UsesKMS reports whether the active generation's keys are held in a KMS.
func (k *Keyring) UsesKMS() bool {
	g, err := k.active()
	if err != nil {
		return false
	}
	_, local := g.caKey.(*ecdsa.PrivateKey)
	return !local
}
