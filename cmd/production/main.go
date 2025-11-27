// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package main provides a production-ready mProxy deployment example
// with metrics, health checks, circuit breakers, rate limiting, and connection pooling.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/absmach/mproxy/examples/simple"
	"github.com/absmach/mproxy/pkg/breaker"
	"github.com/absmach/mproxy/pkg/health"
	"github.com/absmach/mproxy/pkg/metrics"
	"github.com/absmach/mproxy/pkg/pool"
	"github.com/absmach/mproxy/pkg/proxy"
	"github.com/absmach/mproxy/pkg/ratelimit"
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

// Config holds the application configuration.
type Config struct {
	// Observability
	MetricsPort  int    `env:"METRICS_PORT"  envDefault:"9090"`
	HealthPort   int    `env:"HEALTH_PORT"   envDefault:"8080"`
	LogLevel     string `env:"LOG_LEVEL"     envDefault:"info"`
	LogFormat    string `env:"LOG_FORMAT"    envDefault:"json"`

	// Resource Limits
	MaxConnections  int `env:"MAX_CONNECTIONS"  envDefault:"10000"`
	MaxGoroutines   int `env:"MAX_GOROUTINES"   envDefault:"50000"`

	// Connection Pooling
	PoolMaxIdle     int           `env:"POOL_MAX_IDLE"      envDefault:"100"`
	PoolMaxActive   int           `env:"POOL_MAX_ACTIVE"    envDefault:"1000"`
	PoolIdleTimeout time.Duration `env:"POOL_IDLE_TIMEOUT"  envDefault:"5m"`

	// Circuit Breaker
	BreakerMaxFailures   int           `env:"BREAKER_MAX_FAILURES"   envDefault:"5"`
	BreakerResetTimeout  time.Duration `env:"BREAKER_RESET_TIMEOUT"  envDefault:"60s"`
	BreakerTimeout       time.Duration `env:"BREAKER_TIMEOUT"        envDefault:"30s"`

	// Rate Limiting
	RateLimitCapacity  int64 `env:"RATE_LIMIT_CAPACITY"   envDefault:"100"`
	RateLimitRefill    int64 `env:"RATE_LIMIT_REFILL"     envDefault:"10"`
	GlobalRateCapacity int64 `env:"GLOBAL_RATE_CAPACITY"  envDefault:"10000"`
	GlobalRateRefill   int64 `env:"GLOBAL_RATE_REFILL"    envDefault:"1000"`

	// Timeouts
	ReadTimeout     time.Duration `env:"READ_TIMEOUT"      envDefault:"60s"`
	WriteTimeout    time.Duration `env:"WRITE_TIMEOUT"     envDefault:"60s"`
	IdleTimeout     time.Duration `env:"IDLE_TIMEOUT"      envDefault:"300s"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT"  envDefault:"30s"`

	// MQTT Configuration
	MQTTAddress string `env:"MQTT_ADDRESS"        envDefault:":1884"`
	MQTTTarget  string `env:"MQTT_TARGET"         envDefault:"localhost:1883"`
}

func main() {
	// Load configuration
	cfg := Config{}
	if err := godotenv.Load(); err != nil {
		// .env file is optional
	}
	if err := env.Parse(&cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse config: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	logger := setupLogger(cfg.LogLevel, cfg.LogFormat)
	logger.Info("Starting mProxy in production mode",
		slog.Int("max_connections", cfg.MaxConnections),
		slog.Int("max_goroutines", cfg.MaxGoroutines))

	// Create metrics
	m := metrics.New("mproxy")

	// Start metrics server
	go startMetricsServer(cfg.MetricsPort, logger)

	// Create health checker
	healthChecker := health.NewChecker(10 * time.Second)

	// Add health checks
	healthChecker.Register("goroutines", func(ctx context.Context) error {
		count := runtime.NumGoroutine()
		if count > cfg.MaxGoroutines {
			return fmt.Errorf("too many goroutines: %d > %d", count, cfg.MaxGoroutines)
		}
		// Update metric
		m.GoroutinesActive.WithLabelValues("all").Set(float64(count))
		return nil
	})

	healthChecker.Register("memory", func(ctx context.Context) error {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		m.MemoryAllocated.WithLabelValues("heap").Set(float64(stats.HeapAlloc))
		m.MemoryAllocated.WithLabelValues("sys").Set(float64(stats.Sys))
		return nil
	})

	// Start health server
	go startHealthServer(cfg.HealthPort, healthChecker, logger)

	// Create rate limiters
	perClientLimiter := ratelimit.NewLimiter(cfg.RateLimitCapacity, cfg.RateLimitRefill, 10000)
	globalLimiter := ratelimit.NewTokenBucket(cfg.GlobalRateCapacity, cfg.GlobalRateRefill)

	// Create circuit breaker
	cb := breaker.New(breaker.Config{
		MaxFailures:      cfg.BreakerMaxFailures,
		ResetTimeout:     cfg.BreakerResetTimeout,
		SuccessThreshold: 2,
		Timeout:          cfg.BreakerTimeout,
	})

	// Monitor circuit breaker state changes
	cb.OnStateChange(func(from, to breaker.State) {
		logger.Warn("Circuit breaker state changed",
			slog.String("from", from.String()),
			slog.String("to", to.String()))
		m.CircuitBreakerState.WithLabelValues(cfg.MQTTTarget).Set(float64(to))
		if to == breaker.StateOpen {
			m.CircuitBreakerTrips.WithLabelValues(cfg.MQTTTarget).Inc()
		}
	})

	// Create connection pool
	connPool := pool.New(
		func(ctx context.Context) (net.Conn, error) {
			return net.DialTimeout("tcp", cfg.MQTTTarget, 10*time.Second)
		},
		pool.Config{
			MaxIdle:         cfg.PoolMaxIdle,
			MaxActive:       cfg.PoolMaxActive,
			IdleTimeout:     cfg.PoolIdleTimeout,
			MaxConnLifetime: 30 * time.Minute,
			DialTimeout:     10 * time.Second,
			WaitTimeout:     5 * time.Second,
		},
	)
	defer connPool.Close()

	// Add pool health check
	healthChecker.Register("connection_pool", func(ctx context.Context) error {
		idle, active := connPool.Stats()
		m.BackendActiveConnections.WithLabelValues(cfg.MQTTTarget).Set(float64(active))
		logger.Debug("Connection pool stats",
			slog.Int("idle", idle),
			slog.Int("active", active))
		return nil
	})

	// Create handler with rate limiting wrapper
	baseHandler := simple.New(logger)
	rateLimitedHandler := &RateLimitedHandler{
		handler:          baseHandler,
		perClientLimiter: perClientLimiter,
		globalLimiter:    globalLimiter,
		metrics:          m,
		logger:           logger,
	}

	// Create instrumented handler
	instrumentedHandler := &InstrumentedHandler{
		handler: rateLimitedHandler,
		metrics: m,
		logger:  logger,
	}

	// Start MQTT proxy with production settings
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)

	// Configure MQTT proxy
	mqttProxyConfig := proxy.MQTTConfig{
		Host:            "",
		Port:            cfg.MQTTAddress[1:], // Remove leading ':'
		TargetHost:      "localhost",
		TargetPort:      "1883",
		ShutdownTimeout: cfg.ShutdownTimeout,
		Logger:          logger,
	}

	// Extract port from address
	if cfg.MQTTAddress != "" {
		if _, port, err := net.SplitHostPort(cfg.MQTTAddress); err == nil {
			mqttProxyConfig.Port = port
		} else if cfg.MQTTAddress[0] == ':' {
			mqttProxyConfig.Port = cfg.MQTTAddress[1:]
		}
	}

	// Extract host and port from target
	if cfg.MQTTTarget != "" {
		if host, port, err := net.SplitHostPort(cfg.MQTTTarget); err == nil {
			mqttProxyConfig.TargetHost = host
			mqttProxyConfig.TargetPort = port
		}
	}

	mqttProxy, err := proxy.NewMQTT(mqttProxyConfig, instrumentedHandler)
	if err != nil {
		logger.Error("Failed to create MQTT proxy", slog.String("error", err.Error()))
	} else {
		g.Go(func() error {
			address := net.JoinHostPort(mqttProxyConfig.Host, mqttProxyConfig.Port)
			logger.Info("Starting MQTT proxy",
				slog.String("address", address),
				slog.String("target", cfg.MQTTTarget))
			return mqttProxy.Listen(ctx)
		})
	}

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal
	select {
	case sig := <-quit:
		logger.Info("Received shutdown signal", slog.String("signal", sig.String()))
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}

	// Cancel context to stop all servers
	cancel()

	// Wait for all goroutines with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	done := make(chan error)
	go func() {
		done <- g.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			logger.Error("Shutdown error", slog.String("error", err.Error()))
			os.Exit(1)
		}
		logger.Info("Graceful shutdown completed")
	case <-shutdownCtx.Done():
		logger.Warn("Shutdown timeout exceeded, forcing exit")
		os.Exit(1)
	}
}

// setupLogger creates a structured logger with the specified level and format.
func setupLogger(level, format string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// startMetricsServer starts the Prometheus metrics HTTP server.
func startMetricsServer(port int, logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf(":%d", port)
	logger.Info("Starting metrics server", slog.String("address", addr))

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Metrics server error", slog.String("error", err.Error()))
	}
}

// startHealthServer starts the health check HTTP server.
func startHealthServer(port int, checker *health.Checker, logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", checker.HTTPHandler())
	mux.HandleFunc("/ready", checker.ReadinessHandler())
	mux.HandleFunc("/live", health.LivenessHandler())

	addr := fmt.Sprintf(":%d", port)
	logger.Info("Starting health server", slog.String("address", addr))

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Health server error", slog.String("error", err.Error()))
	}
}
