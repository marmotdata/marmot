package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/sftp/sftp"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   sftp.Meta(),
		Source: &sftp.Source{},
	})
}
