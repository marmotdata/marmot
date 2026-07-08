package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/elasticsearch/elasticsearch"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   elasticsearch.Meta(),
		Source: &elasticsearch.Source{},
	})
}
