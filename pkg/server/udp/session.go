// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package udp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/google/uuid"
)

// Session represents a virtual UDP "connection" for a specific client.
// Since UDP is connectionless, we maintain session state per client address.
type Session struct {
	// ID is a unique identifier for this session
	ID string

	// RemoteAddr is the client's UDP address
	RemoteAddr *net.UDPAddr

	// Backend is the connection to the backend server
	Backend *net.UDPConn

	// LastActivity tracks the last time a packet was received/sent
	LastActivity time.Time

	// Context is the handler context for this session
	Context *handler.Context

	// ctx and cancel are used to terminate the session
	ctx    context.Context
	cancel context.CancelFunc

	// mu protects LastActivity updates
	mu sync.Mutex
}

// UpdateActivity updates the last activity timestamp for this session.
func (s *Session) UpdateActivity() {
	s.mu.Lock()
	s.LastActivity = time.Now()
	s.mu.Unlock()
}

// GetLastActivity returns the last activity timestamp.
func (s *Session) GetLastActivity() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.LastActivity
}

// Close closes the session and its backend connection.
func (s *Session) Close() error {
	s.cancel()
	if s.Backend != nil {
		return s.Backend.Close()
	}
	return nil
}

// SessionManager manages multiple UDP sessions keyed by client address.
type SessionManager struct {
	sessions    map[string]*Session
	mu          sync.RWMutex
	logger      *slog.Logger
	wg          sync.WaitGroup
	maxSessions int
}

// NewSessionManager creates a new session manager.
func NewSessionManager(logger *slog.Logger, maxSessions int) *SessionManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &SessionManager{
		sessions:    make(map[string]*Session),
		logger:      logger,
		maxSessions: maxSessions,
	}
}

// GetOrCreate gets an existing session or creates a new one for the given client address.
func (sm *SessionManager) GetOrCreate(ctx context.Context, clientAddr *net.UDPAddr, targetAddr string) (*Session, bool, error) {
	key := clientAddr.String()

	// Try to get existing session (read lock)
	sm.mu.RLock()
	if sess, ok := sm.sessions[key]; ok {
		sm.mu.RUnlock()
		sess.UpdateActivity()
		return sess, false, nil
	}
	sm.mu.RUnlock()

	// Create new session (write lock)
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Double-check in case another goroutine created it
	if sess, ok := sm.sessions[key]; ok {
		sess.UpdateActivity()
		return sess, false, nil
	}

	// Check session limit
	if sm.maxSessions > 0 && len(sm.sessions) >= sm.maxSessions {
		return nil, false, fmt.Errorf("session limit reached (%d), rejecting new session", sm.maxSessions)
	}

	// Dial backend
	backendAddr, err := net.ResolveUDPAddr("udp", targetAddr)
	if err != nil {
		return nil, false, fmt.Errorf("failed to resolve backend address %s: %w", targetAddr, err)
	}

	backend, err := net.DialUDP("udp", nil, backendAddr)
	if err != nil {
		return nil, false, fmt.Errorf("failed to dial backend %s: %w", targetAddr, err)
	}

	sessionID := uuid.New().String()
	sessCtx, sessCancel := context.WithCancel(ctx)

	sess := &Session{
		ID:           sessionID,
		RemoteAddr:   clientAddr,
		Backend:      backend,
		LastActivity: time.Now(),
		Context: &handler.Context{
			SessionID:  sessionID,
			RemoteAddr: clientAddr.String(),
			Protocol:   "udp",
		},
		ctx:    sessCtx,
		cancel: sessCancel,
	}

	sm.sessions[key] = sess

	sm.logger.Debug("new UDP session created",
		slog.String("session", sessionID),
		slog.String("client", clientAddr.String()),
		slog.String("backend", targetAddr))

	return sess, true, nil
}

// Get returns an existing session for the given client address.
func (sm *SessionManager) Get(clientAddr *net.UDPAddr) (*Session, bool) {
	key := clientAddr.String()
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sess, ok := sm.sessions[key]
	return sess, ok
}

// Remove removes a session from the manager.
func (sm *SessionManager) Remove(clientAddr *net.UDPAddr) {
	key := clientAddr.String()
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, key)
}

// Cleanup removes expired sessions based on the timeout.
// Should be called periodically in a background goroutine.
func (sm *SessionManager) Cleanup(ctx context.Context, timeout time.Duration, h handler.Handler) {
	ticker := time.NewTicker(timeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.cleanupExpired(timeout, h)
		}
	}
}

// cleanupExpired removes sessions that haven't been active within the timeout.
func (sm *SessionManager) cleanupExpired(timeout time.Duration, h handler.Handler) {
	now := time.Now()
	var toRemove []string

	sm.mu.RLock()
	for key, sess := range sm.sessions {
		if now.Sub(sess.GetLastActivity()) > timeout {
			toRemove = append(toRemove, key)
		}
	}
	sm.mu.RUnlock()

	if len(toRemove) == 0 {
		return
	}

	sm.mu.Lock()
	for _, key := range toRemove {
		if sess, ok := sm.sessions[key]; ok {
			sm.logger.Debug("session timeout",
				slog.String("session", sess.ID),
				slog.String("client", sess.RemoteAddr.String()))

			// Notify disconnect
			if err := h.OnDisconnect(context.Background(), sess.Context); err != nil {
				sm.logger.Error("disconnect handler error",
					slog.String("session", sess.ID),
					slog.String("error", err.Error()))
			}

			sess.Close()
			delete(sm.sessions, key)
		}
	}
	sm.mu.Unlock()

	sm.logger.Debug("cleaned up expired sessions", slog.Int("count", len(toRemove)))
}

// DrainAll waits for all sessions to complete or forces closure after timeout.
func (sm *SessionManager) DrainAll(timeout time.Duration, h handler.Handler) error {
	sm.logger.Info("draining all UDP sessions")

	sm.mu.RLock()
	sessionCount := len(sm.sessions)
	sm.mu.RUnlock()

	if sessionCount == 0 {
		return nil
	}

	// Wait for sessions to naturally close or timeout
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			sm.mu.RLock()
			count := len(sm.sessions)
			sm.mu.RUnlock()
			if count == 0 {
				close(done)
				return
			}
			select {
			case <-ticker.C:
			}
		}
	}()

	select {
	case <-done:
		sm.logger.Info("all sessions drained")
		return nil
	case <-time.After(timeout):
		sm.logger.Warn("drain timeout exceeded, forcing session closure")
		sm.ForceCloseAll(h)
		return ErrShutdownTimeout
	}
}

// ForceCloseAll forcefully closes all sessions.
func (sm *SessionManager) ForceCloseAll(h handler.Handler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for key, sess := range sm.sessions {
		sm.logger.Debug("force closing session",
			slog.String("session", sess.ID))

		// Notify disconnect
		if err := h.OnDisconnect(context.Background(), sess.Context); err != nil {
			sm.logger.Error("disconnect handler error",
				slog.String("session", sess.ID),
				slog.String("error", err.Error()))
		}

		sess.Close()
		delete(sm.sessions, key)
	}
}

// Count returns the number of active sessions.
func (sm *SessionManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}
