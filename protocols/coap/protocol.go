package coap

import (
	"context"
	"log"
	"time"

	"github.com/absmach/mgate"
)

// CoAPHandler handles CoAP protocol
type CoAPHandler struct {
	logMessages bool
}

func New(logMessages bool) mgate.Handler {
	return &CoAPHandler{logMessages: logMessages}
}

func (h *CoAPHandler) Name() string {
	return "CoAP"
}

func (h *CoAPHandler) Detect(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	// CoAP version should be 1 (bits 6-7 of first byte)
	version := (data[0] >> 6) & 0x03
	return version == 1
}

func (h *CoAPHandler) ClientDataHandler() mgate.HandleFunc {
	return func(ctx context.Context, data []byte, conn mgate.ConnectionInfo) ([]byte, bool, error) {
		if h.logMessages && len(data) >= 4 {
			msgType := (data[0] >> 4) & 0x03
			typeNames := []string{"CON", "NON", "ACK", "RST"}
			log.Printf("[CoAP] %s -> %s", conn.Client.Addr, typeNames[msgType])
		}
		return data, true, nil
	}
}

func (h *CoAPHandler) ServerDataHandler() mgate.HandleFunc {
	return func(ctx context.Context, data []byte, conn mgate.ConnectionInfo) ([]byte, bool, error) {
		return data, true, nil
	}
}

func (h *CoAPHandler) OnConnect(ctx context.Context, conn mgate.ConnectionInfo) error {
	log.Printf("[CoAP] Connection established: %s -> %s", conn.Client.Addr, conn.Server.Addr)
	return nil
}

func (h *CoAPHandler) OnDisconnect(ctx context.Context, conn mgate.ConnectionInfo) error {
	duration := time.Since(conn.StartTime)
	log.Printf("[CoAP] Connection closed: %s (duration: %v)", conn.Client.Addr, duration)
	return nil
}
