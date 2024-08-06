package mproxy

import (
	"context"
	"net"
	"net/http"
)

// Handler is an interface for mProxy hooks.
type Handler interface {
	// Authorization on client `CONNECT`
	// Each of the params are passed by reference, so that it can be changed
	AuthConnect(ctx context.Context) error

	// Authorization on client `PUBLISH`
	// Topic is passed by reference, so that it can be modified
	AuthPublish(ctx context.Context, topic *string, payload *[]byte) error

	// Authorization on client `SUBSCRIBE`
	// Topics are passed by reference, so that they can be modified
	AuthSubscribe(ctx context.Context, topics *[]string) error

	// After client successfully connected
	Connect(ctx context.Context) error

	// After client successfully published
	Publish(ctx context.Context, topic *string, payload *[]byte) error

	// After client successfully subscribed
	Subscribe(ctx context.Context, topics *[]string) error

	// After client unsubscribed
	Unsubscribe(ctx context.Context, topics *[]string) error

	// Disconnect on connection with client lost
	Disconnect(ctx context.Context) error
}

// Interceptor is an interface for mProxy intercept hook.
type Interceptor interface {
	// Intercept is called on every packet flowing through the mProxy.
	// Packets can be modified before being sent to the broker or the client.
	// The error indicates unsuccessful interception and mProxy is cancelling the packet.
	Intercept(ctx context.Context, pkt interface{}) error
}

// Streamer is used for streaming traffic.
type Streamer interface {
	// Stream streams the traffic between conn1 and conn2 in any direction (or both)
	// providing Handler and Interceptos.
	Stream(ctx context.Context, conn1, conn2 net.Conn) error
}

// Forwarder is used for request-response protocols.
type Forwarder interface {
	// Forward forwards the HTTP request and response for HTTP and
	// WS based protocols.
	Forward(rw http.ResponseWriter, r *http.Request)
}
