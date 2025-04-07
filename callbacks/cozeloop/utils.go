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

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/callbacks/cozeloop/internal/async"
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
