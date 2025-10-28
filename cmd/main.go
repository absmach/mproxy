package main

import (
	"log"

	"github.com/absmach/mgate/protocols/nats"
	"github.com/absmach/mgate/tcp"
)

func main() {
	proxy := tcp.New(":8080", ":4222", 32*1024, nats.New(true))
	// proxy1 := mqtt.New(":8080", ":4222", 32*1024, nats.New(true))
	// fmt.Println(proxy1)

	// proxy2 := mqtt.New(":8080", ":4222", 32*1024, http.New(true))
	// fmt.Println(proxy2)
	// Register handlers (they are checked in order)
	// proxy.AddHandler(http.New(true))
	// proxy.AddHandler(ws.New(true))
	// proxy.AddHandler(mqtt.New(true))
	// proxy.AddHandler(nats.New(true))
	// proxy.AddHandler(coap.New(true))
	// DefaultHandler is automatically used as fallback

	if err := proxy.Start(); err != nil {
		log.Fatalf("Failed to start proxy: %v", err)
	}

	// Keep running
	select {}
}
