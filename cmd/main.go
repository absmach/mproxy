package main

import (
	"log"

	"github.com/absmach/mgate"
	"github.com/absmach/mgate/protocols/coap"
	"github.com/absmach/mgate/protocols/http"
	"github.com/absmach/mgate/protocols/mqtt"
	"github.com/absmach/mgate/protocols/nats"
	"github.com/absmach/mgate/protocols/ws"
)

func main() {
	proxy := mgate.NewTCPProxy(":8080", "example.com:80")

	// Register handlers (they are checked in order)
	proxy.AddHandler(http.New(true))
	proxy.AddHandler(ws.New(true))
	proxy.AddHandler(mqtt.New(true))
	proxy.AddHandler(nats.New(true))
	proxy.AddHandler(coap.New(true))
	// DefaultHandler is automatically used as fallback

	if err := proxy.Start(); err != nil {
		log.Fatalf("Failed to start proxy: %v", err)
	}

	// Keep running
	select {}
}
