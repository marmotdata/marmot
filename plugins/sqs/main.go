package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/sqs/sqs"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   sqs.Meta(),
		Source: &sqs.Source{},
	})
}
