package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/sns/sns"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   sns.Meta(),
		Source: &sns.Source{},
	})
}
