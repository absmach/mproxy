package http

import (
	"bytes"
	"context"
	"log"
	"strings"
	"time"

	"github.com/absmach/mgate"
)

// HTTPHandler handles HTTP protocol
type HTTPHandler struct {
	logRequests bool
}

func New(logRequests bool) mgate.Handler {
	return &HTTPHandler{logRequests: logRequests}
}

func (h *HTTPHandler) Name() string {
	return "HTTP"
}

func (h *HTTPHandler) Detect(data []byte) bool {
	methods := []string{"GET ", "POST ", "PUT ", "DELETE ", "HEAD ", "OPTIONS ", "PATCH ", "CONNECT "}
	for _, method := range methods {
		if bytes.HasPrefix(data, []byte(method)) {
			return true
		}
	}
	return false
}

func (h *HTTPHandler) HandleClientData(ctx context.Context, data []byte, conn mgate.ConnectionInfo) ([]byte, bool, error) {
	if h.logRequests {
		lines := strings.Split(string(data), "\r\n")
		if len(lines) > 0 {
			log.Printf("[HTTP] %s -> Request: %s", conn.Client.Addr, lines[0])
		}
	}
	return data, true, nil
}

func (h *HTTPHandler) HandleServerData(ctx context.Context, data []byte, conn mgate.ConnectionInfo) ([]byte, bool, error) {
	return data, true, nil
}

func (h *HTTPHandler) OnConnect(ctx context.Context, conn mgate.ConnectionInfo) error {
	log.Printf("[HTTP] Connection established: %s -> %s", conn.Client.Addr, conn.Server.Addr)
	return nil
}

func (h *HTTPHandler) OnClose(ctx context.Context, conn mgate.ConnectionInfo) error {
	duration := time.Since(conn.StartTime)
	log.Printf("[HTTP] Connection closed: %s (duration: %v)", conn.Client.Addr, duration)
	return nil
}
