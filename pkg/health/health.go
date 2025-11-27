// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package health provides health check and readiness endpoints.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Check represents a single health check.
type Check struct {
	Name        string        `json:"name"`
	Status      Status        `json:"status"`
	Message     string        `json:"message,omitempty"`
	LastChecked time.Time     `json:"last_checked"`
	Duration    time.Duration `json:"duration_ms"`
}

// CheckFunc is a function that performs a health check.
type CheckFunc func(ctx context.Context) error

// Checker manages health checks.
type Checker struct {
	mu     sync.RWMutex
	checks map[string]CheckFunc
	cache  map[string]*Check
	ttl    time.Duration
}

// NewChecker creates a new health checker.
func NewChecker(cacheTTL time.Duration) *Checker {
	if cacheTTL == 0 {
		cacheTTL = 10 * time.Second
	}
	return &Checker{
		checks: make(map[string]CheckFunc),
		cache:  make(map[string]*Check),
		ttl:    cacheTTL,
	}
}

// Register adds a health check.
func (c *Checker) Register(name string, check CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

// Health returns the overall health status.
func (c *Checker) Health(ctx context.Context) (Status, []Check) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var checks []Check
	overallStatus := StatusHealthy

	for name, checkFunc := range c.checks {
		// Check cache
		if cached, ok := c.cache[name]; ok && time.Since(cached.LastChecked) < c.ttl {
			checks = append(checks, *cached)
			if cached.Status != StatusHealthy {
				overallStatus = StatusDegraded
			}
			continue
		}

		// Run check
		start := time.Now()
		err := checkFunc(ctx)
		duration := time.Since(start)

		check := &Check{
			Name:        name,
			LastChecked: time.Now(),
			Duration:    duration,
		}

		if err != nil {
			check.Status = StatusUnhealthy
			check.Message = err.Error()
			overallStatus = StatusDegraded
		} else {
			check.Status = StatusHealthy
		}

		c.cache[name] = check
		checks = append(checks, *check)
	}

	return overallStatus, checks
}

// HTTPHandler returns an HTTP handler for health checks.
func (c *Checker) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		status, checks := c.Health(ctx)

		response := map[string]interface{}{
			"status": status,
			"checks": checks,
		}

		w.Header().Set("Content-Type", "application/json")
		if status == StatusUnhealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else if status == StatusDegraded {
			w.WriteHeader(http.StatusOK) // Still accept traffic
		} else {
			w.WriteHeader(http.StatusOK)
		}

		json.NewEncoder(w).Encode(response)
	}
}

// LivenessHandler returns a simple liveness probe.
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "alive",
		})
	}
}

// ReadinessHandler returns a readiness probe handler.
func (c *Checker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		status, checks := c.Health(ctx)

		response := map[string]interface{}{
			"status": status,
			"checks": checks,
		}

		w.Header().Set("Content-Type", "application/json")
		if status == StatusUnhealthy || status == StatusDegraded {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		json.NewEncoder(w).Encode(response)
	}
}
