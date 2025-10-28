// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"strings"

	coap "github.com/plgd-dev/go-coap/v3"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/mux"
)

const defaultPort = "5683"

func handleRequest(w mux.ResponseWriter, r *mux.Message) {
	resp := w.Conn().AcquireMessage(r.Context())
	defer w.Conn().ReleaseMessage(resp)
	resp.SetCode(codes.Content)
	resp.SetToken(r.Token())
	resp.SetContentFormat(message.TextPlain)
	resp.SetBody(strings.NewReader(fmt.Sprintf("%v OK", r.Code())))
	err := w.Conn().WriteMessage(resp)
	if err != nil {
		log.Printf("Cannot send response: %v", err)
	}
}

func main() {
	r := mux.NewRouter()
	r.DefaultHandle(mux.HandlerFunc(handleRequest))
	log.Println("starting coap server, listening on port " + defaultPort)
	log.Fatal(coap.ListenAndServe("udp", ":"+defaultPort, r))
}
