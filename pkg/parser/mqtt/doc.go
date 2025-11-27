// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package mqtt implements the MQTT protocol parser for mproxy.
//
// # Overview
//
// The MQTT parser inspects MQTT packets to extract authentication credentials
// and authorize protocol operations. It uses the eclipse/paho.mqtt.golang library
// for packet parsing and supports MQTT 3.1.1 protocol.
//
// # Packet Handling
//
// Upstream (Client → Backend):
//   - CONNECT: Extracts username/password, calls AuthConnect
//   - PUBLISH: Extracts topic/payload, calls AuthPublish
//   - SUBSCRIBE: Extracts topics, calls AuthSubscribe
//   - UNSUBSCRIBE: Calls OnUnsubscribe
//   - DISCONNECT: Calls OnDisconnect
//   - PINGREQ: Forwarded without modification
//
// Downstream (Backend → Client):
//   - All packets forwarded without modification
//   - PUBLISH: Calls OnPublish for notification
//
// # Authentication Flow
//
//  1. Client sends CONNECT packet
//  2. Parser extracts ClientID, Username, Password
//  3. Parser calls handler.AuthConnect()
//  4. If authorized, CONNECT is forwarded to backend
//  5. Backend sends CONNACK
//  6. Parser calls handler.OnConnect()
//  7. CONNACK forwarded to client
//
// # Publish Authorization
//
//  1. Client sends PUBLISH packet
//  2. Parser extracts topic and payload
//  3. Parser calls handler.AuthPublish()
//  4. If authorized, PUBLISH forwarded to backend
//  5. Handler can modify topic or payload
//
// # Subscribe Authorization
//
//  1. Client sends SUBSCRIBE packet
//  2. Parser extracts topic filters
//  3. Parser calls handler.AuthSubscribe()
//  4. If authorized, SUBSCRIBE forwarded to backend
//
// # Credential Modification
//
// The handler can modify credentials during AuthConnect:
//
//	func (h *MyHandler) AuthConnect(ctx context.Context, hctx *handler.Context) error {
//		// Verify original credentials
//		if !h.auth.Verify(hctx.Username, hctx.Password) {
//			return errors.New("invalid credentials")
//		}
//		// Replace with backend credentials
//		hctx.Username = "backend-user"
//		hctx.Password = []byte("backend-pass")
//		return nil
//	}
//
// The parser will update the CONNECT packet with the modified credentials
// before forwarding to the backend.
//
// # Protocol Field
//
// The parser sets hctx.Protocol = "mqtt" for all MQTT connections.
package mqtt
