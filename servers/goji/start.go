package goji

import (
	"flag"
	"log"

	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web/middleware"

	"github.com/8legd/RRP/handlers"
	"github.com/8legd/RRP/handlers/batch"
)

func Start(bind string) {
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
	log.Println("Successfully configured RRP to use Goji web framework")
	goji.Serve()
}
