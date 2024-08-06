// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package forwarder

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/absmach/mproxy"
	mptls "github.com/absmach/mproxy/tls"
	"golang.org/x/sync/errgroup"
)

func Listen(ctx context.Context, name string, config mproxy.Config, passer mproxy.Forwarder, logger *slog.Logger) error {
	l, err := net.Listen("tcp", config.Address)
	if err != nil {
		return err
	}

	if config.TLSConfig != nil {
		l = tls.NewListener(l, config.TLSConfig)
	}
	status := mptls.SecurityStatus(config.TLSConfig)

	logger.Info(fmt.Sprintf("%s Proxy server started at %s%s with %s", name, config.Address, config.PathPrefix, status))

	var server http.Server
	g, ctx := errgroup.WithContext(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc(config.PathPrefix, passer.Forward)
	server.Handler = mux

	g.Go(func() error {
		return server.Serve(l)
	})

	g.Go(func() error {
		<-ctx.Done()
		return server.Close()
	})
	if err := g.Wait(); err != nil {
		logger.Info(fmt.Sprintf("%s Proxy server at %s%s with %s exiting with errors", name, config.Address, config.PathPrefix, status), slog.String("error", err.Error()))
	} else {
		logger.Info(fmt.Sprintf("%s Proxy server at %s%s with %s exiting...", name, config.Address, config.PathPrefix, status))
	}
	return nil
}
