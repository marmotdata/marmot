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
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/users"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/models"
	"github.com/marmotdata/marmot/tests/e2e/internal/utils"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type TestEnvironment struct {
	Config               utils.TestConfig
	ContainerManager     *utils.ContainerManager
	APIClient            *client.Marmot
	APIKey               string
	LocalstackID         string
	LocalstackPort       string
	RedpandaID           string
	RedpandaPort         string
	SchemaRegistryPort   string
	KafkaClient          *kgo.Client
	KafkaAdminClient     *kadm.Client
	HasSchemaRegistry    bool
	PostgresPort         string
	MySQLID              string
	MySQLPort            string
	BigQueryEmulatorID   string
	BigQueryEmulatorPort string
}

func SetupTestEnvironment(t *testing.T) (*TestEnvironment, error) {
	ctx := context.Background()
	cm, err := utils.NewContainerManager(ctx)
	require.NoError(t, err)

	config := utils.NewDefaultConfig()

	// Build
	projectRoot := filepath.Join("..", "..")
	require.NoError(t, cm.BuildMarmot(ctx, projectRoot))

	// Start postgres
	_, err = cm.StartPostgres(config)
	require.NoError(t, err)
	require.NoError(t, utils.WaitForPostgres(config))

	postgresID, postgresPort, err := startPostgres(ctx, cm, config.NetworkName)
	require.NoError(t, err)
	require.NoError(t, waitForPostgres(postgresPort))
	config.PostgresContainerName = postgresID

	// Start marmot
	_, err = cm.StartMarmotContainer(config)
	require.NoError(t, err)
	require.NoError(t, utils.WaitForApplication(config.ApplicationPort))

	// Setup API client
	transport := httptransport.New("localhost:"+config.ApplicationPort, "/api/v1", []string{"http"})
	apiClient := client.New(transport, nil)

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
	if env.KafkaClient != nil {
		env.KafkaClient.Close()
	}

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
			return nil
		} else {
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
	if env.Config.PostgresContainerName == "" || env.PostgresPort == "" {
		postgresID, postgresPort, err := startPostgres(ctx, env.ContainerManager, env.Config.NetworkName)
		if err != nil {
			return fmt.Errorf("failed to start PostgreSQL: %w", err)
		}

		if err := waitForPostgres(postgresPort); err != nil {
			return fmt.Errorf("PostgreSQL failed to become ready: %w", err)
		}

		env.PostgresPort = postgresPort
		env.Config.PostgresContainerName = postgresID
	}

	if err := checkPostgresConnection("localhost:"+env.PostgresPort, 10*time.Second); err != nil {
		env.ContainerManager.CleanupContainer(env.Config.PostgresContainerName)
		postgresID, postgresPort, err := startPostgres(ctx, env.ContainerManager, env.Config.NetworkName)
		if err != nil {
			return fmt.Errorf("failed to restart PostgreSQL: %w", err)
		}

		if err := waitForPostgres(postgresPort); err != nil {
			return fmt.Errorf("PostgreSQL failed to become ready after restart: %w", err)
		}

		env.PostgresPort = postgresPort
		env.Config.PostgresContainerName = postgresID
	}

	return nil
}

// EnsureBigQueryStarted ensures the BigQuery emulator container is started
func (env *TestEnvironment) EnsureBigQueryStarted(ctx context.Context) error {
	if env.BigQueryEmulatorID == "" || env.BigQueryEmulatorPort == "" {
		bigqueryID, bigqueryPort, err := startBigQueryEmulator(ctx, env.ContainerManager, env.Config.NetworkName)
		if err != nil {
			return fmt.Errorf("failed to start BigQuery emulator: %w", err)
		}

		if err := waitForBigQueryEmulator(bigqueryPort); err != nil {
			return fmt.Errorf("BigQuery emulator failed to become ready: %w", err)
		}

		env.BigQueryEmulatorID = bigqueryID
		env.BigQueryEmulatorPort = bigqueryPort
	}

	if err := checkBigQueryEmulatorConnection("localhost:"+env.BigQueryEmulatorPort, 10*time.Second); err != nil {
		env.ContainerManager.CleanupContainer(env.BigQueryEmulatorID)
		bigqueryID, bigqueryPort, err := startBigQueryEmulator(ctx, env.ContainerManager, env.Config.NetworkName)
		if err != nil {
			return fmt.Errorf("failed to restart BigQuery emulator: %w", err)
		}

		if err := waitForBigQueryEmulator(bigqueryPort); err != nil {
			return fmt.Errorf("BigQuery emulator failed to become ready after restart: %w", err)
		}

		env.BigQueryEmulatorID = bigqueryID
		env.BigQueryEmulatorPort = bigqueryPort
	}

	return nil
}

func startBigQueryEmulator(ctx context.Context, cm *utils.ContainerManager, networkName string) (string, string, error) {
	bigqueryConfig := &container.Config{
		Image: "ghcr.io/goccy/bigquery-emulator:latest",
		Cmd: []string{
			"--project=test-project",
			"--dataset=production_analytics,staging_data,temp_test",
		},
		ExposedPorts: nat.PortSet{
			"9050/tcp": struct{}{},
		},
	}

	bigqueryHostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
		PortBindings: nat.PortMap{
			"9050/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "9050"}},
		},
	}

	bigqueryID, err := cm.StartContainer(bigqueryConfig, bigqueryHostConfig, "bigquery-emulator-test")
	if err != nil {
		return "", "", fmt.Errorf("failed to start BigQuery emulator container: %w", err)
	}

	return bigqueryID, "9050", nil
}

func waitForBigQueryEmulator(port string) error {
	endpoint := fmt.Sprintf("localhost:%s", port)
	deadline := time.Now().Add(60 * time.Second)

	for time.Now().Before(deadline) {
		if err := checkBigQueryEmulatorConnection(endpoint, 5*time.Second); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for BigQuery emulator to become ready")
}

func checkBigQueryEmulatorConnection(endpoint string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	url := fmt.Sprintf("http://%s", endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// startLocalstack starts a Localstack container for AWS service mocking
func startLocalstack(ctx context.Context, cm *utils.ContainerManager, networkName string) (string, string, error) {
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
			"5432/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "5435"}},
		},
	}

	postgresID, err := cm.StartContainer(postgresConfig, postgresHostConfig, "postgres-test-plugin")
	if err != nil {
		return "", "", fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}

	return postgresID, "5435", nil
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

	time.Sleep(15 * time.Second)

	bootstrapServer := "redpanda-test:9092"
	if err := waitForKafkaBroker(bootstrapServer, 180*time.Second); err != nil {
		return "", "", "", fmt.Errorf("kafka broker not ready: %w", err)
	}

	timeout := 60 * time.Second
	schemaRegistryURL := fmt.Sprintf("http://localhost:8081")
	if err := waitForSchemaRegistry(schemaRegistryURL, timeout); err != nil {
		return "", "", "", fmt.Errorf("schema registry not ready: %w", err)
	}

	return redpandaID, "9092", "8081", nil
}

// Kafka helper functions
func createKafkaClient(bootstrapServer string) (*kgo.Client, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(bootstrapServer),
		kgo.ClientID("marmot-kafka-test-client"),
	}

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

// startMongoDB starts a MongoDB container for testing
func startMongoDB(ctx context.Context, cm *utils.ContainerManager, networkName string) (string, string, error) {
	mongoConfig := &container.Config{
		Image: "mongo:7.0",
		ExposedPorts: nat.PortSet{
			"27017/tcp": struct{}{},
		},
	}

	mongoHostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
		PortBindings: nat.PortMap{
			"27017/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "27017"}},
		},
	}

	mongoID, err := cm.StartContainer(mongoConfig, mongoHostConfig, "mongodb-test")
	if err != nil {
		return "", "", fmt.Errorf("failed to start MongoDB container: %w", err)
	}

	return mongoID, "27017", nil
}

// waitForMongoDB waits for MongoDB to be ready
func waitForMongoDB(port string) error {
	endpoint := fmt.Sprintf("mongodb://localhost:%s", port)
	deadline := time.Now().Add(60 * time.Second)

	for time.Now().Before(deadline) {
		if err := checkMongoDBConnection(endpoint, 5*time.Second); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for MongoDB to become ready")
}

// checkMongoDBConnection checks if MongoDB is responding to connections
func checkMongoDBConnection(endpoint string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(endpoint))
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return err
	}

	return nil
}

func (env *TestEnvironment) EnsureMySQLStarted(ctx context.Context) error {
	if env.MySQLID == "" || env.MySQLPort == "" {
		mysqlID, mysqlPort, err := startMySQL(ctx, env.ContainerManager, env.Config.NetworkName)
		if err != nil {
			return fmt.Errorf("failed to start MySQL: %w", err)
		}

		if err := waitForMySQL(mysqlPort); err != nil {
			return fmt.Errorf("MySQL failed to become ready: %w", err)
		}

		env.MySQLID = mysqlID
		env.MySQLPort = mysqlPort
	}

	if err := checkMySQLConnection("localhost:"+env.MySQLPort, 10*time.Second); err != nil {
		env.ContainerManager.CleanupContainer(env.MySQLID)
		mysqlID, mysqlPort, err := startMySQL(ctx, env.ContainerManager, env.Config.NetworkName)
		if err != nil {
			return fmt.Errorf("failed to restart MySQL: %w", err)
		}

		if err := waitForMySQL(mysqlPort); err != nil {
			return fmt.Errorf("MySQL failed to become ready after restart: %w", err)
		}

		env.MySQLID = mysqlID
		env.MySQLPort = mysqlPort
	}

	return nil
}

func startMySQL(ctx context.Context, cm *utils.ContainerManager, networkName string) (string, string, error) {
	mysqlConfig := &container.Config{
		Image: "mysql:8.0",
		Env: []string{
			"MYSQL_ROOT_PASSWORD=mysql",
			"MYSQL_DATABASE=test",
		},
		ExposedPorts: nat.PortSet{
			"3306/tcp": struct{}{},
		},
	}

	mysqlHostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
		PortBindings: nat.PortMap{
			"3306/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "3307"}},
		},
	}

	mysqlID, err := cm.StartContainer(mysqlConfig, mysqlHostConfig, "mysql-test-plugin")
	if err != nil {
		return "", "", fmt.Errorf("failed to start MySQL container: %w", err)
	}

	return mysqlID, "3307", nil
}

func waitForMySQL(port string) error {
	endpoint := fmt.Sprintf("localhost:%s", port)
	deadline := time.Now().Add(60 * time.Second)

	for time.Now().Before(deadline) {
		if err := checkMySQLConnection(endpoint, 5*time.Second); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for MySQL to become ready")
}

func checkMySQLConnection(endpoint string, timeout time.Duration) error {
	parts := strings.Split(endpoint, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid endpoint format")
	}

	dsn := fmt.Sprintf("root:mysql@tcp(%s)/", endpoint)
	db, err := sql.Open("mysql", dsn)
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
