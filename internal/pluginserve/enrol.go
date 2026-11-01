package pluginserve

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/plugin"
)

// CertLifetime is how long a serving process's certificate lasts. The process enrols again at half-life with its Marmot credential, so once that credential is revoked the process is cut off within one lifetime; deactivating its principal or removing its serve permission cuts it off at the next sweep.
const CertLifetime = 10 * time.Minute

const maxSourcesPerEnrolment = 100

var (
	// ErrInvalidEnrolment is returned for a malformed enrolment request.
	ErrInvalidEnrolment = errors.New("invalid enrolment")
	// ErrForbidden is returned when the principal may not serve.
	ErrForbidden = errors.New("not permitted to serve")
)

// sourceName is deliberately narrower than a pipeline name: a served source's name is chosen by whoever holds a serve credential and appears in URLs, logs and the UI.
var sourceName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// EnrolSource is one source a serving process offers. Config is shown in Marmot for reference only, so the serving process sends it masked and Marmot masks it again.
type EnrolSource struct {
	Name     string         `json:"name"`
	PluginID string         `json:"plugin_id"`
	Config   map[string]any `json:"config,omitempty"`
}

// EnrolInput is a serving process's request for a certificate.
type EnrolInput struct {
	CSR     string        `json:"csr"`
	Sources []EnrolSource `json:"sources"`
}

// EnrolResult is everything a serving process needs to connect: its certificate, the trusted CAs and query signing keys to pin, and where to dial. More than one CA and key are listed while Marmot rotates them.
type EnrolResult struct {
	EnrolmentID string    `json:"enrolment_id"`
	InstanceID  string    `json:"instance_id"`
	Certificate string    `json:"certificate"`
	CABundle    string    `json:"ca_bundle"`
	SigningKeys []string  `json:"signing_keys"`
	ExpiresAt   time.Time `json:"expires_at"`
	// Address is the host:port to dial; when empty the serving process uses its Marmot host name with Port.
	Address string `json:"address,omitempty"`
	Port    string `json:"port"`
}

// SourceClaimer records which principal serves each source, all or nothing. Implemented by runs.ScheduleService.
type SourceClaimer interface {
	ClaimServedSources(ctx context.Context, schedules []*runs.Schedule) error
}

// Enroller issues certificates to serving processes.
type Enroller struct {
	keyring  *Keyring
	store    *Store
	sources  SourceClaimer
	address  string
	port     string
	lifetime time.Duration
	now      func() time.Time
}

func NewEnroller(keyring *Keyring, store *Store, sources SourceClaimer, address, port string) *Enroller {
	return &Enroller{keyring: keyring, store: store, sources: sources, address: address, port: port, lifetime: CertLifetime, now: time.Now}
}

// Enrol claims the requested sources for the principal and issues a short-lived client certificate for the CSR's key, bound to the credential the request authenticated with. The principal must hold ingestion:serve; every source name must be free or already served by this same principal, so a serve credential can never take over a source someone else configured.
func (e *Enroller) Enrol(ctx context.Context, principal auth.Principal, credential auth.Credential, in EnrolInput) (*EnrolResult, error) {
	if principal == nil || !principal.HasPermission("ingestion", "serve") {
		return nil, ErrForbidden
	}
	if err := e.checkCredential(ctx, credential); err != nil {
		return nil, err
	}
	csr, err := ParseCSR(in.CSR)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEnrolment, err)
	}
	if err := validateSources(in.Sources); err != nil {
		return nil, err
	}

	principalType, principalID := string(principal.Type()), principal.ID()
	names := make([]string, 0, len(in.Sources))
	claims := make([]*runs.Schedule, 0, len(in.Sources))
	for _, src := range in.Sources {
		claims = append(claims, &runs.Schedule{
			Name:         src.Name,
			PluginID:     src.PluginID,
			Config:       maskForDisplay(src.PluginID, src.Config),
			ServedByType: &principalType,
			ServedByID:   &principalID,
		})
		names = append(names, src.Name)
	}
	if err := e.sources.ClaimServedSources(ctx, claims); err != nil {
		if errors.Is(err, runs.ErrSourceOwned) {
			return nil, fmt.Errorf("%w: %v", ErrForbidden, err)
		}
		return nil, err
	}

	enrolment := &Enrolment{
		PrincipalType:   principalType,
		PrincipalID:     principalID,
		AuditSubject:    principal.AuditSubject(),
		Sources:         names,
		PublicKeySHA256: publicKeySHA256(csr.RawSubjectPublicKeyInfo),
		Credential:      credential,
		ExpiresAt:       e.now().Add(e.lifetime),
	}
	if err := e.store.createEnrolment(ctx, enrolment); err != nil {
		return nil, err
	}

	der, err := e.keyring.IssueServeCert(csr, enrolment.ID, enrolment.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("issuing certificate: %w", err)
	}

	cas, signingKeys := e.keyring.Bundle()
	return &EnrolResult{
		EnrolmentID: enrolment.ID,
		InstanceID:  e.keyring.InstanceID,
		Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		CABundle:    cas,
		SigningKeys: signingKeys,
		ExpiresAt:   enrolment.ExpiresAt,
		Address:     e.address,
		Port:        e.port,
	}, nil
}

// checkCredential refuses a credential whose revocation Marmot could not see. Every enrolment records what it authenticated with so revoking it closes the connection; an API key issued before keys had lookups cannot be tracked that way, so it has to be replaced before it can serve.
func (e *Enroller) checkCredential(ctx context.Context, c auth.Credential) error {
	if c.Type == "" {
		return fmt.Errorf("%w: unknown credential", ErrForbidden)
	}
	if !c.IsAPIKey() {
		return nil
	}
	ok, err := e.store.apiKeyExists(ctx, c)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: this API key predates revocation tracking; create a new key to serve with", ErrForbidden)
	}
	return nil
}

func validateSources(sources []EnrolSource) error {
	if len(sources) == 0 || len(sources) > maxSourcesPerEnrolment {
		return fmt.Errorf("%w: between 1 and %d sources are required", ErrInvalidEnrolment, maxSourcesPerEnrolment)
	}
	seen := make(map[string]bool, len(sources))
	for _, src := range sources {
		if !sourceName.MatchString(src.Name) {
			return fmt.Errorf("%w: source name %q must be 1-128 letters, digits, dots, dashes or underscores", ErrInvalidEnrolment, src.Name)
		}
		if seen[src.Name] {
			return fmt.Errorf("%w: source %q is listed twice", ErrInvalidEnrolment, src.Name)
		}
		seen[src.Name] = true
		if src.PluginID == "" || len(src.PluginID) > 128 {
			return fmt.Errorf("%w: source %q has no plugin id", ErrInvalidEnrolment, src.Name)
		}
	}
	return nil
}

// maskForDisplay masks a served source's config with the plugin's spec, or drops it when this server does not have the plugin, so Marmot never stores a credential a serving process sent by mistake.
func maskForDisplay(pluginID string, config map[string]any) map[string]any {
	entry, err := plugin.GetRegistry().Get(pluginID)
	if err != nil || config == nil {
		return nil
	}
	return plugin.MaskSensitiveFieldsFromSpec(config, entry.Meta.ConfigSpec)
}
