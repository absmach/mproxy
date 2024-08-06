// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/absmach/mproxy"
	"github.com/absmach/mproxy/examples/simple"
	"github.com/absmach/mproxy/forwarder"
	"github.com/absmach/mproxy/http"
	"github.com/absmach/mproxy/mqtt"
	"github.com/absmach/mproxy/mqtt/websocket"
	"github.com/absmach/mproxy/streamer"
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
)

func main() {
	go func() {
		for {
			time.Sleep(time.Second * 3)
			fmt.Println("RTN", runtime.NumGoroutine())
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fmt.Printf("Alloc = %v MiB", m.Alloc/1024/1024)
			fmt.Printf("\tTotalAlloc = %v MiB", m.TotalAlloc/1024/1024)
			fmt.Printf("\tSys = %v MiB", m.Sys/1024/1024)
			fmt.Printf("\tNumGC = %v\n", m.NumGC)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)

	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(logHandler)

	handler := simple.New(logger)

	var interceptor mproxy.Interceptor

	// Loading .env file to environment
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	// mProxy server Configuration for MQTT without TLS
	mqttConfig, err := mproxy.NewConfig(env.Options{Prefix: mqttWithoutTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for MQTT without TLS
	mqttProxy := mqtt.New(handler, interceptor)

	g.Go(func() error {
		return streamer.Listen(ctx, "MQTT", mqttConfig, mqttProxy, logger)
	})

	// mProxy server Configuration for MQTT with TLS
	mqttTLSConfig, err := mproxy.NewConfig(env.Options{Prefix: mqttWithTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for MQTT with TLS
	mqttTLSProxy := mqtt.New(handler, interceptor)
	g.Go(func() error {
		return streamer.Listen(ctx, "MQTT", mqttTLSConfig, mqttTLSProxy, logger)
	})

	//  mProxy server Configuration for MQTT with mTLS
	mqttMTLSConfig, err := mproxy.NewConfig(env.Options{Prefix: mqttWithmTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for MQTT with mTLS
	mqttMTlsProxy := mqtt.New(handler, interceptor)
	g.Go(func() error {
		return streamer.Listen(ctx, "MQTT", mqttMTLSConfig, mqttMTlsProxy, logger)
	})

	// mProxy server Configuration for MQTT over Websocket without TLS
	wsConfig, err := mproxy.NewConfig(env.Options{Prefix: mqttWSWithoutTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for MQTT over Websocket without TLS
	wsProxy := websocket.New(wsConfig.Target, handler, interceptor, logger)
	g.Go(func() error {
		return forwarder.Listen(ctx, "WS", wsConfig, wsProxy, logger)
	})

	// mProxy server Configuration for MQTT over Websocket with TLS
	wsTLSConfig, err := mproxy.NewConfig(env.Options{Prefix: mqttWSWithTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for MQTT over Websocket with TLS
	wsTLSProxy := websocket.New(wsTLSConfig.Target, handler, interceptor, logger)
	g.Go(func() error {
		return forwarder.Listen(ctx, "WS", wsTLSConfig, wsTLSProxy, logger)
	})

	// mProxy server Configuration for MQTT over Websocket with mTLS
	wsMTLSConfig, err := mproxy.NewConfig(env.Options{Prefix: mqttWSWithmTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for MQTT over Websocket with mTLS
	wsMTLSProxy := websocket.New(wsMTLSConfig.Target, handler, interceptor, logger)
	g.Go(func() error {
		return forwarder.Listen(ctx, "WS", wsMTLSConfig, wsMTLSProxy, logger)
	})

	// mProxy server Configuration for HTTP without TLS
	httpConfig, err := mproxy.NewConfig(env.Options{Prefix: httpWithoutTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for HTTP without TLS
	httpProxy, err := http.NewProxy(httpConfig, handler, logger)
	if err != nil {
		panic(err)
	}
	g.Go(func() error {
		return forwarder.Listen(ctx, "HTTP", httpConfig, httpProxy, logger)
	})

	// mProxy server Configuration for HTTP with TLS
	httpTLSConfig, err := mproxy.NewConfig(env.Options{Prefix: httpWithTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for HTTP with TLS
	httpTLSProxy, err := http.NewProxy(httpTLSConfig, handler, logger)
	if err != nil {
		panic(err)
	}
	g.Go(func() error {
		return forwarder.Listen(ctx, "HTTP", httpTLSConfig, httpTLSProxy, logger)
	})

	// mProxy server Configuration for HTTP with mTLS
	httpMTLSConfig, err := mproxy.NewConfig(env.Options{Prefix: httpWithmTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for HTTP with mTLS
	httpMTLSProxy, err := http.NewProxy(httpMTLSConfig, handler, logger)
	if err != nil {
		panic(err)
	}
	g.Go(func() error {
		return forwarder.Listen(ctx, "HTTP", httpMTLSConfig, httpMTLSProxy, logger)
	})

	g.Go(func() error {
		return StopSignalHandler(ctx, cancel, logger)
	})

	if err := g.Wait(); err != nil {
		logger.Error(fmt.Sprintf("mProxy service terminated with error: %s", err))
	} else {
		logger.Info("mProxy service stopped")
	}
}

func StopSignalHandler(ctx context.Context, cancel context.CancelFunc, logger *slog.Logger) error {
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGABRT)
	select {
	case <-c:
		cancel()
		return nil
	case <-ctx.Done():
		return nil
	}
}
