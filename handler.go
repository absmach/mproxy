package mgate

import "context"

// Handler defines the interface for protocol-specific handling.
type Handler interface {
	// Name returns the protocol name.
	Name() string

	// ClientDataHandler processes data from client to server.
	// Returns modified data and whether to continue proxying.
	ClientDataHandler() HandleFunc

	// ServerDataHandler processes data from server to client.
	// Returns modified data and whether to continue proxying.
	ServerDataHandler() HandleFunc

	// OnConnect is called when a new connection is established.
	OnConnect(ctx context.Context, conn ConnectionInfo) error

	// OnDisconnect is called when the connection is closed.
	OnDisconnect(ctx context.Context, conn ConnectionInfo) error
}

type HandleFunc func(ctx context.Context, data []byte, conn ConnectionInfo) ([]byte, bool, error)
