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
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/callbacks/cozeloop/internal/async"
	"github.com/cloudwego/eino-ext/callbacks/cozeloop/internal/consts"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/cozeloop-go"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
)

func newTraceCallbackHandler(client cozeloop.Client, o *options) callbacks.Handler {
	tracer := &einoTracer{
		client: client,
		parser: newDefaultDataParserWithConcatFuncs(o.concatFuncs, o.enableAggrOutput),
		logger: o.logger,
	}

	if o.parser != nil {
		tracer.parser = o.parser
	}

	rt := &tracespec.Runtime{
		Language: "go",
		Library:  tracespec.VLibEino,
	}

	if o.einoVersionFn != nil {
		rt.LibraryVersion = o.einoVersionFn()
	} else {
		rt.LibraryVersion = readBuildVersion()
	}
	tracer.runtime = rt

	if o.logger == nil {
		o.logger = cozeloop.GetLogger()
	}

	return tracer
}

type einoTracer struct {
	client  cozeloop.Client
	parser  CallbackDataParser
	runtime *tracespec.Runtime
	logger  cozeloop.Logger
}

type AggrMessageOutput struct {
	Messages []*tracespec.ModelMessage `json:"messages"`
	mutex    sync.Mutex
}

func (a *AggrMessageOutput) addMessages(messages ...*tracespec.ModelMessage) {
	if messages == nil || len(messages) == 0 {
		return
	}
	if a.Messages == nil {
		a.Messages = make([]*tracespec.ModelMessage, 0)
	}
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.Messages = append(a.Messages, messages...)
}

func (l *einoTracer) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if info == nil {
		return ctx
	}

	ctx = injectAggrMessageOutputHookToCtx(ctx)
	ctx = injectGraphNodeLevelToCtx(ctx, getGraphNodeLevelFromCtx(ctx)+1)
	ctx = injectToolIDNameMapToCtx(ctx, info, input)

	spanName := info.Name
	if spanName == "" {
		spanName = string(info.Component)
	}

	ctx, span := l.client.StartSpan(ctx, spanName, parseSpanTypeFromComponent(info.Component))

	l.setRunInfo(ctx, span, info)

	if l.parser != nil {
		span.SetTags(ctx, l.parser.ParseInput(ctx, info, input))
	}

	return setTraceVariablesValue(ctx, &async.TraceVariablesValue{
		StartTime: time.Now(),
	})
}

func (l *einoTracer) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	if info == nil {
		return ctx
	}

	span := l.client.GetSpanFromContext(ctx)
	if span == nil {
		l.logger.CtxWarnf(ctx, "[einoTracer][OnEnd] span not found in callback ctx")
		return ctx
	}

	var tags map[string]any
	if l.parser != nil {
		tags = l.parser.ParseOutput(ctx, info, output)
	}

	if stopCh, ok := ctx.Value(async.TraceStreamInputAsyncKey{}).(async.StreamInputAsyncVal); ok {
		<-stopCh
	}

	span.SetTags(ctx, tags)

	span.Finish(ctx)

	return ctx
}

func (l *einoTracer) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	if info == nil {
		return ctx
	}

	span := l.client.GetSpanFromContext(ctx)
	if span == nil {
		l.logger.CtxWarnf(ctx, "[einoTracer][OnError] span not found in callback ctx")
		return ctx
	}

	tags := getErrorTags(ctx, err)

	if stopCh, ok := ctx.Value(async.TraceStreamInputAsyncKey{}).(async.StreamInputAsyncVal); ok {
		<-stopCh
	}

	span.SetTags(ctx, tags)

	span.Finish(ctx)

	return ctx
}

func (l *einoTracer) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	if info == nil {
		input.Close()
		return ctx
	}

	ctx = injectAggrMessageOutputHookToCtx(ctx)
	ctx = injectGraphNodeLevelToCtx(ctx, getGraphNodeLevelFromCtx(ctx)+1)

	spanName := info.Name
	if spanName == "" {
		spanName = string(info.Component)
	}

	ctx, span := l.client.StartSpan(ctx, spanName, parseSpanTypeFromComponent(info.Component))
	stopCh := make(async.StreamInputAsyncVal)
	ctx = context.WithValue(ctx, async.TraceStreamInputAsyncKey{}, stopCh)

	l.setRunInfo(ctx, span, info)

	if l.parser != nil {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					l.logger.CtxWarnf(ctx, "[einoTracer][OnStartWithStreamInput] recovered: %s", e)
				}

				close(stopCh)
			}()

			span.SetTags(ctx, l.parser.ParseStreamInput(ctx, info, input))
		}()
	} else {
		input.Close()
		close(stopCh)
	}

	return setTraceVariablesValue(ctx, &async.TraceVariablesValue{
		StartTime: time.Now(),
	})
}

func (l *einoTracer) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	if info == nil {
		output.Close()
		return ctx
	}

	span := l.client.GetSpanFromContext(ctx)
	if span == nil {
		l.logger.CtxWarnf(ctx, "[einoTracer][OnEndWithStreamOutput] span not found in callback ctx")
		return ctx
	}

	if l.parser != nil {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					l.logger.CtxWarnf(ctx, "[einoTracer][OnEndWithStreamOutput] recovered: %s", e)
				}
			}()

			tags := l.parser.ParseStreamOutput(ctx, info, output)

			if stopCh, ok := ctx.Value(async.TraceStreamInputAsyncKey{}).(async.StreamInputAsyncVal); ok {
				<-stopCh
			}

			span.SetTags(ctx, tags)

			span.Finish(ctx)
		}()
	} else {
		output.Close()
		if stopCh, ok := ctx.Value(async.TraceStreamInputAsyncKey{}).(async.StreamInputAsyncVal); ok {
			<-stopCh
		}

		span.Finish(ctx)
	}

	return ctx
}

func (l *einoTracer) setRunInfo(ctx context.Context, span cozeloop.Span, info *callbacks.RunInfo) {
	span.SetTags(ctx, make(spanTags).
		set(consts.CustomSpanTagKeyComponent, string(info.Component)).
		set(consts.CustomSpanTagKeyName, info.Name).
		set(consts.CustomSpanTagKeyType, info.Type).
		set(tracespec.Runtime_, l.runtime),
	)
	if l.runtime != nil {
		span.SetRuntime(ctx, *l.runtime)
	}
}
