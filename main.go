package main

import (
	"os"

	server "github.com/5idu/aurora/internal"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	opts := server.DefaultOptions()
	srv := server.New(opts)

	if err := srv.Start(); err != nil {
		log.Fatal().Msg(err.Error())
	}
	srv.WaitForShutdown()
}
