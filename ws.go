package mgate

import (
	"bytes"
	"context"
	"log"
	"time"
)

// WebSocketHandler handles WebSocket protocol
type WebSocketHandler struct {
	logFrames bool
}

func NewWebSocketHandler(logFrames bool) *WebSocketHandler {
	return &WebSocketHandler{logFrames: logFrames}
}

func (h *WebSocketHandler) Name() string {
	return "WebSocket"
}

func (h *WebSocketHandler) Detect(data []byte) bool {
	// Check for WebSocket upgrade request
	return bytes.Contains(data, []byte("Upgrade: websocket"))
}

func (h *WebSocketHandler) HandleClientData(ctx context.Context, data []byte, conn ConnectionInfo) ([]byte, bool, error) {
	if h.logFrames {
		log.Printf("[WebSocket] %s -> Frame (%d bytes)", conn.ClientAddr, len(data))
	}
	return data, true, nil
}

func (h *WebSocketHandler) HandleServerData(ctx context.Context, data []byte, conn ConnectionInfo) ([]byte, bool, error) {
	return data, true, nil
}

func (h *WebSocketHandler) OnConnect(ctx context.Context, conn ConnectionInfo) error {
	log.Printf("[WebSocket] Connection established: %s -> %s", conn.ClientAddr, conn.ServerAddr)
	return nil
}

func (h *WebSocketHandler) OnClose(ctx context.Context, conn ConnectionInfo) error {
	duration := time.Since(conn.StartTime)
	log.Printf("[WebSocket] Connection closed: %s (duration: %v)", conn.ClientAddr, duration)
	return nil
}
