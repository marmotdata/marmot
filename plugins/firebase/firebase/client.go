package firebase

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/firestore"
	firebaseapi "google.golang.org/api/firebase/v1beta1"
	firebasedatabase "google.golang.org/api/firebasedatabase/v1beta"
	firestoreadmin "google.golang.org/api/firestore/v1"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// clientOptions builds the options the three REST clients share.
func (s *Source) clientOptions() []option.ClientOption {
	var opts []option.ClientOption

	if s.config.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(s.config.Endpoint))
	}

	return append(opts, s.credentialOptions()...)
}

func (s *Source) credentialOptions() []option.ClientOption {
	switch {
	case s.config.DisableAuth:
		return []option.ClientOption{option.WithoutAuthentication()}
	case s.config.CredentialsJSON != "":
		return []option.ClientOption{option.WithCredentialsJSON([]byte(s.config.CredentialsJSON))}
	case s.config.CredentialsFile != "":
		return []option.ClientOption{option.WithCredentialsFile(s.config.CredentialsFile)}
	}
	return nil
}

func (s *Source) firestoreAdminService(ctx context.Context) (*firestoreadmin.Service, error) {
	service, err := firestoreadmin.NewService(ctx, s.clientOptions()...)
	if err != nil {
		return nil, fmt.Errorf("creating Firestore admin client: %w", err)
	}
	return service, nil
}

func (s *Source) realtimeDatabaseService(ctx context.Context) (*firebasedatabase.Service, error) {
	service, err := firebasedatabase.NewService(ctx, s.clientOptions()...)
	if err != nil {
		return nil, fmt.Errorf("creating Realtime Database client: %w", err)
	}
	return service, nil
}

func (s *Source) firebaseService(ctx context.Context) (*firebaseapi.Service, error) {
	service, err := firebaseapi.NewService(ctx, s.clientOptions()...)
	if err != nil {
		return nil, fmt.Errorf("creating Firebase client: %w", err)
	}
	return service, nil
}

// firestoreClient opens a data client for one Firestore database. Reading
// documents goes through the idiomatic client rather than the REST API
// because it is the only one that can talk to the Firestore emulator.
//
// The library redirects itself to an emulator on its own when
// FIRESTORE_EMULATOR_HOST is set. The endpoint config field does the same
// thing from the ingest config, which needs the connection built by hand: the
// emulator serves plain gRPC on the same port as its REST API, and it rejects
// the collection listing unless the request carries an admin token.
func (s *Source) firestoreClient(ctx context.Context, databaseID string) (*firestore.Client, error) {
	var opts []option.ClientOption

	if useEmulator(s.config.Endpoint, s.config.DisableAuth) {
		host := emulatorHost(s.config.Endpoint)
		conn, err := grpc.NewClient(host,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithPerRPCCredentials(emulatorCredentials{}))
		if err != nil {
			return nil, fmt.Errorf("connecting to %s: %w", host, err)
		}
		// firestore.Client.Close closes a connection passed in this way.
		opts = append(opts, option.WithGRPCConn(conn))
	} else {
		if s.config.Endpoint != "" {
			opts = append(opts, option.WithEndpoint(s.config.Endpoint))
		}
		opts = append(opts, s.credentialOptions()...)
	}

	client, err := firestore.NewClientWithDatabase(ctx, s.config.ProjectID, databaseID, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Firestore client for database %s: %w", databaseID, err)
	}
	return client, nil
}

// useEmulator says whether to dial the endpoint in the clear with the
// emulator's admin token. That drops TLS and sends a token that means nothing
// to Google, so it takes more than an endpoint to turn on: the config has to
// ask for no authentication as well. A custom endpoint on its own is a real
// server and is dialled the ordinary way.
func useEmulator(endpoint string, disableAuth bool) bool {
	return endpoint != "" && disableAuth
}

// emulatorHost turns an endpoint URL into the host and port a gRPC client
// dials. It returns an empty string when no endpoint is configured, which
// leaves the library's own FIRESTORE_EMULATOR_HOST handling in charge.
func emulatorHost(endpoint string) string {
	host := strings.TrimSuffix(endpoint, "/")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	return host
}

// emulatorCredentials sends the fixed token the Firestore emulator accepts as
// admin. Listing collection ids is a metadata operation and the emulator
// refuses it without one. The library sends the same value when it picks up
// FIRESTORE_EMULATOR_HOST itself.
type emulatorCredentials struct{}

func (emulatorCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer owner"}, nil
}

func (emulatorCredentials) RequireTransportSecurity() bool { return false }
