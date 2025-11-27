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
	httpparser "github.com/absmach/mproxy/pkg/parser/http"
)

// HTTPConfig holds configuration for HTTP proxy.
type HTTPConfig struct {
	Host            string
	Port            string
	TargetURL       string
	TLSConfig       *tls.Config
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
}

// HTTPProxy coordinates the HTTP server and parser.
type HTTPProxy struct {
	server *http.Server
	logger *slog.Logger
}

// NewHTTP creates a new HTTP proxy with HTTP server and parser.
func NewHTTP(cfg HTTPConfig, h handler.Handler) (*HTTPProxy, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// Create HTTP parser
	parser, err := httpparser.NewParser(cfg.TargetURL, h, cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP parser: %w", err)
	}

	// Create HTTP server
	address := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:      address,
		Handler:   parser,
		TLSConfig: cfg.TLSConfig,
	}

	return &HTTPProxy{
		server: server,
		logger: cfg.Logger,
	}, nil
}

// Listen starts the HTTP proxy server and blocks until context is cancelled.
func (p *HTTPProxy) Listen(ctx context.Context) error {
	p.logger.Info("HTTP server started", slog.String("address", p.server.Addr))

	// Start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		if p.server.TLSConfig != nil {
			// HTTPS
			errCh <- p.server.ListenAndServeTLS("", "")
		} else {
			// HTTP
			errCh <- p.server.ListenAndServe()
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		p.logger.Info("shutdown signal received, closing HTTP server")

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Graceful shutdown
		if err := p.server.Shutdown(shutdownCtx); err != nil {
			p.logger.Error("error during shutdown", slog.String("error", err.Error()))
			return err
		}

		p.logger.Info("HTTP server shutdown complete")
		return nil

	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
