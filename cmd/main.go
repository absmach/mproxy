// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/absmach/mproxy"
	"github.com/absmach/mproxy/examples/simple"
	"github.com/absmach/mproxy/pkg/parser/mqtt"
	"github.com/absmach/mproxy/pkg/proxy"
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"
)

const (
	mqttWithoutTLS = "MPROXY_MQTT_WITHOUT_TLS_"
	mqttWithTLS    = "MPROXY_MQTT_WITH_TLS_"
	mqttWithmTLS   = "MPROXY_MQTT_WITH_MTLS_"

	mqttWSWithoutTLS = "MPROXY_MQTT_WS_WITHOUT_TLS_"
	mqttWSWithTLS    = "MPROXY_MQTT_WS_WITH_TLS_"
	mqttWSWithmTLS   = "MPROXY_MQTT_WS_WITH_MTLS_"

	httpWithoutTLS = "MPROXY_HTTP_WITHOUT_TLS_"
	httpWithTLS    = "MPROXY_HTTP_WITH_TLS_"
	httpWithmTLS   = "MPROXY_HTTP_WITH_MTLS_"

	coapWithoutDTLS = "MPROXY_COAP_WITHOUT_DTLS_"
	coapWithDTLS    = "MPROXY_COAP_WITH_DTLS_"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)

	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(logHandler)

	// Create handler
	handler := simple.New(logger)

	// Load .env file
	if err := godotenv.Load(); err != nil {
		logger.Warn("no .env file found, using environment variables")
	}

	// Start MQTT proxies
	if err := startMQTTProxy(g, ctx, mqttWithoutTLS, handler, logger); err != nil {
		logger.Warn("MQTT without TLS proxy not started", slog.String("error", err.Error()))
	}

	if err := startMQTTProxy(g, ctx, mqttWithTLS, handler, logger); err != nil {
		logger.Warn("MQTT with TLS proxy not started", slog.String("error", err.Error()))
	}

	if err := startMQTTProxy(g, ctx, mqttWithmTLS, handler, logger); err != nil {
		logger.Warn("MQTT with mTLS proxy not started", slog.String("error", err.Error()))
	}

	// Start MQTT over WebSocket proxies
	if err := startWebSocketProxy(g, ctx, mqttWSWithoutTLS, handler, logger); err != nil {
		logger.Warn("MQTT WebSocket without TLS proxy not started", slog.String("error", err.Error()))
	}

	if err := startWebSocketProxy(g, ctx, mqttWSWithTLS, handler, logger); err != nil {
		logger.Warn("MQTT WebSocket with TLS proxy not started", slog.String("error", err.Error()))
	}

	if err := startWebSocketProxy(g, ctx, mqttWSWithmTLS, handler, logger); err != nil {
		logger.Warn("MQTT WebSocket with mTLS proxy not started", slog.String("error", err.Error()))
	}

	// Start HTTP proxies
	if err := startHTTPProxy(g, ctx, httpWithoutTLS, handler, logger); err != nil {
		logger.Warn("HTTP without TLS proxy not started", slog.String("error", err.Error()))
	}

	if err := startHTTPProxy(g, ctx, httpWithTLS, handler, logger); err != nil {
		logger.Warn("HTTP with TLS proxy not started", slog.String("error", err.Error()))
	}

	if err := startHTTPProxy(g, ctx, httpWithmTLS, handler, logger); err != nil {
		logger.Warn("HTTP with mTLS proxy not started", slog.String("error", err.Error()))
	}

	// Start CoAP proxies
	if err := startCoAPProxy(g, ctx, coapWithoutDTLS, handler, logger); err != nil {
		logger.Warn("CoAP without DTLS proxy not started", slog.String("error", err.Error()))
	}

	if err := startCoAPProxy(g, ctx, coapWithDTLS, handler, logger); err != nil {
		logger.Warn("CoAP with DTLS proxy not started", slog.String("error", err.Error()))
	}

	// Signal handler
	g.Go(func() error {
		return StopSignalHandler(ctx, cancel, logger)
	})

	if err := g.Wait(); err != nil {
		logger.Error(fmt.Sprintf("mProxy service terminated with error: %s", err))
	} else {
		logger.Info("mProxy service stopped")
	}
}

func startMQTTProxy(g *errgroup.Group, ctx context.Context, envPrefix string, handler *simple.Handler, logger *slog.Logger) error {
	cfg, err := mproxy.NewConfig(env.Options{Prefix: envPrefix})
	if err != nil {
		return err
	}

	// Set default values based on the server type
	if cfg.Port == "" {
		switch envPrefix {
		case mqttWithoutTLS:
			cfg.Port = "1884"
		case mqttWithTLS:
			cfg.Port = "8883"
		case mqttWithmTLS:
			cfg.Port = "8884"
		default:
			return fmt.Errorf("port not configured")
		}
	}

	if cfg.TargetHost == "" {
		cfg.TargetHost = "localhost"
	}

	if cfg.TargetPort == "" {
		cfg.TargetPort = "1883"
	}

	mqttCfg := proxy.MQTTConfig{
		Host:            cfg.Host,
		Port:            cfg.Port,
		TargetHost:      cfg.TargetHost,
		TargetPort:      cfg.TargetPort,
		TLSConfig:       cfg.TLSConfig,
		ShutdownTimeout: 30 * time.Second,
		Logger:          logger,
	}

	mqttProxy, err := proxy.NewMQTT(mqttCfg, handler)
	if err != nil {
		return err
	}

	g.Go(func() error {
		return mqttProxy.Listen(ctx)
	})

	logger.Info("MQTT proxy started", slog.String("prefix", envPrefix), slog.String("port", cfg.Port))
	return nil
}

func startWebSocketProxy(g *errgroup.Group, ctx context.Context, envPrefix string, handler *simple.Handler, logger *slog.Logger) error {
	cfg, err := mproxy.NewConfig(env.Options{Prefix: envPrefix})
	if err != nil {
		return err
	}

	// Set default values based on the server type
	if cfg.Port == "" {
		switch envPrefix {
		case mqttWSWithoutTLS:
			cfg.Port = "8083"
		case mqttWSWithTLS:
			cfg.Port = "8084"
		case mqttWSWithmTLS:
			cfg.Port = "8085"
		default:
			return fmt.Errorf("port not configured")
		}
	}

	if cfg.TargetHost == "" {
		cfg.TargetHost = "localhost"
	}

	if cfg.TargetPort == "" {
		cfg.TargetPort = "8000"
	}

	// Build WebSocket target URL
	protocol := cfg.TargetProtocol
	if protocol == "" {
		protocol = "ws"
	}
	targetURL := fmt.Sprintf("%s://%s:%s%s", protocol, cfg.TargetHost, cfg.TargetPort, cfg.TargetPath)

	wsCfg := proxy.WebSocketConfig{
		Host:             cfg.Host,
		Port:             cfg.Port,
		TargetURL:        targetURL,
		UnderlyingParser: &mqtt.Parser{}, // MQTT over WebSocket
		TLSConfig:        cfg.TLSConfig,
		ShutdownTimeout:  30 * time.Second,
		Logger:           logger,
	}

	wsProxy, err := proxy.NewWebSocket(wsCfg, handler)
	if err != nil {
		return err
	}

	g.Go(func() error {
		return wsProxy.Listen(ctx)
	})

	logger.Info("WebSocket proxy started", slog.String("prefix", envPrefix), slog.String("port", cfg.Port))
	return nil
}

func startHTTPProxy(g *errgroup.Group, ctx context.Context, envPrefix string, handler *simple.Handler, logger *slog.Logger) error {
	cfg, err := mproxy.NewConfig(env.Options{Prefix: envPrefix})
	if err != nil {
		return err
	}

	// Set default values based on the server type
	if cfg.Port == "" {
		switch envPrefix {
		case httpWithoutTLS:
			cfg.Port = "8086"
		case httpWithTLS:
			cfg.Port = "8087"
		case httpWithmTLS:
			cfg.Port = "8088"
		default:
			return fmt.Errorf("port not configured")
		}
	}

	if cfg.TargetHost == "" {
		cfg.TargetHost = "localhost"
	}

	if cfg.TargetPort == "" {
		cfg.TargetPort = "8888"
	}

	// Build HTTP target URL
	protocol := cfg.TargetProtocol
	if protocol == "" {
		protocol = "http"
	}
	targetURL := fmt.Sprintf("%s://%s:%s", protocol, cfg.TargetHost, cfg.TargetPort)

	httpCfg := proxy.HTTPConfig{
		Host:            cfg.Host,
		Port:            cfg.Port,
		TargetURL:       targetURL,
		TLSConfig:       cfg.TLSConfig,
		ShutdownTimeout: 30 * time.Second,
		Logger:          logger,
	}

	httpProxy, err := proxy.NewHTTP(httpCfg, handler)
	if err != nil {
		return err
	}

	g.Go(func() error {
		return httpProxy.Listen(ctx)
	})

	logger.Info("HTTP proxy started", slog.String("prefix", envPrefix), slog.String("port", cfg.Port))
	return nil
}

func startCoAPProxy(g *errgroup.Group, ctx context.Context, envPrefix string, handler *simple.Handler, logger *slog.Logger) error {
	cfg, err := mproxy.NewConfig(env.Options{Prefix: envPrefix})
	if err != nil {
		return err
	}

	// Set default values based on the server type
	if cfg.Port == "" {
		switch envPrefix {
		case coapWithoutDTLS:
			cfg.Port = "5682"
		case coapWithDTLS:
			cfg.Port = "5684"
		default:
			return fmt.Errorf("port not configured")
		}
	}

	if cfg.TargetHost == "" {
		cfg.TargetHost = "localhost"
	}

	if cfg.TargetPort == "" {
		cfg.TargetPort = "5683"
	}

	coapCfg := proxy.CoAPConfig{
		Host:            cfg.Host,
		Port:            cfg.Port,
		TargetHost:      cfg.TargetHost,
		TargetPort:      cfg.TargetPort,
		SessionTimeout:  30 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		Logger:          logger,
	}

	coapProxy, err := proxy.NewCoAP(coapCfg, handler)
	if err != nil {
		return err
	}

	g.Go(func() error {
		return coapProxy.Listen(ctx)
	})

	logger.Info("CoAP proxy started", slog.String("prefix", envPrefix), slog.String("port", cfg.Port))
	return nil
}

func StopSignalHandler(ctx context.Context, cancel context.CancelFunc, logger *slog.Logger) error {
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGABRT)
	select {
	case <-c:
		logger.Info("received shutdown signal")
		cancel()
		return nil
	case <-ctx.Done():
		return nil
	}
}
