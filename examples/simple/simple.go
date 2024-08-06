// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package simple

import (
	"context"
	"errors"
	"log/slog"

	"github.com/absmach/mproxy"
	"github.com/absmach/mproxy/pkg/session"
)

var errSessionMissing = errors.New("session is missing")

var _ session.Handler = (*Handler)(nil)

// Handler implements mqtt.Handler interface
type Handler struct {
	logger *slog.Logger
}

// New creates new Event entity
func New(logger *slog.Logger) *Handler {
	return &Handler{
		logger: logger,
	}
}

// AuthConnect is called on device connection,
// prior forwarding to the MQTT broker
func (h *Handler) AuthConnect(ctx context.Context) error {
	return h.logAction(ctx, "AuthConnect", nil, nil)
}

// AuthPublish is called on device publish,
// prior forwarding to the MQTT broker
func (h *Handler) AuthPublish(ctx context.Context, topic *string, payload *[]byte) error {
	return h.logAction(ctx, "AuthPublish", &[]string{*topic}, payload)
}

// AuthSubscribe is called on device publish,
// prior forwarding to the MQTT broker
func (h *Handler) AuthSubscribe(ctx context.Context, topics *[]string) error {
	return h.logAction(ctx, "AuthSubscribe", topics, nil)
}

// Connect - after client successfully connected
func (h *Handler) Connect(ctx context.Context) error {
	return h.logAction(ctx, "Connect", nil, nil)
}

// Publish - after client successfully published
func (h *Handler) Publish(ctx context.Context, topic *string, payload *[]byte) error {
	return h.logAction(ctx, "Publish", &[]string{*topic}, payload)
}

// Subscribe - after client successfully subscribed
func (h *Handler) Subscribe(ctx context.Context, topics *[]string) error {
	return h.logAction(ctx, "Subscribe", topics, nil)
}

// Unsubscribe - after client unsubscribed
func (h *Handler) Unsubscribe(ctx context.Context, topics *[]string) error {
	return h.logAction(ctx, "Unsubscribe", topics, nil)
}

// Disconnect on connection lost
func (h *Handler) Disconnect(ctx context.Context) error {
	return h.logAction(ctx, "Disconnect", nil, nil)
}

func (h *Handler) logAction(ctx context.Context, action string, topics *[]string, payload *[]byte) error {
	s, ok := mproxy.FromContext(ctx)
	args := []interface{}{
		slog.Group("session", slog.String("id", s.ID), slog.String("username", s.Username)),
	}
	if s.Cert.Subject.CommonName != "" {
		args = append(args, slog.Group("cert", slog.String("cn", s.Cert.Subject.CommonName)))
	}
	if topics != nil {
		args = append(args, slog.Any("topics", *topics))
	}
	if payload != nil {
		args = append(args, slog.Any("payload", *payload))
	}
	if !ok {
		args = append(args, slog.Any("error", errSessionMissing))
		h.logger.Error(action+"() failed to complete", args...)
		return errSessionMissing
	}
	h.logger.Info(action+"() completed successfully", args...)

	return nil
}
