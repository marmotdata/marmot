package utils

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TestDatabase struct {
	Name        string
	Collections []TestCollection
}

type TestCollection struct {
	Name      string
	Documents []interface{}
	Indexes   []TestIndex
	IsCapped  bool
	MaxSize   int64
	MaxDocs   int64
}

type TestIndex struct {
	Name   string
	Keys   bson.D
	Unique bool
	Sparse bool
	TTL    *int32
}

type TestView struct {
	Name     string
	ViewOn   string
	Pipeline mongo.Pipeline
	Database string
}

func CreateTestMongoDatabases(ctx context.Context, endpoint string, databases []TestDatabase, views []TestView) error {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(endpoint))
	if err != nil {
		return fmt.Errorf("connecting to MongoDB: %w", err)
	}
	defer client.Disconnect(ctx)

	for _, testDB := range databases {
		db := client.Database(testDB.Name)

		for _, testColl := range testDB.Collections {
			var collection *mongo.Collection

			if testColl.IsCapped {
				opts := options.CreateCollection().SetCapped(true)
				if testColl.MaxSize > 0 {
					opts.SetSizeInBytes(testColl.MaxSize)
				}
				if testColl.MaxDocs > 0 {
					opts.SetMaxDocuments(testColl.MaxDocs)
				}

				err := db.CreateCollection(ctx, testColl.Name, opts)
				if err != nil {
					return fmt.Errorf("creating capped collection %s: %w", testColl.Name, err)
				}
			}

			collection = db.Collection(testColl.Name)

			if len(testColl.Documents) > 0 {
				_, err := collection.InsertMany(ctx, testColl.Documents)
				if err != nil {
					return fmt.Errorf("inserting documents into %s.%s: %w", testDB.Name, testColl.Name, err)
				}
			}

			for _, testIndex := range testColl.Indexes {
				indexModel := mongo.IndexModel{
					Keys:    testIndex.Keys,
					Options: options.Index(),
				}

				if testIndex.Name != "" {
					indexModel.Options.SetName(testIndex.Name)
				}
				if testIndex.Unique {
					indexModel.Options.SetUnique(true)
				}
				if testIndex.Sparse {
					indexModel.Options.SetSparse(true)
				}
				if testIndex.TTL != nil {
					indexModel.Options.SetExpireAfterSeconds(*testIndex.TTL)
				}

				_, err := collection.Indexes().CreateOne(ctx, indexModel)
				if err != nil {
					return fmt.Errorf("creating index %s on %s.%s: %w", testIndex.Name, testDB.Name, testColl.Name, err)
				}
			}
		}
	}

	for _, testView := range views {
		var targetDB *mongo.Database
		if testView.Database != "" {
			targetDB = client.Database(testView.Database)
		} else if len(databases) > 0 {
			targetDB = client.Database(databases[0].Name)
		} else {
			targetDB = client.Database("test_views")
		}

		err := targetDB.CreateView(ctx, testView.Name, testView.ViewOn, testView.Pipeline)
		if err != nil {
			return fmt.Errorf("creating view %s: %w", testView.Name, err)
		}
	}

	return nil
}

func CleanupTestMongoDatabases(ctx context.Context, endpoint string, databaseNames []string) error {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(endpoint))
	if err != nil {
		return fmt.Errorf("connecting to MongoDB: %w", err)
	}
	defer client.Disconnect(ctx)

	for _, dbName := range databaseNames {
		err := client.Database(dbName).Drop(ctx)
		if err != nil {
			return fmt.Errorf("dropping database %s: %w", dbName, err)
		}
	}

	return nil
}
