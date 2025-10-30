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

	"github.com/absmach/mgate"
	"github.com/absmach/mgate/examples/simple"
	"github.com/absmach/mgate/pkg/coap"
	"github.com/absmach/mgate/pkg/http"
	"github.com/absmach/mgate/pkg/mqtt"
	"github.com/absmach/mgate/pkg/mqtt/websocket"
	"github.com/absmach/mgate/pkg/session"
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"
)

const (
	mqttWithoutTLS = "MGATE_MQTT_WITHOUT_TLS_"
	mqttWithTLS    = "MGATE_MQTT_WITH_TLS_"
	mqttWithmTLS   = "MGATE_MQTT_WITH_MTLS_"

	mqttWSWithoutTLS = "MGATE_MQTT_WS_WITHOUT_TLS_"
	mqttWSWithTLS    = "MGATE_MQTT_WS_WITH_TLS_"
	mqttWSWithmTLS   = "MGATE_MQTT_WS_WITH_MTLS_"

	httpWithoutTLS = "MGATE_HTTP_WITHOUT_TLS_"
	httpWithTLS    = "MGATE_HTTP_WITH_TLS_"
	httpWithmTLS   = "MGATE_HTTP_WITH_MTLS_"

	coapWithoutDTLS = "MGATE_COAP_WITHOUT_DTLS_"
	coapWithDTLS    = "MGATE_COAP_WITH_DTLS_"
	coapWithmDTLS   = "MGATE_COAP_WITH_MDTLS_"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)

	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(logHandler)

	handler := simple.New(logger)

	var beforeHandler, afterHandler session.Interceptor

	// Loading .env file to environment
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	// mGate server Configuration for MQTT without TLS
	mqttConfig, err := mgate.NewConfig(env.Options{Prefix: mqttWithoutTLS})
	if err != nil {
		panic(err)
	}

	// mGate server for MQTT without TLS
	mqttProxy := mqtt.New(mqttConfig, handler, beforeHandler, afterHandler, logger)
	g.Go(func() error {
		return mqttProxy.Listen(ctx)
	})

	// mGate server Configuration for MQTT with TLS
	mqttTLSConfig, err := mgate.NewConfig(env.Options{Prefix: mqttWithTLS})
	if err != nil {
		panic(err)
	}

	// mGate server for MQTT with TLS
	mqttTLSProxy := mqtt.New(mqttTLSConfig, handler, beforeHandler, afterHandler, logger)
	g.Go(func() error {
		return mqttTLSProxy.Listen(ctx)
	})

	//  mGate server Configuration for MQTT with mTLS
	mqttMTLSConfig, err := mgate.NewConfig(env.Options{Prefix: mqttWithmTLS})
	if err != nil {
		panic(err)
	}

	// mGate server for MQTT with mTLS
	mqttMTlsProxy := mqtt.New(mqttMTLSConfig, handler, beforeHandler, afterHandler, logger)
	g.Go(func() error {
		return mqttMTlsProxy.Listen(ctx)
	})

	// mGate server Configuration for MQTT over Websocket without TLS
	wsConfig, err := mgate.NewConfig(env.Options{Prefix: mqttWSWithoutTLS})
	if err != nil {
		panic(err)
	}

	// mGate server for MQTT over Websocket without TLS
	wsProxy := websocket.New(wsConfig, handler, beforeHandler, afterHandler, logger)
	g.Go(func() error {
		return wsProxy.Listen(ctx)
	})

	// mGate server Configuration for MQTT over Websocket with TLS
	wsTLSConfig, err := mgate.NewConfig(env.Options{Prefix: mqttWSWithTLS})
	if err != nil {
		panic(err)
	}

	// mGate server for MQTT over Websocket with TLS
	wsTLSProxy := websocket.New(wsTLSConfig, handler, beforeHandler, afterHandler, logger)
	g.Go(func() error {
		return wsTLSProxy.Listen(ctx)
	})

	// mGate server Configuration for MQTT over Websocket with mTLS
	wsMTLSConfig, err := mgate.NewConfig(env.Options{Prefix: mqttWSWithmTLS})
	if err != nil {
		panic(err)
	}

	// mGate server for MQTT over Websocket with mTLS
	wsMTLSProxy := websocket.New(wsMTLSConfig, handler, beforeHandler, afterHandler, logger)
	g.Go(func() error {
		return wsMTLSProxy.Listen(ctx)
	})

	// mGate server Configuration for HTTP without TLS
	httpConfig, err := mgate.NewConfig(env.Options{Prefix: httpWithoutTLS})
	if err != nil {
		panic(err)
	}

	// mGate server for HTTP without TLS
	httpProxy, err := http.NewProxy(httpConfig, handler, logger, []string{}, []string{})
	if err != nil {
		panic(err)
	}
	g.Go(func() error {
		return httpProxy.Listen(ctx)
	})

	// mGate server Configuration for HTTP with TLS
	httpTLSConfig, err := mgate.NewConfig(env.Options{Prefix: httpWithTLS})
	if err != nil {
		panic(err)
	}

	// mGate server for HTTP with TLS
	httpTLSProxy, err := http.NewProxy(httpTLSConfig, handler, logger, []string{}, []string{})
	if err != nil {
		panic(err)
	}
	g.Go(func() error {
		return httpTLSProxy.Listen(ctx)
	})

	// mGate server Configuration for HTTP with mTLS
	httpMTLSConfig, err := mgate.NewConfig(env.Options{Prefix: httpWithmTLS})
	if err != nil {
		panic(err)
	}

	// mGate server for HTTP with mTLS
	httpMTLSProxy, err := http.NewProxy(httpMTLSConfig, handler, logger, []string{}, []string{})
	if err != nil {
		panic(err)
	}
	g.Go(func() error {
		return httpMTLSProxy.Listen(ctx)
	})

	// mGate server Configuration for CoAP without DTLS
	coapConfig, err := mgate.NewConfig(env.Options{Prefix: coapWithoutDTLS})
	if err != nil {
		panic(err)
	}

	// mGate server for CoAP without DTLS
	coapProxy := coap.NewProxy(coapConfig, handler, logger)
	g.Go(func() error {
		return coapProxy.Listen(ctx)
	})

	// mGate server Configuration for CoAP with DTLS
	coapDTLSConfig, err := mgate.NewConfig(env.Options{Prefix: coapWithDTLS})
	if err != nil {
		panic(err)
	}

	// mGate server for CoAP with DTLS
	coapDTLSProxy := coap.NewProxy(coapDTLSConfig, handler, logger)
	g.Go(func() error {
		return coapDTLSProxy.Listen(ctx)
	})

	// mGate server Configuration for CoAP with mDTLS
	coapmDTLSConfig, err := mgate.NewConfig(env.Options{Prefix: coapWithmDTLS})
	if err != nil {
		panic(err)
	}

	// mGate server for CoAP with mDTLS
	coapmDTLSProxy := coap.NewProxy(coapmDTLSConfig, handler, logger)
	g.Go(func() error {
		return coapmDTLSProxy.Listen(ctx)
	})

	g.Go(func() error {
		return StopSignalHandler(ctx, cancel, logger)
	})

	if err := g.Wait(); err != nil {
		logger.Error(fmt.Sprintf("mGate service terminated with error: %s", err))
	} else {
		logger.Info("mGate service stopped")
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
