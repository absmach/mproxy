// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package websocket implements the WebSocket protocol parser for mproxy.
//
// # Overview
//
// The WebSocket parser handles WebSocket upgrade requests and then delegates
// to underlying protocol parsers (typically MQTT over WebSocket). It bridges
// WebSocket connections to the standard parser interface.
//
// # Architecture
//
// WebSocket support has two components:
//
//  1. Parser: Handles WebSocket upgrade and connection setup
//  2. Conn: Adapts websocket.Conn to net.Conn interface
//
// # Connection Flow
//
//	1. Client sends HTTP upgrade request to WebSocket
//	2. Parser upgrades connection using gorilla/websocket
//	3. Parser creates backend WebSocket connection
//	4. Parser wraps both connections as net.Conn using Conn adapter
//	5. Parser delegates to underlying protocol parser (e.g., MQTT)
//	6. Underlying parser handles protocol-specific packets
//
// # Conn Adapter
//
// The Conn type wraps websocket.Conn to implement net.Conn interface:
//
//	type Conn struct {
//		*websocket.Conn
//		reader io.Reader
//	}
//
// This allows WebSocket connections to work with parsers that expect
// io.Reader/io.Writer interfaces (like MQTT parser).
//
// # Read/Write Behavior
//
//   - Read(): Reads from current message, fetching next message when needed
//   - Write(): Writes binary WebSocket message
//   - Close(): Closes WebSocket connection gracefully
//
// # Underlying Parser
//
// The WebSocket parser requires an underlying parser for the protocol
// running over WebSocket:
//
//	mqttParser := &mqtt.Parser{}
//	wsParser := &websocket.Parser{
//		UnderlyingParser: mqttParser,
//	}
//
// Common use cases:
//   - MQTT over WebSocket
//   - CoAP over WebSocket
//   - Custom protocols over WebSocket
//
// # Authentication
//
// Authentication is handled by the underlying protocol parser:
//   - For MQTT over WebSocket: MQTT CONNECT packet carries credentials
//   - For HTTP-based auth: Passed in upgrade request headers
//
// # Protocol Field
//
// The parser sets hctx.Protocol based on the underlying parser:
//   - "mqtt" for MQTT over WebSocket
//   - "coap" for CoAP over WebSocket
//   - Or custom protocol name
//
// # Upgrade Path
//
// By default, WebSocket upgrade happens on any path. Configure the
// server to handle WebSocket on specific paths:
//
//	/mqtt → MQTT over WebSocket
//	/coap → CoAP over WebSocket
package websocket
