// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package pool provides connection pooling for backend connections.
package pool

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	// ErrPoolClosed is returned when the pool is closed.
	ErrPoolClosed = errors.New("connection pool is closed")
	// ErrPoolExhausted is returned when no connections are available.
	ErrPoolExhausted = errors.New("connection pool exhausted")
)

// Config holds connection pool configuration.
type Config struct {
	// MaxIdle is the maximum number of idle connections in the pool.
	MaxIdle int
	// MaxActive is the maximum number of active connections.
	// If 0, there is no limit.
	MaxActive int
	// IdleTimeout is the maximum time a connection can be idle before being closed.
	IdleTimeout time.Duration
	// MaxConnLifetime is the maximum time a connection can be alive.
	MaxConnLifetime time.Duration
	// DialTimeout is the timeout for establishing new connections.
	DialTimeout time.Duration
	// WaitTimeout is the maximum time to wait for a connection when pool is exhausted.
	// If 0, returns error immediately.
	WaitTimeout time.Duration
}

// Conn wraps a net.Conn with metadata.
type Conn struct {
	net.Conn
	createdAt time.Time
	pool      *Pool
}

// Close returns the connection to the pool.
func (c *Conn) Close() error {
	return c.pool.put(c)
}

// DialFunc is a function that creates a new connection.
type DialFunc func(ctx context.Context) (net.Conn, error)

// Pool is a connection pool.
type Pool struct {
	mu          sync.Mutex
	idle        []*Conn
	active      int
	dialFunc    DialFunc
	config      Config
	closed      bool
	waitChan    chan struct{}
}

// New creates a new connection pool.
func New(dialFunc DialFunc, config Config) *Pool {
	if config.MaxIdle <= 0 {
		config.MaxIdle = 10
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 5 * time.Minute
	}
	if config.MaxConnLifetime == 0 {
		config.MaxConnLifetime = 30 * time.Minute
	}
	if config.DialTimeout == 0 {
		config.DialTimeout = 10 * time.Second
	}

	p := &Pool{
		dialFunc: dialFunc,
		config:   config,
		waitChan: make(chan struct{}, 1),
	}

	// Start idle connection cleaner
	go p.cleanIdleConnections()

	return p
}

// Get retrieves a connection from the pool or creates a new one.
func (p *Pool) Get(ctx context.Context) (*Conn, error) {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}

	// Try to get an idle connection
	for len(p.idle) > 0 {
		conn := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]

		// Check if connection is still valid
		if p.isValid(conn) {
			p.active++
			p.mu.Unlock()
			return conn, nil
		}

		// Connection expired, close it
		conn.Conn.Close()
	}

	// Check if we can create a new connection
	if p.config.MaxActive > 0 && p.active >= p.config.MaxActive {
		p.mu.Unlock()

		// Wait for a connection to become available if WaitTimeout is set
		if p.config.WaitTimeout > 0 {
			timer := time.NewTimer(p.config.WaitTimeout)
			defer timer.Stop()

			select {
			case <-p.waitChan:
				return p.Get(ctx)
			case <-timer.C:
				return nil, ErrPoolExhausted
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		return nil, ErrPoolExhausted
	}

	// Create new connection
	p.active++
	p.mu.Unlock()

	dialCtx, cancel := context.WithTimeout(ctx, p.config.DialTimeout)
	defer cancel()

	rawConn, err := p.dialFunc(dialCtx)
	if err != nil {
		p.mu.Lock()
		p.active--
		p.mu.Unlock()
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	conn := &Conn{
		Conn:      rawConn,
		createdAt: time.Now(),
		pool:      p,
	}

	return conn, nil
}

// put returns a connection to the pool.
func (p *Pool) put(conn *Conn) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.active--

	if p.closed || !p.isValid(conn) {
		return conn.Conn.Close()
	}

	if len(p.idle) >= p.config.MaxIdle {
		return conn.Conn.Close()
	}

	p.idle = append(p.idle, conn)

	// Notify waiting goroutines
	select {
	case p.waitChan <- struct{}{}:
	default:
	}

	return nil
}

// isValid checks if a connection is still valid.
func (p *Pool) isValid(conn *Conn) bool {
	// Check max lifetime
	if p.config.MaxConnLifetime > 0 && time.Since(conn.createdAt) > p.config.MaxConnLifetime {
		return false
	}

	// TODO: Add connection health check (send ping)
	return true
}

// cleanIdleConnections periodically closes idle connections that have exceeded IdleTimeout.
func (p *Pool) cleanIdleConnections() {
	ticker := time.NewTicker(p.config.IdleTimeout / 2)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}

		var kept []*Conn
		now := time.Now()

		for _, conn := range p.idle {
			// Simple idle timeout: close connections that have been idle too long
			if p.config.IdleTimeout > 0 && now.Sub(conn.createdAt) > p.config.IdleTimeout {
				conn.Conn.Close()
			} else {
				kept = append(kept, conn)
			}
		}

		p.idle = kept
		p.mu.Unlock()
	}
}

// Close closes the pool and all connections.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	for _, conn := range p.idle {
		conn.Conn.Close()
	}
	p.idle = nil

	return nil
}

// Stats returns pool statistics.
func (p *Pool) Stats() (idle, active int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.idle), p.active
}
