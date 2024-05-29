// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package coap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/absmach/mproxy"
	"github.com/absmach/mproxy/pkg/session"
	"github.com/plgd-dev/go-coap/v3/dtls"
	dtlsServer "github.com/plgd-dev/go-coap/v3/dtls/server"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/message/pool"
	"github.com/plgd-dev/go-coap/v3/mux"
	"github.com/plgd-dev/go-coap/v3/net"
	"github.com/plgd-dev/go-coap/v3/options"
	"github.com/plgd-dev/go-coap/v3/udp"
	udpServer "github.com/plgd-dev/go-coap/v3/udp/server"
)

const startObserve uint32 = 0

var errUnsupportedMethod = errors.New("unsupported CoAP method")

type Proxy struct {
	config  mproxy.Config
	session session.Handler
	logger  *slog.Logger
}

type udpNilMonitor struct{}

func (u *udpNilMonitor) UDPServerApply(cfg *udpServer.Config) {
	cfg.CreateInactivityMonitor = nil
}

func NewUDPNilMonitor() udpServer.Option {
	return &udpNilMonitor{}
}

var _ udpServer.Option = (*udpNilMonitor)(nil)

type dtlsNilMonitor struct{}

func (d *dtlsNilMonitor) DTLSServerApply(cfg *dtlsServer.Config) {
	cfg.CreateInactivityMonitor = nil
}

func NewDTLSNilMonitor() dtlsServer.Option {
	return &dtlsNilMonitor{}
}

var _ udpServer.Option = (*udpNilMonitor)(nil)

func NewProxy(config mproxy.Config, handler session.Handler, logger *slog.Logger) *Proxy {
	return &Proxy{
		config:  config,
		session: handler,
		logger:  logger,
	}
}

func sendErrorMessage(cc mux.Conn, token []byte, err error, code codes.Code) error {
	m := cc.AcquireMessage(cc.Context())
	defer cc.ReleaseMessage(m)
	m.SetCode(code)
	m.SetBody(bytes.NewReader(([]byte)(err.Error())))
	m.SetToken(token)
	m.SetContentFormat(message.TextPlain)
	return cc.WriteMessage(m)
}

func (p *Proxy) postUpstream(cc mux.Conn, req *mux.Message, token []byte) error {
	outbound, err := udp.Dial(p.config.Target)
	if err != nil {
		return err
	}
	defer outbound.Close()

	path, err := req.Options().Path()
	if err != nil {
		return err
	}

	format := message.TextPlain
	if req.HasOption(message.ContentFormat) {
		format, err = req.ContentFormat()
		if err != nil {
			return err
		}
	}

	pm, err := outbound.Post(cc.Context(), path, format, req.Body(), req.Options()...)
	if err != nil {
		return err
	}
	pm.SetToken(token)
	return cc.WriteMessage(pm)
}

func (p *Proxy) getUpstream(cc mux.Conn, req *mux.Message, token []byte) error {
	path, err := req.Options().Path()
	if err != nil {
		return err
	}

	outbound, err := udp.Dial(p.config.Target)
	if err != nil {
		return err
	}
	defer outbound.Close()
	pm, err := outbound.Get(cc.Context(), path, req.Options()...)
	if err != nil {
		return err
	}
	pm.SetToken(token)
	return cc.WriteMessage(pm)
}

func (p *Proxy) observeUpstream(ctx context.Context, cc mux.Conn, opts []message.Option, token []byte, path string) {
	outbound, err := udp.Dial(p.config.Target)
	if err != nil {
		if err := sendErrorMessage(cc, token, err, codes.BadGateway); err != nil {
			p.logger.Error(fmt.Sprintf("cannot send error response: %v", err))
		}
	}
	defer outbound.Close()
	doneObserving := make(chan struct{})

	pm := outbound.AcquireMessage(outbound.Context())
	defer outbound.ReleaseMessage(pm)
	pm.SetToken(token)
	pm.SetCode(codes.GET)
	for _, opt := range opts {
		pm.SetOptionBytes(opt.ID, opt.Value)
	}
	if err := pm.SetPath(path); err != nil {
		if err := sendErrorMessage(cc, token, err, codes.BadOption); err != nil {
			p.logger.Error(fmt.Sprintf("cannot send error response: %v", err))
		}
		return
	}

	obs, err := outbound.DoObserve(pm, func(req *pool.Message) {
		req.SetToken(token)
		if err := cc.WriteMessage(req); err != nil {
			if err := sendErrorMessage(cc, token, err, codes.BadGateway); err != nil {
				p.logger.Error(err.Error())
			}
			p.logger.Error(err.Error())
		}
		if req.Code() == codes.NotFound {
			close(doneObserving)
		}
	})
	if err != nil {
		if err := sendErrorMessage(cc, token, err, codes.BadGateway); err != nil {
			p.logger.Error(fmt.Sprintf("cannot send error response: %v", err))
		}
	}

	select {
	case <-doneObserving:
		if err := obs.Cancel(ctx); err != nil {
			p.logger.Error(fmt.Sprintf("failed to cancel observation:%v", err))
		}
	case <-ctx.Done():
		return
	}
}

func (p *Proxy) CancelObservation(cc mux.Conn, opts []message.Option, token []byte, path string) error {
	outbound, err := udp.Dial(p.config.Target)
	if err != nil {
		if err := sendErrorMessage(cc, token, err, codes.BadGateway); err != nil {
			p.logger.Error(fmt.Sprintf("cannot send error response: %v", err))
		}
	}
	defer outbound.Close()

	pm := outbound.AcquireMessage(outbound.Context())
	defer outbound.ReleaseMessage(pm)
	pm.SetToken(token)
	pm.SetCode(codes.GET)
	for _, opt := range opts {
		pm.SetOptionBytes(opt.ID, opt.Value)
	}
	if err := pm.SetPath(path); err != nil {
		if err := sendErrorMessage(cc, token, err, codes.BadOption); err != nil {
			p.logger.Error(fmt.Sprintf("cannot send error response: %v", err))
		}
		return err
	}
	if err := outbound.WriteMessage(pm); err != nil {
		return err
	}
	pm.SetCode(codes.Content)
	return cc.WriteMessage(pm)
}

func (p *Proxy) handler(w mux.ResponseWriter, r *mux.Message) {
	tok, err := r.Options().GetBytes(message.URIQuery)
	if err != nil {
		if err := sendErrorMessage(w.Conn(), r.Token(), err, codes.Unauthorized); err != nil {
			p.logger.Error(err.Error())
		}
		return
	}
	ctx := session.NewContext(r.Context(), &session.Session{Password: tok})
	if err := p.session.AuthConnect(ctx); err != nil {
		if err := sendErrorMessage(w.Conn(), r.Token(), err, codes.Unauthorized); err != nil {
			p.logger.Error(err.Error())
		}
		return
	}
	path, err := r.Options().Path()
	if err != nil {
		if err := sendErrorMessage(w.Conn(), r.Token(), err, codes.BadOption); err != nil {
			p.logger.Error(err.Error())
		}
		return
	}
	switch r.Code() {
	case codes.GET:
		p.handleGet(ctx, path, w.Conn(), r.Token(), r)

	case codes.POST:
		body, err := r.ReadBody()
		if err != nil {
			if err := sendErrorMessage(w.Conn(), r.Token(), err, codes.BadRequest); err != nil {
				p.logger.Error(err.Error())
			}
			return
		}
		p.handlePost(ctx, w.Conn(), body, r.Token(), path, r)
	default:
		if err := sendErrorMessage(w.Conn(), r.Token(), errUnsupportedMethod, codes.MethodNotAllowed); err != nil {
			p.logger.Error(err.Error())
		}
	}
}

func (p *Proxy) handleGet(ctx context.Context, path string, con mux.Conn, token []byte, r *mux.Message) {
	if err := p.session.AuthSubscribe(ctx, &[]string{path}); err != nil {
		if err := sendErrorMessage(con, token, err, codes.Unauthorized); err != nil {
			p.logger.Error(err.Error())
		}
		return
	}
	if err := p.session.Subscribe(ctx, &[]string{path}); err != nil {
		if err := sendErrorMessage(con, token, err, codes.Unauthorized); err != nil {
			p.logger.Error(err.Error())
		}
		return
	}
	switch {
	case r.HasOption(message.Observe):
		obs, err := r.Options().Observe()
		if err != nil {
			if err := sendErrorMessage(con, r.Token(), err, codes.BadRequest); err != nil {
				p.logger.Error(err.Error())
			}
			return
		}
		switch obs {
		case startObserve:
			go p.observeUpstream(ctx, con, r.Options(), token, path)
		default:
			if err := p.CancelObservation(con, r.Options(), token, path); err != nil {
				p.logger.Error(fmt.Sprintf("error performing cancel observation: %v\n", err))
				if err := sendErrorMessage(con, token, err, codes.BadGateway); err != nil {
					p.logger.Error(err.Error())
				}
				return
			}
		}
	default:
		if err := p.getUpstream(con, r, token); err != nil {
			p.logger.Error(fmt.Sprintf("error performing get: %v\n", err))
			if err := sendErrorMessage(con, token, err, codes.BadGateway); err != nil {
				p.logger.Error(err.Error())
			}
			return
		}
	}
}

func (p *Proxy) handlePost(ctx context.Context, con mux.Conn, body, token []byte, path string, r *mux.Message) {
	if err := p.session.AuthPublish(ctx, &path, &body); err != nil {
		if err := sendErrorMessage(con, token, err, codes.Unauthorized); err != nil {
			p.logger.Error(err.Error())
		}
		return
	}
	if err := p.session.Publish(ctx, &path, &body); err != nil {
		if err := sendErrorMessage(con, token, err, codes.BadRequest); err != nil {
			p.logger.Error(err.Error())
		}
		return
	}
	if err := p.postUpstream(con, r, token); err != nil {
		p.logger.Debug(fmt.Sprintf("error performing post: %v\n", err))
		if err := sendErrorMessage(con, token, err, codes.BadGateway); err != nil {
			p.logger.Error(err.Error())
		}
		return
	}
}

func (p *Proxy) Listen(ctx context.Context) error {
	if p.config.DTLSConfig != nil {
		l, err := net.NewDTLSListener("udp", p.config.Address, p.config.DTLSConfig)
		if err != nil {
			return err
		}
		defer l.Close()

		p.logger.Info(fmt.Sprintf("CoAP proxy server started on port %s with DTLS", p.config.Address))
		var dialOpts []dtlsServer.Option
		dialOpts = append(dialOpts, options.WithMux(mux.HandlerFunc(p.handler)), NewDTLSNilMonitor())

		s := dtls.NewServer(dialOpts...)

		errCh := make(chan error)
		go func() {
			errCh <- s.Serve(l)
		}()

		select {
		case <-ctx.Done():
			p.logger.Info(fmt.Sprintf("CoAP proxy server on port %s with DTLS exiting ...", p.config.Address))
			l.Close()
		case err := <-errCh:
			p.logger.Error(fmt.Sprintf("CoAP proxy server on port %s with DTLS exiting with errors: %s", p.config.Address, err.Error()))
			return err
		}
		return nil
	}
	l, err := net.NewListenUDP("udp", p.config.Address)
	if err != nil {
		return err
	}
	defer l.Close()

	p.logger.Info(fmt.Sprintf("CoAP proxy server started at %s without DTLS", p.config.Address))
	var dialOpts []udpServer.Option
	dialOpts = append(dialOpts, options.WithMux(mux.HandlerFunc(p.handler)), NewUDPNilMonitor())

	s := udp.NewServer(dialOpts...)

	errCh := make(chan error)
	go func() {
		errCh <- s.Serve(l)
	}()

	select {
	case <-ctx.Done():
		p.logger.Info(fmt.Sprintf("CoAP proxy server on port %s without DTLS exiting ...", p.config.Address))
		l.Close()
	case err := <-errCh:
		p.logger.Error(fmt.Sprintf("CoAP proxy server on port %s without DTLS exiting with errors: %s", p.config.Address, err.Error()))
		return err
	}
	return nil
}
