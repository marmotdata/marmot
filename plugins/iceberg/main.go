package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/iceberg/iceberg"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   iceberg.Meta(),
		Source: &iceberg.Source{},
	})
}
