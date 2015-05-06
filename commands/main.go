package main

import (
	"github.com/8legd/batchproxy"

	"github.com/zenazn/goji"
)

func main() {
	goji.Post("/multipart/mixed", batchproxy.MultipartMixed)
	//goji.Post("/multipart/mixed", batchproxy.Debug)
	// TODO support other batch requests e.g. AJAX support?
	goji.Serve()
}
