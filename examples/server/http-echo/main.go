package main

import (
	"log"
	"net/http"
)

const defaultPort = "8888"

func echoHandler(writer http.ResponseWriter, request *http.Request) {
	log.Println("Echoing back request made to " + request.URL.Path + " to client (" + request.RemoteAddr + ")")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Content-Type", request.Header.Get("Content-Type"))
	request.Write(writer)
}

func main() {
	log.Println("starting echo server, listening on port " + defaultPort)
	mux := http.NewServeMux()
	mux.HandleFunc("/", echoHandler)
	if err := http.ListenAndServe(":"+defaultPort, mux); err != nil {
		panic(err)
	}
}
