package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/kinesis/kinesis"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   kinesis.Meta(),
		Source: &kinesis.Source{},
	})
}
