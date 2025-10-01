package mgate

import (
	"context"
	"log"
	"time"
)

// DefaultHandler is a pass-through handler for unknown protocols
type DefaultHandler struct{}

func NewDefaultHandler() *DefaultHandler {
	return &DefaultHandler{}
}

func (h *DefaultHandler) Name() string {
	return "Default"
}

func (h *DefaultHandler) Detect(data []byte) bool {
	return true // Always matches as fallback
}

func (h *DefaultHandler) HandleClientData(ctx context.Context, data []byte, conn ConnectionInfo) ([]byte, bool, error) {
	return data, true, nil
}

func (h *DefaultHandler) HandleServerData(ctx context.Context, data []byte, conn ConnectionInfo) ([]byte, bool, error) {
	return data, true, nil
}

func (h *DefaultHandler) OnConnect(ctx context.Context, conn ConnectionInfo) error {
	log.Printf("[Default] Connection established: %s -> %s", conn.ClientAddr, conn.ServerAddr)
	return nil
}

func (h *DefaultHandler) OnClose(ctx context.Context, conn ConnectionInfo) error {
	duration := time.Since(conn.StartTime)
	log.Printf("[Default] Connection closed: %s (duration: %v)", conn.ClientAddr, duration)
	return nil
}
