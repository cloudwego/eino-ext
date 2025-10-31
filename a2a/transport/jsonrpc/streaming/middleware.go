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

package streaming

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
)

type ServerStreamingCallEndpoint func(ctx context.Context, method string, req interface{}) (ServerStreamingClient, error)

type ServerStreamingCallMiddleware func(ServerStreamingCallEndpoint) ServerStreamingCallEndpoint

func serverStreamingCallChain(mws ...ServerStreamingCallMiddleware) ServerStreamingCallMiddleware {
	return func(endpoint ServerStreamingCallEndpoint) ServerStreamingCallEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			endpoint = mws[i](endpoint)
		}
		return endpoint
	}
}

type ServerStreamingHandleEndpoint func(ctx context.Context, conn core.Connection, req json.RawMessage, srv ServerStreamingServer) error

type ServerStreamingHandleMiddleware func(ServerStreamingHandleEndpoint) ServerStreamingHandleEndpoint

func serverStreamingHandleChain(mws ...ServerStreamingHandleMiddleware) ServerStreamingHandleMiddleware {
	return func(endpoint ServerStreamingHandleEndpoint) ServerStreamingHandleEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			endpoint = mws[i](endpoint)
		}
		return endpoint
	}
}
