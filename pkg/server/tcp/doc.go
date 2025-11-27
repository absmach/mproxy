// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package tcp implements a protocol-agnostic TCP server for mproxy.
//
// # Overview
//
// The TCP server accepts connections and uses pluggable parsers to handle
// protocol-specific packet inspection and authorization. It supports TLS,
// graceful shutdown, and bidirectional streaming.
//
// # Architecture
//
//	┌─────────┐         ┌─────────┐         ┌─────────┐
//	│ Client  │ ←─TCP─→ │  Server │ ←─TCP─→ │ Backend │
//	└─────────┘         └─────────┘         └─────────┘
//	                         ↓
//	                    ┌─────────┐
//	                    │ Parser  │
//	                    └─────────┘
//	                         ↓
//	                    ┌─────────┐
//	                    │ Handler │
//	                    └─────────┘
//
// # Connection Flow
//
//  1. Client connects to server
//  2. Server accepts connection
//  3. Server dials backend
//  4. Server spawns two goroutines:
//     - Upstream: Client → Backend (calls parser.Parse(Upstream))
//     - Downstream: Backend → Client (calls parser.Parse(Downstream))
//  5. Both goroutines run until connection closes
//  6. Server calls handler.OnDisconnect()
//  7. Both connections closed
//
// # Bidirectional Streaming
//
// Each connection has two independent goroutines:
//
//	Upstream goroutine:
//	  for {
//	    parser.Parse(ctx, clientConn, backendConn, Upstream, handler, hctx)
//	  }
//
//	Downstream goroutine:
//	  for {
//	    parser.Parse(ctx, backendConn, clientConn, Downstream, handler, hctx)
//	  }
//
// # Graceful Shutdown
//
// When context is canceled:
//
//  1. Server stops accepting new connections
//  2. Server waits for existing connections (with timeout)
//  3. After ShutdownTimeout, forcefully closes remaining connections
//  4. Returns ErrShutdownTimeout if timeout exceeded
//
// Connection tracking uses sync.WaitGroup:
//
//	server.wg.Add(1)
//	go server.handleConnection(...)
//	defer server.wg.Done()
//
// # TLS Support
//
// Optional TLS termination:
//
//	tlsConfig := &tls.Config{
//		Certificates: []tls.Certificate{cert},
//	}
//	cfg := tcp.Config{
//		Address:       ":8883",
//		TargetAddress: "localhost:1883",
//		TLSConfig:     tlsConfig,
//	}
//
// # Configuration
//
//   - Address: Server listen address (e.g., ":1883")
//   - TargetAddress: Backend address (e.g., "broker:1883")
//   - TLSConfig: Optional TLS configuration
//   - ShutdownTimeout: Max wait time for graceful shutdown (default: 30s)
//   - Logger: Structured logger
//
// # Error Handling
//
//   - Connection errors: Logged and connection closed
//   - Parser errors: Logged, connection closed, OnDisconnect called
//   - Backend dial errors: Logged and client connection closed
//   - Shutdown timeout: Returns ErrShutdownTimeout
//
// # Example
//
//	parser := &mqtt.Parser{}
//	handler := &MyHandler{}
//
//	cfg := tcp.Config{
//		Address:         ":1883",
//		TargetAddress:   "broker:1883",
//		ShutdownTimeout: 30 * time.Second,
//	}
//
//	server := tcp.New(cfg, parser, handler)
//	if err := server.Listen(ctx); err != nil {
//		log.Fatal(err)
//	}
package tcp
