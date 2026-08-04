package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/googledrive/googledrive"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   googledrive.Meta(),
		Source: &googledrive.Source{},
	})
}
