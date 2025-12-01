package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

func (s *Source) configureAuthentication() ([]kgo.Opt, error) {
	if s.config.Authentication == nil {
		return nil, nil
	}

	authType := s.config.Authentication.Type
	if authType == "none" || authType == "" {
		return nil, nil
	}

	if isSASLType(authType) {
		if err := validateSASLConfig(s.config.Authentication); err != nil {
			return nil, err
		}
	}

	mechanism, err := createSASLMechanism(s.config.Authentication)
	if err != nil {
		return nil, err
	}

	var opts []kgo.Opt
	if mechanism != nil {
		opts = append(opts, kgo.SASL(mechanism))
	}

	return opts, nil
}

func isSASLType(authType string) bool {
	return authType == "sasl_plaintext" || authType == "sasl_ssl"
}

func validateSASLConfig(auth *AuthConfig) error {
	if auth.Username == "" {
		return fmt.Errorf("username is required for %s authentication", auth.Type)
	}
	if auth.Password == "" {
		return fmt.Errorf("password is required for %s authentication", auth.Type)
	}
	if auth.Mechanism == "" {
		return fmt.Errorf("mechanism is required for %s authentication", auth.Type)
	}
	return nil
}

func createSASLMechanism(auth *AuthConfig) (sasl.Mechanism, error) {
	if !isSASLType(auth.Type) {
		return nil, nil
	}

	switch auth.Mechanism {
	case "PLAIN":
		return plain.Auth{
			User: auth.Username,
			Pass: auth.Password,
		}.AsMechanism(), nil
	case "SCRAM-SHA-256":
		return scram.Auth{
			User: auth.Username,
			Pass: auth.Password,
		}.AsSha256Mechanism(), nil
	case "SCRAM-SHA-512":
		return scram.Auth{
			User: auth.Username,
			Pass: auth.Password,
		}.AsSha512Mechanism(), nil
	default:
		return nil, fmt.Errorf("unsupported SASL mechanism: %s", auth.Mechanism)
	}
}

func (s *Source) configureTLS() (*kgo.Opt, error) {
	if s.config.TLS == nil || !s.config.TLS.Enabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	tlsConfig.InsecureSkipVerify = s.config.TLS.SkipVerify

	if s.config.TLS.CACertPath != "" {
		caCert, err := os.ReadFile(s.config.TLS.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert file: %w", err)
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA cert to pool")
		}
		tlsConfig.RootCAs = certPool
	}

	if s.config.TLS.CertPath != "" && s.config.TLS.KeyPath != "" {
		cert, err := tls.LoadX509KeyPair(
			s.config.TLS.CertPath,
			s.config.TLS.KeyPath,
		)
		if err != nil {
			return nil, fmt.Errorf("loading client cert/key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	opt := kgo.DialTLSConfig(tlsConfig)
	return &opt, nil
}
