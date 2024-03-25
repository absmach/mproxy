// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/absmach/mproxy"
	"github.com/absmach/mproxy/pkg/session"
	mptls "github.com/absmach/mproxy/pkg/tls"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

// Proxy represents WS Proxy.
type Proxy struct {
	config      mproxy.Config
	handler     session.Handler
	interceptor session.Interceptor
	logger      *slog.Logger
}

// New - creates new WS proxy.
func New(config mproxy.Config, handler session.Handler, interceptor session.Interceptor, logger *slog.Logger) *Proxy {
	config.PrefixPath = strings.TrimSpace(config.PrefixPath)
	switch {
	case config.PrefixPath != "" && config.PrefixPath[0] != '/':
		config.PrefixPath = "/" + config.PrefixPath
	case config.PrefixPath == "":
		config.PrefixPath = "/"
	}
	return &Proxy{
		config:      config,
		handler:     handler,
		interceptor: interceptor,
		logger:      logger,
	}
}

var upgrader = websocket.Upgrader{
	// Timeout for WS upgrade request handshake
	HandshakeTimeout: 10 * time.Second,
	// Paho JS client expecting header Sec-WebSocket-Protocol:mqtt in Upgrade response during handshake.
	Subprotocols: []string{"mqttv3.1", "mqtt"},
	// Allow CORS
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (p Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != p.config.PrefixPath {
		http.NotFound(w, r)
		return
	}
	cconn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.logger.Error("Error upgrading connection", slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go p.pass(r.Context(), cconn)
}

func (p Proxy) pass(ctx context.Context, in *websocket.Conn) {
	defer in.Close()

	dialer := &websocket.Dialer{
		Subprotocols: []string{"mqtt"},
	}
	srv, _, err := dialer.Dial(p.config.Target, nil)
	if err != nil {
		p.logger.Error("Unable to connect to broker", slog.Any("error", err))
		return
	}

	errc := make(chan error, 1)
	inboundConn := newConn(in)
	outboundConn := newConn(srv)

	defer inboundConn.Close()
	defer outboundConn.Close()

	clientCert, err := mptls.ClientCert(in.UnderlyingConn())
	if err != nil {
		p.logger.Error("Failed to get client certificate", slog.Any("error", err))
		return
	}

	err = session.Stream(ctx, inboundConn, outboundConn, p.handler, p.interceptor, clientCert)
	errc <- err
	p.logger.Warn("Broken connection for client", slog.Any("error", err))
}

func (p Proxy) Listen(ctx context.Context) error {
	l, err := net.Listen("tcp", p.config.Address)
	if err != nil {
		return err
	}

	if p.config.TLSConfig != nil {
		l = tls.NewListener(l, p.config.TLSConfig)
	}

	var server http.Server
	g, ctx := errgroup.WithContext(ctx)

	mux := http.NewServeMux()
	mux.Handle(p.config.PrefixPath, p)
	server.Handler = mux

	g.Go(func() error {
		return server.Serve(l)
	})
	p.logger.Info(fmt.Sprintf("MQTT websocket proxy server started at %s%s %s", p.config.Address, p.config.PrefixPath, p.config.Security))

	g.Go(func() error {
		<-ctx.Done()
		return server.Close()
	})
	if err := g.Wait(); err != nil {
		p.logger.Info(fmt.Sprintf("MQTT websocket proxy server at %s%s %s exiting with errors", p.config.Address, p.config.PrefixPath, p.config.Security), slog.String("error", err.Error()))
	} else {
		p.logger.Info(fmt.Sprintf("MQTT websocket proxy server at %s%s %s exiting...", p.config.Address, p.config.PrefixPath, p.config.Security))
	}
	return nil
}
