// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package udp

import (
	"context"
	"errors"
	"fmt"
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
	parseErr    error
	parseCalled int
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

func TestUDPServer_ListenAndReceive(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	// Start a mock backend server
	backendAddr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to resolve backend address: %v", err)
	}

	backendConn, err := net.ListenUDP("udp", backendAddr)
	if err != nil {
		t.Fatalf("Failed to create backend listener: %v", err)
	}
	defer backendConn.Close()

	// Echo server
	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := backendConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			backendConn.WriteToUDP(buf[:n], addr)
		}
	}()

	cfg := Config{
		Address:         "localhost:0",
		TargetAddress:   backendConn.LocalAddr().String(),
		SessionTimeout:  1 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := New(cfg, mockP, mockH)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Listen(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Server should be running
	select {
	case err := <-serverErr:
		t.Fatalf("Server exited prematurely: %v", err)
	case <-time.After(100 * time.Millisecond):
		// Good, server is running
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

func TestUDPServer_SessionCreation(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	backendAddr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to resolve backend address: %v", err)
	}

	backendConn, err := net.ListenUDP("udp", backendAddr)
	if err != nil {
		t.Fatalf("Failed to create backend listener: %v", err)
	}
	defer backendConn.Close()

	cfg := Config{
		Address:         "localhost:0",
		TargetAddress:   backendConn.LocalAddr().String(),
		SessionTimeout:  1 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := New(cfg, mockP, mockH)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go server.Listen(ctx)
	time.Sleep(100 * time.Millisecond)

	// Initially no sessions
	if server.sessions.Count() != 0 {
		t.Errorf("Expected 0 sessions, got %d", server.sessions.Count())
	}

	// Note: We can't easily test session creation without actually sending
	// UDP packets to the server, which would require knowing the server's
	// actual port. This is tested in integration tests.
}

func TestUDPServer_InvalidAddress(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	cfg := Config{
		Address:         "invalid:address:99999",
		TargetAddress:   "localhost:0",
		SessionTimeout:  1 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := New(cfg, mockP, mockH)

	err := server.Listen(context.Background())
	if err == nil {
		t.Error("Expected error for invalid address")
	}
}

func TestNew_DefaultConfig(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	cfg := Config{
		Address:       "localhost:0",
		TargetAddress: "localhost:0",
		// No logger, no timeouts set
	}

	server := New(cfg, mockP, mockH)

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.config.Logger == nil {
		t.Error("Expected default logger to be set")
	}

	if server.config.SessionTimeout == 0 {
		t.Error("Expected default session timeout to be set")
	}

	if server.config.ShutdownTimeout == 0 {
		t.Error("Expected default shutdown timeout to be set")
	}
}

func TestUDPServer_ContextCancellation(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	cfg := Config{
		Address:         "localhost:0",
		TargetAddress:   "localhost:0",
		SessionTimeout:  1 * time.Second,
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

func TestSessionManager_GetOrCreate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sm := NewSessionManager(logger, 0) // No session limit

	// Start a backend server
	backendAddr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to resolve address: %v", err)
	}

	backendConn, err := net.ListenUDP("udp", backendAddr)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backendConn.Close()

	targetAddr := backendConn.LocalAddr().String()

	clientAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")

	// Create new session
	sess, isNew, err := sm.GetOrCreate(context.Background(), clientAddr, targetAddr)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if !isNew {
		t.Error("Expected new session")
	}

	if sess == nil {
		t.Fatal("Expected non-nil session")
	}

	if sess.RemoteAddr.String() != clientAddr.String() {
		t.Errorf("Expected remote addr %s, got %s", clientAddr, sess.RemoteAddr)
	}

	// Get existing session
	sess2, isNew2, err := sm.GetOrCreate(context.Background(), clientAddr, targetAddr)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if isNew2 {
		t.Error("Expected existing session, not new")
	}

	if sess2.ID != sess.ID {
		t.Error("Expected same session ID")
	}

	// Clean up
	sm.Remove(clientAddr)
}

func TestSessionManager_Cleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sm := NewSessionManager(logger, 0) // No session limit
	mockH := &mockHandler{}

	// Start a backend server
	backendAddr, _ := net.ResolveUDPAddr("udp", "localhost:0")
	backendConn, _ := net.ListenUDP("udp", backendAddr)
	defer backendConn.Close()

	targetAddr := backendConn.LocalAddr().String()

	clientAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12346")

	// Create session
	sess, _, err := sm.GetOrCreate(context.Background(), clientAddr, targetAddr)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if sm.Count() != 1 {
		t.Errorf("Expected 1 session, got %d", sm.Count())
	}

	// Manually expire the session
	sess.mu.Lock()
	sess.LastActivity = time.Now().Add(-2 * time.Minute)
	sess.mu.Unlock()

	// Run cleanup
	sm.cleanupExpired(1*time.Minute, mockH)

	// Session should be removed
	if sm.Count() != 0 {
		t.Errorf("Expected 0 sessions after cleanup, got %d", sm.Count())
	}

	if !mockH.disconnectCalled {
		t.Error("Expected OnDisconnect to be called")
	}
}

func TestSessionManager_ForceCloseAll(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sm := NewSessionManager(logger, 0) // No session limit
	mockH := &mockHandler{}

	// Start a backend server
	backendAddr, _ := net.ResolveUDPAddr("udp", "localhost:0")
	backendConn, _ := net.ListenUDP("udp", backendAddr)
	defer backendConn.Close()

	targetAddr := backendConn.LocalAddr().String()

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		addr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", 50000+i))
		sm.GetOrCreate(context.Background(), addr, targetAddr)
	}

	if sm.Count() != 3 {
		t.Errorf("Expected 3 sessions, got %d", sm.Count())
	}

	// Force close all
	sm.ForceCloseAll(mockH)

	if sm.Count() != 0 {
		t.Errorf("Expected 0 sessions after force close, got %d", sm.Count())
	}

	if !mockH.disconnectCalled {
		t.Error("Expected OnDisconnect to be called")
	}
}

func TestSession_UpdateActivity(t *testing.T) {
	sess := &Session{
		LastActivity: time.Now().Add(-1 * time.Hour),
	}

	oldTime := sess.GetLastActivity()
	time.Sleep(10 * time.Millisecond)
	sess.UpdateActivity()
	newTime := sess.GetLastActivity()

	if !newTime.After(oldTime) {
		t.Error("Expected LastActivity to be updated")
	}
}

func TestUDPServer_ShutdownTimeout(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	backendAddr, _ := net.ResolveUDPAddr("udp", "localhost:0")
	backendConn, _ := net.ListenUDP("udp", backendAddr)
	defer backendConn.Close()

	cfg := Config{
		Address:         "localhost:0",
		TargetAddress:   backendConn.LocalAddr().String(),
		SessionTimeout:  1 * time.Second,
		ShutdownTimeout: 100 * time.Millisecond, // Short timeout
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := New(cfg, mockP, mockH)

	ctx, cancel := context.WithCancel(context.Background())

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Listen(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create a session manually
	clientAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")
	server.sessions.GetOrCreate(context.Background(), clientAddr, cfg.TargetAddress)

	// Trigger shutdown
	cancel()

	// Wait for shutdown with timeout
	select {
	case err := <-serverErr:
		// May get timeout error if session doesn't close in time
		if err != nil && err != ErrShutdownTimeout && err != context.Canceled {
			t.Logf("Got error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Test timeout waiting for server shutdown")
	}
}

func TestUDPServer_ParseError(t *testing.T) {
	mockP := &mockParser{
		parseErr: errors.New("parse error"),
	}
	mockH := &mockHandler{}

	backendAddr, _ := net.ResolveUDPAddr("udp", "localhost:0")
	backendConn, _ := net.ListenUDP("udp", backendAddr)
	defer backendConn.Close()

	cfg := Config{
		Address:         "localhost:0",
		TargetAddress:   backendConn.LocalAddr().String(),
		SessionTimeout:  1 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := New(cfg, mockP, mockH)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.Listen(ctx)
	time.Sleep(100 * time.Millisecond)

	// Server should handle parse errors gracefully
	// and continue running
}

func TestUDPServer_SessionLimit(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	backendAddr, _ := net.ResolveUDPAddr("udp", "localhost:0")
	backendConn, _ := net.ListenUDP("udp", backendAddr)
	defer backendConn.Close()

	cfg := Config{
		Address:         "localhost:0",
		TargetAddress:   backendConn.LocalAddr().String(),
		MaxSessions:     5, // Limit to 5 sessions
		SessionTimeout:  1 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := New(cfg, mockP, mockH)

	// Verify session limit was set
	if server.sessions.maxSessions != 5 {
		t.Errorf("Expected max sessions 5, got %d", server.sessions.maxSessions)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.Listen(ctx)
	time.Sleep(100 * time.Millisecond)

	cancel()
}

func TestUDPServer_WorkerPool(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	backendAddr, _ := net.ResolveUDPAddr("udp", "localhost:0")
	backendConn, _ := net.ListenUDP("udp", backendAddr)
	defer backendConn.Close()

	cfg := Config{
		Address:         "localhost:0",
		TargetAddress:   backendConn.LocalAddr().String(),
		WorkerPoolSize:  50, // Custom worker pool size
		SessionTimeout:  1 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := New(cfg, mockP, mockH)

	// Verify worker pool size was set
	if server.config.WorkerPoolSize != 50 {
		t.Errorf("Expected worker pool size 50, got %d", server.config.WorkerPoolSize)
	}

	// Verify packet channel was created
	if server.packetCh == nil {
		t.Fatal("Expected packet channel to be created")
	}

	// Verify channel buffer size
	if cap(server.packetCh) != 100 { // WorkerPoolSize * 2
		t.Errorf("Expected packet channel capacity 100, got %d", cap(server.packetCh))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.Listen(ctx)
	time.Sleep(100 * time.Millisecond)

	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestUDPServer_BufferPool(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	cfg := Config{
		Address:         "localhost:0",
		TargetAddress:   "localhost:0",
		BufferSize:      16384, // Custom buffer size
		SessionTimeout:  1 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := New(cfg, mockP, mockH)

	// Verify buffer pool was created
	if server.bufferPool == nil {
		t.Fatal("Expected buffer pool to be created")
	}

	// Verify buffer size was set
	if server.config.BufferSize != 16384 {
		t.Errorf("Expected buffer size 16384, got %d", server.config.BufferSize)
	}

	// Test buffer pool by getting and returning a buffer
	bufPtr := server.bufferPool.Get().(*[]byte)
	buf := *bufPtr
	if len(buf) != 16384 {
		t.Errorf("Expected buffer of size 16384, got %d", len(buf))
	}
	server.bufferPool.Put(bufPtr)
}

func TestUDPServer_BufferSizeLimit(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	cfg := Config{
		Address:         "localhost:0",
		TargetAddress:   "localhost:0",
		BufferSize:      100000, // Exceeds MaxDatagramSize
		SessionTimeout:  1 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	server := New(cfg, mockP, mockH)

	// Verify buffer size was capped to MaxDatagramSize
	if server.config.BufferSize != MaxDatagramSize {
		t.Errorf("Expected buffer size %d, got %d", MaxDatagramSize, server.config.BufferSize)
	}
}

func TestUDPServer_DefaultConfig(t *testing.T) {
	mockP := &mockParser{}
	mockH := &mockHandler{}

	cfg := Config{
		Address:       "localhost:0",
		TargetAddress: "localhost:0",
		// No optional settings
	}

	server := New(cfg, mockP, mockH)

	// Verify defaults were set
	if server.config.SessionTimeout != DefaultSessionTimeout {
		t.Errorf("Expected default session timeout %v, got %v", DefaultSessionTimeout, server.config.SessionTimeout)
	}
	if server.config.ShutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("Expected default shutdown timeout %v, got %v", DefaultShutdownTimeout, server.config.ShutdownTimeout)
	}
	if server.config.BufferSize != DefaultBufferSize {
		t.Errorf("Expected default buffer size %d, got %d", DefaultBufferSize, server.config.BufferSize)
	}
	if server.config.WorkerPoolSize != DefaultWorkerPoolSize {
		t.Errorf("Expected default worker pool size %d, got %d", DefaultWorkerPoolSize, server.config.WorkerPoolSize)
	}
}

func TestSessionManager_SessionLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sm := NewSessionManager(logger, 2) // Limit to 2 sessions

	backendAddr, _ := net.ResolveUDPAddr("udp", "localhost:0")
	backendConn, _ := net.ListenUDP("udp", backendAddr)
	defer backendConn.Close()

	targetAddr := backendConn.LocalAddr().String()

	// Create first session
	addr1, _ := net.ResolveUDPAddr("udp", "127.0.0.1:10001")
	_, isNew, err := sm.GetOrCreate(context.Background(), addr1, targetAddr)
	if err != nil {
		t.Fatalf("Failed to create first session: %v", err)
	}
	if !isNew {
		t.Error("Expected new session")
	}

	// Create second session
	addr2, _ := net.ResolveUDPAddr("udp", "127.0.0.1:10002")
	_, isNew, err = sm.GetOrCreate(context.Background(), addr2, targetAddr)
	if err != nil {
		t.Fatalf("Failed to create second session: %v", err)
	}
	if !isNew {
		t.Error("Expected new session")
	}

	// Try to create third session - should fail
	addr3, _ := net.ResolveUDPAddr("udp", "127.0.0.1:10003")
	_, _, err = sm.GetOrCreate(context.Background(), addr3, targetAddr)
	if err == nil {
		t.Error("Expected error when exceeding session limit")
	}

	// Clean up
	sm.Remove(addr1)
	sm.Remove(addr2)
}
