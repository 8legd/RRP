package main

import (
	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web/middleware"

	"github.com/8legd/RRP/handlers/batch"
)

func main() {

	// TODO use/write enhanced logging library?

	// To disable default logging
	// log.SetOutput(ioutil.Discard)
	// goji.DefaultMux.Abandon(middleware.Logger)

	// Remove any other unnecessary default middleware
	goji.DefaultMux.Abandon(middleware.RequestID)

	goji.Post("/batch/multipartmixed", batch.MultipartMixed)
	goji.Post("/batch/debug", batch.Debug)

	// TODO support other batch requests e.g. AJAX support?

	goji.Serve()
}
