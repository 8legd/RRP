package goji

import (
	"flag"
	"io/ioutil"
	"log"
	"time"

	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web/middleware"

	"github.com/8legd/RRP/handlers/batch"
	"github.com/8legd/RRP/logging/elf"
)

func Start(bind string) {
	started := time.Now()

	// Disable default logging (we use custom ELF based logging instead - see example below)
	log.SetOutput(ioutil.Discard)
	goji.DefaultMux.Abandon(middleware.Logger)

	// Remove any other unnecessary default middleware
	goji.DefaultMux.Abandon(middleware.RequestID)

	goji.Post("/batch/multipartmixed", batch.MultipartMixed)
	// TODO support other batch requests e.g. AJAX support?

	flag.Set("bind", bind)

	// ELF based logging
	elf.Log("INFO", "Successfully configured and started RRP using Goji web framework", elf.LogOptions{Started: started})
	goji.Serve()
}
