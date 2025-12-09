// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package tcp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser"
	"github.com/google/uuid"
)

// ErrShutdownTimeout is returned when graceful shutdown exceeds the configured timeout.
var ErrShutdownTimeout = errors.New("shutdown timeout exceeded")

// Config holds the TCP server configuration.
type Config struct {
	// Address is the listen address (host:port)
	Address string

	// TargetAddress is the backend server address to proxy to (host:port)
	TargetAddress string

	// TLSConfig is optional TLS configuration for the listener
	TLSConfig *tls.Config

	// ShutdownTimeout is the maximum time to wait for active connections to drain
	// during graceful shutdown. After this timeout, remaining connections are
	// forcefully closed.
	ShutdownTimeout time.Duration

	// MaxConnections is the maximum number of concurrent connections allowed.
	// If 0, no limit is enforced. Default is 0 (unlimited).
	MaxConnections int

	// ReadTimeout is the maximum duration for reading from a connection.
	// If 0, no read timeout is set. Default is 60 seconds.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration for writing to a connection.
	// If 0, no write timeout is set. Default is 60 seconds.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum duration a connection can be idle.
	// If 0, connections never timeout due to idleness. Default is 300 seconds (5 min).
	IdleTimeout time.Duration

	// BufferSize is the size of read/write buffers in bytes.
	// If 0, uses default size of 4KB.
	BufferSize int

	// TCPKeepAlive enables TCP keepalive if > 0. The value specifies the keepalive period.
	// Default is 15 seconds.
	TCPKeepAlive time.Duration

	// DisableNoDelay controls TCP_NODELAY socket option.
	// If false (default), Nagle's algorithm is disabled for lower latency.
	DisableNoDelay bool

	// Logger for server events
	Logger *slog.Logger
}

// Server is a protocol-agnostic TCP server that accepts connections and
// proxies them to a backend server using a pluggable parser.
type Server struct {
	config     Config
	parser     parser.Parser
	handler    handler.Handler
	wg         sync.WaitGroup
	mu         sync.Mutex
	bufferPool *sync.Pool
	connSem    chan struct{} // semaphore for connection limiting
}

// New creates a new TCP server with the given configuration, parser, and handler.
func New(cfg Config, p parser.Parser, h handler.Handler) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 60 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 60 * time.Second
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 300 * time.Second
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 4096
	}
	if cfg.TCPKeepAlive == 0 {
		cfg.TCPKeepAlive = 15 * time.Second
	}

	// Create buffer pool for efficient memory reuse
	bufferPool := &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, cfg.BufferSize)
			return &buf
		},
	}

	// Create connection semaphore if limit is set
	var connSem chan struct{}
	if cfg.MaxConnections > 0 {
		connSem = make(chan struct{}, cfg.MaxConnections)
	}

	return &Server{
		config:     cfg,
		parser:     p,
		handler:    h,
		bufferPool: bufferPool,
		connSem:    connSem,
	}
}

// Listen starts the TCP server and blocks until the context is cancelled.
// It implements graceful shutdown with connection draining.
func (s *Server) Listen(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Address, err)
	}

	// Wrap with TLS if configured
	if s.config.TLSConfig != nil {
		listener = tls.NewListener(listener, s.config.TLSConfig)
		s.config.Logger.Info("TLS enabled", slog.String("address", s.config.Address))
	}

	s.config.Logger.Info("TCP server started", slog.String("address", s.config.Address))

	// Create a separate context for active connections
	// This allows us to control when to forcefully close connections
	connCtx, connCancel := context.WithCancel(context.Background())
	defer connCancel()

	// Accept loop
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					// Expected error during shutdown
					return
				default:
					s.config.Logger.Error("failed to accept connection", slog.String("error", err.Error()))
					continue
				}
			}

			// Apply connection limit if configured
			if s.connSem != nil {
				select {
				case s.connSem <- struct{}{}:
					// Acquired semaphore slot
				case <-ctx.Done():
					conn.Close()
					return
				default:
					// Connection limit reached, reject connection
					s.config.Logger.Warn("connection limit reached, rejecting connection",
						slog.String("remote", conn.RemoteAddr().String()))
					conn.Close()
					continue
				}
			}

			// Configure TCP connection options
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				if err := s.configureTCPConn(tcpConn); err != nil {
					s.config.Logger.Error("failed to configure TCP connection",
						slog.String("error", err.Error()))
					if s.connSem != nil {
						<-s.connSem
					}
					conn.Close()
					continue
				}
			}

			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				defer func() {
					if s.connSem != nil {
						<-s.connSem // Release semaphore slot
					}
				}()
				if err := s.handleConn(connCtx, conn); err != nil && !errors.Is(err, io.EOF) {
					s.config.Logger.Debug("connection handler error",
						slog.String("remote", conn.RemoteAddr().String()),
						slog.String("error", err.Error()))
				}
			}()
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	s.config.Logger.Info("shutdown signal received, closing listener")

	// Close the listener to stop accepting new connections
	if err := listener.Close(); err != nil {
		s.config.Logger.Error("error closing listener", slog.String("error", err.Error()))
	}

	// Wait for accept loop to finish
	<-acceptDone

	// Wait for active connections to drain with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.config.Logger.Info("all connections closed gracefully")
		return nil
	case <-time.After(s.config.ShutdownTimeout):
		s.config.Logger.Warn("shutdown timeout exceeded, forcing connection closure")
		// Cancel context to force close remaining connections
		connCancel()
		// Give a little more time for forced closure
		select {
		case <-done:
			return ErrShutdownTimeout
		case <-time.After(1 * time.Second):
			return ErrShutdownTimeout
		}
	}
}

// handleConn processes a single client connection by:
// 1. Creating a handler context with connection metadata
// 2. Dialing the backend server
// 3. Starting bidirectional streaming with the parser
// 4. Cleaning up both connections when done.
func (s *Server) handleConn(ctx context.Context, inbound net.Conn) error {
	defer inbound.Close()

	sessionID := uuid.New().String()

	// Create handler context
	hctx := &handler.Context{
		SessionID:  sessionID,
		RemoteAddr: inbound.RemoteAddr().String(),
		Protocol:   "tcp",
	}

	// Extract client certificate if using TLS
	if tlsConn, ok := inbound.(*tls.Conn); ok {
		if err := tlsConn.Handshake(); err != nil {
			return fmt.Errorf("TLS handshake failed: %w", err)
		}
		state := tlsConn.ConnectionState()
		if len(state.PeerCertificates) > 0 {
			hctx.Cert = state.PeerCertificates[0]
		}
	}

	// Dial backend server
	outbound, err := net.Dial("tcp", s.config.TargetAddress)
	if err != nil {
		return fmt.Errorf("failed to dial backend %s: %w", s.config.TargetAddress, err)
	}
	defer outbound.Close()

	s.config.Logger.Debug("connection established",
		slog.String("session", sessionID),
		slog.String("client", hctx.RemoteAddr),
		slog.String("backend", s.config.TargetAddress))

	// Start bidirectional streaming
	errCh := make(chan error, 2)

	// Upstream: client → backend
	go func() {
		err := s.stream(ctx, inbound, outbound, parser.Upstream, hctx)
		errCh <- err
	}()

	// Downstream: backend → client
	go func() {
		err := s.stream(ctx, outbound, inbound, parser.Downstream, hctx)
		errCh <- err
	}()

	// Wait for either direction to complete
	var streamErr error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && !errors.Is(err, io.EOF) {
			if streamErr == nil {
				streamErr = err
			}
		}
	}

	// Notify disconnect
	if err := s.handler.OnDisconnect(context.Background(), hctx); err != nil {
		s.config.Logger.Error("disconnect handler error",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}

	s.config.Logger.Debug("connection closed",
		slog.String("session", sessionID))

	return streamErr
}

// stream continuously parses packets in one direction until an error or context cancellation.
func (s *Server) stream(ctx context.Context, r, w net.Conn, dir parser.Direction, hctx *handler.Context) error {
	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Set read deadline if configured
		if s.config.ReadTimeout > 0 {
			if err := r.SetReadDeadline(time.Now().Add(s.config.ReadTimeout)); err != nil {
				return fmt.Errorf("failed to set read deadline: %w", err)
			}
		}

		// Set write deadline if configured
		if s.config.WriteTimeout > 0 {
			if err := w.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout)); err != nil {
				return fmt.Errorf("failed to set write deadline: %w", err)
			}
		}

		// Parse one packet
		if err := s.parser.Parse(ctx, r, w, dir, s.handler, hctx); err != nil {
			return err
		}
	}
}

// configureTCPConn sets TCP socket options for optimal performance and resilience.
func (s *Server) configureTCPConn(conn *net.TCPConn) error {
	// Enable TCP keepalive to detect dead connections
	if s.config.TCPKeepAlive > 0 {
		if err := conn.SetKeepAlive(true); err != nil {
			return fmt.Errorf("failed to enable keepalive: %w", err)
		}
		if err := conn.SetKeepAlivePeriod(s.config.TCPKeepAlive); err != nil {
			return fmt.Errorf("failed to set keepalive period: %w", err)
		}
	}

	// Disable Nagle's algorithm for lower latency unless explicitly disabled
	if !s.config.DisableNoDelay {
		if err := conn.SetNoDelay(true); err != nil {
			return fmt.Errorf("failed to set TCP_NODELAY: %w", err)
		}
	}

	return nil
}
