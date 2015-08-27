package goji

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web"
	"github.com/zenazn/goji/web/middleware"

	"github.com/8legd/RRP/handlers/batch"
	"github.com/8legd/RRP/logging/elf"
)

func Start(bind string) {
	started := time.Now()

	// Disable default logging (we use custom ELF based logging instead)
	log.SetOutput(ioutil.Discard)
	goji.DefaultMux.Abandon(middleware.Logger)

	// Custom middleware
	custom := func(c *web.C, h http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// add `x-request-id` header if not present, as per heroku (https://devcenter.heroku.com/articles/http-request-id)
			if r.Header != nil {
				if ri := r.Header["x-request-id"]; len(ri) == 0 {
					if reqID, ok := c.Env["reqID"].(string); ok { // goji provides a request id by default as part of its web context object
						r.Header.Set("x-request-id", reqID)
					}
				}
			}
			h.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
	goji.Use(custom)

	goji.Post("/batch/multipartmixed", batch.MultipartMixed)
	// TODO support other batch requests e.g. AJAX support?

	flag.Set("bind", bind)

	// ELF based logging
	elf.Log("INFO", "Successfully configured and started RRP using Goji web framework", elf.LogOptions{Started: started})
	goji.Serve()
}
