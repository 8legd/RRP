package batch

import (
	"log"
	"net/http"
	"net/http/httputil"
)

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
