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

package cozeloop

import (
	"context"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"

	"github.com/coze-dev/cozeloop-go"
)

func NewLoopHandler(client cozeloop.Client, opts ...Option) callbacks.Handler {
	var handler callbacks.Handler

	o := &options{
		enableTracing: true,
	}

	for _, opt := range opts {
		opt(o)
	}

	if o.enableTracing {
		handler = newTraceCallbackHandler(client, o)
	}

	return &Handler{
		Client:  client,
		handler: handler,
	}
}

type Handler struct {
	cozeloop.Client

	// internal fields
	handler callbacks.Handler
}

func (h *Handler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if h == nil {
		return ctx
	}
	info = completeRunInfo(info)
	if h.handler != nil {
		ctx = h.handler.OnStart(ctx, info, input)
	}

	return ctx
}

func (h *Handler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if h == nil {
		return ctx
	}
	info = completeRunInfo(info)
	if h.handler != nil {
		ctx = h.handler.OnEnd(ctx, info, output)
	}

	return ctx
}

func (h *Handler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if h == nil {
		return ctx
	}
	info = completeRunInfo(info)
	if h.handler != nil {
		ctx = h.handler.OnError(ctx, info, err)
	}

	return ctx
}

func (h *Handler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	if h == nil {
		return ctx
	}
	info = completeRunInfo(info)
	if h.handler == nil {
		input.Close()
		return ctx
	}

	ctx = h.handler.OnStartWithStreamInput(ctx, info, input)

	return ctx
}

func (h *Handler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	if h == nil {
		return ctx
	}
	info = completeRunInfo(info)
	if h.handler == nil {
		output.Close()
		return ctx
	}

	ctx = h.handler.OnEndWithStreamOutput(ctx, info, output)

	return ctx
}

func completeRunInfo(info *callbacks.RunInfo) *callbacks.RunInfo {
	if info != nil && len(info.Name) == 0 {
		nInfo := *info
		nInfo.Name = nInfo.Type + string(nInfo.Component)
		return &nInfo
	}
	return info
}
