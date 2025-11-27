// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser"
)

// Parser implements HTTP reverse proxy with authorization.
// Note: HTTP is request/response based, not streaming, so it doesn't
// use the Parse method. Instead it implements http.Handler.
type Parser struct {
	target *httputil.ReverseProxy
	handler handler.Handler
	logger  *slog.Logger
}

var _ http.Handler = (*Parser)(nil)

// NewParser creates a new HTTP parser with the given target URL and handler.
func NewParser(targetURL string, h handler.Handler, logger *slog.Logger) (*Parser, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target URL: %w", err)
	}

	if logger == nil {
		logger = slog.Default()
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Customize director to preserve original request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Preserve original host if needed
		req.Host = target.Host
	}

	return &Parser{
		target:  proxy,
		handler: h,
		logger:  logger,
	}, nil
}

// ServeHTTP implements http.Handler interface.
// It extracts credentials, authorizes the request, and proxies to the backend.
func (p *Parser) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract credentials from multiple sources
	username, password := p.extractAuth(r)

	// Create handler context
	hctx := &handler.Context{
		SessionID:  r.Header.Get("X-Request-ID"), // Use request ID if available
		Username:   username,
		Password:   []byte(password),
		RemoteAddr: r.RemoteAddr,
		Protocol:   "http",
	}

	// Authorize connection
	if err := p.handler.AuthConnect(r.Context(), hctx); err != nil {
		p.logger.Debug("connection authorization failed",
			slog.String("remote", r.RemoteAddr),
			slog.String("error", err.Error()))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Read body for publish authorization
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		p.logger.Error("failed to read request body",
			slog.String("error", err.Error()))
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Restore body for reverse proxy
	r.Body = io.NopCloser(bytes.NewBuffer(payload))

	// Use request URI as "topic"
	topic := r.RequestURI

	// Authorize publish for write methods
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		if err := p.handler.AuthPublish(r.Context(), hctx, &topic, &payload); err != nil {
			p.logger.Debug("publish authorization failed",
				slog.String("remote", r.RemoteAddr),
				slog.String("method", r.Method),
				slog.String("uri", r.RequestURI),
				slog.String("error", err.Error()))
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Update request with potentially modified topic (URI) and payload
		if topic != r.RequestURI {
			newURL, err := url.Parse(topic)
			if err == nil {
				r.URL = newURL
				r.RequestURI = topic
			}
		}

		// Update body if modified
		r.Body = io.NopCloser(bytes.NewBuffer(payload))
		r.ContentLength = int64(len(payload))

		// Notify successful publish
		if err := p.handler.OnPublish(r.Context(), hctx, topic, payload); err != nil {
			p.logger.Error("publish notification error",
				slog.String("error", err.Error()))
		}
	}

	// Notify successful connection
	if err := p.handler.OnConnect(r.Context(), hctx); err != nil {
		p.logger.Error("connection notification error",
			slog.String("error", err.Error()))
	}

	// Proxy the request
	p.target.ServeHTTP(w, r)
}

// extractAuth extracts authentication credentials from the request.
// It tries multiple sources in order:
// 1. Basic Authentication header
// 2. "authorization" query parameter
// 3. "Authorization" header (Bearer token, etc.)
func (p *Parser) extractAuth(r *http.Request) (username, password string) {
	// Try Basic Auth first
	if user, pass, ok := r.BasicAuth(); ok {
		return user, pass
	}

	// Try query parameter
	if auth := r.URL.Query().Get("authorization"); auth != "" {
		return "", auth
	}

	// Try Authorization header (raw value)
	if auth := r.Header.Get("Authorization"); auth != "" {
		return "", auth
	}

	return "", ""
}

// Parse implements parser.Parser interface but is not used for HTTP.
// HTTP uses ServeHTTP instead since it's request/response based.
func (p *Parser) Parse(ctx context.Context, r io.Reader, w io.Writer, dir parser.Direction, h handler.Handler, hctx *handler.Context) error {
	// Not used for HTTP - HTTP uses ServeHTTP instead
	return fmt.Errorf("Parse not supported for HTTP parser, use ServeHTTP")
}
