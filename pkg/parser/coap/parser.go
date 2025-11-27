// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package coap

import (
	"context"
	"fmt"
	"io"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/message/pool"
	"github.com/plgd-dev/go-coap/v3/udp/coder"
)

// Parser implements the parser.Parser interface for CoAP protocol.
// This is a simple parser that extracts basic auth information and forwards packets.
type Parser struct{}

var _ parser.Parser = (*Parser)(nil)

// Parse reads one CoAP message from r, processes it, and writes to w.
// CoAP is a datagram protocol, so each Parse call handles one complete message.
func (p *Parser) Parse(ctx context.Context, r io.Reader, w io.Writer, dir parser.Direction, h handler.Handler, hctx *handler.Context) error {
	// Read message data
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read CoAP message: %w", err)
	}

	// Parse CoAP message
	msg := pool.NewMessage(ctx)
	defer msg.Reset()

	_, err = msg.UnmarshalWithDecoder(coder.DefaultCoder, data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CoAP message: %w", err)
	}

	// Process based on direction
	if dir == parser.Upstream {
		// Client → Backend
		if err := p.handleUpstream(ctx, msg, h, hctx); err != nil {
			return err
		}
	}
	// Downstream packets are forwarded as-is

	// Write original data (we're not modifying packets in this simple version)
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("failed to write CoAP message: %w", err)
	}

	return nil
}

// handleUpstream processes upstream (client→backend) CoAP messages.
// This is a simplified implementation that extracts auth and calls handlers
// but doesn't modify packets.
func (p *Parser) handleUpstream(ctx context.Context, msg *pool.Message, h handler.Handler, hctx *handler.Context) error {
	// Update protocol
	hctx.Protocol = "coap"

	// Extract auth from query string parameter: ?auth=<key>
	authKey := extractAuthFromQuery(msg)
	if authKey != "" {
		hctx.Password = []byte(authKey)
	}

	// Extract path
	path, err := msg.Options().Path()
	if err != nil {
		path = "/"
	}

	// Authorize connection
	if err := h.AuthConnect(ctx, hctx); err != nil {
		return fmt.Errorf("connection authorization failed: %w", err)
	}

	// Handle based on CoAP method code
	code := msg.Code()
	switch code {
	case codes.POST, codes.PUT:
		// POST/PUT is treated as publish
		payload := []byte{} // Simplified: not extracting actual payload
		if err := h.AuthPublish(ctx, hctx, &path, &payload); err != nil {
			return fmt.Errorf("publish authorization failed: %w", err)
		}
		_ = h.OnPublish(ctx, hctx, path, payload)

	case codes.GET:
		// Check if this is an observe request (subscription)
		obs, err := msg.Options().Observe()
		if err == nil && obs == 0 {
			// This is a subscribe request
			topics := []string{path}
			if err := h.AuthSubscribe(ctx, hctx, &topics); err != nil {
				return fmt.Errorf("subscribe authorization failed: %w", err)
			}
			_ = h.OnSubscribe(ctx, hctx, topics)
		}
	}

	return nil
}

// extractAuthFromQuery extracts the auth parameter from query string.
// CoAP uses URI-Query options: ?auth=<key>
func extractAuthFromQuery(msg *pool.Message) string {
	queries, err := msg.Options().Queries()
	if err != nil {
		return ""
	}

	for _, query := range queries {
		// Parse query string: auth=value
		if len(query) > 5 && query[:5] == "auth=" {
			return query[5:]
		}
	}

	return ""
}
