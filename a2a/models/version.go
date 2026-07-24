/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package models

// ProtocolVersion identifies an A2A protocol wire version.
//
// eino-ext supports two wire versions simultaneously so that existing
// deployments can migrate from v0.3 to v1.0 without a flag day:
//   - ProtocolVersion03 is the legacy "category/action" JSON-RPC binding
//     (message/send, tasks/get, ...) used by A2A v0.2.x/v0.3.
//   - ProtocolVersion10 is the A2A v1.0 binding whose JSON-RPC method names
//     are aligned with the a2a.proto service definitions (SendMessage, ...).
//
// The Go-facing data structures in this package stay in their v0.3 shape;
// the version only affects how requests/responses are serialized on the wire,
// which is handled by the transport-level codec.
type ProtocolVersion string

const (
	// ProtocolVersion03 is the A2A v0.3 (a.k.a. 0.2.x) JSON-RPC binding.
	ProtocolVersion03 ProtocolVersion = "0.3"
	// ProtocolVersion10 is the A2A v1.0 JSON-RPC binding.
	ProtocolVersion10 ProtocolVersion = "1.0"
)

// HeaderA2AVersion is the HTTP header a client sends to declare which A2A
// protocol version its request bodies use. An absent/empty header is
// interpreted by servers as ProtocolVersion03 for backward compatibility.
const HeaderA2AVersion = "A2A-Version"
