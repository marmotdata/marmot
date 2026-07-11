package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/bigquery/bigquery"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   bigquery.Meta(),
		Source: &bigquery.Source{},
	})
}
