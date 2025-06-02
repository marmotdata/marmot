package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/go-openapi/strfmt"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/assets"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/lineage"
	"github.com/marmotdata/marmot/tests/e2e/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncAPIIngestion(t *testing.T) {
	specsDir, err := filepath.Abs("/tmp/asyncapi-specs")
	require.NoError(t, err, "Failed to get absolute path")

	if err := os.RemoveAll(specsDir); err != nil && !os.IsNotExist(err) {
		require.NoError(t, err, "Failed to clean up existing specs directory")
	}
	require.NoError(t, os.MkdirAll(specsDir, 0777), "Failed to create specs directory")

	kafkaSpec := `
asyncapi: "2.5.0"
info:
  title: "Order Processing Service"
  version: "1.0.0"
  description: "Service for processing customer orders"
channels:
  order/created:
    publish:
      message:
        payload:
          type: object
          properties:
            orderId:
              type: string
              format: uuid
            customerId:
              type: string
            items:
              type: array
              items:
                type: object
    bindings:
      kafka:
        topic: "orders.created"
        partitions: 5
        replicas: 3
        topicConfiguration:
          cleanup.policy: ["delete"]
          retention.ms: 604800000
          retention.bytes: 1073741824
  order/processed:
    subscribe:
      message:
        payload:
          type: object
          properties:
            orderId:
              type: string
              format: uuid
            status:
              type: string
              enum: ["completed", "rejected"]
    bindings:
      kafka:
        topic: "orders.processed"
        partitions: 3
        replicas: 3
`

	awsSpec := `
asyncapi: "2.5.0"
info:
  title: "Notification Service"
  version: "1.2.0"
  description: "Service for handling notifications"
channels:
  notification/email:
    publish:
      message:
        payload:
          type: object
          properties:
            to:
              type: string
              format: email
            subject:
              type: string
            body:
              type: string
    bindings:
      sns:
        name: "email-notifications"
        tags:
          environment: "production"
          team: "notifications"
  notification/sms:
    publish:
      message:
        payload:
          type: object
          properties:
            phoneNumber:
              type: string
            message:
              type: string
    bindings:
      sqs:
        queue:
          name: "sms-notifications"
          fifoQueue: true
          contentBasedDeduplication: true
          tags:
            environment: "production"
            team: "notifications"
`

	amqpSpec := `
asyncapi: "2.5.0"
info:
  title: "User Service"
  version: "0.9.0"
  description: "Service for user management"
channels:
  user/created:
    publish:
      message:
        payload:
          type: object
          properties:
            userId:
              type: string
              format: uuid
            username:
              type: string
            email:
              type: string
              format: email
    bindings:
      amqp:
        is: "routingKey"
        exchange:
          name: "user-events"
          type: "topic"
          durable: true
          autoDelete: false
          vhost: "/"
  user/updated:
    publish:
      message:
        payload:
          type: object
          properties:
            userId:
              type: string
              format: uuid
            changedFields:
              type: array
              items:
                type: string
    bindings:
      amqp:
        is: "routingKey"
        exchange:
          name: "user-events"
          type: "topic"
          durable: true
          autoDelete: false
          vhost: "/"
`

	mixedSpec := `
asyncapi: "2.5.0"
info:
  title: "Integration Service"
  version: "1.0.0"
  description: "Service that integrates different messaging systems"
channels:
  event/process:
    publish:
      message:
        payload:
          type: object
          properties:
            eventId:
              type: string
              format: uuid
            timestamp:
              type: string
              format: date-time
            data:
              type: object
    bindings:
      kafka:
        topic: "events.process"
        partitions: 3
        replicas: 1
  event/processed:
    subscribe:
      message:
        payload:
          type: object
          properties:
            eventId:
              type: string
              format: uuid
            result:
              type: string
    bindings:
      sqs:
        queue:
          name: "processed-events"
          messageRetentionPeriod: 86400
  event/result:
    subscribe:
      message:
        payload:
          type: object
          properties:
            eventId:
              type: string
              format: uuid
            notificationType:
              type: string
    bindings:
      sns:
        name: "event-results"
  event/notify:
    subscribe:
      message:
        payload:
          type: object
          properties:
            eventId:
              type: string
              format: uuid
            recipient:
              type: string
    bindings:
      amqp:
        is: "routingKey"
        exchange:
          name: "notification-events"
          type: "topic"
          durable: true
`

	kafkaSpecPath := filepath.Join(specsDir, "kafka-orders.yaml")
	require.NoError(t, os.WriteFile(kafkaSpecPath, []byte(kafkaSpec), 0666))

	awsSpecPath := filepath.Join(specsDir, "aws-notifications.yaml")
	require.NoError(t, os.WriteFile(awsSpecPath, []byte(awsSpec), 0666))

	amqpSpecPath := filepath.Join(specsDir, "amqp-users.yaml")
	require.NoError(t, os.WriteFile(amqpSpecPath, []byte(amqpSpec), 0666))

	mixedSpecPath := filepath.Join(specsDir, "mixed-integration.yaml")
	require.NoError(t, os.WriteFile(mixedSpecPath, []byte(mixedSpec), 0666))

	files, err := os.ReadDir(specsDir)
	require.NoError(t, err, "Error reading spec directory")
	for _, file := range files {
		fileInfo, err := os.Stat(filepath.Join(specsDir, file.Name()))
		require.NoError(t, err)
		t.Logf("Spec file found: %s (size: %d, mode: %s)",
			file.Name(), fileInfo.Size(), fileInfo.Mode().String())
	}

	t.Logf("Contents of %s:", specsDir)
	for _, file := range files {
		t.Logf("  - %s", file.Name())
	}

	testFile := filepath.Join(specsDir, "test-file.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0666))

	configContent := fmt.Sprintf(`
runs:
  - asyncapi:
      spec_path: "/tmp/asyncapi-specs"
      resolve_external_docs: true
      tags:
        - "asyncapi"
        - "api"
        - "messaging"
`)

	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey,
			"-H", "http://marmot-test:8080"},
		configContent,
		fmt.Sprintf("%s:/tmp/asyncapi-specs", specsDir),
	)
	require.NoError(t, err)

	debugCmd := []string{"ls", "-la", "/tmp/asyncapi-specs"}
	containerConfig := &container.Config{
		Image: "marmot:test",
		Cmd:   debugCmd,
	}
	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(env.Config.NetworkName),
		Binds:       []string{fmt.Sprintf("%s:/tmp/asyncapi-specs", specsDir)},
	}
	debugContainerID, err := env.ContainerManager.StartContainer(containerConfig, hostConfig, "")
	require.NoError(t, err)
	defer env.ContainerManager.CleanupContainer(debugContainerID)

	debugOutput, err := env.ContainerManager.ExecCommand(debugContainerID, []string{"cat", "/tmp/asyncapi-specs/kafka-orders.yaml"})
	t.Logf("Debug container output: %s", debugOutput)

	t.Log("Ingest command executed, waiting for assets...")

	var resp *assets.GetAssetsListOK
	found := false

	for i := 0; i < 10; i++ {
		time.Sleep(3 * time.Second)

		params := assets.NewGetAssetsListParams()
		resp, err = env.APIClient.Assets.GetAssetsList(params)
		require.NoError(t, err)

		if len(resp.Payload.Assets) > 0 {
			found = true
			break
		}

		t.Logf("No assets found yet (attempt %d/10)", i+1)
	}

	require.True(t, found, "No assets found after multiple attempts")
	t.Logf("Total assets found: %d", len(resp.Payload.Assets))

	for i, asset := range resp.Payload.Assets {
		t.Logf("Asset %d: %s (Type: %s)", i+1, asset.Name, asset.Type)
	}

	kafkaService := utils.FindAssetByName(resp.Payload.Assets, "Order Processing Service")
	require.NotNil(t, kafkaService, "Kafka service not found")
	assert.Equal(t, "Service", kafkaService.Type)
	assert.Contains(t, kafkaService.Providers, "AsyncAPI")
	assert.Contains(t, kafkaService.Tags, "asyncapi")
	assert.Contains(t, kafkaService.Tags, "api")
	assert.Contains(t, kafkaService.Tags, "messaging")

	orderCreatedTopic := utils.FindAssetByName(resp.Payload.Assets, "orders.created")
	require.NotNil(t, orderCreatedTopic, "orders.created topic not found")
	assert.Equal(t, "Topic", orderCreatedTopic.Type)
	assert.Contains(t, orderCreatedTopic.Providers, "Kafka")

	orderProcessedTopic := utils.FindAssetByName(resp.Payload.Assets, "orders.processed")
	require.NotNil(t, orderProcessedTopic, "orders.processed topic not found")
	assert.Equal(t, "Topic", orderProcessedTopic.Type)
	assert.Contains(t, orderProcessedTopic.Providers, "Kafka")

	require.NotNil(t, orderCreatedTopic.Metadata, "orders.created topic metadata should not be nil")
	orderCreatedMetadata, ok := orderCreatedTopic.Metadata.(map[string]interface{})
	require.True(t, ok, "orders.created metadata should be a map[string]interface{}")

	partitions, ok := orderCreatedMetadata["partitions"]
	require.True(t, ok, "partitions field should exist in metadata")
	partitionsInt, err := partitions.(json.Number).Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(5), partitionsInt)

	replicas, ok := orderCreatedMetadata["replicas"]
	require.True(t, ok, "replicas field should exist in metadata")
	replicasInt, err := replicas.(json.Number).Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(3), replicasInt)

	notificationService := utils.FindAssetByName(resp.Payload.Assets, "Notification Service")
	require.NotNil(t, notificationService, "Notification service not found")
	assert.Equal(t, "Service", notificationService.Type)
	assert.Contains(t, notificationService.Providers, "AsyncAPI")

	emailTopic := utils.FindAssetByName(resp.Payload.Assets, "email-notifications")
	require.NotNil(t, emailTopic, "email-notifications topic not found")
	assert.Equal(t, "Topic", emailTopic.Type)
	assert.Contains(t, emailTopic.Providers, "SNS")

	smsQueue := utils.FindAssetByName(resp.Payload.Assets, "sms-notifications")
	require.NotNil(t, smsQueue, "sms-notifications queue not found")
	assert.Equal(t, "Queue", smsQueue.Type)
	assert.Contains(t, smsQueue.Providers, "SQS")

	require.NotNil(t, smsQueue.Metadata, "sms-notifications queue metadata should not be nil")
	smsQueueMetadata, ok := smsQueue.Metadata.(map[string]interface{})
	require.True(t, ok, "sms-notifications queue metadata should be a map[string]interface{}")

	fifoQueue, ok := smsQueueMetadata["fifo_queue"]
	require.True(t, ok, "fifo_queue field should exist in metadata")
	assert.Equal(t, true, fifoQueue)

	userService := utils.FindAssetByName(resp.Payload.Assets, "User Service")
	require.NotNil(t, userService, "User service not found")
	assert.Equal(t, "Service", userService.Type)
	assert.Contains(t, userService.Providers, "AsyncAPI")

	// TODO: fix AMQP. We're currently not creating Exchange assets, need to read up more on AMQP and determine
	// if it should just be metadata on the Queue or a unique asset with lineage

	// userCreatedQueue := utils.FindAssetByName(resp.Payload.Assets, "user-created")
	// require.NotNil(t, userCreatedQueue, "AMQP user-created exchange not found")
	// assert.Equal(t, "Queue", userCreatedQueue.Type)
	// assert.Contains(t, userCreatedQueue.Providers, "AMQP")

	// require.NotNil(t, userCreatedQueue.Metadata, "user-events exchange metadata should not be nil")
	// amqpMetadata, ok := userCreatedQueue.Metadata.(map[string]interface{})
	// require.True(t, ok, "user-events exchange metadata should be a map[string]interface{}")

	// exchangeType, ok := amqpMetadata["exchange_type"]
	// require.True(t, ok, "exchange_type field should exist in metadata")
	// assert.Equal(t, "topic", exchangeType)
	//
	// exchangeDurable, ok := amqpMetadata["exchange_durable"]
	// require.True(t, ok, "exchange_durable field should exist in metadata")
	// assert.Equal(t, true, exchangeDurable)
	//
	// exchangeAutoDelete, ok := amqpMetadata["exchange_auto_delete"]
	// require.True(t, ok, "exchange_auto_delete field should exist in metadata")
	// assert.Equal(t, false, exchangeAutoDelete)
	//
	// queueVhost, ok := amqpMetadata["queue_vhost"]
	// require.True(t, ok, "queue_vhost field should exist in metadata")
	// assert.Equal(t, "/", queueVhost)

	integrationService := utils.FindAssetByName(resp.Payload.Assets, "Integration Service")
	require.NotNil(t, integrationService, "Integration service not found")
	assert.Equal(t, "Service", integrationService.Type)
	assert.Contains(t, integrationService.Providers, "AsyncAPI")

	lineageParams := lineage.NewGetLineageAssetsIDParams().WithID(strfmt.UUID(integrationService.ID))
	lineageResp, err := env.APIClient.Lineage.GetLineageAssetsID(lineageParams)
	require.NoError(t, err)

	assert.Greater(t, len(lineageResp.Payload.Edges), 0, "No lineage edges found for Integration Service")

	producesFound := false
	consumesFound := false
	for _, edge := range lineageResp.Payload.Edges {
		if edge.Type == "PRODUCES" {
			producesFound = true
		}
		if edge.Type == "CONSUMES" {
			consumesFound = true
		}
	}
	assert.True(t, producesFound, "No PRODUCES lineage found")
	assert.True(t, consumesFound, "No CONSUMES lineage found")

	t.Log("Cleaning up created assets...")
	assetIDs := []string{
		kafkaService.ID,
		notificationService.ID,
		userService.ID,
		integrationService.ID,
	}

	for _, id := range assetIDs {
		_, err := env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(id))
		assert.NoError(t, err, "Failed to delete asset %s", id)
	}
}
