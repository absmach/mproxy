// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package simple

import (
	"context"
	"log/slog"

	"github.com/absmach/mproxy/pkg/handler"
)

var _ handler.Handler = (*Handler)(nil)

// Handler is a simple example handler that logs all events.
type Handler struct {
	logger *slog.Logger
}

// New creates a new example handler.
func New(logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		logger: logger,
	}
}

// AuthConnect authorizes a client connection.
func (h *Handler) AuthConnect(ctx context.Context, hctx *handler.Context) error {
	h.logger.Info("AuthConnect",
		slog.String("session", hctx.SessionID),
		slog.String("username", hctx.Username),
		slog.String("client_id", hctx.ClientID),
		slog.String("remote", hctx.RemoteAddr),
		slog.String("protocol", hctx.Protocol))
	return nil
}

// AuthPublish authorizes a publish operation.
func (h *Handler) AuthPublish(ctx context.Context, hctx *handler.Context, topic *string, payload *[]byte) error {
	h.logger.Info("AuthPublish",
		slog.String("session", hctx.SessionID),
		slog.String("username", hctx.Username),
		slog.String("topic", *topic),
		slog.Int("payload_size", len(*payload)))
	return nil
}

// AuthSubscribe authorizes a subscribe operation.
func (h *Handler) AuthSubscribe(ctx context.Context, hctx *handler.Context, topics *[]string) error {
	h.logger.Info("AuthSubscribe",
		slog.String("session", hctx.SessionID),
		slog.String("username", hctx.Username),
		slog.Any("topics", *topics))
	return nil
}

// OnConnect is called after successful connection.
func (h *Handler) OnConnect(ctx context.Context, hctx *handler.Context) error {
	h.logger.Info("OnConnect",
		slog.String("session", hctx.SessionID),
		slog.String("username", hctx.Username),
		slog.String("client_id", hctx.ClientID))
	return nil
}

// OnPublish is called after successful publish.
func (h *Handler) OnPublish(ctx context.Context, hctx *handler.Context, topic string, payload []byte) error {
	h.logger.Info("OnPublish",
		slog.String("session", hctx.SessionID),
		slog.String("username", hctx.Username),
		slog.String("topic", topic),
		slog.Int("payload_size", len(payload)))
	return nil
}

// OnSubscribe is called after successful subscription.
func (h *Handler) OnSubscribe(ctx context.Context, hctx *handler.Context, topics []string) error {
	h.logger.Info("OnSubscribe",
		slog.String("session", hctx.SessionID),
		slog.String("username", hctx.Username),
		slog.Any("topics", topics))
	return nil
}

// OnUnsubscribe is called after unsubscription.
func (h *Handler) OnUnsubscribe(ctx context.Context, hctx *handler.Context, topics []string) error {
	h.logger.Info("OnUnsubscribe",
		slog.String("session", hctx.SessionID),
		slog.String("username", hctx.Username),
		slog.Any("topics", topics))
	return nil
}

// OnDisconnect is called when a client disconnects.
func (h *Handler) OnDisconnect(ctx context.Context, hctx *handler.Context) error {
	h.logger.Info("OnDisconnect",
		slog.String("session", hctx.SessionID),
		slog.String("username", hctx.Username))
	return nil
}
