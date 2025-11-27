// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/metrics"
	"github.com/absmach/mproxy/pkg/ratelimit"
)

// RateLimitedHandler wraps a handler with rate limiting.
type RateLimitedHandler struct {
	handler          handler.Handler
	perClientLimiter *ratelimit.Limiter
	globalLimiter    *ratelimit.TokenBucket
	metrics          *metrics.Metrics
	logger           *slog.Logger
}

// AuthConnect implements handler.Handler with rate limiting.
func (h *RateLimitedHandler) AuthConnect(ctx context.Context, hctx *handler.Context) error {
	// Check global rate limit
	if !h.globalLimiter.Allow() {
		h.metrics.RateLimitedRequests.WithLabelValues(hctx.Protocol, "global").Inc()
		h.logger.Warn("Global rate limit exceeded",
			slog.String("remote", hctx.RemoteAddr),
			slog.String("protocol", hctx.Protocol))
		return ratelimit.ErrRateLimitExceeded
	}

	// Check per-client rate limit
	clientID := hctx.RemoteAddr
	if hctx.ClientID != "" {
		clientID = hctx.ClientID
	}

	if !h.perClientLimiter.Allow(clientID) {
		h.metrics.RateLimitedRequests.WithLabelValues(hctx.Protocol, "per_client").Inc()
		h.logger.Warn("Per-client rate limit exceeded",
			slog.String("client", clientID),
			slog.String("protocol", hctx.Protocol))
		return ratelimit.ErrRateLimitExceeded
	}

	return h.handler.AuthConnect(ctx, hctx)
}

// AuthPublish implements handler.Handler with rate limiting.
func (h *RateLimitedHandler) AuthPublish(ctx context.Context, hctx *handler.Context, topic *string, payload *[]byte) error {
	// Could add payload size rate limiting here
	return h.handler.AuthPublish(ctx, hctx, topic, payload)
}

// AuthSubscribe implements handler.Handler.
func (h *RateLimitedHandler) AuthSubscribe(ctx context.Context, hctx *handler.Context, topics *[]string) error {
	return h.handler.AuthSubscribe(ctx, hctx, topics)
}

// OnConnect implements handler.Handler.
func (h *RateLimitedHandler) OnConnect(ctx context.Context, hctx *handler.Context) error {
	return h.handler.OnConnect(ctx, hctx)
}

// OnPublish implements handler.Handler.
func (h *RateLimitedHandler) OnPublish(ctx context.Context, hctx *handler.Context, topic string, payload []byte) error {
	return h.handler.OnPublish(ctx, hctx, topic, payload)
}

// OnSubscribe implements handler.Handler.
func (h *RateLimitedHandler) OnSubscribe(ctx context.Context, hctx *handler.Context, topics []string) error {
	return h.handler.OnSubscribe(ctx, hctx, topics)
}

// OnUnsubscribe implements handler.Handler.
func (h *RateLimitedHandler) OnUnsubscribe(ctx context.Context, hctx *handler.Context, topics []string) error {
	return h.handler.OnUnsubscribe(ctx, hctx, topics)
}

// OnDisconnect implements handler.Handler.
func (h *RateLimitedHandler) OnDisconnect(ctx context.Context, hctx *handler.Context) error {
	return h.handler.OnDisconnect(ctx, hctx)
}

// InstrumentedHandler wraps a handler with metrics instrumentation.
type InstrumentedHandler struct {
	handler handler.Handler
	metrics *metrics.Metrics
	logger  *slog.Logger
}

// AuthConnect implements handler.Handler with metrics.
func (h *InstrumentedHandler) AuthConnect(ctx context.Context, hctx *handler.Context) error {
	start := time.Now()
	h.metrics.AuthAttempts.WithLabelValues(hctx.Protocol, "connect").Inc()

	err := h.handler.AuthConnect(ctx, hctx)

	if err != nil {
		h.metrics.AuthFailures.WithLabelValues(hctx.Protocol, "connect", "unauthorized").Inc()
	}

	duration := time.Since(start).Seconds()
	h.metrics.RequestDuration.WithLabelValues(hctx.Protocol, "connect").Observe(duration)

	return err
}

// AuthPublish implements handler.Handler with metrics.
func (h *InstrumentedHandler) AuthPublish(ctx context.Context, hctx *handler.Context, topic *string, payload *[]byte) error {
	start := time.Now()
	h.metrics.AuthAttempts.WithLabelValues(hctx.Protocol, "publish").Inc()

	if payload != nil {
		h.metrics.RequestSize.WithLabelValues(hctx.Protocol).Observe(float64(len(*payload)))
	}

	err := h.handler.AuthPublish(ctx, hctx, topic, payload)

	if err != nil {
		h.metrics.AuthFailures.WithLabelValues(hctx.Protocol, "publish", "unauthorized").Inc()
	}

	duration := time.Since(start).Seconds()
	h.metrics.RequestDuration.WithLabelValues(hctx.Protocol, "publish").Observe(duration)

	status := "success"
	if err != nil {
		status = "error"
	}
	h.metrics.RequestsTotal.WithLabelValues(hctx.Protocol, "publish", status).Inc()

	return err
}

// AuthSubscribe implements handler.Handler with metrics.
func (h *InstrumentedHandler) AuthSubscribe(ctx context.Context, hctx *handler.Context, topics *[]string) error {
	start := time.Now()
	h.metrics.AuthAttempts.WithLabelValues(hctx.Protocol, "subscribe").Inc()

	err := h.handler.AuthSubscribe(ctx, hctx, topics)

	if err != nil {
		h.metrics.AuthFailures.WithLabelValues(hctx.Protocol, "subscribe", "unauthorized").Inc()
	}

	duration := time.Since(start).Seconds()
	h.metrics.RequestDuration.WithLabelValues(hctx.Protocol, "subscribe").Observe(duration)

	status := "success"
	if err != nil {
		status = "error"
	}
	h.metrics.RequestsTotal.WithLabelValues(hctx.Protocol, "subscribe", status).Inc()

	return err
}

// OnConnect implements handler.Handler with metrics.
func (h *InstrumentedHandler) OnConnect(ctx context.Context, hctx *handler.Context) error {
	h.metrics.ActiveConnections.WithLabelValues(hctx.Protocol, "client").Inc()
	h.metrics.TotalConnections.WithLabelValues(hctx.Protocol, "client", "accepted").Inc()

	return h.handler.OnConnect(ctx, hctx)
}

// OnPublish implements handler.Handler with metrics.
func (h *InstrumentedHandler) OnPublish(ctx context.Context, hctx *handler.Context, topic string, payload []byte) error {
	if hctx.Protocol == "mqtt" {
		h.metrics.MQTTPackets.WithLabelValues("publish", "upstream").Inc()
	}

	return h.handler.OnPublish(ctx, hctx, topic, payload)
}

// OnSubscribe implements handler.Handler with metrics.
func (h *InstrumentedHandler) OnSubscribe(ctx context.Context, hctx *handler.Context, topics []string) error {
	if hctx.Protocol == "mqtt" {
		h.metrics.MQTTPackets.WithLabelValues("subscribe", "upstream").Inc()
	}

	return h.handler.OnSubscribe(ctx, hctx, topics)
}

// OnUnsubscribe implements handler.Handler with metrics.
func (h *InstrumentedHandler) OnUnsubscribe(ctx context.Context, hctx *handler.Context, topics []string) error {
	if hctx.Protocol == "mqtt" {
		h.metrics.MQTTPackets.WithLabelValues("unsubscribe", "upstream").Inc()
	}

	return h.handler.OnUnsubscribe(ctx, hctx, topics)
}

// OnDisconnect implements handler.Handler with metrics.
func (h *InstrumentedHandler) OnDisconnect(ctx context.Context, hctx *handler.Context) error {
	h.metrics.ActiveConnections.WithLabelValues(hctx.Protocol, "client").Dec()

	return h.handler.OnDisconnect(ctx, hctx)
}
