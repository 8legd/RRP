package main

import (
	"log"
	"os"

	"github.com/8legd/RRP/servers/goji"
)

func main() {
	bind := os.Getenv("RRP_BIND")
	if bind == "" {
		log.Fatal("Missing RRP_BIND environmental variable")
	}
	// TODO check bind format is valid - better usage error

	goji.Start(bind)
}
