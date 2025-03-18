package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	httptransport "github.com/go-openapi/runtime/client"
	_ "github.com/lib/pq"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/users"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/models"
	"github.com/marmotdata/marmot/tests/e2e/internal/utils"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

type TestEnvironment struct {
	Config             utils.TestConfig
	ContainerManager   *utils.ContainerManager
	APIClient          *client.Marmot
	APIKey             string
	LocalstackID       string
	LocalstackPort     string
	RedpandaID         string
	RedpandaPort       string
	SchemaRegistryPort string
	KafkaClient        *kgo.Client
	KafkaAdminClient   *kadm.Client
	IcebergRESTID      string
	IcebergRESTPort    string
	HasSchemaRegistry  bool
	PostgresPort       string
}

func SetupTestEnvironment(t *testing.T) (*TestEnvironment, error) {
	ctx := context.Background()
	cm, err := utils.NewContainerManager(ctx)
	require.NoError(t, err)

	config := utils.NewDefaultConfig()

	// Build Marmot
	projectRoot := filepath.Join("..", "..")
	require.NoError(t, cm.BuildMarmot(ctx, projectRoot))

	// Start Postgres
	_, err = cm.StartPostgres(config)
	require.NoError(t, err)
	require.NoError(t, utils.WaitForPostgres(config))

	// Start PostgreSQL for testing
	postgresID, postgresPort, err := startPostgres(ctx, cm, config.NetworkName)
	require.NoError(t, err)
	require.NoError(t, waitForPostgres(postgresPort))
	config.PostgresContainerName = postgresID

	// Start Marmot
	_, err = cm.StartMarmotContainer(config)
	require.NoError(t, err)
	require.NoError(t, utils.WaitForApplication(config.ApplicationPort))

	// Setup API client
	transport := httptransport.New("localhost:"+config.ApplicationPort, "/api/v1", []string{"http"})
	apiClient := client.New(transport, nil)

	// Login and create API key
	username := "admin"
	password := "admin"
	loginParams := users.NewPostUsersLoginParams()
	loginParams.SetCredentials(&models.UsersLoginRequest{
		Username: &username,
		Password: &password,
	})

	loginResp, err := apiClient.Users.PostUsersLogin(loginParams)
	require.NoError(t, err)

	transport.DefaultAuthentication = httptransport.BearerToken(loginResp.Payload.AccessToken)

	// Create API key
	keyName := "test-key"
	createKeyParams := users.NewPostUsersApikeysParams()
	createKeyParams.SetKey(&models.UsersCreateAPIKeyRequest{
		Name: &keyName,
	})

	keyResp, err := apiClient.Users.PostUsersApikeys(createKeyParams)
	require.NoError(t, err)

	// Start Localstack - shared across all tests
	localstackID, localstackPort, err := startLocalstack(ctx, cm, config.NetworkName)
	require.NoError(t, err)
	require.NoError(t, waitForLocalstack())

	// Initialize test environment
	env := &TestEnvironment{
		Config:           config,
		ContainerManager: cm,
		APIClient:        apiClient,
		APIKey:           keyResp.Payload.Key,
		LocalstackID:     localstackID,
		LocalstackPort:   localstackPort,
		PostgresPort:     postgresPort,
	}

	return env, nil
}

func (env *TestEnvironment) Cleanup() {
	// Close Kafka client if it exists
	if env.KafkaClient != nil {
		env.KafkaClient.Close()
	}

	// Clean up all containers
	env.ContainerManager.Close()
}

// EnsureRedpandaStarted ensures the Redpanda container is started
func (env *TestEnvironment) EnsureRedpandaStarted(ctx context.Context, withSchemaRegistry bool) error {
	// Skip if already started with the right configuration
	if env.RedpandaID != "" {
		if withSchemaRegistry && !env.HasSchemaRegistry {
			// Need to recreate with schema registry
			env.ContainerManager.CleanupContainer(env.RedpandaID)
			env.RedpandaID = ""
		} else if !withSchemaRegistry && env.HasSchemaRegistry {
			// Using existing instance is fine, schema registry is a bonus
			return nil
		} else {
			// Already started with correct configuration
			return nil
		}
	}

	var err error
	var redpandaID, redpandaPort, schemaRegistryPort string

	if withSchemaRegistry {
		redpandaID, redpandaPort, schemaRegistryPort, err = startRedpandaWithSchemaRegistry(ctx, env.ContainerManager, env.Config.NetworkName)
	} else {
		redpandaID, redpandaPort, err = startRedpanda(ctx, env.ContainerManager, env.Config.NetworkName)
	}

	if err != nil {
		return err
	}

	// Create Kafka client
	kafkaClient, err := createKafkaClient(fmt.Sprintf("redpanda-test:%s", redpandaPort))
	if err != nil {
		return err
	}

	env.RedpandaID = redpandaID
	env.RedpandaPort = redpandaPort
	env.SchemaRegistryPort = schemaRegistryPort
	env.KafkaClient = kafkaClient
	env.KafkaAdminClient = kadm.NewClient(kafkaClient)
	env.HasSchemaRegistry = withSchemaRegistry

	return nil
}

// EnsurePostgresStarted ensures the PostgreSQL container is started for testing
func (env *TestEnvironment) EnsurePostgresStarted(ctx context.Context) error {
	// Use the PostgreSQL instance that was started during environment setup
	if env.Config.PostgresContainerName == "" || env.PostgresPort == "" {
		// If PostgreSQL wasn't started during setup, start it now
		postgresID, postgresPort, err := startPostgres(ctx, env.ContainerManager, env.Config.NetworkName)
		if err != nil {
			return fmt.Errorf("failed to start PostgreSQL: %w", err)
		}

		// Wait for PostgreSQL to be ready
		if err := waitForPostgres(postgresPort); err != nil {
			return fmt.Errorf("PostgreSQL failed to become ready: %w", err)
		}

		env.PostgresPort = postgresPort
		env.Config.PostgresContainerName = postgresID
	}

	// Check if PostgreSQL is running
	if err := checkPostgresConnection("localhost:"+env.PostgresPort, 10*time.Second); err != nil {
		// If not running, restart it
		env.ContainerManager.CleanupContainer(env.Config.PostgresContainerName)
		postgresID, postgresPort, err := startPostgres(ctx, env.ContainerManager, env.Config.NetworkName)
		if err != nil {
			return fmt.Errorf("failed to restart PostgreSQL: %w", err)
		}

		// Wait for PostgreSQL to be ready
		if err := waitForPostgres(postgresPort); err != nil {
			return fmt.Errorf("PostgreSQL failed to become ready after restart: %w", err)
		}

		env.PostgresPort = postgresPort
		env.Config.PostgresContainerName = postgresID
	}

	return nil
}

// EnsureIcebergRESTStarted ensures the Iceberg REST server is started
func (env *TestEnvironment) EnsureIcebergRESTStarted(ctx context.Context) error {
	if env.IcebergRESTID != "" {
		// Already started, check if it's still healthy
		err := waitForIcebergRESTServer(fmt.Sprintf("localhost:%s", env.IcebergRESTPort), 10*time.Second)
		if err == nil {
			return nil // Container is healthy
		}

		// Container doesn't seem to be responsive, let's recreate it
		env.ContainerManager.CleanupContainer(env.IcebergRESTID)
		env.IcebergRESTID = ""
	}

	icebergRESTID, icebergRESTPort, err := startIcebergRESTServer(ctx, env.ContainerManager, env.Config.NetworkName)
	if err != nil {
		return fmt.Errorf("failed to start Iceberg REST server: %w", err)
	}

	// Wait for server to be ready
	err = waitForIcebergRESTServer(fmt.Sprintf("localhost:%s", icebergRESTPort), 120*time.Second)
	if err != nil {
		env.ContainerManager.CleanupContainer(icebergRESTID)
		return fmt.Errorf("Iceberg REST server failed to become ready: %w", err)
	}

	env.IcebergRESTID = icebergRESTID
	env.IcebergRESTPort = icebergRESTPort

	return nil
}

// startLocalstack starts a Localstack container for AWS service mocking
func startLocalstack(ctx context.Context, cm *utils.ContainerManager, networkName string) (string, string, error) {
	// Setup Localstack container with AWS services enabled
	localstackConfig := &container.Config{
		Image: "localstack/localstack:latest",
		Env: []string{
			"SERVICES=sns,sqs,s3",
			"DEBUG=1",
			"AWS_DEFAULT_REGION=us-east-1",
		},
		ExposedPorts: nat.PortSet{
			"4566/tcp": struct{}{},
		},
	}

	localstackHostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
		PortBindings: nat.PortMap{
			"4566/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "4566"}},
		},
	}

	localstackID, err := cm.StartContainer(localstackConfig, localstackHostConfig, "localstack-test")
	if err != nil {
		return "", "", fmt.Errorf("failed to start Localstack container: %w", err)
	}

	return localstackID, "4566", nil
}

// startPostgres starts a PostgreSQL container for testing
func startPostgres(ctx context.Context, cm *utils.ContainerManager, networkName string) (string, string, error) {
	// Setup PostgreSQL container
	postgresConfig := &container.Config{
		Image: "postgres:14",
		Env: []string{
			"POSTGRES_PASSWORD=postgres",
			"POSTGRES_USER=postgres",
		},
		ExposedPorts: nat.PortSet{
			"5432/tcp": struct{}{},
		},
	}

	postgresHostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
		PortBindings: nat.PortMap{
			"5432/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "5433"}}, // Changed port to 5433
		},
	}

	postgresID, err := cm.StartContainer(postgresConfig, postgresHostConfig, "postgres-test-plugin")
	if err != nil {
		return "", "", fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}

	return postgresID, "5433", nil
}

func waitForLocalstack() error {
	client := &http.Client{Timeout: 5 * time.Second}
	url := "http://localhost:4566/_localstack/health"

	for i := 0; i < 60; i++ {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timeout waiting for localstack")
}

// checkPostgresConnection checks if PostgreSQL is responding to connections
func checkPostgresConnection(endpoint string, timeout time.Duration) error {
	db, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=postgres password=postgres sslmode=disable",
		strings.Split(endpoint, ":")[0],
		strings.Split(endpoint, ":")[1]))
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return err
	}

	return nil
}

// waitForPostgres waits for PostgreSQL to be ready
func waitForPostgres(port string) error {
	endpoint := fmt.Sprintf("localhost:%s", port)
	deadline := time.Now().Add(60 * time.Second)

	for time.Now().Before(deadline) {
		if err := checkPostgresConnection(endpoint, 5*time.Second); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for PostgreSQL to become ready")
}

// startRedpanda starts a Redpanda container for testing
func startRedpanda(ctx context.Context, cm *utils.ContainerManager, networkName string) (string, string, error) {
	// Setup Redpanda container
	redpandaConfig := &container.Config{
		Image: "redpandadata/redpanda:latest",
		Cmd: []string{
			"redpanda",
			"start",
			"--smp=1",
			"--memory=1G",
			"--reserve-memory=0M",
			"--overprovisioned",
			"--node-id=0",
			"--check=false",
			"--kafka-addr=0.0.0.0:9092",
			"--advertise-kafka-addr=redpanda-test:9092",
		},
		ExposedPorts: nat.PortSet{
			"9092/tcp": struct{}{},
		},
	}

	redpandaHostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
		PortBindings: nat.PortMap{
			"9092/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "9092"}},
		},
	}

	redpandaID, err := cm.StartContainer(redpandaConfig, redpandaHostConfig, "redpanda-test")
	if err != nil {
		return "", "", fmt.Errorf("failed to start Redpanda container: %w", err)
	}

	time.Sleep(30 * time.Second)

	bootstrapServer := "redpanda-test:9092"
	if err := waitForKafkaBroker(bootstrapServer, 180*time.Second); err != nil {
		return "", "", fmt.Errorf("kafka broker not ready: %w", err)
	}

	return redpandaID, "9092", nil
}

// startRedpandaWithSchemaRegistry starts Redpanda with Schema Registry enabled
func startRedpandaWithSchemaRegistry(ctx context.Context, cm *utils.ContainerManager, networkName string) (string, string, string, error) {
	// Setup Redpanda container with Schema Registry
	redpandaConfig := &container.Config{
		Image: "redpandadata/redpanda:latest",
		Cmd: []string{
			"redpanda",
			"start",
			"--smp=1",
			"--memory=1G",
			"--reserve-memory=0M",
			"--overprovisioned",
			"--node-id=0",
			"--check=false",
			"--kafka-addr=0.0.0.0:9092",
			"--advertise-kafka-addr=redpanda-test:9092",
			"--schema-registry-addr=0.0.0.0:8081",
		},
		ExposedPorts: nat.PortSet{
			"9092/tcp": struct{}{},
			"8081/tcp": struct{}{},
		},
	}

	redpandaHostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
		PortBindings: nat.PortMap{
			"9092/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "9092"}},
			"8081/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "8081"}},
		},
	}

	redpandaID, err := cm.StartContainer(redpandaConfig, redpandaHostConfig, "redpanda-test")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to start Redpanda container: %w", err)
	}

	// Wait for Redpanda and Schema Registry to start
	time.Sleep(15 * time.Second)

	bootstrapServer := "redpanda-test:9092"
	if err := waitForKafkaBroker(bootstrapServer, 180*time.Second); err != nil {
		return "", "", "", fmt.Errorf("kafka broker not ready: %w", err)
	}

	// Check if Schema Registry is up
	timeout := 60 * time.Second
	schemaRegistryURL := fmt.Sprintf("http://localhost:8081")
	if err := waitForSchemaRegistry(schemaRegistryURL, timeout); err != nil {
		return "", "", "", fmt.Errorf("schema registry not ready: %w", err)
	}

	return redpandaID, "9092", "8081", nil
}

// startIcebergRESTServer starts an Iceberg REST catalog server
func startIcebergRESTServer(ctx context.Context, cm *utils.ContainerManager, networkName string) (string, string, error) {
	// We'll use Tabular's REST service for this test
	// Create a volume for local storage instead of using S3
	icebergRESTConfig := &container.Config{
		Image: "tabulario/iceberg-rest:0.5.0",
		Env: []string{
			"CATALOG_BACKEND=memory",
			"CATALOG_WAREHOUSE=/tmp/warehouse",
			"WAREHOUSE_PATH=/tmp/warehouse",
			"CATALOG_IMPL=org.apache.iceberg.rest.RESTCatalog",
		},
		ExposedPorts: nat.PortSet{
			"8181/tcp": struct{}{},
		},
	}

	icebergRESTHostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
		PortBindings: nat.PortMap{
			"8181/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "8181"}},
		},
	}

	icebergRESTID, err := cm.StartContainer(icebergRESTConfig, icebergRESTHostConfig, "rest-test")
	if err != nil {
		return "", "", fmt.Errorf("failed to start Iceberg REST server container: %w", err)
	}

	return icebergRESTID, "8181", nil
}

// waitForIcebergRESTServer waits for the Iceberg REST server to be ready
func waitForIcebergRESTServer(endpoint string, timeout time.Duration) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Test multiple endpoints to ensure the service is fully ready
	urls := []string{
		fmt.Sprintf("http://%s/v1/config", endpoint),
		fmt.Sprintf("http://%s/v1/namespaces", endpoint),
	}

	deadline := time.Now().Add(timeout)
	attempt := 0

	for time.Now().Before(deadline) {
		attempt++
		allEndpointsOk := true

		for _, url := range urls {
			resp, err := client.Get(url)
			if err != nil {
				allEndpointsOk = false
				break
			}

			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				allEndpointsOk = false
				break
			}
		}

		if allEndpointsOk {
			return nil
		}

		// Backoff with a cap of 5 seconds
		backoffTime := float64(attempt * 500)
		if backoffTime > 5000 {
			backoffTime = 5000
		}
		sleepTime := time.Duration(backoffTime) * time.Millisecond
		time.Sleep(sleepTime)
	}

	return fmt.Errorf("timeout waiting for Iceberg REST server at %s after %d attempts", endpoint, attempt)
}

// Kafka helper functions
func createKafkaClient(bootstrapServer string) (*kgo.Client, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(bootstrapServer),
		kgo.ClientID("marmot-kafka-test-client"),
	}

	// Create Kafka client
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Kafka client: %w", err)
	}

	return client, nil
}

func waitForKafkaBroker(bootstrapServer string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		opts := []kgo.Opt{
			kgo.SeedBrokers(bootstrapServer),
			kgo.ClientID("healthcheck"),
		}

		client, err := kgo.NewClient(opts...)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		admin := kadm.NewClient(client)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err = admin.ListTopics(ctx)
		cancel()
		client.Close()

		if err == nil {
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for Kafka broker to be ready")
}

// waitForSchemaRegistry waits for Schema Registry to be available
func waitForSchemaRegistry(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(fmt.Sprintf("%s/subjects", baseURL))
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for Schema Registry to be ready")
}
