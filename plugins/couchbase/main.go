package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/couchbase/couchbase"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   couchbase.Meta(),
		Source: &couchbase.Source{},
	})
}
