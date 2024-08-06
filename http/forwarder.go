// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/absmach/mproxy"
)

const contentType = "application/json"

// ErrMissingAuthentication returned when no basic or Authorization header is set.
var ErrMissingAuthentication = errors.New("missing authorization")

// proxy represents HTTP proxy.
type proxy struct {
	config  mproxy.Config
	target  *httputil.ReverseProxy
	session mproxy.Handler
	logger  *slog.Logger
}

func NewProxy(config mproxy.Config, handler mproxy.Handler, logger *slog.Logger) (mproxy.Forwarder, error) {
	target, err := url.Parse(config.Target)
	if err != nil {
		return proxy{}, err
	}

	return proxy{
		config:  config,
		target:  httputil.NewSingleHostReverseProxy(target),
		session: handler,
		logger:  logger,
	}, nil
}

func (p proxy) Forward(w http.ResponseWriter, r *http.Request) {
	// Metrics and health endpoints are served directly.
	if r.URL.Path == "/metrics" || r.URL.Path == "/health" {
		p.target.ServeHTTP(w, r)
		return
	}

	if !strings.HasPrefix(r.URL.Path, p.config.PathPrefix) {
		http.NotFound(w, r)
		return
	}

	username, password, ok := r.BasicAuth()
	switch {
	case ok:
		break
	case r.Header.Get("Authorization") != "":
		password = r.Header.Get("Authorization")
	default:
		encodeError(w, http.StatusBadGateway, ErrMissingAuthentication)
		return
	}

	s := &mproxy.Session{
		Password: []byte(password),
		Username: username,
	}
	ctx := mproxy.NewContext(r.Context(), s)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		encodeError(w, http.StatusBadRequest, err)
		p.logger.Error("Failed to read body", slog.Any("error", err))
		return
	}
	if err := r.Body.Close(); err != nil {
		encodeError(w, http.StatusInternalServerError, err)
		p.logger.Error("Failed to close body", slog.Any("error", err))
		return
	}

	// r.Body is reset to ensure it can be safely copied by httputil.ReverseProxy.
	// no close method is required since NopClose Close() always returns nill.
	r.Body = io.NopCloser(bytes.NewBuffer(payload))
	if err := p.session.AuthConnect(ctx); err != nil {
		encodeError(w, http.StatusUnauthorized, err)
		p.logger.Error("Failed to authorize connect", slog.Any("error", err))
		return
	}
	if err := p.session.Publish(ctx, &r.RequestURI, &payload); err != nil {
		encodeError(w, http.StatusBadRequest, err)
		p.logger.Error("Failed to publish", slog.Any("error", err))
		return
	}
	p.target.ServeHTTP(w, r)
}

func encodeError(w http.ResponseWriter, statusCode int, err error) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", contentType)
	if err := json.NewEncoder(w).Encode(err); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
