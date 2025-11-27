// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser/coap"
	"github.com/absmach/mproxy/pkg/server/udp"
)

// CoAPConfig holds configuration for CoAP proxy.
type CoAPConfig struct {
	Host            string
	Port            string
	TargetHost      string
	TargetPort      string
	SessionTimeout  time.Duration
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
}

// CoAPProxy coordinates the CoAP UDP server and parser.
type CoAPProxy struct {
	server *udp.Server
}

// NewCoAP creates a new CoAP proxy with UDP server and CoAP parser.
func NewCoAP(cfg CoAPConfig, h handler.Handler) (*CoAPProxy, error) {
	// Create CoAP parser
	parser := &coap.Parser{}

	// Create UDP server config
	address := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	targetAddress := fmt.Sprintf("%s:%s", cfg.TargetHost, cfg.TargetPort)

	serverCfg := udp.Config{
		Address:         address,
		TargetAddress:   targetAddress,
		SessionTimeout:  cfg.SessionTimeout,
		ShutdownTimeout: cfg.ShutdownTimeout,
		Logger:          cfg.Logger,
	}

	// Create UDP server
	server := udp.New(serverCfg, parser, h)

	return &CoAPProxy{
		server: server,
	}, nil
}

// Listen starts the CoAP proxy server and blocks until context is cancelled.
func (p *CoAPProxy) Listen(ctx context.Context) error {
	return p.server.Listen(ctx)
}
