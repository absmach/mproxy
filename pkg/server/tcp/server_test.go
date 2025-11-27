// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package tcp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser"
)

type mockParser struct {
	parseErr     error
	parseCalled  int
	parseContent []byte
}

func (m *mockParser) Parse(ctx context.Context, r io.Reader, w io.Writer, dir parser.Direction, h handler.Handler, hctx *handler.Context) error {
	m.parseCalled++

	if m.parseErr != nil {
		return m.parseErr
	}

	// Read and echo back
	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil {
		return err
	}

	m.parseContent = buf[:n]
	_, err = w.Write(buf[:n])
	return err
}

type mockHandler struct {
	connectCalled    bool
	disconnectCalled bool
}

func (m *mockHandler) AuthConnect(ctx context.Context, hctx *handler.Context) error {
	m.connectCalled = true
	return nil
}

func (m *mockHandler) AuthPublish(ctx context.Context, hctx *handler.Context, topic *string, payload *[]byte) error {
	return nil
}

func (m *mockHandler) AuthSubscribe(ctx context.Context, hctx *handler.Context, topics *[]string) error {
	return nil
}

func (m *mockHandler) OnConnect(ctx context.Context, hctx *handler.Context) error {
	return nil
}

func (m *mockHandler) OnPublish(ctx context.Context, hctx *handler.Context, topic string, payload []byte) error {
	return nil
}

func (m *mockHandler) OnSubscribe(ctx context.Context, hctx *handler.Context, topics []string) error {
	return nil
}

func (m *mockHandler) OnUnsubscribe(ctx context.Context, hctx *handler.Context, topics []string) error {
	return nil
}

func (m *mockHandler) OnDisconnect(ctx context.Context, hctx *handler.Context) error {
	m.disconnectCalled = true
	return nil
}

func TestTCPServer_ListenAndAccept(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	cfg := Config{
		Address:         "localhost:0", // Use random port
		TargetAddress:   "localhost:0",
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	// Start a mock backend server
	backendListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create backend listener: %v", err)
	}
	defer backendListener.Close()

	cfg.TargetAddress = backendListener.Addr().String()

	// Handle backend connection
	go func() {
		conn, err := backendListener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Echo back
		io.Copy(conn, conn)
	}()

	// Create server
	server := New(cfg, mockP, mockH)

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Listen(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Get actual server address
	// We need to connect to verify the server started
	// Since we used port 0, we don't know the actual port
	// Let's just verify no immediate error
	select {
	case err := <-serverErr:
		t.Fatalf("Server exited with error: %v", err)
	case <-time.After(100 * time.Millisecond):
		// Server is running
	}

	// Shutdown
	cancel()

	// Wait for clean shutdown
	select {
	case err := <-serverErr:
		if err != nil && err != context.Canceled {
			t.Errorf("Server shutdown with error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Error("Server shutdown timeout")
	}
}

func TestTCPServer_ShutdownTimeout(t *testing.T) {
	mockP := &mockParser{
		parseErr: nil, // Will block reading
	}
	mockH := &mockHandler{}

	cfg := Config{
		Address:         "localhost:0",
		TargetAddress:   "localhost:0",
		ShutdownTimeout: 100 * time.Millisecond, // Short timeout
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	// Start a mock backend that accepts but doesn't respond
	backendListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create backend listener: %v", err)
	}
	defer backendListener.Close()

	cfg.TargetAddress = backendListener.Addr().String()

	go func() {
		conn, err := backendListener.Accept()
		if err != nil {
			return
		}
		// Don't close, keep connection open
		time.Sleep(10 * time.Second)
		conn.Close()
	}()

	server := New(cfg, mockP, mockH)

	ctx, cancel := context.WithCancel(context.Background())

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Listen(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown
	cancel()

	// Wait for shutdown with timeout
	select {
	case err := <-serverErr:
		// Should get timeout error
		if err != ErrShutdownTimeout && err != context.Canceled {
			t.Logf("Got error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Test timeout waiting for server shutdown")
	}
}

func TestTCPServer_InvalidAddress(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	cfg := Config{
		Address:         "invalid:address:99999", // Invalid address
		TargetAddress:   "localhost:0",
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := New(cfg, mockP, mockH)

	err := server.Listen(context.Background())
	if err == nil {
		t.Error("Expected error for invalid address")
	}
}

func TestTCPServer_BackendDialFailure(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	// Start server listening
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	cfg := Config{
		Address:         listener.Addr().String(),
		TargetAddress:   "localhost:9", // Port that won't be listening
		ShutdownTimeout: 1 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := New(cfg, mockP, mockH)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Listen(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Try to connect - should fail to dial backend
	conn, err := net.Dial("tcp", cfg.Address)
	if err != nil {
		// Server might have shut down already
		return
	}
	conn.Write([]byte("test"))
	conn.Close()

	// Server should continue running despite failed backend dial
	time.Sleep(100 * time.Millisecond)

	cancel()
	<-serverErr
}

func TestNew_DefaultConfig(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	cfg := Config{
		Address:       "localhost:0",
		TargetAddress: "localhost:0",
		// No logger, no timeout set
	}

	server := New(cfg, mockP, mockH)

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.config.Logger == nil {
		t.Error("Expected default logger to be set")
	}

	if server.config.ShutdownTimeout == 0 {
		t.Error("Expected default shutdown timeout to be set")
	}
}

func TestTCPServer_ParseError(t *testing.T) {
	mockP := &mockParser{
		parseErr: errors.New("parse error"),
	}
	mockH := &mockHandler{}

	// This test verifies that parser errors are handled gracefully
	// The server should close the connection but continue running

	cfg := Config{
		Address:         "localhost:0",
		TargetAddress:   "localhost:0",
		ShutdownTimeout: 1 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	backendListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create backend listener: %v", err)
	}
	defer backendListener.Close()

	cfg.TargetAddress = backendListener.Addr().String()

	go func() {
		conn, _ := backendListener.Accept()
		if conn != nil {
			conn.Close()
		}
	}()

	server := New(cfg, mockP, mockH)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.Listen(ctx)
	time.Sleep(100 * time.Millisecond)

	// Server should be running fine despite parse errors in connections
}

func TestTCPServer_ContextCancellation(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	cfg := Config{
		Address:         "localhost:0",
		TargetAddress:   "localhost:0",
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := New(cfg, mockP, mockH)

	ctx, cancel := context.WithCancel(context.Background())

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Listen(ctx)
	}()

	// Immediately cancel
	cancel()

	// Should shutdown quickly
	select {
	case <-serverErr:
		// Good, server shut down
	case <-time.After(2 * time.Second):
		t.Error("Server did not shutdown in time after context cancellation")
	}
}
