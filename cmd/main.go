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

	"github.com/absmach/mproxy"
	"github.com/absmach/mproxy/examples/simple"
	"github.com/absmach/mproxy/pkg/coap"
	"github.com/absmach/mproxy/pkg/http"
	"github.com/absmach/mproxy/pkg/mqtt"
	"github.com/absmach/mproxy/pkg/mqtt/websocket"
	"github.com/absmach/mproxy/pkg/session"
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

	handler := simple.New(logger)

	var interceptor session.Interceptor

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
	mqttProxy := mqtt.NewProxy(mqttConfig, handler, interceptor, logger)
	g.Go(func() error {
		return mqttProxy.Listen(ctx)
	})

	// mProxy server Configuration for MQTT with TLS
	mqttTLSConfig, err := mproxy.NewConfig(env.Options{Prefix: mqttWithTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for MQTT with TLS
	mqttTLSProxy := mqtt.NewProxy(mqttTLSConfig, handler, interceptor, logger)
	g.Go(func() error {
		return mqttTLSProxy.Listen(ctx)
	})

	//  mProxy server Configuration for MQTT with mTLS
	mqttMTLSConfig, err := mproxy.NewConfig(env.Options{Prefix: mqttWithmTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for MQTT with mTLS
	mqttMTlsProxy := mqtt.NewProxy(mqttMTLSConfig, handler, interceptor, logger)
	g.Go(func() error {
		return mqttMTlsProxy.Listen(ctx)
	})

	// mProxy server Configuration for MQTT over Websocket without TLS
	wsConfig, err := mproxy.NewConfig(env.Options{Prefix: mqttWSWithoutTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for MQTT over Websocket without TLS
	wsProxy := websocket.NewProxy(wsConfig, handler, interceptor, logger)
	g.Go(func() error {
		return wsProxy.Listen(ctx)
	})

	// mProxy server Configuration for MQTT over Websocket with TLS
	wsTLSConfig, err := mproxy.NewConfig(env.Options{Prefix: mqttWSWithTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for MQTT over Websocket with TLS
	wsTLSProxy := websocket.NewProxy(wsTLSConfig, handler, interceptor, logger)
	g.Go(func() error {
		return wsTLSProxy.Listen(ctx)
	})

	// mProxy server Configuration for MQTT over Websocket with mTLS
	wsMTLSConfig, err := mproxy.NewConfig(env.Options{Prefix: mqttWSWithmTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for MQTT over Websocket with mTLS
	wsMTLSProxy := websocket.NewProxy(wsMTLSConfig, handler, interceptor, logger)
	g.Go(func() error {
		return wsMTLSProxy.Listen(ctx)
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
		return httpProxy.Listen(ctx)
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
		return httpTLSProxy.Listen(ctx)
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
		return httpMTLSProxy.Listen(ctx)
	})

	// mProxy server Configuration for CoAP without DTLS
	coapConfig, err := mproxy.NewConfig(env.Options{Prefix: coapWithoutDTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for CoAP without DTLS
	coapProxy := coap.NewProxy(coapConfig, handler, logger)
	g.Go(func() error {
		return coapProxy.Listen(ctx)
	})

	// mProxy server Configuration for CoAP with DTLS
	coapDTLSConfig, err := mproxy.NewConfig(env.Options{Prefix: coapWithDTLS})
	if err != nil {
		panic(err)
	}

	// mProxy server for CoAP with DTLS
	coapDTLSProxy := coap.NewProxy(coapDTLSConfig, handler, logger)
	g.Go(func() error {
		return coapDTLSProxy.Listen(ctx)
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
