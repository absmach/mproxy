// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Parser implements WebSocket protocol handling.
// It upgrades HTTP connections to WebSocket and then delegates to an
// underlying protocol parser (typically MQTT over WebSocket).
type Parser struct {
	upgrader         websocket.Upgrader
	targetURL        string
	underlyingParser parser.Parser
	handler          handler.Handler
	logger           *slog.Logger
}

var _ http.Handler = (*Parser)(nil)

// NewParser creates a new WebSocket parser.
// underlyingParser is the protocol parser to use after WebSocket upgrade (e.g., MQTT parser).
func NewParser(targetURL string, underlyingParser parser.Parser, h handler.Handler, logger *slog.Logger) *Parser {
	if logger == nil {
		logger = slog.Default()
	}

	return &Parser{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Make this configurable
				return true
			},
		},
		targetURL:        targetURL,
		underlyingParser: underlyingParser,
		handler:          h,
		logger:           logger,
	}
}

// ServeHTTP implements http.Handler interface.
// It handles WebSocket upgrade and proxies the connection.
func (p *Parser) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Upgrade client connection to WebSocket
	clientConn, err := p.upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.logger.Error("failed to upgrade client connection",
			slog.String("remote", r.RemoteAddr),
			slog.String("error", err.Error()))
		return
	}
	defer clientConn.Close()

	p.logger.Debug("websocket connection upgraded",
		slog.String("remote", r.RemoteAddr))

	// Build backend WebSocket URL
	targetURL, err := p.buildTargetURL(r)
	if err != nil {
		p.logger.Error("failed to build target URL",
			slog.String("error", err.Error()))
		return
	}

	// Dial backend WebSocket
	serverConn, _, err := websocket.DefaultDialer.Dial(targetURL, nil)
	if err != nil {
		p.logger.Error("failed to dial backend WebSocket",
			slog.String("target", targetURL),
			slog.String("error", err.Error()))
		return
	}
	defer serverConn.Close()

	p.logger.Debug("connected to backend WebSocket",
		slog.String("target", targetURL))

	// Wrap connections as net.Conn
	clientNetConn := NewConn(clientConn)
	serverNetConn := NewConn(serverConn)

	// Create handler context
	sessionID := uuid.New().String()
	hctx := &handler.Context{
		SessionID:  sessionID,
		RemoteAddr: r.RemoteAddr,
		Protocol:   "websocket",
	}

	// Create context for this session
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start bidirectional streaming with underlying protocol parser
	errCh := make(chan error, 2)

	// Upstream: client → backend
	go func() {
		err := p.stream(ctx, clientNetConn, serverNetConn, parser.Upstream, hctx)
		errCh <- err
	}()

	// Downstream: backend → client
	go func() {
		err := p.stream(ctx, serverNetConn, clientNetConn, parser.Downstream, hctx)
		errCh <- err
	}()

	// Wait for either direction to complete
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && err != io.EOF {
			p.logger.Debug("stream error",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		}
	}

	// Notify disconnect
	if err := p.handler.OnDisconnect(context.Background(), hctx); err != nil {
		p.logger.Error("disconnect handler error",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}

	p.logger.Debug("websocket connection closed",
		slog.String("session", sessionID))
}

// stream continuously parses packets in one direction.
func (p *Parser) stream(ctx context.Context, r, w io.ReadWriter, dir parser.Direction, hctx *handler.Context) error {
	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Parse one packet using the underlying protocol parser
		if err := p.underlyingParser.Parse(ctx, r, w, dir, p.handler, hctx); err != nil {
			return err
		}
	}
}

// buildTargetURL constructs the backend WebSocket URL from the request.
func (p *Parser) buildTargetURL(r *http.Request) (string, error) {
	target, err := url.Parse(p.targetURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse target URL: %w", err)
	}

	// Preserve the request path
	target.Path = r.URL.Path
	target.RawQuery = r.URL.RawQuery

	return target.String(), nil
}

// Parse implements parser.Parser interface but is not used for WebSocket.
// WebSocket uses ServeHTTP instead since it requires HTTP upgrade.
func (p *Parser) Parse(ctx context.Context, r io.Reader, w io.Writer, dir parser.Direction, h handler.Handler, hctx *handler.Context) error {
	// Not used for WebSocket - WebSocket uses ServeHTTP instead
	return fmt.Errorf("Parse not supported for WebSocket parser, use ServeHTTP")
}
