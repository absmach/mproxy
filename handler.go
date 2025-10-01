package mgate

import "context"

// ProtocolHandler defines the interface for protocol-specific handling
type ProtocolHandler interface {
	// Name returns the protocol name
	Name() string

	// Detect checks if this handler can handle the connection based on initial data
	Detect(data []byte) bool

	// HandleClientData processes data from client to server
	// Returns modified data and whether to continue proxying
	HandleClientData(ctx context.Context, data []byte, conn ConnectionInfo) ([]byte, bool, error)

	// HandleServerData processes data from server to client
	// Returns modified data and whether to continue proxying
	HandleServerData(ctx context.Context, data []byte, conn ConnectionInfo) ([]byte, bool, error)

	// OnConnect is called when a new connection is established
	OnConnect(ctx context.Context, conn ConnectionInfo) error

	// OnClose is called when the connection is closed
	OnClose(ctx context.Context, conn ConnectionInfo) error
}
