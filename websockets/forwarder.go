// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package websockets

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/absmach/mproxy"
	"github.com/absmach/mproxy/pkg/session"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

var (
	upgrader               = websocket.Upgrader{}
	ErrAuthorizationNotSet = errors.New("authorization not set")
)

type Proxy struct {
	target  string
	handler mproxy.Handler
	logger  *slog.Logger
}

func (p *Proxy) Forward(w http.ResponseWriter, r *http.Request) {
	var token string
	headers := http.Header{}
	switch {
	case len(r.URL.Query()["authorization"]) != 0:
		token = r.URL.Query()["authorization"][0]
	case r.Header.Get("Authorization") != "":
		token = r.Header.Get("Authorization")
		headers.Add("Authorization", token)
	default:
		http.Error(w, ErrAuthorizationNotSet.Error(), http.StatusUnauthorized)
		return
	}

	target := fmt.Sprintf("%s%s", p.target, r.RequestURI)

	targetConn, _, err := websocket.DefaultDialer.Dial(target, headers)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer targetConn.Close()

	topic := r.URL.Path
	s := mproxy.Session{Password: []byte(token)}
	ctx := mproxy.NewContext(context.Background(), &s)
	if err := p.handler.AuthConnect(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if err := p.handler.AuthSubscribe(ctx, &[]string{topic}); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if err := p.handler.Subscribe(ctx, &[]string{topic}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	inConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.logger.Warn("WS Proxy failed to upgrade connection", slog.Any("error", err))
		return
	}
	defer inConn.Close()

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return p.stream(ctx, topic, inConn, targetConn, true)
	})
	g.Go(func() error {
		return p.stream(ctx, topic, targetConn, inConn, false)
	})

	if err := g.Wait(); err != nil {
		if err := p.handler.Unsubscribe(ctx, &[]string{topic}); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p.logger.Error("WS Proxy terminated", slog.Any("error", err))
		return
	}
}

func (p *Proxy) stream(ctx context.Context, topic string, src, dest *websocket.Conn, upstream bool) error {
	for {
		messageType, payload, err := src.ReadMessage()
		if err != nil {
			return err
		}
		if upstream {
			if err := p.handler.AuthPublish(ctx, &topic, &payload); err != nil {
				return err
			}
			if err := p.handler.Publish(ctx, &topic, &payload); err != nil {
				return err
			}
		}
		if err := dest.WriteMessage(messageType, payload); err != nil {
			return err
		}
	}
}

func NewProxy(target string, logger *slog.Logger, handler session.Handler) mproxy.Forwarder {
	return &Proxy{
		target:  target,
		logger:  logger,
		handler: handler,
	}
}
