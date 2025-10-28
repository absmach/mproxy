package mqtt

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/absmach/mgate"
)

// MQTTHandler handles MQTT protocol
type MQTTHandler struct {
	logPackets bool
}

func New(logPackets bool) mgate.Handler {
	return &MQTTHandler{logPackets: logPackets}
}

func (h *MQTTHandler) Name() string {
	return "MQTT"
}

func (h *MQTTHandler) Detect(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	// Check for MQTT CONNECT packet (0x10)
	packetType := data[0] >> 4
	return packetType == 1
}

func (h *MQTTHandler) ClientDataHandler() mgate.HandleFunc {
	return func(ctx context.Context, data []byte, conn mgate.ConnectionInfo) ([]byte, bool, error) {
		if h.logPackets && len(data) > 0 {
			packetType := data[0] >> 4
			packetNames := map[byte]string{
				1: "CONNECT", 2: "CONNACK", 3: "PUBLISH", 4: "PUBACK",
				8: "SUBSCRIBE", 9: "SUBACK", 12: "PINGREQ", 13: "PINGRESP", 14: "DISCONNECT",
			}
			name := packetNames[packetType]
			if name == "" {
				name = fmt.Sprintf("UNKNOWN(%d)", packetType)
			}
			log.Printf("[MQTT] %s -> %s", conn.Client.Addr, name)
		}
		return data, true, nil
	}
}

func (h *MQTTHandler) ServerDataHandler() mgate.HandleFunc {
	return func(ctx context.Context, data []byte, conn mgate.ConnectionInfo) ([]byte, bool, error) {
		return data, true, nil
	}
}

func (h *MQTTHandler) OnConnect(ctx context.Context, conn mgate.ConnectionInfo) error {
	log.Printf("[MQTT] Connection established: %s -> %s", conn.Client.Addr, conn.Server.Addr)
	return nil
}

func (h *MQTTHandler) OnDisconnect(ctx context.Context, conn mgate.ConnectionInfo) error {
	duration := time.Since(conn.StartTime)
	log.Printf("[MQTT] Connection closed: %s (duration: %v)", conn.Client.Addr, duration)
	return nil
}
