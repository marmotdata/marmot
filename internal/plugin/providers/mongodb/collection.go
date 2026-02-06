package mongodb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *Source) discoverCollections(ctx context.Context, dbName string) ([]asset.Asset, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	db := s.client.Database(dbName)

	filter := bson.D{}
	collOptions := options.ListCollections().SetNameOnly(false)
	cursor, err := db.ListCollections(timeoutCtx, filter, collOptions)
	if err != nil {
		return nil, fmt.Errorf("listing collections in %s: %w", dbName, err)
	}
	defer cursor.Close(ctx)

	var assets []asset.Asset

	for cursor.Next(timeoutCtx) {
		var collInfo struct {
			Name    string `bson:"name"`
			Type    string `bson:"type"`
			Options bson.M `bson:"options"`
			Info    bson.M `bson:"info"`
			IdIndex bson.D `bson:"idIndex"`
		}

		if err := cursor.Decode(&collInfo); err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to decode collection info")
			continue
		}

		collName := collInfo.Name

		if strings.HasPrefix(collName, "system.") {
			continue
		}

		isView := collInfo.Type == "view"
		if isView && !s.config.IncludeViews {
			continue
		}

		metadata := make(map[string]interface{})
		metadata["host"] = s.config.Host
		metadata["port"] = s.config.Port
		metadata["database"] = dbName
		metadata["collection"] = collName
		metadata["object_type"] = collInfo.Type

		statsCtx, statsCancel := context.WithTimeout(ctx, 10*time.Second)
		collStats := bson.M{}
		err := db.RunCommand(statsCtx, bson.D{{Key: "collStats", Value: collName}}).Decode(&collStats)
		statsCancel()

		if err == nil {
			if size, ok := collStats["size"].(int64); ok {
				metadata["size"] = size
			} else if size, ok := collStats["size"].(float64); ok {
				metadata["size"] = int64(size)
			}

			if count, ok := collStats["count"].(int64); ok {
				metadata["document_count"] = count
			} else if count, ok := collStats["count"].(float64); ok {
				metadata["document_count"] = int64(count)
			}

			if capped, ok := collStats["capped"].(bool); ok && capped {
				metadata["capped"] = true
				if maxSize, ok := collStats["maxSize"].(int64); ok {
					metadata["max_size"] = maxSize
				} else if maxSize, ok := collStats["maxSize"].(float64); ok {
					metadata["max_size"] = int64(maxSize)
				}
			} else {
				metadata["capped"] = false
			}

			if _, ok := collStats["wiredTiger"].(bson.M); ok {
				metadata["storage_engine"] = "wiredTiger"
			} else if _, ok := collStats["inMemory"].(bson.M); ok {
				metadata["storage_engine"] = "inMemory"
			}

			if sharded, ok := collStats["sharded"].(bool); ok && sharded {
				metadata["sharding_enabled"] = true
				if shardKey, ok := collInfo.Options["shardKey"].(bson.D); ok {
					metadata["shard_key"] = shardKeyToString(shardKey)
				}
			} else {
				metadata["sharding_enabled"] = false
			}
		}

		if validationInfo, ok := collInfo.Options["validator"].(bson.M); ok {
			if _, ok := validationInfo["$jsonSchema"].(bson.M); ok {
				if level, ok := collInfo.Options["validationLevel"].(string); ok {
					metadata["validation_level"] = level
				}
				if action, ok := collInfo.Options["validationAction"].(string); ok {
					metadata["validation_action"] = action
				}
			}
		}

		if isView {
			if viewOn, ok := collInfo.Options["viewOn"].(string); ok {
				metadata["view_on"] = viewOn
			}
			if pipeline, ok := collInfo.Options["pipeline"].(bson.A); ok {
				pipelineJSON, _ := bson.MarshalExtJSON(pipeline, false, false)
				metadata["pipeline"] = string(pipelineJSON)
			}
		}

		var assetType string
		var assetDesc string

		if isView {
			assetType = "View"
			assetDesc = fmt.Sprintf("MongoDB view %s.%s", dbName, collName)
		} else {
			assetType = "Collection"
			assetDesc = fmt.Sprintf("MongoDB collection %s.%s", dbName, collName)
		}

		mrnValue := mrn.New(assetType, "MongoDB", collName)
		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, asset.Asset{
			Name:        &collName,
			MRN:         &mrnValue,
			Type:        assetType,
			Providers:   []string{"MongoDB"},
			Description: &assetDesc,
			Metadata:    metadata,
			Tags:        processedTags,
			Sources: []asset.AssetSource{{
				Name:       "MongoDB",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterating collections in %s: %w", dbName, err)
	}

	return assets, nil
}

func shardKeyToString(shardKey bson.D) string {
	var parts []string
	for _, elem := range shardKey {
		parts = append(parts, fmt.Sprintf("%s:%d", elem.Key, elem.Value))
	}
	return strings.Join(parts, ",")
}
