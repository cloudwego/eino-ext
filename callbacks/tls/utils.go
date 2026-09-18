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

package tls

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/bytedance/sonic"
	sem_ai "github.com/cloudwego/eino-ext/callbacks/tls/semconv"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	DefaultVersion  = "1.0.1"
	DefaultSpanName = "unset"
)

func getName(info *callbacks.RunInfo) string {
	if info == nil {
		return ""
	}
	if len(info.Name) != 0 {
		return info.Name
	}
	return strings.TrimSpace(info.Type + " " + string(info.Component))
}

func completeRunInfo(info *callbacks.RunInfo) *callbacks.RunInfo {
	if info != nil && len(info.Name) == 0 {
		nInfo := *info
		nInfo.Name = getName(info)
		return &nInfo
	}

	return info
}

func convModelCallbackInput(in []callbacks.CallbackInput) []*model.CallbackInput {
	ret := make([]*model.CallbackInput, len(in))
	for i, c := range in {
		ret[i] = model.ConvCallbackInput(c)
	}
	return ret
}

func extractModelInput(ins []*model.CallbackInput) (config *model.Config, messages []*schema.Message, extra map[string]interface{}, err error) {
	var mas [][]*schema.Message
	for _, in := range ins {
		if in == nil {
			continue
		}
		if len(in.Messages) > 0 {
			mas = append(mas, in.Messages)
		}
		if len(in.Extra) > 0 {
			extra = in.Extra
		}
		if in.Config != nil {
			config = in.Config
		}
	}
	if len(mas) == 0 {
		return config, []*schema.Message{}, extra, nil
	}
	messages, err = concatMessageArray(mas)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("concat messages failed: %v", err)
	}
	return config, messages, extra, nil
}

func convModelCallbackOutput(out []callbacks.CallbackOutput) []*model.CallbackOutput {
	ret := make([]*model.CallbackOutput, len(out))
	for i, c := range out {
		ret[i] = model.ConvCallbackOutput(c)
	}
	return ret
}

func extractModelOutput(outs []*model.CallbackOutput) (usage *model.TokenUsage, messages []*schema.Message, extra map[string]interface{}, config *model.Config, err error) {
	masMap := make(map[schema.RoleType][]*schema.Message)
	for _, out := range outs {
		if out == nil {
			continue
		}
		if out.TokenUsage != nil {
			usage = out.TokenUsage
		}
		if out.Message != nil {
			if _, ok := masMap[out.Message.Role]; !ok {
				masMap[out.Message.Role] = make([]*schema.Message, 0)
			}
			masMap[out.Message.Role] = append(masMap[out.Message.Role], out.Message)
		}
		if out.Extra != nil {
			extra = out.Extra
		}
		if out.Config != nil {
			config = out.Config
		}
	}
	if len(masMap) == 0 {
		return usage, nil, extra, config, nil
	}
	messages = make([]*schema.Message, 0)
	for _, mas := range masMap {
		message, err := schema.ConcatMessages(mas)
		if err != nil {
			log.Printf("concat message failed: %v", err)
		} else {
			messages = append(messages, message)
		}
	}

	return usage, messages, extra, config, nil
}

func concatMessageArray(mas [][]*schema.Message) ([]*schema.Message, error) {
	if len(mas) == 0 {
		return nil, fmt.Errorf("message array is empty")
	}
	arrayLen := len(mas[0])

	ret := make([]*schema.Message, arrayLen)
	slicesToConcat := make([][]*schema.Message, arrayLen)

	for _, ma := range mas {
		if len(ma) != arrayLen {
			return nil, fmt.Errorf("unexpected array length. "+
				"Got %d, expected %d", len(ma), arrayLen)
		}

		for i := 0; i < arrayLen; i++ {
			m := ma[i]
			if m != nil {
				slicesToConcat[i] = append(slicesToConcat[i], m)
			}
		}
	}

	for i, slice := range slicesToConcat {
		if len(slice) == 0 {
			ret[i] = nil
		} else if len(slice) == 1 {
			ret[i] = slice[0]
		} else {
			cm, err := schema.ConcatMessages(slice)
			if err != nil {
				return nil, err
			}

			ret[i] = cm
		}
	}

	return ret, nil
}

func convSchemaMessage(in []*schema.Message) []*model.CallbackInput {
	ret := make([]*model.CallbackInput, len(in))
	for i, c := range in {
		ret[i] = model.ConvCallbackInput(c)
	}
	return ret
}

type tlsToolIDNameMapKey struct{}

func injectToolIDNameMapToCtx(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	if info == nil || info.Component != compose.ComponentOfToolsNode {
		return ctx
	}

	message, ok := input.(*schema.Message)
	if !ok || message == nil {
		return ctx
	}

	toolIDNameMap := make(map[string]string, len(message.ToolCalls))
	for _, toolCall := range message.ToolCalls {
		toolIDNameMap[toolCall.ID] = toolCall.Function.Name
	}

	return context.WithValue(ctx, tlsToolIDNameMapKey{}, toolIDNameMap)
}

func getToolIDNameMapFromCtx(ctx context.Context) map[string]string {
	toolIDNameMap, ok := ctx.Value(tlsToolIDNameMapKey{}).(map[string]string)
	if !ok {
		return nil
	}

	return toolIDNameMap
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

// _eino_run_component_to_span_kind 转换 component 到 tls 可以识别的 span_type
//   - 当前框架相比于之前缺失的后续需要补齐, 当前按照`TASK`处理
//   - compose 相关概念的 component 概念(Chain/Graph/...), 当前也先按照`TASK`处理
func _eino_run_component_to_span_kind(c components.Component) string {
	switch c {
	case components.ComponentOfChatModel:
		return sem_ai.GEN_AI_SPAN_KIND_LLM

	case components.ComponentOfEmbedding:
		return sem_ai.GEN_AI_SPAN_KIND_EMBEDDING

	case components.ComponentOfRetriever:
		return sem_ai.GEN_AI_SPAN_KIND_RETRIEVER

	case components.ComponentOfTool:
		return sem_ai.GEN_AI_SPAN_KIND_TOOL

	default:
		return sem_ai.GEN_AI_SPAN_KIND_TASK
	}
}
