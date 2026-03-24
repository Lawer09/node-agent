package main

import (
	"log"

	"singbox-node-agent/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
