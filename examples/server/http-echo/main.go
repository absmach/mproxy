// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

const defaultPort = "8888"

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow any origin (similar to your CORS setting)
		return true
	},
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Got request at  " + r.URL.Path)
	http.NotFound(w, r)
}

func echoHandler(writer http.ResponseWriter, request *http.Request) {
	log.Println("Echoing back request made to " + request.URL.Path + " to client (" + request.RemoteAddr + ")")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Content-Type", request.Header.Get("Content-Type"))
	_ = request.Write(writer)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Upgrading to WebSocket connection from " + r.RemoteAddr)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	conn.SetCloseHandler(func(code int, text string) error {
		log.Printf("WebSocket closed by remote client code %d, text %s\n", code, text)
		return nil
	})

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}
		log.Printf("Received message from %s: %s", r.RemoteAddr, string(message))

		err = conn.WriteMessage(messageType, message)
		if err != nil {
			log.Println("Write error:", err)
			break
		}
	}
}

func main() {
	log.Println("Starting echo server, listening on port " + defaultPort)
	mux := http.NewServeMux()
	mux.HandleFunc("/messages/http", echoHandler)
	mux.HandleFunc("/messages/ws", wsHandler)
	mux.HandleFunc("/", notFoundHandler)
	if err := http.ListenAndServe(":"+defaultPort, mux); err != nil {
		panic(err)
	}
}
