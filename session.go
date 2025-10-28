package mgate

import (
	"context"
	"crypto/x509"
	"time"
)

// ConnectionInfo holds metadata about the connection
type ConnectionInfo struct {
	StartTime time.Time
	Client    ClientSession
	Server    ServerSession
}

// ClientSession stores client session data.
type ClientSession struct {
	ID       string
	Username string
	Secret   []byte
	Addr     string
	Certs    []*x509.Certificate
}

type ServerSession struct {
	Addr  string
	Certs []*x509.Certificate
}

// The sessionKey type is unexported to prevent collisions with context keys defined in
// other packages.
type sessionKey struct{}

// NewContext stores Session in context.Context values.
// It uses pointer to the session so it can be modified by handler.
func NewContext(ctx context.Context, s *ClientSession) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

// FromContext retrieves Session from context.Context.
// Second value indicates if session is present in the context
// and if it's safe to use it (it's not nil).
func FromContext(ctx context.Context) (*ClientSession, bool) {
	if s, ok := ctx.Value(sessionKey{}).(*ClientSession); ok && s != nil {
		return s, true
	}
	return nil, false
}
