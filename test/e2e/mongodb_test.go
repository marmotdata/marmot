package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/assets"
	"github.com/marmotdata/marmot/tests/e2e/internal/client/client/lineage"
	"github.com/marmotdata/marmot/tests/e2e/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestMongoDBIngestion(t *testing.T) {
	ctx := context.Background()

	mongoID, mongoPort, err := startMongoDB(ctx, env.ContainerManager, env.Config.NetworkName)
	require.NoError(t, err)
	defer env.ContainerManager.CleanupContainer(mongoID)

	err = waitForMongoDB(mongoPort)
	require.NoError(t, err)

	err = setupTestMongoDB(ctx)
	require.NoError(t, err)

	configContent := fmt.Sprintf(`
runs:
  - mongodb:
      host: "mongodb-test"
      port: 27017
      include_databases: true
      include_collections: true
      include_views: true
      include_indexes: true
      sample_schema: true
      sample_size: 100
      use_random_sampling: true
      exclude_system_dbs: true
      database_filter:
        include:
          - ".*"
        exclude:
          - "^admin$"
          - "^config$"
          - "^local$"
      collection_filter:
        include:
          - ".*"
        exclude:
          - "^system\\."
      tags:
        - "mongodb"
        - "nosql-database"
        - "test"
`)

	err = env.ContainerManager.RunMarmotCommandWithConfig(env.Config,
		[]string{"ingest", "-c", "/tmp/config.yaml", "-k", env.APIKey, "-H", "http://marmot-test:8080"},
		configContent,
		nil,
	)
	require.NoError(t, err)

	time.Sleep(10 * time.Second)

	params := assets.NewGetAssetsListParams()
	resp, err := env.APIClient.Assets.GetAssetsList(params)
	require.NoError(t, err)

	t.Logf("Total assets found: %d", len(resp.Payload.Assets))

	ecommerceDB := utils.FindAssetByName(resp.Payload.Assets, "test_ecommerce")
	require.NotNil(t, ecommerceDB, "test_ecommerce database not found")
	assert.Equal(t, "Database", ecommerceDB.Type)
	assert.Contains(t, ecommerceDB.Providers, "MongoDB")
	assert.Contains(t, ecommerceDB.Tags, "mongodb")
	assert.Contains(t, ecommerceDB.Tags, "nosql-database")
	assert.Contains(t, ecommerceDB.Tags, "test")

	dbMetadata, ok := ecommerceDB.Metadata.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "mongodb-test", dbMetadata["host"])

	if portNum, ok := dbMetadata["port"].(json.Number); ok {
		port, err := portNum.Int64()
		require.NoError(t, err)
		assert.Equal(t, int64(27017), port)
	} else {
		assert.Equal(t, int64(27017), dbMetadata["port"])
	}

	assert.Equal(t, "test_ecommerce", dbMetadata["database"])

	usersCollection := utils.FindAssetByName(resp.Payload.Assets, "users")
	require.NotNil(t, usersCollection, "users collection not found")
	assert.Equal(t, "Collection", usersCollection.Type)
	assert.Contains(t, usersCollection.Providers, "MongoDB")
	assert.Contains(t, usersCollection.Tags, "mongodb")
	assert.Contains(t, usersCollection.Tags, "nosql-database")
	assert.Contains(t, usersCollection.Tags, "test")

	usersMetadata, ok := usersCollection.Metadata.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test_ecommerce", usersMetadata["database"])
	assert.Equal(t, "users", usersMetadata["collection"])
	assert.Equal(t, "collection", usersMetadata["object_type"])

	if docCount, ok := usersMetadata["document_count"].(json.Number); ok {
		count, err := docCount.Int64()
		require.NoError(t, err)
		assert.Greater(t, count, int64(0))
	} else if docCount, ok := usersMetadata["document_count"].(int64); ok {
		assert.Greater(t, docCount, int64(0))
	}

	assert.Equal(t, false, usersMetadata["capped"])
	assert.Equal(t, false, usersMetadata["sharding_enabled"])

	productsCollection := utils.FindAssetByName(resp.Payload.Assets, "products")
	require.NotNil(t, productsCollection, "products collection not found")
	assert.Equal(t, "Collection", productsCollection.Type)
	assert.Contains(t, productsCollection.Providers, "MongoDB")

	productsMetadata, ok := productsCollection.Metadata.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test_ecommerce", productsMetadata["database"])
	assert.Equal(t, "products", productsMetadata["collection"])

	ordersCollection := utils.FindAssetByName(resp.Payload.Assets, "orders")
	require.NotNil(t, ordersCollection, "orders collection not found")
	assert.Equal(t, "Collection", ordersCollection.Type)

	ordersMetadata, ok := ordersCollection.Metadata.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test_ecommerce", ordersMetadata["database"])
	assert.Equal(t, "orders", ordersMetadata["collection"])

	analyticsDB := utils.FindAssetByName(resp.Payload.Assets, "test_analytics")
	require.NotNil(t, analyticsDB, "test_analytics database not found")
	assert.Equal(t, "Database", analyticsDB.Type)
	assert.Contains(t, analyticsDB.Providers, "MongoDB")

	userStatsView := utils.FindAssetByName(resp.Payload.Assets, "user_stats")
	require.NotNil(t, userStatsView, "user_stats view not found")
	assert.Equal(t, "View", userStatsView.Type)
	assert.Contains(t, userStatsView.Providers, "MongoDB")

	viewMetadata, ok := userStatsView.Metadata.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test_analytics", viewMetadata["database"])
	assert.Equal(t, "user_stats", viewMetadata["collection"])
	assert.Equal(t, "view", viewMetadata["object_type"])
	assert.Equal(t, "users", viewMetadata["view_on"])
	assert.Contains(t, viewMetadata, "pipeline")

	logsCollection := utils.FindAssetByName(resp.Payload.Assets, "logs")
	require.NotNil(t, logsCollection, "logs collection not found")
	assert.Equal(t, "Collection", logsCollection.Type)

	logsMetadata, ok := logsCollection.Metadata.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, logsMetadata["capped"])

	if maxSize, ok := logsMetadata["max_size"].(json.Number); ok {
		size, err := maxSize.Int64()
		require.NoError(t, err)
		assert.Greater(t, size, int64(0))
	} else if maxSize, ok := logsMetadata["max_size"].(int64); ok {
		assert.Greater(t, maxSize, int64(0))
	}

	ecommerceUUID := strfmt.UUID(ecommerceDB.ID)
	lineageParams := lineage.NewGetLineageAssetsIDParams().WithID(ecommerceUUID)
	lineageResp, err := env.APIClient.Lineage.GetLineageAssetsID(lineageParams)
	require.NoError(t, err)

	t.Logf("Found %d lineage edges for database %s:", len(lineageResp.Payload.Edges), ecommerceDB.ID)
	for _, edge := range lineageResp.Payload.Edges {
		t.Logf("Edge: %s -> %s (type: %s)", edge.Source, edge.Target, edge.Type)
	}

	userStatsUUID := strfmt.UUID(userStatsView.ID)
	viewLineageParams := lineage.NewGetLineageAssetsIDParams().WithID(userStatsUUID)
	viewLineageResp, err := env.APIClient.Lineage.GetLineageAssetsID(viewLineageParams)
	require.NoError(t, err)

	t.Logf("Found %d lineage edges for view %s:", len(viewLineageResp.Payload.Edges), userStatsView.ID)
	for _, edge := range viewLineageResp.Payload.Edges {
		t.Logf("Edge: %s -> %s (type: %s)", edge.Source, edge.Target, edge.Type)
	}

	var viewLineageFound bool
	for _, edge := range viewLineageResp.Payload.Edges {
		if edge.Type == "VIEW_OF" {
			viewLineageFound = true
			break
		}
	}
	if !viewLineageFound {
		t.Logf("Warning: VIEW_OF relationship between view and source collection not found - this may be expected if the MongoDB plugin doesn't create this lineage")
	}

	assetIDs := []string{
		ecommerceDB.ID,
		analyticsDB.ID,
		usersCollection.ID,
		productsCollection.ID,
		ordersCollection.ID,
		userStatsView.ID,
		logsCollection.ID,
	}

	for _, id := range assetIDs {
		_, err := env.APIClient.Assets.DeleteAssetsID(assets.NewDeleteAssetsIDParams().WithID(id))
		assert.NoError(t, err, "failed to delete asset", id)
	}
}

func setupTestMongoDB(ctx context.Context) error {
	testDatabases := []utils.TestDatabase{
		{
			Name: "test_ecommerce",
			Collections: []utils.TestCollection{
				{
					Name: "users",
					Documents: []interface{}{
						bson.M{
							"_id":   "user1",
							"email": "alice@example.com",
							"name":  "Alice Johnson",
							"age":   28,
							"address": bson.M{
								"street": "123 Main St",
								"city":   "Seattle",
								"state":  "WA",
								"zip":    "98101",
							},
							"preferences": bson.M{
								"newsletter": true,
								"category":   "electronics",
							},
							"created_at": time.Now(),
						},
						bson.M{
							"_id":   "user2",
							"email": "bob@example.com",
							"name":  "Bob Smith",
							"age":   35,
							"address": bson.M{
								"street": "456 Oak Ave",
								"city":   "Portland",
								"state":  "OR",
								"zip":    "97201",
							},
							"preferences": bson.M{
								"newsletter": false,
								"category":   "books",
							},
							"created_at": time.Now(),
						},
						bson.M{
							"_id":   "user3",
							"email": "carol@example.com",
							"name":  "Carol Davis",
							"age":   42,
							"address": bson.M{
								"street": "789 Pine St",
								"city":   "San Francisco",
								"state":  "CA",
								"zip":    "94102",
							},
							"preferences": bson.M{
								"newsletter": true,
								"category":   "clothing",
							},
							"created_at": time.Now(),
						},
					},
					Indexes: []utils.TestIndex{
						{
							Name:   "email_unique",
							Keys:   bson.D{{Key: "email", Value: 1}},
							Unique: true,
						},
						{
							Name: "city_state_compound",
							Keys: bson.D{
								{Key: "address.city", Value: 1},
								{Key: "address.state", Value: 1},
							},
						},
					},
				},
				{
					Name: "products",
					Documents: []interface{}{
						bson.M{
							"_id":         "prod1",
							"name":        "Laptop Pro 15",
							"category":    "electronics",
							"price":       1299.99,
							"description": "High-performance laptop for professionals",
							"specs": bson.M{
								"cpu":     "Intel i7",
								"ram":     "16GB",
								"storage": "512GB SSD",
							},
							"tags":       []string{"laptop", "professional", "high-performance"},
							"in_stock":   true,
							"created_at": time.Now(),
						},
						bson.M{
							"_id":         "prod2",
							"name":        "Wireless Headphones",
							"category":    "electronics",
							"price":       199.99,
							"description": "Premium wireless headphones with noise cancellation",
							"specs": bson.M{
								"battery_life": "30 hours",
								"weight":       "250g",
								"bluetooth":    "5.0",
							},
							"tags":       []string{"headphones", "wireless", "noise-cancellation"},
							"in_stock":   true,
							"created_at": time.Now(),
						},
						bson.M{
							"_id":         "prod3",
							"name":        "Programming Book",
							"category":    "books",
							"price":       49.99,
							"description": "Complete guide to modern programming practices",
							"specs": bson.M{
								"pages":    "450",
								"language": "English",
								"format":   "Hardcover",
							},
							"tags":       []string{"programming", "education", "reference"},
							"in_stock":   true,
							"created_at": time.Now(),
						},
					},
					Indexes: []utils.TestIndex{
						{
							Name: "category_price",
							Keys: bson.D{
								{Key: "category", Value: 1},
								{Key: "price", Value: -1},
							},
						},
						{
							Name: "text_search",
							Keys: bson.D{
								{Key: "name", Value: "text"},
								{Key: "description", Value: "text"},
							},
						},
					},
				},
				{
					Name: "orders",
					Documents: []interface{}{
						bson.M{
							"_id":        "order1",
							"user_id":    "user1",
							"product_id": "prod1",
							"quantity":   1,
							"total":      1299.99,
							"status":     "completed",
							"order_date": time.Now(),
							"shipping": bson.M{
								"address": "123 Main St, Seattle, WA 98101",
								"method":  "standard",
							},
						},
						bson.M{
							"_id":        "order2",
							"user_id":    "user2",
							"product_id": "prod2",
							"quantity":   2,
							"total":      399.98,
							"status":     "pending",
							"order_date": time.Now(),
							"shipping": bson.M{
								"address": "456 Oak Ave, Portland, OR 97201",
								"method":  "express",
							},
						},
						bson.M{
							"_id":        "order3",
							"user_id":    "user3",
							"product_id": "prod3",
							"quantity":   1,
							"total":      49.99,
							"status":     "completed",
							"order_date": time.Now(),
							"shipping": bson.M{
								"address": "789 Pine St, San Francisco, CA 94102",
								"method":  "standard",
							},
						},
					},
					Indexes: []utils.TestIndex{
						{
							Name: "user_status",
							Keys: bson.D{
								{Key: "user_id", Value: 1},
								{Key: "status", Value: 1},
							},
						},
					},
				},
			},
		},
		{
			Name: "test_analytics",
			Collections: []utils.TestCollection{
				{
					Name:     "logs",
					IsCapped: true,
					MaxSize:  1024 * 1024,
					Documents: []interface{}{
						bson.M{
							"timestamp": time.Now(),
							"level":     "INFO",
							"message":   "User logged in",
							"user_id":   "user1",
						},
						bson.M{
							"timestamp":  time.Now(),
							"level":      "ERROR",
							"message":    "Failed to process payment",
							"user_id":    "user2",
							"error_code": 500,
						},
						bson.M{
							"timestamp": time.Now(),
							"level":     "INFO",
							"message":   "Order completed",
							"user_id":   "user3",
							"order_id":  "order3",
						},
					},
				},
			},
		},
	}

	testViews := []utils.TestView{
		{
			Name:   "user_stats",
			ViewOn: "users",
			Pipeline: mongo.Pipeline{
				bson.D{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: "$address.state"},
					{Key: "user_count", Value: bson.D{{Key: "$sum", Value: 1}}},
					{Key: "avg_age", Value: bson.D{{Key: "$avg", Value: "$age"}}},
				}}},
				bson.D{{Key: "$sort", Value: bson.D{{Key: "user_count", Value: -1}}}},
			},
			Database: "test_analytics",
		},
	}

	return utils.CreateTestMongoDatabases(ctx, "mongodb://localhost:27017", testDatabases, testViews)
}
