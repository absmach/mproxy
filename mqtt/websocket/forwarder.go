// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/absmach/mproxy"
	"github.com/absmach/mproxy/mqtt"
	"github.com/gorilla/websocket"
)

type proxy struct {
	handler     mproxy.Handler
	interceptor mproxy.Interceptor
	logger      *slog.Logger
	target      string
}

// New - creates new WS proxy passer.
func New(target string, handler mproxy.Handler, interceptor mproxy.Interceptor, logger *slog.Logger) mproxy.Forwarder {
	return &proxy{
		target:      target,
		handler:     handler,
		interceptor: interceptor,
		logger:      logger,
	}
}

var upgrader = websocket.Upgrader{
	// Timeout for WS upgrade request handshake.
	HandshakeTimeout: 10 * time.Second,
	// Paho JS client expecting header Sec-WebSocket-Protocol:mqtt in Upgrade response during handshake.
	Subprotocols: []string{"mqttv3.1", "mqtt"},
	// Allow CORS
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (p proxy) Forward(w http.ResponseWriter, r *http.Request) {
	cconn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.logger.Error("Error upgrading connection", slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go p.pass(cconn)
}

func (p proxy) pass(in *websocket.Conn) {
	defer in.Close()
	// Using a new context so as to avoiding infinitely long traces.
	// And also avoiding proxy cancellation due to parent context cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dialer := &websocket.Dialer{
		Subprotocols: []string{"mqtt"},
	}
	out, _, err := dialer.Dial(p.target, nil)
	if err != nil {
		p.logger.Error("Unable to connect to broker", slog.Any("error", err))
		return
	}

	errc := make(chan error, 1)
	inboundConn := newConn(in)
	outboundConn := newConn(out)

	defer inboundConn.Close()
	defer outboundConn.Close()

	streamer := mqtt.New(p.handler, p.interceptor)
	err = streamer.Stream(ctx, inboundConn, outboundConn)
	errc <- err
	p.logger.Warn("Broken connection for client", slog.Any("error", err))
}
