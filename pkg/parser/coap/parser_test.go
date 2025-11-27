// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package coap

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/message/pool"
	"github.com/plgd-dev/go-coap/v3/udp/coder"
)

type mockHandler struct {
	connectErr   error
	publishErr   error
	subscribeErr error

	connectCalled   bool
	publishCalled   bool
	subscribeCalled bool

	lastHctx    *handler.Context
	lastPath    string
	lastPayload []byte
	lastTopics  []string
}

func (m *mockHandler) AuthConnect(ctx context.Context, hctx *handler.Context) error {
	m.connectCalled = true
	m.lastHctx = hctx
	return m.connectErr
}

func (m *mockHandler) AuthPublish(ctx context.Context, hctx *handler.Context, topic *string, payload *[]byte) error {
	m.publishCalled = true
	m.lastPath = *topic
	m.lastPayload = *payload
	return m.publishErr
}

func (m *mockHandler) AuthSubscribe(ctx context.Context, hctx *handler.Context, topics *[]string) error {
	m.subscribeCalled = true
	m.lastTopics = *topics
	return m.subscribeErr
}

func (m *mockHandler) OnConnect(ctx context.Context, hctx *handler.Context) error {
	return nil
}

func (m *mockHandler) OnPublish(ctx context.Context, hctx *handler.Context, topic string, payload []byte) error {
	return nil
}

func (m *mockHandler) OnSubscribe(ctx context.Context, hctx *handler.Context, topics []string) error {
	return nil
}

func (m *mockHandler) OnUnsubscribe(ctx context.Context, hctx *handler.Context, topics []string) error {
	return nil
}

func (m *mockHandler) OnDisconnect(ctx context.Context, hctx *handler.Context) error {
	return nil
}

func TestCoAPParser_ParsePOST(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create POST message
	ctx := context.Background()
	msg := pool.NewMessage(ctx)
	defer msg.Reset()

	msg.SetCode(codes.POST)
	msg.SetMessageID(123)
	msg.SetType(message.Confirmable)

	// Marshal message
	data, err := msg.MarshalWithEncoder(coder.DefaultCoder)
	if err != nil {
		t.Fatalf("Failed to marshal CoAP message: %v", err)
	}

	// Parse message
	reader := bytes.NewReader(data)
	var writer bytes.Buffer
	hctx := &handler.Context{}

	err = p.Parse(ctx, reader, &writer, parser.Upstream, mock, hctx)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify handler was called
	if !mock.connectCalled {
		t.Error("Expected AuthConnect to be called")
	}
	if !mock.publishCalled {
		t.Error("Expected AuthPublish to be called")
	}

	// Verify protocol was set
	if mock.lastHctx.Protocol != "coap" {
		t.Errorf("Expected protocol 'coap', got '%s'", mock.lastHctx.Protocol)
	}
}

func TestCoAPParser_ParseGET(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create GET message
	ctx := context.Background()
	msg := pool.NewMessage(ctx)
	defer msg.Reset()

	msg.SetCode(codes.GET)
	msg.SetMessageID(124)
	msg.SetType(message.Confirmable)

	// Marshal message
	data, err := msg.MarshalWithEncoder(coder.DefaultCoder)
	if err != nil {
		t.Fatalf("Failed to marshal CoAP message: %v", err)
	}

	// Parse message
	reader := bytes.NewReader(data)
	var writer bytes.Buffer
	hctx := &handler.Context{}

	err = p.Parse(ctx, reader, &writer, parser.Upstream, mock, hctx)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify connect was called
	if !mock.connectCalled {
		t.Error("Expected AuthConnect to be called")
	}

	// GET without observe should not call subscribe
	if mock.subscribeCalled {
		t.Error("Did not expect AuthSubscribe to be called for simple GET")
	}
}

func TestCoAPParser_ParsePUT(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create PUT message
	ctx := context.Background()
	msg := pool.NewMessage(ctx)
	defer msg.Reset()

	msg.SetCode(codes.PUT)
	msg.SetMessageID(126)
	msg.SetType(message.Confirmable)

	// Marshal message
	data, err := msg.MarshalWithEncoder(coder.DefaultCoder)
	if err != nil {
		t.Fatalf("Failed to marshal CoAP message: %v", err)
	}

	// Parse message
	reader := bytes.NewReader(data)
	var writer bytes.Buffer
	hctx := &handler.Context{}

	err = p.Parse(ctx, reader, &writer, parser.Upstream, mock, hctx)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify publish was called (PUT is treated as publish)
	if !mock.publishCalled {
		t.Error("Expected AuthPublish to be called for PUT")
	}
}

func TestCoAPParser_ParseDELETE(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create DELETE message
	ctx := context.Background()
	msg := pool.NewMessage(ctx)
	defer msg.Reset()

	msg.SetCode(codes.DELETE)
	msg.SetMessageID(127)
	msg.SetType(message.Confirmable)

	// Marshal message
	data, err := msg.MarshalWithEncoder(coder.DefaultCoder)
	if err != nil {
		t.Fatalf("Failed to marshal CoAP message: %v", err)
	}

	// Parse message
	reader := bytes.NewReader(data)
	var writer bytes.Buffer
	hctx := &handler.Context{}

	err = p.Parse(ctx, reader, &writer, parser.Upstream, mock, hctx)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// DELETE should just forward without specific auth
	if !mock.connectCalled {
		t.Error("Expected AuthConnect to be called")
	}
}

func TestCoAPParser_AuthError(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{
		connectErr: errors.New("auth failed"),
	}

	// Create POST message
	ctx := context.Background()
	msg := pool.NewMessage(ctx)
	defer msg.Reset()

	msg.SetCode(codes.POST)
	msg.SetMessageID(128)
	msg.SetType(message.Confirmable)

	// Marshal message
	data, err := msg.MarshalWithEncoder(coder.DefaultCoder)
	if err != nil {
		t.Fatalf("Failed to marshal CoAP message: %v", err)
	}

	// Parse message - should return error
	reader := bytes.NewReader(data)
	var writer bytes.Buffer
	hctx := &handler.Context{}

	err = p.Parse(ctx, reader, &writer, parser.Upstream, mock, hctx)
	if err == nil {
		t.Error("Expected error from Parse() when auth fails")
	}
}

func TestCoAPParser_InvalidMessage(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Invalid CoAP message
	reader := bytes.NewReader([]byte{0xFF, 0xFF, 0xFF})
	var writer bytes.Buffer
	hctx := &handler.Context{}

	err := p.Parse(context.Background(), reader, &writer, parser.Upstream, mock, hctx)
	if err == nil {
		t.Error("Expected error from Parse() with invalid message")
	}
}

func TestCoAPParser_Downstream(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create response message
	ctx := context.Background()
	msg := pool.NewMessage(ctx)
	defer msg.Reset()

	msg.SetCode(codes.Content)
	msg.SetMessageID(129)
	msg.SetType(message.Acknowledgement)

	// Marshal message
	data, err := msg.MarshalWithEncoder(coder.DefaultCoder)
	if err != nil {
		t.Fatalf("Failed to marshal CoAP message: %v", err)
	}

	// Parse message as downstream
	reader := bytes.NewReader(data)
	var writer bytes.Buffer
	hctx := &handler.Context{}

	err = p.Parse(ctx, reader, &writer, parser.Downstream, mock, hctx)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Downstream messages should just be forwarded
	if writer.Len() == 0 {
		t.Error("Expected message to be written to output")
	}
}

func TestCoAPParser_ReadError(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create reader that returns error
	errReader := &errorReader{err: errors.New("read error")}
	var writer bytes.Buffer
	hctx := &handler.Context{}

	err := p.Parse(context.Background(), errReader, &writer, parser.Upstream, mock, hctx)
	if err == nil {
		t.Error("Expected error from Parse() with failing reader")
	}
}

// errorReader is a reader that always returns an error.
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
