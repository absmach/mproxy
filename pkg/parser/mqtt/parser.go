// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package mqtt

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser"
	"github.com/eclipse/paho.mqtt.golang/packets"
)

var (
	// ErrUnauthorized is returned when authorization fails.
	ErrUnauthorized = errors.New("unauthorized")
)

// Parser implements the parser.Parser interface for MQTT protocol.
type Parser struct{}

var _ parser.Parser = (*Parser)(nil)

// Parse reads one MQTT packet from r, processes it, and writes to w.
// It implements bidirectional packet inspection and modification:
// - Upstream (client→backend): Extracts auth, authorizes, may modify
// - Downstream (backend→client): Usually just forwards, may authorize broker actions
func (p *Parser) Parse(ctx context.Context, r io.Reader, w io.Writer, dir parser.Direction, h handler.Handler, hctx *handler.Context) error {
	// Read MQTT packet
	pkt, err := packets.ReadPacket(r)
	if err != nil {
		return err
	}

	// Process based on direction
	if dir == parser.Upstream {
		// Client → Backend
		if err := p.handleUpstream(ctx, pkt, h, hctx); err != nil {
			return err
		}
	} else {
		// Backend → Client
		if err := p.handleDownstream(ctx, pkt, h, hctx); err != nil {
			return err
		}
	}

	// Write packet to destination
	if err := pkt.Write(w); err != nil {
		return fmt.Errorf("failed to write packet: %w", err)
	}

	return nil
}

// handleUpstream processes upstream (client→backend) packets.
func (p *Parser) handleUpstream(ctx context.Context, pkt packets.ControlPacket, h handler.Handler, hctx *handler.Context) error {
	switch packet := pkt.(type) {
	case *packets.ConnectPacket:
		return p.handleConnect(ctx, packet, h, hctx)

	case *packets.PublishPacket:
		return p.handlePublish(ctx, packet, h, hctx)

	case *packets.SubscribePacket:
		return p.handleSubscribe(ctx, packet, h, hctx)

	case *packets.UnsubscribePacket:
		return p.handleUnsubscribe(ctx, packet, h, hctx)

	case *packets.DisconnectPacket:
		return p.handleDisconnect(ctx, h, hctx)

	default:
		// Other packets (PINGREQ, PUBACK, PUBREC, PUBREL, PUBCOMP, etc.) are forwarded as-is
		return nil
	}
}

// handleDownstream processes downstream (backend→client) packets.
// We may want to authorize broker-initiated publishes here.
func (p *Parser) handleDownstream(ctx context.Context, pkt packets.ControlPacket, h handler.Handler, hctx *handler.Context) error {
	switch packet := pkt.(type) {
	case *packets.PublishPacket:
		// Broker-initiated publish (retained message, subscription delivery)
		// Treat as subscribe authorization
		topic := packet.TopicName
		topics := []string{topic}
		if err := h.AuthSubscribe(ctx, hctx, &topics); err != nil {
			return err
		}
		// Update topic if modified
		if len(topics) > 0 {
			packet.TopicName = topics[0]
		}
		return nil

	default:
		// Other packets are forwarded as-is
		return nil
	}
}

// handleConnect processes MQTT CONNECT packets.
func (p *Parser) handleConnect(ctx context.Context, packet *packets.ConnectPacket, h handler.Handler, hctx *handler.Context) error {
	// Extract credentials from CONNECT packet
	hctx.ClientID = packet.ClientIdentifier
	hctx.Username = packet.Username
	hctx.Password = packet.Password

	// Update protocol
	hctx.Protocol = "mqtt"

	// Authorize connection
	if err := h.AuthConnect(ctx, hctx); err != nil {
		// TODO: Send CONNACK with appropriate return code
		return fmt.Errorf("connection authorization failed: %w", err)
	}

	// Update packet with potentially modified credentials
	packet.ClientIdentifier = hctx.ClientID
	packet.Username = hctx.Username
	packet.Password = hctx.Password

	// Notify successful connection
	if err := h.OnConnect(ctx, hctx); err != nil {
		// Log but don't fail the connection
		return nil
	}

	return nil
}

// handlePublish processes MQTT PUBLISH packets.
func (p *Parser) handlePublish(ctx context.Context, packet *packets.PublishPacket, h handler.Handler, hctx *handler.Context) error {
	topic := packet.TopicName
	payload := packet.Payload

	// Authorize publish (allows modification)
	if err := h.AuthPublish(ctx, hctx, &topic, &payload); err != nil {
		return fmt.Errorf("publish authorization failed: %w", err)
	}

	// Update packet with potentially modified topic/payload
	packet.TopicName = topic
	packet.Payload = payload

	// Notify successful publish (immutable copies)
	if err := h.OnPublish(ctx, hctx, topic, payload); err != nil {
		// Log but don't fail the publish
		return nil
	}

	return nil
}

// handleSubscribe processes MQTT SUBSCRIBE packets.
func (p *Parser) handleSubscribe(ctx context.Context, packet *packets.SubscribePacket, h handler.Handler, hctx *handler.Context) error {
	// Extract topics
	topics := make([]string, len(packet.Topics))
	copy(topics, packet.Topics)

	// Authorize subscription (allows modification)
	if err := h.AuthSubscribe(ctx, hctx, &topics); err != nil {
		return fmt.Errorf("subscribe authorization failed: %w", err)
	}

	// Update packet with potentially modified topics
	if len(topics) != len(packet.Topics) {
		// Topic list was modified - update both topics and QoS arrays
		packet.Topics = topics
		// Pad or truncate QoS to match
		if len(packet.Qoss) < len(topics) {
			for i := len(packet.Qoss); i < len(topics); i++ {
				packet.Qoss = append(packet.Qoss, 0)
			}
		} else if len(packet.Qoss) > len(topics) {
			packet.Qoss = packet.Qoss[:len(topics)]
		}
	} else {
		packet.Topics = topics
	}

	// Notify successful subscription (immutable copy)
	if err := h.OnSubscribe(ctx, hctx, topics); err != nil {
		// Log but don't fail the subscription
		return nil
	}

	return nil
}

// handleUnsubscribe processes MQTT UNSUBSCRIBE packets.
func (p *Parser) handleUnsubscribe(ctx context.Context, packet *packets.UnsubscribePacket, h handler.Handler, hctx *handler.Context) error {
	topics := make([]string, len(packet.Topics))
	copy(topics, packet.Topics)

	// Notify unsubscription (immutable copy)
	if err := h.OnUnsubscribe(ctx, hctx, topics); err != nil {
		// Log but don't fail the unsubscription
		return nil
	}

	return nil
}

// handleDisconnect processes MQTT DISCONNECT packets.
func (p *Parser) handleDisconnect(ctx context.Context, h handler.Handler, hctx *handler.Context) error {
	// Notify disconnection
	if err := h.OnDisconnect(ctx, hctx); err != nil {
		// Log but don't fail the disconnection
		return nil
	}

	return nil
}
