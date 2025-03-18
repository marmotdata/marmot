package main

import (
	"github.com/marmotdata/marmot/internal/cmd"
	"github.com/rs/zerolog/log"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		log.Fatal().Err(err)
	}
}
