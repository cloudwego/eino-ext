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

	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc/core"
)

type Options struct {
	ssHdls map[string]ServerStreamingHandleEndpoint

	ssCallMWs []ServerStreamingCallMiddleware
	ssHdlMWs  []ServerStreamingHandleMiddleware
}

type Option func(options *Options)

func WithServerStreamingHandler(method string, ep ServerStreamingHandleEndpoint) Option {
	return func(options *Options) {
		if options.ssHdls == nil {
			options.ssHdls = make(map[string]ServerStreamingHandleEndpoint)
		}
		options.ssHdls[method] = ep
	}
}

func ConvertConnectionOption(opts ...Option) []core.Option {
	var res []core.Option
	options := new(Options)
	for _, opt := range opts {
		opt(options)
	}

	for method, ssHdl := range options.ssHdls {
		hdl := ssHdl
		hdl = serverStreamingHandleChain(options.ssHdlMWs...)(hdl)
		res = append(res, core.WithRequestHandler(method, func(ctx context.Context, conn core.Connection, req *core.Request, async core.ServerAsync) error {
			return convertRequestHandler(hdl)(ctx, conn, req, async)
		}))
	}

	return res
}

func convertRequestHandler(ep ServerStreamingHandleEndpoint) core.RequestHandleEndpoint {
	return func(ctx context.Context, conn core.Connection, req *core.Request, async core.ServerAsync) (err error) {
		defer func() {
			if rawPanic := recover(); rawPanic != nil {
				err = core.ConvertError(rawPanic.(error))
				async.FinishStreaming(ctx, err)
			}
		}()
		if epErr := ep(ctx, conn, req.Params, &serverStreamingServer{async: async}); epErr != nil {
			err = core.ConvertError(epErr)
			// when endpoint throws error, we just ignore the error thrown by SendStreaming
			async.FinishStreaming(ctx, err)
			return
		}
		return async.FinishStreaming(ctx, nil)
	}
}
