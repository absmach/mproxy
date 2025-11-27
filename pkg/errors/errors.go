// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package errors provides structured error handling for mProxy.
package errors

import (
	"errors"
	"fmt"
)

// Common error types
var (
	// ErrUnauthorized indicates authentication or authorization failure.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrInvalidInput indicates invalid input data.
	ErrInvalidInput = errors.New("invalid input")

	// ErrTimeout indicates an operation timeout.
	ErrTimeout = errors.New("timeout")

	// ErrConnectionClosed indicates the connection was closed.
	ErrConnectionClosed = errors.New("connection closed")

	// ErrProtocolViolation indicates a protocol-level error.
	ErrProtocolViolation = errors.New("protocol violation")

	// ErrBackendUnavailable indicates the backend is unavailable.
	ErrBackendUnavailable = errors.New("backend unavailable")

	// ErrRateLimited indicates rate limit exceeded.
	ErrRateLimited = errors.New("rate limit exceeded")

	// ErrSizeLimitExceeded indicates size limit exceeded.
	ErrSizeLimitExceeded = errors.New("size limit exceeded")

	// ErrInvalidOrigin indicates invalid WebSocket origin.
	ErrInvalidOrigin = errors.New("invalid origin")
)

// ProxyError wraps an error with additional context.
type ProxyError struct {
	Op        string // Operation that failed
	Protocol  string // Protocol (mqtt, http, coap, websocket)
	SessionID string // Session identifier
	RemoteAddr string // Client address
	Err       error  // Underlying error
}

// Error implements the error interface.
func (e *ProxyError) Error() string {
	if e.SessionID != "" {
		return fmt.Sprintf("%s %s [%s] %s: %v", e.Protocol, e.Op, e.SessionID, e.RemoteAddr, e.Err)
	}
	return fmt.Sprintf("%s %s %s: %v", e.Protocol, e.Op, e.RemoteAddr, e.Err)
}

// Unwrap returns the underlying error.
func (e *ProxyError) Unwrap() error {
	return e.Err
}

// New creates a new ProxyError.
func New(op, protocol, sessionID, remoteAddr string, err error) error {
	if err == nil {
		return nil
	}
	return &ProxyError{
		Op:        op,
		Protocol:  protocol,
		SessionID: sessionID,
		RemoteAddr: remoteAddr,
		Err:       err,
	}
}

// Wrap wraps an error with context.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
