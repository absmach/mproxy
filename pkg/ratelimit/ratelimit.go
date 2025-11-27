// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package ratelimit provides rate limiting using token bucket algorithm.
package ratelimit

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrRateLimitExceeded is returned when rate limit is exceeded.
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// TokenBucket implements the token bucket algorithm for rate limiting.
type TokenBucket struct {
	mu           sync.Mutex
	capacity     int64
	tokens       int64
	refillRate   int64 // tokens per second
	lastRefill   time.Time
}

// NewTokenBucket creates a new token bucket rate limiter.
// capacity is the maximum number of tokens.
// refillRate is the number of tokens added per second.
func NewTokenBucket(capacity, refillRate int64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request should be allowed.
// Returns true if allowed, false if rate limited.
func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1)
}

// AllowN checks if N requests should be allowed.
func (tb *TokenBucket) AllowN(n int64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}

	return false
}

// refill adds tokens based on elapsed time.
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()

	tokensToAdd := int64(elapsed * float64(tb.refillRate))
	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		tb.lastRefill = now
	}
}

// Available returns the number of available tokens.
func (tb *TokenBucket) Available() int64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	return tb.tokens
}

// Limiter manages per-client rate limiters.
type Limiter struct {
	mu           sync.RWMutex
	limiters     map[string]*TokenBucket
	capacity     int64
	refillRate   int64
	maxClients   int
	cleanupTimer *time.Timer
}

// NewLimiter creates a new rate limiter with per-client tracking.
func NewLimiter(capacity, refillRate int64, maxClients int) *Limiter {
	if maxClients == 0 {
		maxClients = 10000
	}

	l := &Limiter{
		limiters:   make(map[string]*TokenBucket),
		capacity:   capacity,
		refillRate: refillRate,
		maxClients: maxClients,
	}

	// Periodic cleanup of inactive limiters
	l.cleanupTimer = time.AfterFunc(5*time.Minute, l.cleanup)

	return l
}

// Allow checks if a request from the given client should be allowed.
func (l *Limiter) Allow(clientID string) bool {
	return l.AllowN(clientID, 1)
}

// AllowN checks if N requests from the given client should be allowed.
func (l *Limiter) AllowN(clientID string, n int64) bool {
	l.mu.RLock()
	tb, exists := l.limiters[clientID]
	l.mu.RUnlock()

	if !exists {
		l.mu.Lock()
		// Double-check after acquiring write lock
		tb, exists = l.limiters[clientID]
		if !exists {
			// Check if we've exceeded max clients
			if len(l.limiters) >= l.maxClients {
				l.mu.Unlock()
				return false
			}

			tb = NewTokenBucket(l.capacity, l.refillRate)
			l.limiters[clientID] = tb
		}
		l.mu.Unlock()
	}

	return tb.AllowN(n)
}

// Remove removes a client's rate limiter.
func (l *Limiter) Remove(clientID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.limiters, clientID)
}

// cleanup removes inactive limiters to prevent unbounded growth.
func (l *Limiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Simple cleanup: if we have too many limiters, clear half of them
	if len(l.limiters) > l.maxClients*2 {
		count := 0
		target := l.maxClients
		newLimiters := make(map[string]*TokenBucket)

		for k, v := range l.limiters {
			if count < target {
				newLimiters[k] = v
				count++
			}
		}

		l.limiters = newLimiters
	}

	// Schedule next cleanup
	l.cleanupTimer = time.AfterFunc(5*time.Minute, l.cleanup)
}

// Stats returns limiter statistics.
func (l *Limiter) Stats() (clients int) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.limiters)
}

// Close stops the cleanup timer.
func (l *Limiter) Close() {
	if l.cleanupTimer != nil {
		l.cleanupTimer.Stop()
	}
}
