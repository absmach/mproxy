package mgate

import (
	"context"
	"fmt"
	"log"
	"time"
)

// MQTTHandler handles MQTT protocol
type MQTTHandler struct {
	logPackets bool
}

func NewMQTTHandler(logPackets bool) *MQTTHandler {
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

func (h *MQTTHandler) HandleClientData(ctx context.Context, data []byte, conn ConnectionInfo) ([]byte, bool, error) {
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
		log.Printf("[MQTT] %s -> %s", conn.ClientAddr, name)
	}
	return data, true, nil
}

func (h *MQTTHandler) HandleServerData(ctx context.Context, data []byte, conn ConnectionInfo) ([]byte, bool, error) {
	return data, true, nil
}

func (h *MQTTHandler) OnConnect(ctx context.Context, conn ConnectionInfo) error {
	log.Printf("[MQTT] Connection established: %s -> %s", conn.ClientAddr, conn.ServerAddr)
	return nil
}

func (h *MQTTHandler) OnClose(ctx context.Context, conn ConnectionInfo) error {
	duration := time.Since(conn.StartTime)
	log.Printf("[MQTT] Connection closed: %s (duration: %v)", conn.ClientAddr, duration)
	return nil
}
