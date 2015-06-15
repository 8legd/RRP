package main

import (
	"flag"
	"log"
	"os"

	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web/middleware"

	"github.com/8legd/RRP/handlers"
	"github.com/8legd/RRP/handlers/batch"
)

func main() {

	bind := os.Getenv("RRP_BIND")
	if bind == "" {
		log.Fatal("Missing RRP_BIND environmental variable")
	}
	// TODO usage error if missing RRP_BIND env var

	// TODO use/write enhanced logging library?

	// To disable default logging
	// log.SetOutput(ioutil.Discard)
	// goji.DefaultMux.Abandon(middleware.Logger)

	// Remove any other unnecessary default middleware
	goji.DefaultMux.Abandon(middleware.RequestID)

	goji.Post("/batch/multipartmixed", batch.MultipartMixed)
	goji.Post("/batch/debug", handlers.Debug)
	// TODO support other batch requests e.g. AJAX support?

	flag.Set("bind", bind)
	goji.Serve()
}
