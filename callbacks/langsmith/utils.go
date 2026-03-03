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

package langsmith

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func runInfoToName(info *callbacks.RunInfo) string {
	if len(info.Name) != 0 {
		return info.Name
	}
	return info.Type + string(info.Component)
}

func runInfoToRunType(info *callbacks.RunInfo) RunType {
	switch info.Component {
	case components.ComponentOfChatModel:
		return RunTypeLLM
	case components.ComponentOfTool:
		return RunTypeTool
	default:
		return RunTypeChain
	}
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

func extractModelOutput(outs []*model.CallbackOutput) (usage *model.TokenUsage, message *schema.Message, extra map[string]interface{}, err error) {
	var mas []*schema.Message
	for _, out := range outs {
		if out == nil {
			continue
		}
		if out.TokenUsage != nil {
			usage = out.TokenUsage
		}
		if out.Message != nil {
			mas = append(mas, out.Message)
		}
		if out.Extra != nil {
			extra = out.Extra
		}
	}
	if len(mas) == 0 {
		return usage, &schema.Message{}, extra, nil
	}
	message, err = schema.ConcatMessages(mas)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("concat message failed: %v", err)
	}
	return usage, message, extra, nil
}

func concatMessageArray(mas [][]*schema.Message) ([]*schema.Message, error) {
	if len(mas) == 0 {
		return []*schema.Message{}, nil
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

func GetOrInitState(ctx context.Context) (context.Context, *LangsmithState) {
	if state, ok := ctx.Value(langsmithStateKey{}).(*LangsmithState); ok && state != nil {
		return ctx, state
	}

	// 从 context 初始化
	opts, _ := ctx.Value(langsmithTraceOptionKey{}).(*traceOptions)
	if opts == nil {
		opts = &traceOptions{}
	}

	traceID := opts.TraceID
	parentID := opts.ParentID
	parentDottedOrder := opts.ParentDottedOrder
	state := &LangsmithState{
		TraceID:           traceID,
		ParentRunID:       parentID,
		ParentDottedOrder: parentDottedOrder,
	}
	return context.WithValue(ctx, langsmithStateKey{}, state), state
}

func GetState(ctx context.Context) (context.Context, *LangsmithState) {
	if state, ok := ctx.Value(langsmithStateKey{}).(*LangsmithState); ok && state != nil {
		return ctx, state
	} else {
		return ctx, nil
	}
}

func SafeDeepCopyMetadata(original map[string]interface{}) map[string]interface{} {
	if original == nil {
		return map[string]interface{}{"metadata": map[string]interface{}{}}
	}

	// 使用 json 序列化/反序列化实现并发安全的深拷贝
	data, err := json.Marshal(original)
	if err != nil {
		// 如果序列化失败，返回一个新的空 map
		return map[string]interface{}{"metadata": map[string]interface{}{}}
	}

	var copyData map[string]interface{}
	if err := json.Unmarshal(data, &copyData); err != nil {
		// 如果反序列化失败，返回一个新的空 map
		return map[string]interface{}{"metadata": map[string]interface{}{}}
	}

	// 确保 metadata 字段存在
	if copyData == nil {
		copyData = make(map[string]interface{})
	}
	if _, ok := copyData["metadata"]; !ok {
		copyData["metadata"] = make(map[string]interface{})
	}

	return copyData
}

func SafeDeepCopySyncMapMetadata(original *sync.Map) map[string]interface{} {
	if original == nil {
		return map[string]interface{}{"metadata": map[string]interface{}{}}
	}

	copyData := make(map[string]interface{})
	original.Range(func(k, v interface{}) bool {
		copyData[k.(string)] = v
		return true
	})

	// 确保 metadata 字段存在
	if copyData == nil {
		copyData = make(map[string]interface{})
	}
	if _, ok := copyData["metadata"]; !ok {
		copyData["metadata"] = make(map[string]interface{})
	}

	return copyData
}
