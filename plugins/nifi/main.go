package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/nifi/nifi"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   nifi.Meta(),
		Source: &nifi.Source{},
	})
}
