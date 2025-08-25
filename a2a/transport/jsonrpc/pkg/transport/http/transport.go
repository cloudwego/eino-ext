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

package http

import "github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"

type httpClientTransport struct {
	rounder *clientRounder
}

func (h *httpClientTransport) ClientCapability() (core.ClientRounder, bool) {
	return h.rounder, true
}

func (h *httpClientTransport) ServerCapability() (core.ServerRounder, bool) {
	return nil, false
}

type httpServerTransport struct {
	rounder *serverRounder
}

func (h *httpServerTransport) ClientCapability() (core.ClientRounder, bool) {
	return nil, false
}

func (h *httpServerTransport) ServerCapability() (core.ServerRounder, bool) {
	return h.rounder, true
}
