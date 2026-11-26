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

package core

import (
	"context"
	"encoding/json"
)

// client side middleware

type CallEndpoint func(ctx context.Context, method string, req, resp interface{}) error

type CallMiddleware func(CallEndpoint) CallEndpoint

func callChain(mws ...CallMiddleware) CallMiddleware {
	return func(endpoint CallEndpoint) CallEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			endpoint = mws[i](endpoint)
		}
		return endpoint
	}
}

// server side middleware

type HandleEndpoint func(ctx context.Context, conn Connection, req json.RawMessage) (interface{}, error)

type HandleMiddleware func(HandleEndpoint) HandleEndpoint

func handleChain(mws ...HandleMiddleware) HandleMiddleware {
	return func(endpoint HandleEndpoint) HandleEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			endpoint = mws[i](endpoint)
		}
		return endpoint
	}
}

type NotificationHandleEndpoint func(ctx context.Context, conn Connection, params json.RawMessage) error

type RequestHandleEndpoint requestHandlerFunc
