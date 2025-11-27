// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package metrics provides Prometheus instrumentation for mProxy.
package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	once sync.Once
	reg  *prometheus.Registry
)

// Metrics holds all Prometheus metrics for mProxy.
type Metrics struct {
	// Connection metrics
	ActiveConnections *prometheus.GaugeVec
	TotalConnections  *prometheus.CounterVec
	ConnectionErrors  *prometheus.CounterVec
	ConnectionDuration *prometheus.HistogramVec

	// Request metrics
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RequestSize      *prometheus.HistogramVec
	ResponseSize     *prometheus.HistogramVec

	// Backend metrics
	BackendRequestsTotal    *prometheus.CounterVec
	BackendErrors           *prometheus.CounterVec
	BackendDuration         *prometheus.HistogramVec
	BackendActiveConnections *prometheus.GaugeVec

	// Circuit breaker metrics
	CircuitBreakerState *prometheus.GaugeVec
	CircuitBreakerTrips *prometheus.CounterVec

	// Rate limiter metrics
	RateLimitedRequests *prometheus.CounterVec

	// Resource metrics
	GoroutinesActive *prometheus.GaugeVec
	MemoryAllocated  *prometheus.GaugeVec

	// Auth metrics
	AuthAttempts *prometheus.CounterVec
	AuthFailures *prometheus.CounterVec

	// Protocol-specific metrics
	MQTTPackets     *prometheus.CounterVec
	HTTPRequests    *prometheus.CounterVec
	CoAPMessages    *prometheus.CounterVec
	WebSocketFrames *prometheus.CounterVec
}

// New creates a new Metrics instance with all counters, gauges, and histograms.
func New(namespace string) *Metrics {
	if namespace == "" {
		namespace = "mproxy"
	}

	m := &Metrics{
		ActiveConnections: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "active_connections",
				Help:      "Number of currently active connections",
			},
			[]string{"protocol", "type"},
		),
		TotalConnections: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "connections_total",
				Help:      "Total number of connections",
			},
			[]string{"protocol", "type", "status"},
		),
		ConnectionErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "connection_errors_total",
				Help:      "Total number of connection errors",
			},
			[]string{"protocol", "type", "error_type"},
		),
		ConnectionDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "connection_duration_seconds",
				Help:      "Connection duration in seconds",
				Buckets:   []float64{.01, .05, .1, .5, 1, 5, 10, 30, 60, 300, 600},
			},
			[]string{"protocol", "type"},
		),
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "requests_total",
				Help:      "Total number of requests processed",
			},
			[]string{"protocol", "method", "status"},
		),
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "request_duration_seconds",
				Help:      "Request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"protocol", "method"},
		),
		RequestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "request_size_bytes",
				Help:      "Request size in bytes",
				Buckets:   []float64{100, 1000, 10000, 100000, 1000000, 10000000},
			},
			[]string{"protocol"},
		),
		ResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "response_size_bytes",
				Help:      "Response size in bytes",
				Buckets:   []float64{100, 1000, 10000, 100000, 1000000, 10000000},
			},
			[]string{"protocol"},
		),
		BackendRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "backend_requests_total",
				Help:      "Total number of backend requests",
			},
			[]string{"backend", "status"},
		),
		BackendErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "backend_errors_total",
				Help:      "Total number of backend errors",
			},
			[]string{"backend", "error_type"},
		),
		BackendDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "backend_duration_seconds",
				Help:      "Backend request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"backend"},
		),
		BackendActiveConnections: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "backend_active_connections",
				Help:      "Number of active backend connections",
			},
			[]string{"backend"},
		),
		CircuitBreakerState: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "circuit_breaker_state",
				Help:      "Circuit breaker state (0=closed, 1=half_open, 2=open)",
			},
			[]string{"backend"},
		),
		CircuitBreakerTrips: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "circuit_breaker_trips_total",
				Help:      "Total number of circuit breaker trips",
			},
			[]string{"backend"},
		),
		RateLimitedRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "rate_limited_requests_total",
				Help:      "Total number of rate limited requests",
			},
			[]string{"protocol", "limiter_type"},
		),
		GoroutinesActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "goroutines_active",
				Help:      "Number of active goroutines by component",
			},
			[]string{"component"},
		),
		MemoryAllocated: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "memory_allocated_bytes",
				Help:      "Memory allocated in bytes",
			},
			[]string{"type"},
		),
		AuthAttempts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "auth_attempts_total",
				Help:      "Total number of authentication attempts",
			},
			[]string{"protocol", "type"},
		),
		AuthFailures: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "auth_failures_total",
				Help:      "Total number of authentication failures",
			},
			[]string{"protocol", "type", "reason"},
		),
		MQTTPackets: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "mqtt_packets_total",
				Help:      "Total number of MQTT packets",
			},
			[]string{"packet_type", "direction"},
		),
		HTTPRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		CoAPMessages: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "coap_messages_total",
				Help:      "Total number of CoAP messages",
			},
			[]string{"method", "code"},
		),
		WebSocketFrames: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "websocket_frames_total",
				Help:      "Total number of WebSocket frames",
			},
			[]string{"frame_type", "direction"},
		),
	}

	return m
}

// ObserveConnection tracks a connection lifecycle.
func (m *Metrics) ObserveConnection(protocol, connType string, f func() error) error {
	m.ActiveConnections.WithLabelValues(protocol, connType).Inc()
	defer m.ActiveConnections.WithLabelValues(protocol, connType).Dec()

	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		m.ConnectionDuration.WithLabelValues(protocol, connType).Observe(duration)
	}()

	err := f()
	status := "success"
	if err != nil {
		status = "error"
	}
	m.TotalConnections.WithLabelValues(protocol, connType, status).Inc()

	return err
}

// ObserveRequest tracks a request lifecycle.
func (m *Metrics) ObserveRequest(protocol, method string, f func() (string, error)) error {
	start := time.Now()

	status, err := f()
	duration := time.Since(start).Seconds()

	m.RequestsTotal.WithLabelValues(protocol, method, status).Inc()
	m.RequestDuration.WithLabelValues(protocol, method).Observe(duration)

	return err
}
