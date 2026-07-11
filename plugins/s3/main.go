package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/s3/s3"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   s3.Meta(),
		Source: &s3.Source{},
	})
}
