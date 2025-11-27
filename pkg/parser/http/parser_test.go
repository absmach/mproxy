// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/absmach/mproxy/pkg/handler"
	"github.com/absmach/mproxy/pkg/parser"
)

type mockHandler struct {
	connectErr      error
	publishErr      error
	connectCalled   bool
	publishCalled   bool
	onConnectCalled bool
	onPublishCalled bool
	lastHctx        *handler.Context
	lastTopic       string
	lastPayload     []byte
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
	return nil
}

func (m *mockHandler) OnConnect(ctx context.Context, hctx *handler.Context) error {
	m.onConnectCalled = true
	return nil
}

func (m *mockHandler) OnPublish(ctx context.Context, hctx *handler.Context, topic string, payload []byte) error {
	m.onPublishCalled = true
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

func TestNewParser_ValidURL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{}

	p, err := NewParser("http://localhost:8080", mock, logger)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	if p == nil {
		t.Fatal("Expected parser to be non-nil")
	}

	if p.handler != mock {
		t.Error("Expected handler to be set correctly")
	}
}

func TestNewParser_InvalidURL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{}

	_, err := NewParser("://invalid-url", mock, logger)
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestNewParser_NilLogger(t *testing.T) {
	mock := &mockHandler{}

	p, err := NewParser("http://localhost:8080", mock, nil)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	if p.logger == nil {
		t.Error("Expected logger to be set to default")
	}
}

func TestHTTPParser_BasicAuth(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{}

	p, err := NewParser(backend.URL, mock, logger)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.SetBasicAuth("testuser", "testpass")

	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if !mock.connectCalled {
		t.Error("Expected AuthConnect to be called")
	}

	if mock.lastHctx.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", mock.lastHctx.Username)
	}

	if string(mock.lastHctx.Password) != "testpass" {
		t.Errorf("Expected password 'testpass', got '%s'", string(mock.lastHctx.Password))
	}

	if mock.lastHctx.Protocol != "http" {
		t.Errorf("Expected protocol 'http', got '%s'", mock.lastHctx.Protocol)
	}
}

func TestHTTPParser_QueryParamAuth(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{}

	p, err := NewParser(backend.URL, mock, logger)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test?authorization=token123", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if !mock.connectCalled {
		t.Error("Expected AuthConnect to be called")
	}

	if string(mock.lastHctx.Password) != "token123" {
		t.Errorf("Expected password 'token123', got '%s'", string(mock.lastHctx.Password))
	}
}

func TestHTTPParser_HeaderAuth(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{}

	p, err := NewParser(backend.URL, mock, logger)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer token456")
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if !mock.connectCalled {
		t.Error("Expected AuthConnect to be called")
	}

	if string(mock.lastHctx.Password) != "Bearer token456" {
		t.Errorf("Expected password 'Bearer token456', got '%s'", string(mock.lastHctx.Password))
	}
}

func TestHTTPParser_POST_AuthPublish(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{}

	p, err := NewParser(backend.URL, mock, logger)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	body := strings.NewReader("test payload")
	req := httptest.NewRequest(http.MethodPost, "/api/data", body)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if !mock.connectCalled {
		t.Error("Expected AuthConnect to be called")
	}

	if !mock.publishCalled {
		t.Error("Expected AuthPublish to be called for POST")
	}

	if !mock.onPublishCalled {
		t.Error("Expected OnPublish to be called")
	}

	if mock.lastTopic != "/api/data" {
		t.Errorf("Expected topic '/api/data', got '%s'", mock.lastTopic)
	}

	if string(mock.lastPayload) != "test payload" {
		t.Errorf("Expected payload 'test payload', got '%s'", string(mock.lastPayload))
	}
}

func TestHTTPParser_PUT_AuthPublish(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{}

	p, err := NewParser(backend.URL, mock, logger)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	body := strings.NewReader("update payload")
	req := httptest.NewRequest(http.MethodPut, "/api/data/1", body)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if !mock.publishCalled {
		t.Error("Expected AuthPublish to be called for PUT")
	}
}

func TestHTTPParser_PATCH_AuthPublish(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{}

	p, err := NewParser(backend.URL, mock, logger)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	body := strings.NewReader("patch payload")
	req := httptest.NewRequest(http.MethodPatch, "/api/data/1", body)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if !mock.publishCalled {
		t.Error("Expected AuthPublish to be called for PATCH")
	}
}

func TestHTTPParser_GET_NoPublish(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{}

	p, err := NewParser(backend.URL, mock, logger)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if mock.publishCalled {
		t.Error("Did not expect AuthPublish to be called for GET")
	}

	if !mock.onConnectCalled {
		t.Error("Expected OnConnect to be called")
	}
}

func TestHTTPParser_AuthConnectFailure(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{
		connectErr: errors.New("auth failed"),
	}

	p, err := NewParser(backend.URL, mock, logger)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	if !mock.connectCalled {
		t.Error("Expected AuthConnect to be called")
	}

	if mock.onConnectCalled {
		t.Error("Did not expect OnConnect to be called after auth failure")
	}
}

func TestHTTPParser_AuthPublishFailure(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{
		publishErr: errors.New("publish auth failed"),
	}

	p, err := NewParser(backend.URL, mock, logger)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	body := strings.NewReader("test payload")
	req := httptest.NewRequest(http.MethodPost, "/api/data", body)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, rec.Code)
	}

	if !mock.publishCalled {
		t.Error("Expected AuthPublish to be called")
	}

	if mock.onPublishCalled {
		t.Error("Did not expect OnPublish to be called after auth failure")
	}
}

func TestHTTPParser_ParseNotSupported(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{}

	p, err := NewParser("http://localhost:8080", mock, logger)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	var buf bytes.Buffer
	err = p.Parse(context.Background(), &buf, &buf, parser.Upstream, mock, &handler.Context{})
	if err == nil {
		t.Error("Expected error from Parse() method")
	}

	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("Expected 'not supported' error, got: %v", err)
	}
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

func TestHTTPParser_BodyReadError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockHandler{}

	p, err := NewParser(backend.URL, mock, logger)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/test", &errorReader{err: io.ErrUnexpectedEOF})
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
