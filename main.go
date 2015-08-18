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
	// TODO usage error if missing RRP_BIND env var

	goji.Start(bind)
}
