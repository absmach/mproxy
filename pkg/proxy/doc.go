// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package proxy provides high-level protocol proxy coordinators that wire together
// servers, parsers, and handlers.
//
// # Overview
//
// Proxy coordinators are convenience wrappers that combine the three core components:
//  1. Server (TCP or UDP)
//  2. Parser (protocol-specific)
//  3. Handler (business logic)
//
// # Architecture
//
//	Application
//	     ↓
//	┌─────────────┐
//	│   Proxy     │  (Coordinator)
//	│ - MQTTProxy │
//	│ - CoAPProxy │
//	│ - HTTPProxy │
//	│ - WSProxy   │
//	└─────────────┘
//	     ↓
//	┌─────────────┐
//	│   Server    │  (Transport)
//	│ - TCP       │
//	│ - UDP       │
//	└─────────────┘
//	     ↓
//	┌─────────────┐
//	│   Parser    │  (Protocol)
//	│ - MQTT      │
//	│ - CoAP      │
//	│ - HTTP      │
//	│ - WebSocket │
//	└─────────────┘
//	     ↓
//	┌─────────────┐
//	│   Handler   │  (Business Logic)
//	└─────────────┘
//
// # Available Proxies
//
//   - MQTTProxy: MQTT over TCP
//   - CoAPProxy: CoAP over UDP
//   - HTTPProxy: HTTP over TCP
//   - WebSocketProxy: WebSocket (with underlying protocol) over TCP
//
// # Configuration
//
// Each proxy has a protocol-specific config struct:
//
//	MQTTConfig:
//	  - Host, Port: Server listen address
//	  - TargetHost, TargetPort: Backend address
//	  - TLSConfig: Optional TLS
//	  - ShutdownTimeout: Graceful shutdown timeout
//	  - Logger: Structured logger
//
//	CoAPConfig:
//	  - Host, Port: Server listen address
//	  - TargetHost, TargetPort: Backend address
//	  - DTLSConfig: Optional DTLS (future)
//	  - SessionTimeout: UDP session timeout
//	  - ShutdownTimeout: Graceful shutdown timeout
//	  - Logger: Structured logger
//
// # Usage Pattern
//
//  1. Create handler implementation
//  2. Create proxy config
//  3. Create proxy with handler
//  4. Start proxy
//
// Example:
//
//	handler := &MyHandler{}
//
//	cfg := proxy.MQTTConfig{
//		Host:            "0.0.0.0",
//		Port:            "1883",
//		TargetHost:      "broker",
//		TargetPort:      "1883",
//		ShutdownTimeout: 30 * time.Second,
//	}
//
//	mqttProxy, err := proxy.NewMQTT(cfg, handler)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	ctx := context.Background()
//	if err := mqttProxy.Listen(ctx); err != nil {
//		log.Fatal(err)
//	}
//
// # Multiple Proxies
//
// Run multiple protocol proxies simultaneously:
//
//	g, ctx := errgroup.WithContext(context.Background())
//
//	g.Go(func() error {
//		return mqttProxy.Listen(ctx)
//	})
//
//	g.Go(func() error {
//		return coapProxy.Listen(ctx)
//	})
//
//	g.Go(func() error {
//		return httpProxy.Listen(ctx)
//	})
//
//	if err := g.Wait(); err != nil {
//		log.Fatal(err)
//	}
//
// # Graceful Shutdown
//
// All proxies support context-based graceful shutdown:
//
//	ctx, cancel := context.WithCancel(context.Background())
//
//	go func() {
//		<-sigterm
//		cancel()
//	}()
//
//	if err := proxy.Listen(ctx); err != nil {
//		log.Printf("shutdown: %v", err)
//	}
//
// # TLS/DTLS Termination
//
// Proxies support TLS (TCP-based) and DTLS (UDP-based) termination:
//
//	cert, _ := tls.LoadX509KeyPair("cert.pem", "key.pem")
//	tlsConfig := &tls.Config{
//		Certificates: []tls.Certificate{cert},
//	}
//
//	cfg := proxy.MQTTConfig{
//		Host:       "0.0.0.0",
//		Port:       "8883",
//		TLSConfig:  tlsConfig,
//		// ...
//	}
//
// This allows mproxy to act as an ingress component that terminates
// encryption and forwards plain traffic to backend services.
//
// # Handler Integration
//
// The same handler can be used across all proxies:
//
//	handler := &UnifiedHandler{
//		authService: authSvc,
//	}
//
//	mqttProxy, _ := proxy.NewMQTT(mqttCfg, handler)
//	coapProxy, _ := proxy.NewCoAP(coapCfg, handler)
//	httpProxy, _ := proxy.NewHTTP(httpCfg, handler)
//
// The handler.Context.Protocol field distinguishes protocol types.
package proxy
