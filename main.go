package main

import (
	"github.com/8legd/batchproxy"

	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web/middleware"
)

func main() {

	// TODO use/write enhanced logging library?

	// To disable default logging
	// log.SetOutput(ioutil.Discard)
	// goji.DefaultMux.Abandon(middleware.Logger)

	// Remove any other unnecessary default middleware
	goji.DefaultMux.Abandon(middleware.RequestID)

	goji.Post("/multipart/mixed", batchproxy.MultipartMixed)
	goji.Post("/debug", batchproxy.DumpRequest)

	// TODO support other batch requests e.g. AJAX support?

	goji.Serve()
}
