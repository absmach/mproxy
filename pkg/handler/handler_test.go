// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"errors"
	"testing"
)

func TestNoopHandler(t *testing.T) {
	handler := &NoopHandler{}
	ctx := context.Background()
	hctx := &Context{
		SessionID:  "test-session",
		Username:   "testuser",
		Password:   []byte("testpass"),
		ClientID:   "client123",
		RemoteAddr: "127.0.0.1:1234",
		Protocol:   "mqtt",
	}

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "AuthConnect",
			fn:   func() error { return handler.AuthConnect(ctx, hctx) },
		},
		{
			name: "AuthPublish",
			fn: func() error {
				topic := "test/topic"
				payload := []byte("test payload")
				return handler.AuthPublish(ctx, hctx, &topic, &payload)
			},
		},
		{
			name: "AuthSubscribe",
			fn: func() error {
				topics := []string{"test/topic"}
				return handler.AuthSubscribe(ctx, hctx, &topics)
			},
		},
		{
			name: "OnConnect",
			fn:   func() error { return handler.OnConnect(ctx, hctx) },
		},
		{
			name: "OnPublish",
			fn:   func() error { return handler.OnPublish(ctx, hctx, "test/topic", []byte("payload")) },
		},
		{
			name: "OnSubscribe",
			fn:   func() error { return handler.OnSubscribe(ctx, hctx, []string{"test/topic"}) },
		},
		{
			name: "OnUnsubscribe",
			fn:   func() error { return handler.OnUnsubscribe(ctx, hctx, []string{"test/topic"}) },
		},
		{
			name: "OnDisconnect",
			fn:   func() error { return handler.OnDisconnect(ctx, hctx) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err != nil {
				t.Errorf("%s() returned error: %v", tt.name, err)
			}
		})
	}
}

// MockHandler is a mock implementation for testing.
type MockHandler struct {
	ConnectErr     error
	PublishErr     error
	SubscribeErr   error
	OnConnectErr   error
	OnPublishErr   error
	OnSubscribeErr error

	ConnectCalled      bool
	PublishCalled      bool
	SubscribeCalled    bool
	OnConnectCalled    bool
	OnPublishCalled    bool
	OnSubscribeCalled  bool
	OnUnsubCalled      bool
	OnDisconnectCalled bool

	LastTopic   string
	LastPayload []byte
	LastTopics  []string
}

func (m *MockHandler) AuthConnect(ctx context.Context, hctx *Context) error {
	m.ConnectCalled = true
	return m.ConnectErr
}

func (m *MockHandler) AuthPublish(ctx context.Context, hctx *Context, topic *string, payload *[]byte) error {
	m.PublishCalled = true
	m.LastTopic = *topic
	m.LastPayload = *payload
	return m.PublishErr
}

func (m *MockHandler) AuthSubscribe(ctx context.Context, hctx *Context, topics *[]string) error {
	m.SubscribeCalled = true
	m.LastTopics = *topics
	return m.SubscribeErr
}

func (m *MockHandler) OnConnect(ctx context.Context, hctx *Context) error {
	m.OnConnectCalled = true
	return m.OnConnectErr
}

func (m *MockHandler) OnPublish(ctx context.Context, hctx *Context, topic string, payload []byte) error {
	m.OnPublishCalled = true
	return m.OnPublishErr
}

func (m *MockHandler) OnSubscribe(ctx context.Context, hctx *Context, topics []string) error {
	m.OnSubscribeCalled = true
	return m.OnSubscribeErr
}

func (m *MockHandler) OnUnsubscribe(ctx context.Context, hctx *Context, topics []string) error {
	m.OnUnsubCalled = true
	return nil
}

func (m *MockHandler) OnDisconnect(ctx context.Context, hctx *Context) error {
	m.OnDisconnectCalled = true
	return nil
}

func TestMockHandler(t *testing.T) {
	mock := &MockHandler{
		ConnectErr: errors.New("connection error"),
	}

	ctx := context.Background()
	hctx := &Context{
		SessionID: "test",
		Username:  "user",
	}

	// Test AuthConnect with error
	err := mock.AuthConnect(ctx, hctx)
	if err == nil {
		t.Error("Expected error from AuthConnect")
	}
	if !mock.ConnectCalled {
		t.Error("Expected ConnectCalled to be true")
	}

	// Test AuthPublish
	mock.PublishErr = nil
	topic := "test/topic"
	payload := []byte("test payload")
	err = mock.AuthPublish(ctx, hctx, &topic, &payload)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !mock.PublishCalled {
		t.Error("Expected PublishCalled to be true")
	}
	if mock.LastTopic != topic {
		t.Errorf("Expected topic %s, got %s", topic, mock.LastTopic)
	}
	if string(mock.LastPayload) != string(payload) {
		t.Errorf("Expected payload %s, got %s", payload, mock.LastPayload)
	}

	// Test AuthSubscribe
	topics := []string{"topic1", "topic2"}
	err = mock.AuthSubscribe(ctx, hctx, &topics)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !mock.SubscribeCalled {
		t.Error("Expected SubscribeCalled to be true")
	}
	if len(mock.LastTopics) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(mock.LastTopics))
	}

	// Test notification methods
	err = mock.OnConnect(ctx, hctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !mock.OnConnectCalled {
		t.Error("Expected OnConnectCalled to be true")
	}

	err = mock.OnDisconnect(ctx, hctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !mock.OnDisconnectCalled {
		t.Error("Expected OnDisconnectCalled to be true")
	}
}
