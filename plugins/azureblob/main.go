package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/azureblob/azureblob"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   azureblob.Meta(),
		Source: &azureblob.Source{},
	})
}
