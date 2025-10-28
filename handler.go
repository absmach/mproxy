package mgate

import "context"

// Handler defines the interface for protocol-specific handling.
type Handler interface {
	// Name returns the protocol name.
	Name() string

	// HandleClientData processes data from client to server.
	// Returns modified data and whether to continue proxying.
	HandleClientData(ctx context.Context, data []byte, conn ConnectionInfo) ([]byte, bool, error)

	// HandleServerData processes data from server to client.
	// Returns modified data and whether to continue proxying.
	HandleServerData(ctx context.Context, data []byte, conn ConnectionInfo) ([]byte, bool, error)

	// OnConnect is called when a new connection is established.
	OnConnect(ctx context.Context, conn ConnectionInfo) error

	// OnDisconnect is called when the connection is closed.
	OnDisconnect(ctx context.Context, conn ConnectionInfo) error
}
