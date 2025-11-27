// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package udp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser"
)

const (
	// DefaultSessionTimeout is the default timeout for idle UDP sessions.
	DefaultSessionTimeout = 30 * time.Second

	// DefaultShutdownTimeout is the default timeout for graceful shutdown.
	DefaultShutdownTimeout = 30 * time.Second

	// MaxDatagramSize is the maximum size of a UDP datagram.
	MaxDatagramSize = 65535
)

var (
	// ErrShutdownTimeout is returned when graceful shutdown exceeds the configured timeout.
	ErrShutdownTimeout = errors.New("shutdown timeout exceeded")
)

// Config holds the UDP server configuration.
type Config struct {
	// Address is the listen address (host:port)
	Address string

	// TargetAddress is the backend server address to proxy to (host:port)
	TargetAddress string

	// SessionTimeout is the idle timeout for UDP sessions
	// If no packets are received/sent for this duration, the session is closed
	SessionTimeout time.Duration

	// ShutdownTimeout is the maximum time to wait for active sessions to drain
	// during graceful shutdown
	ShutdownTimeout time.Duration

	// Logger for server events
	Logger *slog.Logger
}

// Server is a protocol-agnostic UDP server that manages sessions and
// proxies datagrams to a backend server using a pluggable parser.
type Server struct {
	config   Config
	parser   parser.Parser
	handler  handler.Handler
	sessions *SessionManager
}

// New creates a new UDP server with the given configuration, parser, and handler.
func New(cfg Config, p parser.Parser, h handler.Handler) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.SessionTimeout == 0 {
		cfg.SessionTimeout = DefaultSessionTimeout
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = DefaultShutdownTimeout
	}

	return &Server{
		config:   cfg,
		parser:   p,
		handler:  h,
		sessions: NewSessionManager(cfg.Logger),
	}
}

// Listen starts the UDP server and blocks until the context is cancelled.
// It implements graceful shutdown with session draining.
func (s *Server) Listen(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", s.config.Address)
	if err != nil {
		return fmt.Errorf("failed to resolve address %s: %w", s.config.Address, err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Address, err)
	}
	defer conn.Close()

	s.config.Logger.Info("UDP server started",
		slog.String("address", s.config.Address),
		slog.Duration("session_timeout", s.config.SessionTimeout))

	// Start session cleanup goroutine
	cleanupCtx, cleanupCancel := context.WithCancel(ctx)
	defer cleanupCancel()
	go s.sessions.Cleanup(cleanupCtx, s.config.SessionTimeout, s.handler)

	// Read loop
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buffer := make([]byte, MaxDatagramSize)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			n, clientAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				select {
				case <-ctx.Done():
					// Expected error during shutdown
					return
				default:
					s.config.Logger.Error("failed to read UDP packet",
						slog.String("error", err.Error()))
					continue
				}
			}

			// Process packet in a new goroutine
			datagram := make([]byte, n)
			copy(datagram, buffer[:n])

			go func(addr *net.UDPAddr, data []byte) {
				if err := s.handlePacket(ctx, conn, addr, data); err != nil {
					s.config.Logger.Debug("packet handler error",
						slog.String("client", addr.String()),
						slog.String("error", err.Error()))
				}
			}(clientAddr, datagram)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	s.config.Logger.Info("shutdown signal received, closing listener")

	// Close the connection to stop reading
	if err := conn.Close(); err != nil {
		s.config.Logger.Error("error closing listener", slog.String("error", err.Error()))
	}

	// Wait for read loop to finish
	<-readDone

	// Drain sessions with timeout
	return s.sessions.DrainAll(s.config.ShutdownTimeout, s.handler)
}

// handlePacket processes a single UDP packet by:
// 1. Getting or creating a session for the client
// 2. Parsing the packet with the protocol parser
// 3. Forwarding to the backend
// 4. Starting downstream reader if this is a new session
func (s *Server) handlePacket(ctx context.Context, listener *net.UDPConn, clientAddr *net.UDPAddr, data []byte) error {
	// Get or create session
	sess, isNew, err := s.sessions.GetOrCreate(ctx, clientAddr, s.config.TargetAddress)
	if err != nil {
		return fmt.Errorf("failed to get/create session: %w", err)
	}

	// Parse packet (upstream: client → backend)
	reader := bytes.NewReader(data)
	writer := &udpWriter{conn: sess.Backend}

	if err := s.parser.Parse(ctx, reader, writer, parser.Upstream, s.handler, sess.Context); err != nil {
		s.config.Logger.Debug("parser error",
			slog.String("session", sess.ID),
			slog.String("direction", "upstream"),
			slog.String("error", err.Error()))
		// Don't return error - continue processing other packets
	}

	// If this is a new session, start downstream reader
	if isNew {
		go s.readDownstream(sess, listener)
	}

	return nil
}

// readDownstream continuously reads packets from the backend and forwards to the client.
func (s *Server) readDownstream(sess *Session, listener *net.UDPConn) {
	defer func() {
		// Remove session when downstream reader exits
		s.sessions.Remove(sess.RemoteAddr)
		if err := s.handler.OnDisconnect(context.Background(), sess.Context); err != nil {
			s.config.Logger.Error("disconnect handler error",
				slog.String("session", sess.ID),
				slog.String("error", err.Error()))
		}
		sess.Close()
		s.config.Logger.Debug("downstream reader closed",
			slog.String("session", sess.ID))
	}()

	buffer := make([]byte, MaxDatagramSize)
	for {
		select {
		case <-sess.ctx.Done():
			return
		default:
		}

		// Set read deadline to check context periodically
		if err := sess.Backend.SetReadDeadline(time.Now().Add(s.config.SessionTimeout)); err != nil {
			s.config.Logger.Error("failed to set read deadline",
				slog.String("session", sess.ID),
				slog.String("error", err.Error()))
			return
		}

		n, err := sess.Backend.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Check if session is still active
				if time.Since(sess.GetLastActivity()) > s.config.SessionTimeout {
					s.config.Logger.Debug("session timeout",
						slog.String("session", sess.ID))
					return
				}
				continue
			}
			s.config.Logger.Debug("backend read error",
				slog.String("session", sess.ID),
				slog.String("error", err.Error()))
			return
		}

		sess.UpdateActivity()

		// Parse packet (downstream: backend → client)
		reader := bytes.NewReader(buffer[:n])
		writer := &udpClientWriter{conn: listener, addr: sess.RemoteAddr}

		if err := s.parser.Parse(sess.ctx, reader, writer, parser.Downstream, s.handler, sess.Context); err != nil {
			s.config.Logger.Debug("parser error",
				slog.String("session", sess.ID),
				slog.String("direction", "downstream"),
				slog.String("error", err.Error()))
			// Continue processing other packets
		}
	}
}

// udpWriter is an io.Writer that writes to a UDP connection.
type udpWriter struct {
	conn *net.UDPConn
}

func (w *udpWriter) Write(p []byte) (n int, err error) {
	return w.conn.Write(p)
}

// udpClientWriter is an io.Writer that writes to a specific UDP client address.
type udpClientWriter struct {
	conn *net.UDPConn
	addr *net.UDPAddr
}

func (w *udpClientWriter) Write(p []byte) (n int, err error) {
	return w.conn.WriteToUDP(p, w.addr)
}
