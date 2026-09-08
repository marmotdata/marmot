package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/firebase/firebase"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   firebase.Meta(),
		Source: &firebase.Source{},
	})
}
