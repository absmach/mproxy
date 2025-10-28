package nats

import (
	"bytes"
	"context"
	"log"
	"time"

	"github.com/absmach/mgate"
)

// NATSHandler handles NATS protocol
type NATSHandler struct {
	logOps bool
}

func New(logOps bool) mgate.Handler {
	return &NATSHandler{logOps: logOps}
}

func (h *NATSHandler) Name() string {
	return "NATS"
}

func (h *NATSHandler) Detect(data []byte) bool {
	// NATS server sends "INFO {..." as first message
	// NATS client sends "CONNECT {..." as first message
	if len(data) < 4 {
		return false
	}
	return bytes.HasPrefix(data, []byte("INFO")) ||
		bytes.HasPrefix(data, []byte("CONNECT")) ||
		bytes.HasPrefix(data, []byte("PUB ")) ||
		bytes.HasPrefix(data, []byte("SUB ")) ||
		bytes.HasPrefix(data, []byte("PING"))
}

func (h *NATSHandler) HandleClientData() mgate.HandleFunc {
	return func(ctx context.Context, data []byte, conn mgate.ConnectionInfo) ([]byte, bool, error) {
		if h.logOps {
			h.logNATSOperation(data, conn.Client.Addr, "->")
		}
		return data, true, nil
	}
}

func (h *NATSHandler) HandleServerData() mgate.HandleFunc {
	return func(ctx context.Context, data []byte, conn mgate.ConnectionInfo) ([]byte, bool, error) {
		if h.logOps {
			h.logNATSOperation(data, "Server", "<-")
		}
		return data, true, nil
	}
}

func (h *NATSHandler) logNATSOperation(data []byte, addr string, direction string) {
	lines := bytes.Split(data, []byte("\r\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		// Parse NATS protocol operations
		parts := bytes.SplitN(line, []byte(" "), 2)
		if len(parts) == 0 {
			continue
		}

		op := string(parts[0])
		switch op {
		case "INFO", "CONNECT":
			log.Printf("[NATS] %s %s %s", addr, direction, op)
		case "PUB":
			if len(parts) > 1 {
				subParts := bytes.SplitN(parts[1], []byte(" "), 3)
				if len(subParts) >= 2 {
					subject := string(subParts[0])
					log.Printf("[NATS] %s %s PUB %s", addr, direction, subject)
				}
			}
		case "SUB":
			if len(parts) > 1 {
				subParts := bytes.SplitN(parts[1], []byte(" "), 3)
				if len(subParts) >= 1 {
					subject := string(subParts[0])
					log.Printf("[NATS] %s %s SUB %s", addr, direction, subject)
				}
			}
		case "UNSUB":
			log.Printf("[NATS] %s %s UNSUB", addr, direction)
		case "MSG":
			if len(parts) > 1 {
				subParts := bytes.SplitN(parts[1], []byte(" "), 4)
				if len(subParts) >= 1 {
					subject := string(subParts[0])
					log.Printf("[NATS] %s %s MSG %s", addr, direction, subject)
				}
			}
		case "PING":
			log.Printf("[NATS] %s %s PING", addr, direction)
		case "PONG":
			log.Printf("[NATS] %s %s PONG", addr, direction)
		case "+OK":
			log.Printf("[NATS] %s %s +OK", addr, direction)
		case "-ERR":
			if len(parts) > 1 {
				log.Printf("[NATS] %s %s -ERR %s", addr, direction, string(parts[1]))
			}
		}
	}
}

func (h *NATSHandler) OnConnect(ctx context.Context, conn mgate.ConnectionInfo) error {
	log.Printf("[NATS] Connection established: %s -> %s", conn.Client.Addr, conn.Server.Addr)
	return nil
}

func (h *NATSHandler) OnDisconnect(ctx context.Context, conn mgate.ConnectionInfo) error {
	duration := time.Since(conn.StartTime)
	log.Printf("[NATS] Connection closed: %s (duration: %v)", conn.Client.Addr, duration)
	return nil
}
