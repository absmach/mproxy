// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser"
	"github.com/absmach/mproxy/pkg/parser/websocket"
)

// WebSocketConfig holds configuration for WebSocket proxy.
type WebSocketConfig struct {
	Host            string
	Port            string
	TargetURL       string
	UnderlyingParser parser.Parser // The protocol parser to use after WS upgrade (e.g., MQTT)
	TLSConfig       *tls.Config
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
}

// WebSocketProxy coordinates the WebSocket server and parser.
type WebSocketProxy struct {
	server *http.Server
	logger *slog.Logger
}

// NewWebSocket creates a new WebSocket proxy with HTTP server and WebSocket parser.
func NewWebSocket(cfg WebSocketConfig, h handler.Handler) (*WebSocketProxy, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// Create WebSocket parser
	parser := websocket.NewParser(cfg.TargetURL, cfg.UnderlyingParser, h, cfg.Logger)

	// Create HTTP server
	address := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:      address,
		Handler:   parser,
		TLSConfig: cfg.TLSConfig,
	}

	return &WebSocketProxy{
		server: server,
		logger: cfg.Logger,
	}, nil
}

// Listen starts the WebSocket proxy server and blocks until context is cancelled.
func (p *WebSocketProxy) Listen(ctx context.Context) error {
	p.logger.Info("WebSocket server started", slog.String("address", p.server.Addr))

	// Start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		if p.server.TLSConfig != nil {
			// WSS
			errCh <- p.server.ListenAndServeTLS("", "")
		} else {
			// WS
			errCh <- p.server.ListenAndServe()
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		p.logger.Info("shutdown signal received, closing WebSocket server")

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Graceful shutdown
		if err := p.server.Shutdown(shutdownCtx); err != nil {
			p.logger.Error("error during shutdown", slog.String("error", err.Error()))
			return err
		}

		p.logger.Info("WebSocket server shutdown complete")
		return nil

	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
