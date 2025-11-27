// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package http implements the HTTP protocol parser for mproxy.
//
// # Overview
//
// The HTTP parser uses Go's httputil.ReverseProxy to handle HTTP requests
// and responses. It extracts authentication credentials from various sources
// and authorizes HTTP operations.
//
// # Authentication Sources
//
// The parser extracts credentials from multiple sources (in order of precedence):
//
//  1. HTTP Basic Auth header:
//     Authorization: Basic base64(username:password)
//
//  2. Authorization query parameter:
//     /path?authorization=token123
//
//  3. Authorization header (Bearer token):
//     Authorization: Bearer token123
//
// # Request Flow
//
//	1. Client sends HTTP request
//	2. Parser extracts auth credentials
//	3. Parser calls handler.AuthConnect()
//	4. For POST/PUT/PATCH:
//	   - Parser reads request body
//	   - Parser calls handler.AuthPublish()
//	   - Handler can modify body
//	5. Request forwarded to backend via reverse proxy
//	6. Backend sends response
//	7. Response forwarded to client
//
// # Method Mapping
//
// HTTP methods are mapped to handler operations:
//   - GET: AuthConnect only (read operation)
//   - POST, PUT, PATCH: AuthConnect + AuthPublish (write operation)
//   - DELETE: AuthConnect only
//   - HEAD, OPTIONS: AuthConnect only
//
// # Path as Topic
//
// The HTTP request path is used as the "topic" in AuthPublish:
//   - POST /channels/123/messages → topic "/channels/123/messages"
//
// # Body Modification
//
// The handler can modify the request body during AuthPublish:
//
//	func (h *MyHandler) AuthPublish(ctx context.Context, hctx *handler.Context, topic *string, payload *[]byte) error {
//		// Verify authorization
//		if !h.auth.CanPublish(hctx.Username, *topic) {
//			return errors.New("forbidden")
//		}
//		// Modify payload
//		*payload = append(*payload, []byte(" [modified]")...)
//		return nil
//	}
//
// # Protocol Field
//
// The parser sets hctx.Protocol = "http" for all HTTP connections.
//
// # Reverse Proxy Features
//
// The parser uses httputil.ReverseProxy, which provides:
//   - Automatic header forwarding
//   - Connection pooling
//   - WebSocket upgrade support
//   - Error handling
package http
