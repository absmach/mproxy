// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package mqtt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/absmach/mproxy"
	"github.com/absmach/mproxy/tls"
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

type streamer struct {
	h  mproxy.Handler
	ic mproxy.Interceptor
}

func New(h mproxy.Handler, ic mproxy.Interceptor) mproxy.Streamer {
	return &streamer{
		h:  h,
		ic: ic,
	}
}

// Stream starts proxy between client and broker.
func (s *streamer) Stream(ctx context.Context, in, out net.Conn) error {
	cert, err := tls.ClientCert(in)
	if err != nil {
		return err
	}
	session := mproxy.Session{
		Cert: cert,
	}
	ctx = mproxy.NewContext(ctx, &session)
	errs := make(chan error, 2)
	go stream(ctx, Up, in, out, s.h, s.ic, errs)
	go stream(ctx, Down, out, in, s.h, s.ic, errs)

	// Handle whichever error happens first.
	// The other routine won't be blocked when writing
	// to the errors channel because it is buffered.
	err = <-errs

	disconnectErr := s.h.Disconnect(ctx)

	return errors.Join(err, disconnectErr)
}

func stream(ctx context.Context, dir Direction, r, w net.Conn, h mproxy.Handler, ic mproxy.Interceptor, errs chan error) {
	for {
		// Read from one connection.
		pkt, err := packets.ReadPacket(r)
		if err != nil {
			errs <- wrap(ctx, err, dir)
			return
		}

		switch dir {
		case Up:
			if err = authorize(ctx, pkt, h); err != nil {
				errs <- wrap(ctx, err, dir)
				return
			}
		default:
			if p, ok := pkt.(*packets.PublishPacket); ok {
				if err = h.AuthPublish(ctx, &p.TopicName, &p.Payload); err != nil {
					pkt = packets.NewControlPacket(packets.Disconnect).(*packets.DisconnectPacket)
					err = pkt.Write(w)
					errs <- wrap(ctx, err, dir)
					return
				}
			}
		}

		if ic != nil {
			if err := ic.Intercept(ctx, &pkt); err != nil {
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

func authorize(ctx context.Context, pkt packets.ControlPacket, h mproxy.Handler) error {
	switch p := pkt.(type) {
	case *packets.ConnectPacket:
		s, ok := mproxy.FromContext(ctx)
		if ok {
			s.ID = p.ClientIdentifier
			s.Username = p.Username
			s.Password = p.Password
		}

		ctx = mproxy.NewContext(ctx, s)
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

func notify(ctx context.Context, pkt packets.ControlPacket, h mproxy.Handler) error {
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
	if s, ok := mproxy.FromContext(ctx); ok {
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
