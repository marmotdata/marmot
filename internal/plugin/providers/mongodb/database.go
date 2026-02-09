package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"go.mongodb.org/mongo-driver/bson"
)

func (s *Source) discoverDatabases(ctx context.Context) ([]asset.Asset, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	dbs, err := s.client.ListDatabaseNames(timeoutCtx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("listing database names: %w", err)
	}

	var assets []asset.Asset

	for _, dbName := range dbs {
		if s.config.ExcludeSystemDbs && (dbName == "admin" || dbName == "config" || dbName == "local") {
			continue
		}

		statsCtx, statsCancel := context.WithTimeout(ctx, 15*time.Second)
		dbStats := bson.M{}
		err := s.client.Database(dbName).RunCommand(statsCtx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&dbStats)
		statsCancel()

		metadata := make(map[string]interface{})
		metadata["host"] = s.config.Host
		metadata["port"] = s.config.Port
		metadata["database"] = dbName
		metadata["created"] = time.Now().Format("2006-01-02 15:04:05")

		if err == nil {
			if size, ok := dbStats["dataSize"].(int64); ok {
				metadata["size"] = size
			} else if size, ok := dbStats["dataSize"].(float64); ok {
				metadata["size"] = int64(size)
			}

			if collections, ok := dbStats["collections"].(int32); ok {
				metadata["collection_count"] = collections
			} else if collections, ok := dbStats["collections"].(float64); ok {
				metadata["collection_count"] = int32(collections)
			}

			if views, ok := dbStats["views"].(int32); ok {
				metadata["view_count"] = views
			} else if views, ok := dbStats["views"].(float64); ok {
				metadata["view_count"] = int32(views)
			}

			if indexes, ok := dbStats["indexes"].(int32); ok {
				metadata["index_count"] = indexes
			} else if indexes, ok := dbStats["indexes"].(float64); ok {
				metadata["index_count"] = int32(indexes)
			}
		}

		mrnValue := mrn.New("Database", "MongoDB", dbName)

		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, asset.Asset{
			Name:      &dbName,
			MRN:       &mrnValue,
			Type:      "Database",
			Providers: []string{"MongoDB"},
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

	return assets, nil
}
