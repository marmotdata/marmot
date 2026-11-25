package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/github/github"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   github.Meta(),
		Source: &github.Source{},
	})
}
