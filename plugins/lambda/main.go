package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/lambda/lambda"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   lambda.Meta(),
		Source: &lambda.Source{},
	})
}
