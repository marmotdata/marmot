package mongodb

import (
	"context"
	"fmt"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"go.mongodb.org/mongo-driver/bson"
)

func (s *Source) discoverDatabases(ctx context.Context) ([]pluginsdk.Asset, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	dbs, err := s.client.ListDatabaseNames(timeoutCtx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("listing database names: %w", err)
	}

	var assets []pluginsdk.Asset

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
			if size, ok := asInt64(dbStats["dataSize"]); ok {
				metadata["size"] = size
			}

			if collections, ok := asInt64(dbStats["collections"]); ok {
				metadata["collection_count"] = collections
			}

			if views, ok := asInt64(dbStats["views"]); ok {
				metadata["view_count"] = views
			}

			if indexes, ok := asInt64(dbStats["indexes"]); ok {
				metadata["index_count"] = indexes
			}
		}

		mrnValue := mrn.New("Database", "MongoDB", dbName)

		processedTags := pluginsdk.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, pluginsdk.Asset{
			Name:      &dbName,
			MRN:       &mrnValue,
			Type:      "Database",
			Providers: []string{"MongoDB"},
			Metadata:  metadata,
			Tags:      processedTags,
			Sources: []pluginsdk.AssetSource{{
				Name:       "MongoDB",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})
	}

	return assets, nil
}
