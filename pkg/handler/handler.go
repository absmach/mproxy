// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"crypto/x509"
)

// Context contains connection metadata and credentials extracted from packets.
// It is passed to Handler methods to provide auth context.
type Context struct {
	// SessionID is a unique identifier for this connection/session
	SessionID string

	// Username extracted from auth headers (MQTT username, HTTP basic auth, etc.)
	Username string

	// Password extracted from auth headers (raw bytes, not hashed)
	Password []byte

	// ClientID extracted from protocol-specific connect packets (e.g., MQTT client ID)
	ClientID string

	// RemoteAddr is the client's network address
	RemoteAddr string

	// Protocol indicates the protocol being used (mqtt, coap, http, ws)
	Protocol string

	// Cert is the client's TLS certificate (if using mTLS)
	Cert *x509.Certificate
}

// Handler defines authorization and notification callbacks for protocol events.
// Protocol parsers call these methods at appropriate points in the packet lifecycle.
//
// Authorization methods (AuthConnect, AuthPublish, AuthSubscribe) are called BEFORE
// forwarding packets to the backend. They can:
// - Return an error to reject the action
// - Modify mutable parameters (topic, payload, topics) via pointers
// - Update the handler context
//
// Notification methods (OnConnect, OnPublish, etc.) are called AFTER successful actions
// for audit logging, metrics, or post-processing. Errors from these methods are logged
// but don't prevent the action.
type Handler interface {
	// AuthConnect authorizes a client connection attempt.
	// Called when a client sends a CONNECT packet (MQTT), initial request (HTTP),
	// or first datagram (CoAP).
	// Return an error to reject the connection.
	AuthConnect(ctx context.Context, hctx *Context) error

	// AuthPublish authorizes a publish/write operation.
	// For MQTT: PUBLISH packet
	// For HTTP: POST/PUT request
	// For CoAP: POST request
	// The topic and payload can be modified via their pointers before forwarding.
	// Return an error to reject the publish.
	AuthPublish(ctx context.Context, hctx *Context, topic *string, payload *[]byte) error

	// AuthSubscribe authorizes a subscription operation.
	// For MQTT: SUBSCRIBE packet
	// For CoAP: GET with Observe option
	// The topics list can be modified via the pointer to filter subscriptions.
	// Return an error to reject the subscription.
	AuthSubscribe(ctx context.Context, hctx *Context, topics *[]string) error

	// OnConnect is called after a successful connection is established.
	// This is a notification hook for audit logging or metrics.
	OnConnect(ctx context.Context, hctx *Context) error

	// OnPublish is called after a successful publish operation.
	// This is a notification hook for audit logging or metrics.
	// Note: topic and payload are immutable copies (not pointers).
	OnPublish(ctx context.Context, hctx *Context, topic string, payload []byte) error

	// OnSubscribe is called after a successful subscription.
	// This is a notification hook for audit logging or metrics.
	// Note: topics is an immutable copy (not a pointer).
	OnSubscribe(ctx context.Context, hctx *Context, topics []string) error

	// OnUnsubscribe is called after a successful unsubscription.
	// This is a notification hook for audit logging or metrics.
	OnUnsubscribe(ctx context.Context, hctx *Context, topics []string) error

	// OnDisconnect is called when a client disconnects (gracefully or due to error).
	// This is a notification hook for cleanup, audit logging, or metrics.
	OnDisconnect(ctx context.Context, hctx *Context) error
}

// NoopHandler is a Handler implementation that allows all operations.
// Useful for testing or when no authorization is needed.
type NoopHandler struct{}

var _ Handler = (*NoopHandler)(nil)

func (h *NoopHandler) AuthConnect(ctx context.Context, hctx *Context) error {
	return nil
}

func (h *NoopHandler) AuthPublish(ctx context.Context, hctx *Context, topic *string, payload *[]byte) error {
	return nil
}

func (h *NoopHandler) AuthSubscribe(ctx context.Context, hctx *Context, topics *[]string) error {
	return nil
}

func (h *NoopHandler) OnConnect(ctx context.Context, hctx *Context) error {
	return nil
}

func (h *NoopHandler) OnPublish(ctx context.Context, hctx *Context, topic string, payload []byte) error {
	return nil
}

func (h *NoopHandler) OnSubscribe(ctx context.Context, hctx *Context, topics []string) error {
	return nil
}

func (h *NoopHandler) OnUnsubscribe(ctx context.Context, hctx *Context, topics []string) error {
	return nil
}

func (h *NoopHandler) OnDisconnect(ctx context.Context, hctx *Context) error {
	return nil
}
