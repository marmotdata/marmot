package mongodb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func (s *Source) connect(ctx context.Context) error {
	var uri string

	if s.config.ConnectionURI != "" {
		uri = s.config.ConnectionURI
	} else {
		authPart := ""
		if s.config.User != "" {
			authPart = fmt.Sprintf("%s:%s@", s.config.User, s.config.Password)
		}

		authSource := ""
		if s.config.AuthSource != "" {
			authSource = fmt.Sprintf("authSource=%s", s.config.AuthSource)
		}

		tlsParam := ""
		if s.config.TLS {
			tlsParam = "tls=true"
			if s.config.TLSInsecure {
				tlsParam += "&tlsInsecure=true"
			}
		}

		params := []string{}
		for _, param := range []string{authSource, tlsParam} {
			if param != "" {
				params = append(params, param)
			}
		}

		paramStr := ""
		if len(params) > 0 {
			paramStr = "?" + strings.Join(params, "&")
		}

		uri = fmt.Sprintf("mongodb://%s%s:%d/%s", authPart, s.config.Host, s.config.Port, paramStr)
	}

	clientOptions := options.Client().ApplyURI(uri)
	clientOptions.SetConnectTimeout(15 * time.Second)
	clientOptions.SetServerSelectionTimeout(15 * time.Second)

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client, err := mongo.Connect(timeoutCtx, clientOptions)
	if err != nil {
		return fmt.Errorf("connecting to MongoDB: %w", err)
	}

	if err = client.Ping(timeoutCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return fmt.Errorf("pinging MongoDB server: %w", err)
	}

	s.client = client
	return nil
}

func (s *Source) disconnect(ctx context.Context) {
	if s.client != nil {
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = s.client.Disconnect(timeoutCtx)
		s.client = nil
	}
}
