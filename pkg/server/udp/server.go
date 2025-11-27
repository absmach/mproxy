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
	"sync"
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

	// DefaultBufferSize is the default buffer size for UDP packets.
	DefaultBufferSize = 8192

	// DefaultWorkerPoolSize is the default number of workers for packet processing.
	DefaultWorkerPoolSize = 100
)

// ErrShutdownTimeout is returned when graceful shutdown exceeds the configured timeout.
var ErrShutdownTimeout = errors.New("shutdown timeout exceeded")

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

	// MaxSessions is the maximum number of concurrent UDP sessions allowed.
	// If 0, no limit is enforced. Default is 0 (unlimited).
	MaxSessions int

	// BufferSize is the size of datagram read buffers in bytes.
	// If 0, uses DefaultBufferSize (8192 bytes).
	// Must not exceed MaxDatagramSize (65535).
	BufferSize int

	// WorkerPoolSize is the number of goroutines in the packet processing pool.
	// If 0, uses DefaultWorkerPoolSize (100).
	// Increasing this can improve throughput under high load.
	WorkerPoolSize int

	// ReadBufferSize sets the socket receive buffer size (SO_RCVBUF).
	// If 0, uses system default.
	ReadBufferSize int

	// WriteBufferSize sets the socket send buffer size (SO_SNDBUF).
	// If 0, uses system default.
	WriteBufferSize int

	// Logger for server events
	Logger *slog.Logger
}

// packetJob represents a packet processing job for the worker pool.
type packetJob struct {
	conn       *net.UDPConn
	clientAddr *net.UDPAddr
	data       []byte
}

// Server is a protocol-agnostic UDP server that manages sessions and
// proxies datagrams to a backend server using a pluggable parser.
type Server struct {
	config     Config
	parser     parser.Parser
	handler    handler.Handler
	sessions   *SessionManager
	bufferPool *sync.Pool
	packetCh   chan packetJob
	workerWg   sync.WaitGroup
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
	if cfg.BufferSize == 0 {
		cfg.BufferSize = DefaultBufferSize
	}
	if cfg.BufferSize > MaxDatagramSize {
		cfg.BufferSize = MaxDatagramSize
	}
	if cfg.WorkerPoolSize == 0 {
		cfg.WorkerPoolSize = DefaultWorkerPoolSize
	}

	// Create buffer pool for efficient memory reuse
	bufferPool := &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, cfg.BufferSize)
			return &buf
		},
	}

	// Create packet channel for worker pool
	// Buffered channel to prevent blocking the reader
	packetCh := make(chan packetJob, cfg.WorkerPoolSize*2)

	return &Server{
		config:     cfg,
		parser:     p,
		handler:    h,
		sessions:   NewSessionManager(cfg.Logger, cfg.MaxSessions),
		bufferPool: bufferPool,
		packetCh:   packetCh,
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

	// Configure socket buffer sizes if specified
	if s.config.ReadBufferSize > 0 {
		if err := conn.SetReadBuffer(s.config.ReadBufferSize); err != nil {
			s.config.Logger.Warn("failed to set read buffer size",
				slog.String("error", err.Error()))
		}
	}
	if s.config.WriteBufferSize > 0 {
		if err := conn.SetWriteBuffer(s.config.WriteBufferSize); err != nil {
			s.config.Logger.Warn("failed to set write buffer size",
				slog.String("error", err.Error()))
		}
	}

	s.config.Logger.Info("UDP server started",
		slog.String("address", s.config.Address),
		slog.Duration("session_timeout", s.config.SessionTimeout),
		slog.Int("worker_pool_size", s.config.WorkerPoolSize),
		slog.Int("buffer_size", s.config.BufferSize))

	// Start worker pool for packet processing
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()
	s.startWorkerPool(workerCtx, conn)

	// Start session cleanup goroutine
	cleanupCtx, cleanupCancel := context.WithCancel(ctx)
	defer cleanupCancel()
	go s.sessions.Cleanup(cleanupCtx, s.config.SessionTimeout, s.handler)

	// Read loop
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Get buffer from pool
			bufPtr := s.bufferPool.Get().(*[]byte)
			buffer := *bufPtr

			n, clientAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				s.bufferPool.Put(bufPtr) // Return buffer to pool
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

			// Make a copy of the data for processing
			datagram := make([]byte, n)
			copy(datagram, buffer[:n])
			s.bufferPool.Put(bufPtr) // Return buffer to pool immediately

			// Send packet to worker pool (non-blocking)
			select {
			case s.packetCh <- packetJob{
				conn:       conn,
				clientAddr: clientAddr,
				data:       datagram,
			}:
				// Packet queued successfully
			case <-ctx.Done():
				return
			default:
				// Worker pool is full, drop packet and log warning
				s.config.Logger.Warn("worker pool full, dropping packet",
					slog.String("client", clientAddr.String()))
			}
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

	// Close packet channel and wait for workers to finish
	close(s.packetCh)
	workerCancel()
	s.workerWg.Wait()
	s.config.Logger.Info("all workers stopped")

	// Drain sessions with timeout
	return s.sessions.DrainAll(s.config.ShutdownTimeout, s.handler)
}

// startWorkerPool starts the worker goroutines for packet processing.
func (s *Server) startWorkerPool(ctx context.Context, listener *net.UDPConn) {
	for i := 0; i < s.config.WorkerPoolSize; i++ {
		s.workerWg.Add(1)
		go func(workerID int) {
			defer s.workerWg.Done()
			s.packetWorker(ctx, listener, workerID)
		}(i)
	}
	s.config.Logger.Info("worker pool started", slog.Int("workers", s.config.WorkerPoolSize))
}

// packetWorker processes packets from the packet channel.
func (s *Server) packetWorker(ctx context.Context, listener *net.UDPConn, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-s.packetCh:
			if !ok {
				// Channel closed, worker should exit
				return
			}
			if err := s.handlePacket(ctx, listener, job.clientAddr, job.data); err != nil {
				s.config.Logger.Debug("packet handler error",
					slog.Int("worker", workerID),
					slog.String("client", job.clientAddr.String()),
					slog.String("error", err.Error()))
			}
		}
	}
}

// handlePacket processes a single UDP packet by:
// 1. Getting or creating a session for the client
// 2. Parsing the packet with the protocol parser
// 3. Forwarding to the backend
// 4. Starting downstream reader if this is a new session.
func (s *Server) handlePacket(ctx context.Context, listener *net.UDPConn, clientAddr *net.UDPAddr, data []byte) error {
	// Get or create session
	sess, isNew, err := s.sessions.GetOrCreate(ctx, clientAddr, s.config.TargetAddress)
	if err != nil {
		// Session limit reached or other error
		s.config.Logger.Warn("failed to get/create session",
			slog.String("client", clientAddr.String()),
			slog.String("error", err.Error()))
		return err
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

	for {
		select {
		case <-sess.ctx.Done():
			return
		default:
		}

		// Get buffer from pool
		bufPtr := s.bufferPool.Get().(*[]byte)
		buffer := *bufPtr

		// Set read deadline to check context periodically
		if err := sess.Backend.SetReadDeadline(time.Now().Add(s.config.SessionTimeout)); err != nil {
			s.bufferPool.Put(bufPtr)
			s.config.Logger.Error("failed to set read deadline",
				slog.String("session", sess.ID),
				slog.String("error", err.Error()))
			return
		}

		n, err := sess.Backend.Read(buffer)
		if err != nil {
			s.bufferPool.Put(bufPtr)
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

		// Return buffer to pool
		s.bufferPool.Put(bufPtr)
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
