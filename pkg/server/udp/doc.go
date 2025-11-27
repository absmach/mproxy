// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package udp implements a protocol-agnostic UDP server with session management for mproxy.
//
// # Overview
//
// The UDP server handles connectionless UDP traffic by creating sessions for each
// unique client address. It supports DTLS, graceful shutdown, and session timeout.
//
// # Architecture
//
//	┌─────────┐         ┌─────────┐         ┌─────────┐
//	│ Client  │ ←─UDP─→ │  Server │ ←─UDP─→ │ Backend │
//	└─────────┘         └─────────┘         └─────────┘
//	     │                   │                    │
//	     │                   ↓                    │
//	     │             ┌──────────┐              │
//	     └─ Session ──→│  Session │ ─────────────┘
//	                   │ Manager  │
//	                   └──────────┘
//	                        ↓
//	                   ┌─────────┐
//	                   │ Parser  │
//	                   └─────────┘
//	                        ↓
//	                   ┌─────────┐
//	                   │ Handler │
//	                   └─────────┘
//
// # Session Management
//
// Since UDP is connectionless, the server creates sessions to track clients:
//
//	Session Key: Client IP:Port
//	Session Contents:
//	  - ID: Unique session identifier
//	  - RemoteAddr: Client's UDP address
//	  - Backend: UDP connection to backend
//	  - LastActivity: Timestamp of last packet
//	  - Context: Handler context for this session
//
// # Packet Flow
//
//	1. Client sends UDP packet to server
//	2. Server identifies client by IP:Port
//	3. Server gets or creates session for client
//	4. Server spawns goroutines (first packet only):
//	   - Upstream: Client → Backend
//	   - Downstream: Backend → Client
//	5. Parser.Parse() called with packet data
//	6. Packet forwarded to backend
//	7. Session LastActivity updated
//
// # Session Lifecycle
//
//	Create:
//	  - First packet from new client IP:Port
//	  - Create backend UDP connection
//	  - Spawn upstream/downstream goroutines
//	  - Add to session map
//
//	Active:
//	  - Packets update LastActivity timestamp
//	  - Session kept alive while packets flowing
//
//	Timeout:
//	  - No packets for SessionTimeout duration
//	  - Cleanup goroutine detects expired session
//	  - Calls handler.OnDisconnect()
//	  - Closes backend connection
//	  - Removes from session map
//
// # Bidirectional Streaming
//
// Each session has two goroutines:
//
//	Upstream goroutine:
//	  for {
//	    // Wait for packet from client
//	    data := <-session.upstreamChan
//	    parser.Parse(ctx, bytes.NewReader(data), backendConn, Upstream, handler, hctx)
//	  }
//
//	Downstream goroutine:
//	  for {
//	    // Read from backend
//	    n, _ := backendConn.Read(buf)
//	    parser.Parse(ctx, bytes.NewReader(buf[:n]), clientWriter, Downstream, handler, hctx)
//	  }
//
// # Graceful Shutdown
//
// When context is canceled:
//
//	1. Server stops receiving new packets
//	2. Server calls ForceCloseAll() on session manager
//	3. Each session:
//	   - Calls handler.OnDisconnect()
//	   - Cancels session context
//	   - Closes backend connection
//	4. Server waits for all goroutines to finish (with timeout)
//	5. Returns ErrShutdownTimeout if timeout exceeded
//
// # Session Cleanup
//
// Background goroutine periodically cleans up expired sessions:
//
//	ticker := time.NewTicker(SessionTimeout / 2)
//	for range ticker.C {
//	  sessionManager.cleanupExpired(SessionTimeout, handler)
//	}
//
// # DTLS Support
//
// Optional DTLS termination (future):
//
//	dtlsConfig := &dtls.Config{
//		Certificates: []tls.Certificate{cert},
//	}
//	cfg := udp.Config{
//		Address:       ":5684",
//		TargetAddress: "localhost:5683",
//		DTLSConfig:    dtlsConfig,
//	}
//
// # Configuration
//
//   - Address: Server listen address (e.g., ":5683")
//   - TargetAddress: Backend address (e.g., "broker:5683")
//   - DTLSConfig: Optional DTLS configuration (future)
//   - SessionTimeout: Max idle time before session cleanup (default: 30s)
//   - ShutdownTimeout: Max wait time for graceful shutdown (default: 30s)
//   - Logger: Structured logger
//
// # Error Handling
//
//   - Session creation errors: Logged and packet dropped
//   - Parser errors: Logged, session continues
//   - Backend errors: Logged, session may be closed
//   - Shutdown timeout: Returns ErrShutdownTimeout
//
// # Example
//
//	parser := &coap.Parser{}
//	handler := &MyHandler{}
//
//	cfg := udp.Config{
//		Address:         ":5683",
//		TargetAddress:   "broker:5683",
//		SessionTimeout:  30 * time.Second,
//		ShutdownTimeout: 30 * time.Second,
//	}
//
//	server := udp.New(cfg, parser, handler)
//	if err := server.Listen(ctx); err != nil {
//		log.Fatal(err)
//	}
package udp
