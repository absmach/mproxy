// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package parser defines the interface for protocol-specific packet inspection and modification.
//
// # Architecture Overview
//
// Parsers are the core protocol-handling components in mproxy. They sit between the
// transport layer (TCP/UDP servers) and the business logic layer (handlers), inspecting
// protocol-specific packets to extract authentication credentials and authorize operations.
//
// # Parser Interface
//
// The Parser interface has a single method:
//
//	Parse(ctx context.Context, r io.Reader, w io.Writer, dir Direction, h handler.Handler, hctx *handler.Context) error
//
// This method is called by servers for each packet/message in both directions:
//   - Upstream (Client → Backend): Extracts auth, calls handler.Auth* methods
//   - Downstream (Backend → Client): Can modify responses, calls handler.On* methods
//
// # Bidirectional Flow
//
// Parsers handle packets flowing in both directions:
//
//	Upstream (Client → Backend):
//	  1. Read packet from client (r io.Reader)
//	  2. Parse and extract auth credentials
//	  3. Call handler.Auth* methods
//	  4. If authorized, write packet to backend (w io.Writer)
//	  5. May modify packet (e.g., update credentials)
//
//	Downstream (Backend → Client):
//	  1. Read packet from backend (r io.Reader)
//	  2. Parse packet
//	  3. Call handler.On* notification methods
//	  4. Write packet to client (w io.Writer)
//	  5. May modify packet if needed
//
// # Direction
//
// The Direction type indicates packet flow:
//   - Upstream: Client → Backend (requests, publishes, subscribes)
//   - Downstream: Backend → Client (responses, messages from broker)
//
// # Protocol-Specific Parsers
//
// Each protocol has its own parser implementation:
//   - parser/mqtt: MQTT protocol parser
//   - parser/coap: CoAP protocol parser
//   - parser/http: HTTP protocol parser
//   - parser/websocket: WebSocket protocol parser
//
// # Integration with Servers
//
// Servers call Parse() for each packet/message:
//
//	TCP Server:
//	  - Two goroutines per connection (upstream, downstream)
//	  - Each goroutine calls Parse() continuously
//
//	UDP Server:
//	  - One goroutine per session per direction
//	  - Each goroutine calls Parse() for received packets
//
// # Example
//
//	type MQTTParser struct{}
//
//	func (p *MQTTParser) Parse(ctx context.Context, r io.Reader, w io.Writer, dir parser.Direction, h handler.Handler, hctx *handler.Context) error {
//		packet, err := packets.ReadPacket(r)
//		if err != nil {
//			return err
//		}
//
//		if dir == parser.Upstream {
//			switch pkt := packet.(type) {
//			case *packets.ConnectPacket:
//				// Extract credentials
//				hctx.Username = pkt.Username
//				hctx.Password = pkt.Password
//				// Authorize
//				if err := h.AuthConnect(ctx, hctx); err != nil {
//					return err
//				}
//			}
//		}
//
//		// Forward packet
//		return packet.Write(w)
//	}
package parser
