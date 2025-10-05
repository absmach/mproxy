package ws

import (
	"bytes"
	"context"
	"log"
	"time"

	"github.com/absmach/mgate"
)

// WebSocketHandler handles WebSocket protocol
type WebSocketHandler struct {
	logFrames bool
}

func New(logFrames bool) *WebSocketHandler {
	return &WebSocketHandler{logFrames: logFrames}
}

func (h *WebSocketHandler) Name() string {
	return "WebSocket"
}

func (h *WebSocketHandler) Detect(data []byte) bool {
	// Check for WebSocket upgrade request
	return bytes.Contains(data, []byte("Upgrade: websocket"))
}

func (h *WebSocketHandler) HandleClientData(ctx context.Context, data []byte, conn mgate.ConnectionInfo) ([]byte, bool, error) {
	if h.logFrames {
		log.Printf("[WebSocket] %s -> Frame (%d bytes)", conn.ClientAddr, len(data))
	}
	return data, true, nil
}

func (h *WebSocketHandler) HandleServerData(ctx context.Context, data []byte, conn mgate.ConnectionInfo) ([]byte, bool, error) {
	return data, true, nil
}

func (h *WebSocketHandler) OnConnect(ctx context.Context, conn mgate.ConnectionInfo) error {
	log.Printf("[WebSocket] Connection established: %s -> %s", conn.ClientAddr, conn.ServerAddr)
	return nil
}

func (h *WebSocketHandler) OnClose(ctx context.Context, conn mgate.ConnectionInfo) error {
	duration := time.Since(conn.StartTime)
	log.Printf("[WebSocket] Connection closed: %s (duration: %v)", conn.ClientAddr, duration)
	return nil
}
