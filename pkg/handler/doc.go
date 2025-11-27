// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package handler provides the core interface that links protocol parsers to business logic.
//
// # Architecture Overview
//
// The Handler interface serves as the bridge between protocol-specific parsers and
// application-level authorization and event handling. When a protocol parser (MQTT, CoAP,
// HTTP, WebSocket) extracts authentication credentials or protocol-specific operations
// from packets, it calls the corresponding Handler methods.
//
// # Data Flow
//
//	Client → Parser (extracts auth) → Handler (authorizes) → Server → Backend
//	Backend → Server → Parser (modifies if needed) → Handler (notifies) → Client
//
// # Handler Methods
//
// Authorization methods (Auth*) are called before forwarding packets:
//   - AuthConnect: Verifies client credentials during connection
//   - AuthPublish: Authorizes message publication
//   - AuthSubscribe: Authorizes topic subscriptions
//
// Notification methods (On*) are called after successful operations:
//   - OnConnect: Notifies successful connection
//   - OnPublish: Notifies message publication
//   - OnSubscribe: Notifies subscription
//   - OnUnsubscribe: Notifies unsubscription
//   - OnDisconnect: Notifies disconnection
//
// # Context
//
// The Context struct carries session metadata across all handler calls:
//   - SessionID: Unique identifier for this connection/session
//   - Username, Password: Extracted credentials
//   - ClientID: Protocol-specific client identifier
//   - RemoteAddr: Client's network address
//   - Protocol: Protocol name (mqtt, coap, http, ws)
//   - Cert: Client certificate for TLS connections
//
// # Implementation
//
// Applications implement the Handler interface to integrate mproxy with their
// authorization systems. The NoopHandler provides a pass-through implementation
// for testing or when no authorization is needed.
//
// # Example
//
//	type MyHandler struct {
//		authService AuthService
//	}
//
//	func (h *MyHandler) AuthConnect(ctx context.Context, hctx *handler.Context) error {
//		return h.authService.Authenticate(hctx.Username, hctx.Password)
//	}
//
//	func (h *MyHandler) AuthPublish(ctx context.Context, hctx *handler.Context, topic *string, payload *[]byte) error {
//		return h.authService.AuthorizePublish(hctx.Username, *topic)
//	}
package handler
