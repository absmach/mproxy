// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Conn is a websocket wrapper that satisfies the net.Conn interface.
// It allows WebSocket connections to be used with stream-based parsers.
type Conn struct {
	*websocket.Conn
	r   io.Reader
	rio sync.Mutex
	wio sync.Mutex
}

// NewConn wraps a websocket.Conn to implement net.Conn interface.
func NewConn(ws *websocket.Conn) net.Conn {
	return &Conn{
		Conn: ws,
	}
}

// SetDeadline sets both the read and write deadlines.
func (c *Conn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	err := c.SetWriteDeadline(t)
	return err
}

// Write writes data to the websocket as a binary message.
func (c *Conn) Write(p []byte) (int, error) {
	c.wio.Lock()
	defer c.wio.Unlock()

	err := c.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Read reads the current websocket frame.
// It handles message framing by reading complete messages.
func (c *Conn) Read(p []byte) (int, error) {
	c.rio.Lock()
	defer c.rio.Unlock()
	for {
		if c.r == nil {
			// Advance to next message
			var err error
			_, c.r, err = c.NextReader()
			if err != nil {
				return 0, err
			}
		}
		n, err := c.r.Read(p)
		if err == io.EOF {
			// At end of message
			c.r = nil
			if n > 0 {
				return n, nil
			}
			// No data read, continue to next message
			continue
		}
		return n, err
	}
}

// Close closes the websocket connection.
func (c *Conn) Close() error {
	return c.Conn.Close()
}
