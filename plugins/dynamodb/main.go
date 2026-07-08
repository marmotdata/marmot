package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/dynamodb/dynamodb"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   dynamodb.Meta(),
		Source: &dynamodb.Source{},
	})
}
