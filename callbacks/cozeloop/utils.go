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
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/callbacks/cozeloop/internal/async"
	"github.com/cloudwego/eino-ext/callbacks/cozeloop/internal/consts"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
)

func getErrorTags(_ context.Context, err error) spanTags {
	return make(spanTags).
		set(tracespec.Error, err.Error())
}

type spanTags map[string]any

func (t spanTags) setTags(kv map[string]any) spanTags {
	for k, v := range kv {
		t.set(k, v)
	}

	return t
}

func (t spanTags) set(key string, value any) spanTags {
	if t == nil || value == nil {
		return t
	}

	if _, found := t[key]; found {
		return t
	}

	switch k := reflect.TypeOf(value).Kind(); k {
	case reflect.Array,
		reflect.Interface,
		reflect.Map,
		reflect.Pointer,
		reflect.Slice,
		reflect.Struct:
		value = toJson(value, false)
	default:

	}

	t[key] = value

	return t
}

func (t spanTags) setIfNotZero(key string, val any) {
	if val == nil {
		return
	}

	rv := reflect.ValueOf(val)
	if rv.IsValid() && rv.IsZero() {
		return
	}

	t.set(key, val)
}

func (t spanTags) setFromExtraIfNotZero(key string, extra map[string]any) {
	if extra == nil {
		return
	}

	t.setIfNotZero(key, extra[key])
}

func setTraceVariablesValue(ctx context.Context, val *async.TraceVariablesValue) context.Context {
	if val == nil {
		return ctx
	}

	return context.WithValue(ctx, async.TraceVariablesKey{}, val)
}

func getTraceVariablesValue(ctx context.Context) (*async.TraceVariablesValue, bool) {
	val, ok := ctx.Value(async.TraceVariablesKey{}).(*async.TraceVariablesValue)
	return val, ok
}

func toJson(v any, bStream bool) string {
	if v == nil {
		return fmt.Sprintf("%s", errors.New("try to marshal nil error"))
	}
	if bStream {
		v = map[string]any{"stream": v}
	}
	b, err := sonic.MarshalString(v)
	if err != nil {
		return fmt.Sprintf("%s", err.Error())
	}
	return b
}

func getGraphNodeLevelFromCtx(ctx context.Context) int64 {
	level, ok := ctx.Value(consts.CozeLoopGraphNodeLevel).(int64)
	if ok {
		return level
	}

	return 0
}

func injectGraphNodeLevelToCtx(ctx context.Context, level int64) context.Context {
	return context.WithValue(ctx, consts.CozeLoopGraphNodeLevel, level)
}

func injectAggrMessageOutputHookToCtx(ctx context.Context) context.Context {
	_, ok := ctx.Value(consts.CozeLoopAggrMessageOutput).(*AggrMessageOutput)
	if !ok {
		ctx = context.WithValue(ctx, consts.CozeLoopAggrMessageOutput, &AggrMessageOutput{
			Messages: make([]*tracespec.ModelMessage, 0),
			mutex:    sync.Mutex{},
		})
	}

	return ctx
}

func getAggrMessageOutputHookFromCtx(ctx context.Context) (*AggrMessageOutput, bool) {
	temp, ok := ctx.Value(consts.CozeLoopAggrMessageOutput).(*AggrMessageOutput)
	return temp, ok
}

func injectToolIDNameMapToCtx(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if info.Component == compose.ComponentOfToolsNode {
		message, ok := input.(*schema.Message)
		if ok {
			toolIDNameMap := make(map[string]string)
			for _, toolCall := range message.ToolCalls {
				toolIDNameMap[toolCall.ID] = toolCall.Function.Name
			}
			ctx = context.WithValue(ctx, consts.CozeLoopToolIDNameMap, toolIDNameMap)
		}
	}

	return ctx
}

func getToolIDNameMapFromCtx(ctx context.Context) map[string]string {
	temp, ok := ctx.Value(consts.CozeLoopToolIDNameMap).(map[string]string)
	if ok {
		return temp
	}

	return nil
}
