// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"context"
	"io"

	"github.com/absmach/mproxy/pkg/handler"
)

// Direction indicates the direction of packet flow.
type Direction int

const (
	// Upstream represents packets flowing from client to backend server.
	Upstream Direction = iota

	// Downstream represents packets flowing from backend server to client.
	Downstream
)

// String returns a string representation of the direction.
func (d Direction) String() string {
	switch d {
	case Upstream:
		return "upstream"
	case Downstream:
		return "downstream"
	default:
		return "unknown"
	}
}

// Parser handles protocol-specific packet processing.
// Implementations are responsible for:
//  1. Reading protocol packets from the reader
//  2. Extracting auth credentials and updating the handler context
//  3. Calling appropriate handler methods (AuthConnect, AuthPublish, etc.)
//  4. Modifying packets if needed (based on handler modifications)
//  5. Writing packets to the writer
//
// Parse is called in a loop for bidirectional streaming. It should:
// - Read exactly one packet/message from r
// - Process and authorize it
// - Write exactly one packet/message to w
// - Return an error to close the connection
// - Return io.EOF for clean connection closure
type Parser interface {
	// Parse reads one packet from r, processes it, and writes to w.
	// The direction indicates packet flow (Upstream or Downstream).
	// The handler h is called for authorization and notifications.
	// The handler context hctx contains connection metadata and is updated
	// with packet-specific credentials (username, password, clientID).
	//
	// For Upstream packets:
	// - Extract credentials and update hctx
	// - Call Auth* methods before forwarding
	// - Call On* methods after successful forwarding
	//
	// For Downstream packets:
	// - Minimal processing (usually just forward)
	// - May call Auth* methods for broker-initiated actions
	//
	// Returns nil if packet was processed successfully.
	// Returns io.EOF for clean connection closure.
	// Returns other errors for abnormal termination.
	Parse(ctx context.Context, r io.Reader, w io.Writer, dir Direction, h handler.Handler, hctx *handler.Context) error
}
