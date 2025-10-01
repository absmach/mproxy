package main

import (
	"log"

	"github.com/absmach/mgate"
)

func main() {
	proxy := mgate.NewTCPProxy(":8080", "example.com:80")

	// Register handlers (they are checked in order)
	proxy.AddHandler(mgate.NewHTTPHandler(true))
	proxy.AddHandler(mgate.NewWebSocketHandler(true))
	proxy.AddHandler(mgate.NewMQTTHandler(true))
	proxy.AddHandler(mgate.NewNATSHandler(true))
	proxy.AddHandler(mgate.NewCoAPHandler(true))
	// DefaultHandler is automatically used as fallback

	if err := proxy.Start(); err != nil {
		log.Fatalf("Failed to start proxy: %v", err)
	}

	// Keep running
	select {}
}
