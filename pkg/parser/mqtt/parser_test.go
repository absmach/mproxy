// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package mqtt

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser"
	"github.com/eclipse/paho.mqtt.golang/packets"
)

type mockHandler struct {
	connectErr   error
	publishErr   error
	subscribeErr error

	connectCalled    bool
	publishCalled    bool
	subscribeCalled  bool
	unsubCalled      bool
	disconnectCalled bool

	lastHctx    *handler.Context
	lastTopic   string
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
	m.lastTopic = *topic
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
	m.unsubCalled = true
	return nil
}

func (m *mockHandler) OnDisconnect(ctx context.Context, hctx *handler.Context) error {
	m.disconnectCalled = true
	return nil
}

func TestMQTTParser_ParseConnect(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create CONNECT packet
	connectPkt := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	connectPkt.ClientIdentifier = "test-client"
	connectPkt.Username = "testuser"
	connectPkt.Password = []byte("testpass")
	connectPkt.UsernameFlag = true
	connectPkt.PasswordFlag = true
	connectPkt.ProtocolName = "MQTT"
	connectPkt.ProtocolVersion = 4

	// Serialize packet
	var buf bytes.Buffer
	if err := connectPkt.Write(&buf); err != nil {
		t.Fatalf("Failed to write CONNECT packet: %v", err)
	}

	// Parse packet
	var outBuf bytes.Buffer
	hctx := &handler.Context{}

	err := p.Parse(context.Background(), &buf, &outBuf, parser.Upstream, mock, hctx)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify handler was called
	if !mock.connectCalled {
		t.Error("Expected AuthConnect to be called")
	}

	// Verify credentials were extracted and passed to handler
	if mock.lastHctx.ClientID != "test-client" {
		t.Errorf("Expected ClientID 'test-client', got '%s'", mock.lastHctx.ClientID)
	}
	if mock.lastHctx.Username != "testuser" {
		t.Errorf("Expected Username 'testuser', got '%s'", mock.lastHctx.Username)
	}
	if string(mock.lastHctx.Password) != "testpass" {
		t.Errorf("Expected Password 'testpass', got '%s'", mock.lastHctx.Password)
	}

	// Verify packet was written to output
	if outBuf.Len() == 0 {
		t.Error("Expected packet to be written to output")
	}
}

func TestMQTTParser_ParsePublish(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create PUBLISH packet
	publishPkt := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	publishPkt.TopicName = "test/topic"
	publishPkt.Payload = []byte("test payload")
	publishPkt.Qos = 0

	// Serialize packet
	var buf bytes.Buffer
	if err := publishPkt.Write(&buf); err != nil {
		t.Fatalf("Failed to write PUBLISH packet: %v", err)
	}

	// Parse packet
	var outBuf bytes.Buffer
	hctx := &handler.Context{
		Username: "testuser",
	}

	err := p.Parse(context.Background(), &buf, &outBuf, parser.Upstream, mock, hctx)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify handler was called
	if !mock.publishCalled {
		t.Error("Expected AuthPublish to be called")
	}

	// Verify topic and payload were captured
	if mock.lastTopic != "test/topic" {
		t.Errorf("Expected topic 'test/topic', got '%s'", mock.lastTopic)
	}
	if string(mock.lastPayload) != "test payload" {
		t.Errorf("Expected payload 'test payload', got '%s'", mock.lastPayload)
	}
}

func TestMQTTParser_ParseSubscribe(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create SUBSCRIBE packet
	subscribePkt := packets.NewControlPacket(packets.Subscribe).(*packets.SubscribePacket)
	subscribePkt.Topics = []string{"topic1", "topic2"}
	subscribePkt.Qoss = []byte{0, 1}
	subscribePkt.MessageID = 1

	// Serialize packet
	var buf bytes.Buffer
	if err := subscribePkt.Write(&buf); err != nil {
		t.Fatalf("Failed to write SUBSCRIBE packet: %v", err)
	}

	// Parse packet
	var outBuf bytes.Buffer
	hctx := &handler.Context{}

	err := p.Parse(context.Background(), &buf, &outBuf, parser.Upstream, mock, hctx)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify handler was called
	if !mock.subscribeCalled {
		t.Error("Expected AuthSubscribe to be called")
	}

	// Verify topics were captured
	if len(mock.lastTopics) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(mock.lastTopics))
	}
	if mock.lastTopics[0] != "topic1" || mock.lastTopics[1] != "topic2" {
		t.Errorf("Expected topics [topic1, topic2], got %v", mock.lastTopics)
	}
}

func TestMQTTParser_ParseUnsubscribe(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create UNSUBSCRIBE packet
	unsubPkt := packets.NewControlPacket(packets.Unsubscribe).(*packets.UnsubscribePacket)
	unsubPkt.Topics = []string{"topic1"}
	unsubPkt.MessageID = 1

	// Serialize packet
	var buf bytes.Buffer
	if err := unsubPkt.Write(&buf); err != nil {
		t.Fatalf("Failed to write UNSUBSCRIBE packet: %v", err)
	}

	// Parse packet
	var outBuf bytes.Buffer
	hctx := &handler.Context{}

	err := p.Parse(context.Background(), &buf, &outBuf, parser.Upstream, mock, hctx)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify handler was called
	if !mock.unsubCalled {
		t.Error("Expected OnUnsubscribe to be called")
	}
}

func TestMQTTParser_ParseDisconnect(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create DISCONNECT packet
	disconnectPkt := packets.NewControlPacket(packets.Disconnect).(*packets.DisconnectPacket)

	// Serialize packet
	var buf bytes.Buffer
	if err := disconnectPkt.Write(&buf); err != nil {
		t.Fatalf("Failed to write DISCONNECT packet: %v", err)
	}

	// Parse packet
	var outBuf bytes.Buffer
	hctx := &handler.Context{}

	err := p.Parse(context.Background(), &buf, &outBuf, parser.Upstream, mock, hctx)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify handler was called
	if !mock.disconnectCalled {
		t.Error("Expected OnDisconnect to be called")
	}
}

func TestMQTTParser_AuthError(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{
		connectErr: errors.New("auth failed"),
	}

	// Create CONNECT packet
	connectPkt := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	connectPkt.ClientIdentifier = "test-client"
	connectPkt.Username = "baduser"
	connectPkt.Password = []byte("badpass")
	connectPkt.UsernameFlag = true
	connectPkt.PasswordFlag = true
	connectPkt.ProtocolName = "MQTT"
	connectPkt.ProtocolVersion = 4

	// Serialize packet
	var buf bytes.Buffer
	if err := connectPkt.Write(&buf); err != nil {
		t.Fatalf("Failed to write CONNECT packet: %v", err)
	}

	// Parse packet - should return error
	var outBuf bytes.Buffer
	hctx := &handler.Context{}

	err := p.Parse(context.Background(), &buf, &outBuf, parser.Upstream, mock, hctx)
	if err == nil {
		t.Error("Expected error from Parse() when auth fails")
	}
}

func TestMQTTParser_InvalidPacket(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Invalid packet data
	buf := bytes.NewReader([]byte{0xFF, 0xFF, 0xFF})

	var outBuf bytes.Buffer
	hctx := &handler.Context{}

	err := p.Parse(context.Background(), buf, &outBuf, parser.Upstream, mock, hctx)
	if err == nil {
		t.Error("Expected error from Parse() with invalid packet")
	}
}

func TestMQTTParser_DownstreamPublish(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create PUBLISH packet from broker
	publishPkt := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	publishPkt.TopicName = "test/topic"
	publishPkt.Payload = []byte("broker message")
	publishPkt.Qos = 0

	// Serialize packet
	var buf bytes.Buffer
	if err := publishPkt.Write(&buf); err != nil {
		t.Fatalf("Failed to write PUBLISH packet: %v", err)
	}

	// Parse packet as downstream
	var outBuf bytes.Buffer
	hctx := &handler.Context{}

	err := p.Parse(context.Background(), &buf, &outBuf, parser.Downstream, mock, hctx)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify packet was forwarded
	if outBuf.Len() == 0 {
		t.Error("Expected packet to be written to output")
	}
}

func TestMQTTParser_ReadError(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create a reader that returns error
	errReader := &errorReader{err: errors.New("read error")}

	var outBuf bytes.Buffer
	hctx := &handler.Context{}

	err := p.Parse(context.Background(), errReader, &outBuf, parser.Upstream, mock, hctx)
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

func TestMQTTParser_WriteError(t *testing.T) {
	p := &Parser{}
	mock := &mockHandler{}

	// Create PINGREQ packet (simple packet)
	pingPkt := packets.NewControlPacket(packets.Pingreq)

	var buf bytes.Buffer
	if err := pingPkt.Write(&buf); err != nil {
		t.Fatalf("Failed to write PINGREQ packet: %v", err)
	}

	// Create a writer that returns error
	errWriter := &errorWriter{err: errors.New("write error")}

	hctx := &handler.Context{}

	err := p.Parse(context.Background(), &buf, errWriter, parser.Upstream, mock, hctx)
	if err == nil {
		t.Error("Expected error from Parse() with failing writer")
	}
}

// errorWriter is a writer that always returns an error.
type errorWriter struct {
	err error
}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	return 0, e.err
}
