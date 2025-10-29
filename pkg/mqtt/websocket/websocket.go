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

	"github.com/absmach/mgate"
	"github.com/absmach/mgate/pkg/session"
	mptls "github.com/absmach/mgate/pkg/tls"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

// Proxy represents WS Proxy.
type Proxy struct {
	config        mgate.Config
	handler       session.Handler
	beforeHandler session.Interceptor
	afterHandler  session.Interceptor
	logger        *slog.Logger
}

// New - creates new WS proxy.
func New(config mgate.Config, handler session.Handler, beforeHandler, afterHandler session.Interceptor, logger *slog.Logger) *Proxy {
	return &Proxy{
		config:        config,
		handler:       handler,
		beforeHandler: beforeHandler,
		afterHandler:  afterHandler,
		logger:        logger,
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
	if !strings.HasPrefix(r.URL.Path, p.config.PathPrefix) {
		http.NotFound(w, r)
		return
	}
	cconn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.logger.Error("Error upgrading connection", slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//nolint:contextcheck // new context is created in pass method
	go p.pass(cconn)
}

func (p Proxy) pass(in *websocket.Conn) {
	defer in.Close()
	// Using a new context so as to avoiding infinitely long traces.
	// And also avoiding proxy cancellation due to parent context cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dialer := &websocket.Dialer{
		Subprotocols: []string{"mqtt"},
	}

	target := fmt.Sprintf("%s://%s:%s%s", p.config.TargetProtocol, p.config.TargetHost, p.config.TargetPort, p.config.TargetPath)

	srv, _, err := dialer.Dial(target, nil)
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

	err = session.Stream(ctx, inboundConn, outboundConn, p.handler, p.beforeHandler, p.afterHandler, clientCert)
	errc <- err
	p.logger.Warn("Broken connection for client", slog.Any("error", err))
}

func (p Proxy) Listen(ctx context.Context) error {
	listenAddress := net.JoinHostPort(p.config.Host, p.config.Port)
	l, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return err
	}

	if p.config.TLSConfig != nil {
		l = tls.NewListener(l, p.config.TLSConfig)
	}

	var server http.Server
	g, ctx := errgroup.WithContext(ctx)

	mux := http.NewServeMux()

	mux.Handle(p.config.PathPrefix, p)
	server.Handler = mux

	g.Go(func() error {
		return server.Serve(l)
	})
	status := mptls.SecurityStatus(p.config.TLSConfig)

	p.logger.Info(fmt.Sprintf("MQTT websocket proxy server started at %s%s with %s", listenAddress, p.config.PathPrefix, status))

	g.Go(func() error {
		<-ctx.Done()
		return server.Close()
	})
	if err := g.Wait(); err != nil {
		p.logger.Info(fmt.Sprintf("MQTT websocket proxy server at %s%s with %s exiting with errors", listenAddress, p.config.PathPrefix, status), slog.String("error", err.Error()))
	} else {
		p.logger.Info(fmt.Sprintf("MQTT websocket proxy server at %s%s with %s exiting...", listenAddress, p.config.PathPrefix, status))
	}
	return nil
}
