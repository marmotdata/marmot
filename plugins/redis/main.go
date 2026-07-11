package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/redis/redis"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   redis.Meta(),
		Source: &redis.Source{},
	})
}
