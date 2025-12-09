// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

// Package coap implements the CoAP protocol parser for mproxy.
//
// # Overview
//
// The CoAP parser inspects CoAP messages to extract authentication credentials
// and authorize protocol operations. It uses the plgd-dev/go-coap/v3 library
// for message parsing and supports CoAP over UDP.
//
// # Message Handling
//
// Upstream (Client → Backend):
//   - POST: Extracts auth from query, calls AuthConnect and AuthPublish
//   - PUT: Extracts auth from query, calls AuthConnect and AuthPublish
//   - GET: Extracts auth from query, calls AuthConnect
//   - GET with Observe: Also calls AuthSubscribe
//   - DELETE: Calls AuthConnect only
//
// Downstream (Backend → Client):
//   - All messages forwarded without modification
//
// # Authentication
//
// CoAP authentication is extracted from the "auth" query parameter:
//
//	coap://localhost:5683/channels/123/messages?auth=token123
//
// The auth token is stored in hctx.Password (as []byte) and can be used
// by the handler for authorization.
//
// # Publish Flow (POST/PUT)
//
//  1. Client sends POST/PUT message
//  2. Parser extracts path and payload
//  3. Parser extracts auth token from query
//  4. Parser calls handler.AuthConnect()
//  5. Parser calls handler.AuthPublish()
//  6. If authorized, message forwarded to backend
//
// # Subscribe Flow (GET with Observe)
//
//  1. Client sends GET with Observe option
//  2. Parser extracts path
//  3. Parser extracts auth token from query
//  4. Parser calls handler.AuthConnect()
//  5. Parser calls handler.AuthSubscribe()
//  6. If authorized, message forwarded to backend
//
// # Path as Topic
//
// CoAP uses the URI path as the "topic" equivalent:
//   - Path "/channels/123/messages" → topic "channels/123/messages"
//   - Used in AuthPublish and AuthSubscribe calls
//
// # Protocol Field
//
// The parser sets hctx.Protocol = "coap" for all CoAP connections.
//
// # Limitations
//
// This is a simplified CoAP parser focused on authorization:
//   - Does not handle blockwise transfers
//   - Does not cache observe relationships
//   - Auth token extracted only from query parameter
//   - Does not support DTLS credential extraction
package coap
