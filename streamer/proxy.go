// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package streamer

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/absmach/mproxy"
	mptls "github.com/absmach/mproxy/tls"
	"golang.org/x/sync/errgroup"
)

// Listen of the server, this will block.
func Listen(ctx context.Context, name string, config mproxy.Config, streamer mproxy.Streamer, logger *slog.Logger) error {
	l, err := net.Listen("tcp", config.Address)
	if err != nil {
		return err
	}

	if config.TLSConfig != nil {
		l = tls.NewListener(l, config.TLSConfig)
	}
	status := mptls.SecurityStatus(config.TLSConfig)
	logger.Info(fmt.Sprintf("Proxy server started at %s  with %s", config.Address, status))
	g, ctx := errgroup.WithContext(ctx)

	// Acceptor loop
	g.Go(func() error {
		return accept(ctx, streamer, config.Target, l, *logger)
	})

	g.Go(func() error {
		<-ctx.Done()
		return l.Close()
	})
	if err := g.Wait(); err != nil {
		logger.Info(fmt.Sprintf("%s Proxy server at %s with %s exiting with errors", name, config.Address, status), slog.String("error", err.Error()))
	} else {
		logger.Info(fmt.Sprintf("%s Proxy server at %s with %s exiting...", name, config.Address, status))
	}
	return nil
}

func accept(ctx context.Context, streamer mproxy.Streamer, target string, l net.Listener, logger slog.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			in, err := l.Accept()
			if err != nil {
				logger.Warn("Accept error " + err.Error())
				continue
			}
			logger.Info("Accepted new client")
			go func() {
				defer close(in, logger)
				out, err := net.Dial("tcp", target)
				if err != nil {
					logger.Error("Cannot connect to remote broker " + target + " due to: " + err.Error())
					return
				}
				defer close(out, logger)

				if err = streamer.Stream(ctx, in, out); err != io.EOF {
					logger.Warn(err.Error())
				}
			}()
		}
	}
}

func close(conn net.Conn, logger slog.Logger) {
	if err := conn.Close(); err != nil {
		logger.Warn(fmt.Sprintf("Error closing connection %s", err.Error()))
	}
}
