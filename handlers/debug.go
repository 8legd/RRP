package handlers

import (
	"log"
	"net/http"
	"net/http/httputil"
)

// Debug simply logs an incoming HTTP request.
// TODO return it as the response in whatever format the incoming content type is?
func Debug(w http.ResponseWriter, r *http.Request) {
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		log.Fatal(err)
	} else {
		log.Print("<-- DumpRequest -->")
		log.Print(string(dump))
		log.Print("<-- /DumpRequest -->")
	}
}
