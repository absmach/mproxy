// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser/mqtt"
	"github.com/absmach/mproxy/pkg/server/tcp"
)

// MQTTConfig holds configuration for MQTT proxy.
type MQTTConfig struct {
	Host            string
	Port            string
	TargetHost      string
	TargetPort      string
	TLSConfig       *tls.Config
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
}

// MQTTProxy coordinates the MQTT TCP server and parser.
type MQTTProxy struct {
	server *tcp.Server
}

// NewMQTT creates a new MQTT proxy with TCP server and MQTT parser.
func NewMQTT(cfg MQTTConfig, h handler.Handler) (*MQTTProxy, error) {
	// Create MQTT parser
	parser := &mqtt.Parser{}

	// Create TCP server config
	address := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	targetAddress := fmt.Sprintf("%s:%s", cfg.TargetHost, cfg.TargetPort)

	serverCfg := tcp.Config{
		Address:         address,
		TargetAddress:   targetAddress,
		TLSConfig:       cfg.TLSConfig,
		ShutdownTimeout: cfg.ShutdownTimeout,
		Logger:          cfg.Logger,
	}

	// Create TCP server
	server := tcp.New(serverCfg, parser, h)

	return &MQTTProxy{
		server: server,
	}, nil
}

// Listen starts the MQTT proxy server and blocks until context is cancelled.
func (p *MQTTProxy) Listen(ctx context.Context) error {
	return p.server.Listen(ctx)
}
