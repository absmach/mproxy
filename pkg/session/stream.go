// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

type Direction int

const (
	Up Direction = iota
	Down
)

const unknownID = "unknown"

var (
	errBroker = "failed to proxy from MQTT client with id %s to MQTT broker with error: %s"
	errClient = "failed to proxy from MQTT broker to client with id %s with error: %s"
)

// Stream starts proxy between client and broker.
func Stream(ctx context.Context, in, out net.Conn, h Handler, preIc, postIc Interceptor, cert x509.Certificate) error {
	s := Session{
		Cert: cert,
	}
	ctx = NewContext(ctx, &s)
	errs := make(chan error, 2)

	go stream(ctx, Up, in, out, h, preIc, postIc, errs)
	go stream(ctx, Down, out, in, h, preIc, postIc, errs)

	// Handle whichever error happens first.
	// The other routine won't be blocked when writing
	// to the errors channel because it is buffered.
	err := <-errs

	disconnectErr := h.Disconnect(ctx)

	return errors.Join(err, disconnectErr)
}

func stream(ctx context.Context, dir Direction, r, w net.Conn, h Handler, preIc, postIc Interceptor, errs chan error) {
	for {
		// Read from one connection.
		pkt, err := packets.ReadPacket(r)
		if err != nil {
			errs <- wrap(ctx, err, dir)
			return
		}

		if preIc != nil {
			pkt, err = preIc.Intercept(ctx, pkt, dir)
			if err != nil {
				errs <- wrap(ctx, err, dir)
				return
			}
		}

		switch dir {
		case Up:
			if err = authorize(ctx, pkt, h); err != nil {
				if _, ok := pkt.(*packets.PublishPacket); ok {
					pkt = packets.NewControlPacket(packets.Disconnect)
					if wErr := pkt.Write(w); wErr != nil {
						err = errors.Join(err, wErr)
					}
				}
				errs <- wrap(ctx, err, dir)
				return
			}
		default:
			if p, ok := pkt.(*packets.PublishPacket); ok {
				topics := []string{p.TopicName}
				// The broker sends subscription messages to the client as Publish Packets.
				// We need to check if the Publish packet sent by the broker is allowed to be received to by the client.
				// Therefore, we are using handler.AuthSubscribe instead of handler.AuthPublish.
				if err = h.AuthSubscribe(ctx, &topics); err != nil {
					pkt = packets.NewControlPacket(packets.Disconnect).(*packets.DisconnectPacket)
					if wErr := pkt.Write(w); wErr != nil {
						err = errors.Join(err, wErr)
					}
					errs <- wrap(ctx, err, dir)
					return
				}
			}
		}

		if postIc != nil {
			pkt, err = postIc.Intercept(ctx, pkt, dir)
			if err != nil {
				errs <- wrap(ctx, err, dir)
				return
			}
		}

		// Send to another.
		if err := pkt.Write(w); err != nil {
			errs <- wrap(ctx, err, dir)
			return
		}

		// Notify only for packets sent from client to broker (incoming packets).
		if dir == Up {
			if err := notify(ctx, pkt, h); err != nil {
				errs <- wrap(ctx, err, dir)
			}
		}
	}
}

func authorize(ctx context.Context, pkt packets.ControlPacket, h Handler) error {
	switch p := pkt.(type) {
	case *packets.ConnectPacket:
		s, ok := FromContext(ctx)
		if ok {
			s.ID = p.ClientIdentifier
			s.Username = p.Username
			s.Password = p.Password
		}

		ctx = NewContext(ctx, s)
		if err := h.AuthConnect(ctx); err != nil {
			return err
		}
		// Copy back to the packet in case values are changed by Event handler.
		// This is specific to CONN, as only that package type has credentials.
		p.ClientIdentifier = s.ID
		p.Username = s.Username
		p.Password = s.Password
		return nil
	case *packets.PublishPacket:
		return h.AuthPublish(ctx, &p.TopicName, &p.Payload)
	case *packets.SubscribePacket:
		return h.AuthSubscribe(ctx, &p.Topics)
	default:
		return nil
	}
}

func notify(ctx context.Context, pkt packets.ControlPacket, h Handler) error {
	switch p := pkt.(type) {
	case *packets.ConnectPacket:
		return h.Connect(ctx)
	case *packets.PublishPacket:
		return h.Publish(ctx, &p.TopicName, &p.Payload)
	case *packets.SubscribePacket:
		return h.Subscribe(ctx, &p.Topics)
	case *packets.UnsubscribePacket:
		return h.Unsubscribe(ctx, &p.Topics)
	default:
		return nil
	}
}

func wrap(ctx context.Context, err error, dir Direction) error {
	if err == io.EOF {
		return err
	}
	cid := unknownID
	if s, ok := FromContext(ctx); ok {
		cid = s.ID
	}
	switch dir {
	case Up:
		return fmt.Errorf(errClient, cid, err.Error())
	case Down:
		return fmt.Errorf(errBroker, cid, err.Error())
	default:
		return err
	}
}
