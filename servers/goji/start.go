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
			if r.Header != nil {

				// Add default User-Agent of `RRP <version>` if none is specified in the request
				// TODO remove hard coded version and set on build - need to setup our automated build first :)
				if ua := r.Header["User-Agent"]; len(ua) == 0 {
					r.Header.Set("User-Agent", "RRP 1.0.1")
				}

				// add `x-request-id` header if not present, as per heroku (https://devcenter.heroku.com/articles/http-request-id)
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
