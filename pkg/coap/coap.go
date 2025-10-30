// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package coap

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/absmach/mgate"
	"github.com/absmach/mgate/pkg/session"
	mptls "github.com/absmach/mgate/pkg/tls"
	"github.com/pion/dtls/v3"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/message/pool"
	"github.com/plgd-dev/go-coap/v3/udp/coder"
	"golang.org/x/sync/errgroup"
)

const (
	bufferSize   uint64 = 1280
	startObserve uint32 = 0
	authQuery           = "auth"
)

type Conn struct {
	clientAddr *net.UDPAddr
	serverConn *net.UDPConn
	started    atomic.Bool
}

type Proxy struct {
	config  mgate.Config
	session session.Handler
	logger  *slog.Logger
	connMap map[string]*Conn
	mutex   sync.Mutex
}

func NewProxy(config mgate.Config, handler session.Handler, logger *slog.Logger) *Proxy {
	return &Proxy{
		config:  config,
		session: handler,
		logger:  logger,
		connMap: make(map[string]*Conn),
	}
}

func (p *Proxy) proxyUDP(ctx context.Context, l *net.UDPConn) {
	buffer := make([]byte, bufferSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, clientAddr, err := l.ReadFromUDP(buffer)
			if err != nil {
				p.logger.Error("failed to read from UDP", slog.String("error", err.Error()))
				return
			}
			conn, err := p.newConn(clientAddr)
			if err != nil {
				p.logger.Error("failed to create new connection", slog.String("error", err.Error()))
				continue
			}
			//nolint:contextcheck // upUDP does not need context
			p.upUDP(conn, buffer[:n], l)
		}
	}
}

func (p *Proxy) Listen(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(p.config.Host, p.config.Port))
	if err != nil {
		p.logger.Error("failed to resolve UDP address", slog.String("error", err.Error()))
		return err
	}
	g, ctx := errgroup.WithContext(ctx)
	switch {
	case p.config.DTLSConfig != nil:
		l, err := dtls.Listen("udp", addr, p.config.DTLSConfig)
		if err != nil {
			return err
		}
		defer l.Close()

		g.Go(func() error {
			p.proxyDTLS(ctx, l)
			return nil
		})

		g.Go(func() error {
			<-ctx.Done()
			return l.Close()
		})
	default:
		l, err := net.ListenUDP("udp", addr)
		if err != nil {
			return err
		}
		defer l.Close()

		g.Go(func() error {
			p.proxyUDP(ctx, l)
			return nil
		})

		g.Go(func() error {
			<-ctx.Done()
			return l.Close()
		})
	}

	status := mptls.SecurityStatus(p.config.DTLSConfig)
	p.logger.Info(fmt.Sprintf("COAP proxy server started at %s with %s", net.JoinHostPort(p.config.Host, p.config.Port), status))

	if err := g.Wait(); err != nil {
		p.logger.Info(fmt.Sprintf("COAP proxy server at %s exiting with errors", net.JoinHostPort(p.config.Host, p.config.Port)), slog.String("error", err.Error()))
	} else {
		p.logger.Info(fmt.Sprintf("COAP proxy server at %s exiting...", net.JoinHostPort(p.config.Host, p.config.Port)))
	}
	return nil
}

func (p *Proxy) newConn(clientAddr *net.UDPAddr) (*Conn, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	conn, ok := p.connMap[clientAddr.String()]
	if !ok {
		conn = &Conn{clientAddr: clientAddr}
		addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(p.config.TargetHost, p.config.TargetPort))
		if err != nil {
			return nil, err
		}
		t, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			return nil, err
		}
		conn.serverConn = t
		p.connMap[clientAddr.String()] = conn
	}
	return conn, nil
}

func (p *Proxy) upUDP(conn *Conn, buffer []byte, l *net.UDPConn) {
	if msg, err := p.handleCoAPMessage(context.Background(), buffer); err != nil {
		data := p.encodeErrorResponse(context.Background(), msg, err)
		if len(data) > 0 {
			if _, werr := l.WriteToUDP(data, conn.clientAddr); werr != nil {
				p.logger.Error("failed to send error response", slog.String("err", werr.Error()))
			}
		}
		return
	}

	if _, err := conn.serverConn.Write(buffer); err != nil {
		return
	}

	// Start the downstream reader once the first upstream write succeeds.
	if conn.started.CompareAndSwap(false, true) {
		go p.downUDP(context.Background(), l, conn)
	}
}

func (p *Proxy) downUDP(ctx context.Context, l *net.UDPConn, conn *Conn) {
	buffer := make([]byte, bufferSize)
	for {
		select {
		case <-ctx.Done():
			p.closeConn(conn)
			return
		default:
		}
		err := conn.serverConn.SetReadDeadline(time.Now().Add(30 * time.Second))
		if err != nil {
			return
		}
		n, err := conn.serverConn.Read(buffer)
		if err != nil {
			p.closeConn(conn)
			return
		}
		_, err = l.WriteToUDP(buffer[:n], conn.clientAddr)
		if err != nil {
			return
		}
	}
}

func (p *Proxy) closeConn(conn *Conn) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	delete(p.connMap, conn.clientAddr.String())
	conn.serverConn.Close()
}

func (p *Proxy) proxyDTLS(ctx context.Context, l net.Listener) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conn, err := l.Accept()
		if err != nil {
			p.logger.Warn("Accept error " + err.Error())
			continue
		}
		p.logger.Info("Accepted new client")
		go p.handleDTLS(ctx, conn)
	}
}

func (p *Proxy) handleDTLS(ctx context.Context, inbound net.Conn) {
	defer inbound.Close()
	outboundAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(p.config.TargetHost, p.config.TargetPort))
	if err != nil {
		p.logger.Error("cannot resolve remote broker address " + net.JoinHostPort(p.config.TargetHost, p.config.TargetPort) + " due to: " + err.Error())
		return
	}

	outbound, err := net.DialUDP("udp", nil, outboundAddr)
	if err != nil {
		p.logger.Error("cannot connect to remote broker " + outboundAddr.String() + " due to: " + err.Error())
		return
	}
	defer outbound.Close()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		p.dtlsUp(gCtx, outbound, inbound)
		return nil
	})

	g.Go(func() error {
		p.dtlsDown(inbound, outbound)
		return nil
	})

	if err := g.Wait(); err != nil {
		p.logger.Error("DTLS proxy error", slog.String("error", err.Error()))
	}
}

func (p *Proxy) dtlsUp(ctx context.Context, outbound *net.UDPConn, inbound net.Conn) {
	buffer := make([]byte, bufferSize)
	for {
		n, err := inbound.Read(buffer)
		if err != nil {
			return
		}
		if msg, err := p.handleCoAPMessage(ctx, buffer[:n]); err != nil {
			data := p.encodeErrorResponse(ctx, msg, err)
			if len(data) > 0 {
				if _, werr := inbound.Write(data); werr != nil {
					p.logger.Error("failed to send error response", slog.String("err", werr.Error()))
				}
			}
			return
		}

		if _, err = outbound.Write(buffer[:n]); err != nil {
			return
		}
	}
}

func (p *Proxy) dtlsDown(inbound net.Conn, outbound *net.UDPConn) {
	buffer := make([]byte, bufferSize)
	for {
		err := outbound.SetReadDeadline(time.Now().Add(1 * time.Minute))
		if err != nil {
			return
		}
		n, err := outbound.Read(buffer)
		if err != nil {
			return
		}

		if _, err = inbound.Write(buffer[:n]); err != nil {
			return
		}
	}
}

func (p *Proxy) handleCoAPMessage(ctx context.Context, buffer []byte) (*pool.Message, error) {
	var payload []byte
	var path string
	msg := pool.NewMessage(ctx)
	_, err := msg.UnmarshalWithDecoder(coder.DefaultCoder, buffer)
	if err != nil {
		return msg, err
	}
	if msg.Code() != codes.POST && msg.Code() != codes.GET {
		return msg, nil
	}

	authKey, err := parseKey(msg)
	if err != nil {
		return msg, err
	}

	path, err = msg.Path()
	if err != nil {
		return msg, err
	}

	ctx = session.NewContext(ctx, &session.Session{Password: []byte(authKey)})

	if msg.Body() != nil {
		payload, err = io.ReadAll(msg.Body())
		if err != nil {
			return msg, err
		}
	}

	switch msg.Code() {
	case codes.POST:
		if err := p.session.AuthConnect(ctx); err != nil {
			return msg, err
		}
		if err := p.session.AuthPublish(ctx, &path, &payload); err != nil {
			return msg, err
		}
		if err := p.session.Publish(ctx, &path, &payload); err != nil {
			return msg, err
		}
	case codes.GET:
		if err := p.session.AuthConnect(ctx); err != nil {
			return msg, err
		}
		if obs, err := msg.Options().Observe(); err == nil {
			if obs == startObserve {
				if err := p.session.AuthSubscribe(ctx, &[]string{path}); err != nil {
					return msg, err
				}
				if err := p.session.Subscribe(ctx, &[]string{path}); err != nil {
					return msg, err
				}
			}
		}
	}

	return msg, nil
}

func (p *Proxy) encodeErrorResponse(ctx context.Context, msg *pool.Message, err error) []byte {
	resp := pool.NewMessage(ctx)
	resp.SetToken(msg.Token())
	resp.SetMessageID(msg.MessageID())
	resp.SetType(msg.Type())
	for _, opt := range msg.Options() {
		resp.AddOptionBytes(opt.ID, opt.Value)
	}
	cpe, ok := err.(COAPProxyError)
	if !ok {
		cpe = NewCOAPProxyError(codes.BadRequest, err)
	}
	resp.SetCode(cpe.StatusCode())
	data, err := resp.MarshalWithEncoder(coder.DefaultCoder)
	if err != nil {
		p.logger.Error("failed to marshal error response message", slog.String("err", err.Error()))
		return nil
	}
	return data
}

func parseKey(msg *pool.Message) (string, error) {
	authKey, err := msg.Options().GetString(message.URIQuery)
	if err != nil {
		return "", NewCOAPProxyError(codes.BadRequest, err)
	}
	vars := strings.Split(authKey, "=")
	if len(vars) != 2 || vars[0] != authQuery {
		return "", nil
	}
	return vars[1], nil
}
