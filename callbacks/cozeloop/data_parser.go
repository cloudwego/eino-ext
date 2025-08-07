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
	"encoding/json"
	"io"
	"reflect"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
)

// CallbackDataParser tag parser for trace
// Implement CallbackDataParser and replace defaultDataParser by WithCallbackDataParser if needed
type CallbackDataParser interface {
	ParseInput(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) map[string]any
	ParseOutput(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) map[string]any
	ParseStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) map[string]any
	ParseStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) map[string]any
}

func NewDefaultDataParser(enableAggrMessageOutput bool) CallbackDataParser {
	return &defaultDataParser{concatFuncs: make(map[reflect.Type]any), enableAggrMessageOutput: enableAggrMessageOutput}
}

func newDefaultDataParserWithConcatFuncs(concatFuncs map[reflect.Type]any, enableAggrMessageOutput bool) CallbackDataParser {
	if concatFuncs == nil {
		return NewDefaultDataParser(enableAggrMessageOutput)
	}
	return &defaultDataParser{concatFuncs: concatFuncs, enableAggrMessageOutput: enableAggrMessageOutput}
}

type defaultDataParser struct {
	concatFuncs             map[reflect.Type]any
	enableAggrMessageOutput bool
}

func (d defaultDataParser) ParseInput(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) map[string]any {
	if info == nil {
		return nil
	}

	level := getGraphNodeLevelFromCtx(ctx)
	collectOutput, _ := getAggrMessageOutputHookFromCtx(ctx)

	tags := make(spanTags)

	switch info.Component {
	case components.ComponentOfChatModel:
		cbInput := model.ConvCallbackInput(input)
		if cbInput != nil {
			tags.set(tracespec.Input, convertModelInput(cbInput))

			if cbInput.Config != nil {
				tags.set(tracespec.ModelName, cbInput.Config.Model)
				tags.set(tracespec.CallOptions, convertModelCallOption(cbInput.Config))
			}
		}

		tags.set(tracespec.ModelProvider, info.Type)

	case components.ComponentOfPrompt:
		cbInput := prompt.ConvCallbackInput(input)
		if cbInput != nil {
			tags.set(tracespec.Input, convertPromptInput(cbInput))
			tags.setFromExtraIfNotZero(tracespec.PromptKey, cbInput.Extra)
			tags.setFromExtraIfNotZero(tracespec.PromptVersion, cbInput.Extra)
			tags.setFromExtraIfNotZero(tracespec.PromptProvider, cbInput.Extra)
		}

	case components.ComponentOfEmbedding:
		cbInput := embedding.ConvCallbackInput(input)
		if cbInput != nil {
			tags.set(tracespec.Input, cbInput.Texts)

			if cbInput.Config != nil {
				tags.set(tracespec.ModelName, cbInput.Config.Model)
			}
		}

	case components.ComponentOfRetriever:
		cbInput := retriever.ConvCallbackInput(input)
		if cbInput != nil {
			tags.set(tracespec.Input, parseAny(ctx, cbInput.Query, false))
			tags.set(tracespec.CallOptions, convertRetrieverCallOption(cbInput))

			tags.setFromExtraIfNotZero(tracespec.VikingDBName, cbInput.Extra)
			tags.setFromExtraIfNotZero(tracespec.VikingDBRegion, cbInput.Extra)

			tags.setFromExtraIfNotZero(tracespec.ESName, cbInput.Extra)
			tags.setFromExtraIfNotZero(tracespec.ESIndex, cbInput.Extra)
			tags.setFromExtraIfNotZero(tracespec.ESCluster, cbInput.Extra)
		}

		tags.set(tracespec.RetrieverProvider, info.Type)

	case components.ComponentOfIndexer:
		cbInput := indexer.ConvCallbackInput(input)
		if cbInput != nil {
			// rewrite if not suitable here
			tags.set(tracespec.Input, parseAny(ctx, cbInput.Docs, false))
		}

	case compose.ComponentOfLambda:
		tags.set(tracespec.Input, parseAny(ctx, input, false))

	default:
		messages, ok := input.([]*schema.Message)
		if ok && level == 1 {
			collectOutput.addMessages(iterSlice(messages, convertModelMessage)...)
		}
		tags.set(tracespec.Input, parseAny(ctx, input, false))
	}

	return tags
}

func (d defaultDataParser) ParseOutput(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) map[string]any {
	if info == nil {
		return nil
	}

	tags := make(spanTags)

	level := getGraphNodeLevelFromCtx(ctx)
	collectOutput, _ := getAggrMessageOutputHookFromCtx(ctx)

	switch info.Component {
	case components.ComponentOfChatModel:
		cbOutput := model.ConvCallbackOutput(output)
		if cbOutput != nil {
			finalOutput := convertModelOutput(cbOutput)
			if level == 2 {
				if len(finalOutput.Choices) > 0 {
					collectOutput.addMessages(finalOutput.Choices[0].Message)
				}
			}
			tags.set(tracespec.Output, finalOutput)

			if cbOutput.TokenUsage != nil {
				tags.set(tracespec.Tokens, cbOutput.TokenUsage.TotalTokens).
					set(tracespec.InputTokens, cbOutput.TokenUsage.PromptTokens).
					set(tracespec.OutputTokens, cbOutput.TokenUsage.CompletionTokens)
			}
		}

		tags.set(tracespec.Stream, false)

		if tv, ok := getTraceVariablesValue(ctx); ok {
			tags.set(tracespec.LatencyFirstResp, time.Since(tv.StartTime).Microseconds())
		}

	case components.ComponentOfPrompt:
		cbOutput := prompt.ConvCallbackOutput(output)
		if cbOutput != nil {
			finalOutput := convertPromptOutput(cbOutput)
			if level == 2 {
				collectOutput.addMessages(finalOutput.Prompts...)
			}
			tags.set(tracespec.Output, finalOutput)
		}

	case components.ComponentOfEmbedding:
		cbOutput := embedding.ConvCallbackOutput(output)
		if cbOutput != nil {
			tags.set(tracespec.Output, parseAny(ctx, cbOutput.Embeddings, false))

			if cbOutput.TokenUsage != nil {
				tags.set(tracespec.Tokens, cbOutput.TokenUsage.TotalTokens).
					set(tracespec.InputTokens, cbOutput.TokenUsage.PromptTokens).
					set(tracespec.OutputTokens, cbOutput.TokenUsage.CompletionTokens)
			}

			if cbOutput.Config != nil {
				tags.set(tracespec.ModelName, cbOutput.Config.Model)
			}
		}

	case components.ComponentOfIndexer:
		cbOutput := indexer.ConvCallbackOutput(output)
		if cbOutput != nil {
			tags.set(tracespec.Output, parseAny(ctx, cbOutput.IDs, false))
		}

	case components.ComponentOfRetriever:
		cbOutput := retriever.ConvCallbackOutput(output)
		if cbOutput != nil {
			// rewrite if not suitable here
			tags.set(tracespec.Output, convertRetrieverOutput(cbOutput))
		}

	case compose.ComponentOfLambda:
		messages, ok := output.([]*schema.Message)
		if ok && level == 2 {
			collectOutput.addMessages(iterSlice(messages, convertModelMessage)...)
		}
		tags.set(tracespec.Output, parseAny(ctx, output, false))

	case compose.ComponentOfToolsNode:
		messages, ok := output.([]*schema.Message)
		if ok && level == 2 {
			collectOutput.addMessages(iterSliceWithCtx(ctx, iterSlice(messages, convertModelMessage), addToolName)...)
		}
		tags.set(tracespec.Output, parseAny(ctx, output, false))
	default:
		if level == 1 && d.enableAggrMessageOutput {
			tags.set(tracespec.Output, collectOutput)
		} else {
			messages, ok := output.([]*schema.Message)
			if ok && level == 2 {
				collectOutput.addMessages(iterSlice(messages, convertModelMessage)...)
			}
			tags.set(tracespec.Output, parseAny(ctx, output, false))
		}
	}

	return tags
}

func (d defaultDataParser) ParseStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) map[string]any {
	defer input.Close()

	if info == nil {
		return nil
	}

	tags := make(spanTags)

	switch info.Component {
	default:
		chunks, recvErr := d.ParseDefaultStreamInput(ctx, input)
		if recvErr != nil {
			return tags.setTags(getErrorTags(ctx, recvErr))
		}

		// try concat
		i, concatErr := d.tryConcatChunks(chunks)
		if concatErr != nil {
			return tags.setTags(getErrorTags(ctx, concatErr))
		}

		tags.set(tracespec.Input, parseAny(ctx, i, true))
	}

	return tags
}

func (d defaultDataParser) ParseStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) map[string]any {
	defer output.Close()

	if info == nil {
		return nil
	}

	tags := make(spanTags)

	switch info.Component {
	case components.ComponentOfChatModel:
		tags = d.ParseChatModelStreamOutput(ctx, output)

		tags.set(tracespec.Stream, true)
		tags.set(tracespec.ModelProvider, info.Type)

	default:
		chunks, recvErr := d.ParseDefaultStreamOutput(ctx, output)
		if recvErr != nil {
			return tags.setTags(getErrorTags(ctx, recvErr))
		}

		// try concat
		o, concatErr := d.tryConcatChunks(chunks)
		if concatErr != nil {
			return tags.setTags(getErrorTags(ctx, concatErr))
		}

		tags.set(tracespec.Output, parseAny(ctx, o, true))
	}

	return tags
}

func (d defaultDataParser) ParseChatModelStreamOutput(ctx context.Context, output *schema.StreamReader[callbacks.CallbackOutput]) map[string]any {
	var (
		chunks  []*schema.Message
		onceSet bool
		tags    = make(spanTags)
		usage   *model.TokenUsage
	)

	level := getGraphNodeLevelFromCtx(ctx)
	collectOutput, _ := getAggrMessageOutputHookFromCtx(ctx)

	for {
		item, recvErr := output.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				break
			}

			return tags.setTags(getErrorTags(ctx, recvErr))
		}

		cbOutput := model.ConvCallbackOutput(item)
		if cbOutput == nil {
			continue
		}

		if cbOutput.Message != nil {
			chunks = append(chunks, cbOutput.Message)
		}

		if cbOutput.TokenUsage != nil {
			usage = &model.TokenUsage{
				PromptTokens:     cbOutput.TokenUsage.PromptTokens,
				CompletionTokens: cbOutput.TokenUsage.CompletionTokens,
				TotalTokens:      cbOutput.TokenUsage.TotalTokens,
			}
		}

		if cbOutput.Config != nil && !onceSet {
			onceSet = true

			if tv, ok := getTraceVariablesValue(ctx); ok {
				tags.set(tracespec.LatencyFirstResp, time.Since(tv.StartTime).Microseconds())
			}
		}
	}

	if msg, concatErr := schema.ConcatMessages(chunks); concatErr != nil { // unexpected
		finalOutput := parseAny(ctx, chunks, true)
		tags.set(tracespec.Output, finalOutput)
		if level == 2 {
			collectOutput.addMessages(&tracespec.ModelMessage{
				Role:    string(schema.Assistant),
				Content: parseAny(ctx, chunks, true),
			})
		}
	} else {
		tags.set(tracespec.Output, convertModelOutput(&model.CallbackOutput{Message: msg}))
		if level == 2 {
			collectOutput.addMessages(convertModelMessage(msg))
		}
	}

	if usage != nil {
		tags.set(tracespec.Tokens, usage.TotalTokens).
			set(tracespec.InputTokens, usage.PromptTokens).
			set(tracespec.OutputTokens, usage.CompletionTokens)
	}

	return tags
}

func (d defaultDataParser) ParseDefaultStreamInput(ctx context.Context, input *schema.StreamReader[callbacks.CallbackInput]) (chunks []any, err error) {
	for {
		item, recvErr := input.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				break
			}

			return chunks, recvErr
		}

		chunks = append(chunks, item)
	}

	return chunks, nil
}

func (d defaultDataParser) ParseDefaultStreamOutput(ctx context.Context, output *schema.StreamReader[callbacks.CallbackOutput]) (chunks []any, err error) {
	for {
		item, recvErr := output.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				break
			}

			return chunks, recvErr
		}

		chunks = append(chunks, item)
	}

	return chunks, nil
}

func (d defaultDataParser) tryConcatChunks(chunks []any) (any, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}

	val := reflect.ValueOf(chunks[0])
	typ := val.Type()
	if fn := d.getConcatFunc(typ); fn != nil {
		s := reflect.MakeSlice(reflect.SliceOf(typ), 0, len(chunks))
		for _, chunk := range chunks {
			s = reflect.Append(s, reflect.ValueOf(chunk))
		}

		var concatErr error
		val, concatErr = fn(s)
		if concatErr != nil {
			return nil, concatErr
		}

		return val.Interface(), nil
	}

	return chunks, nil
}

func (d defaultDataParser) getConcatFunc(typ reflect.Type) func(reflect.Value) (reflect.Value, error) {
	if fn, ok := d.concatFuncs[typ]; ok {
		return func(a reflect.Value) (reflect.Value, error) {
			rvs := reflect.ValueOf(fn).Call([]reflect.Value{a})
			var err error
			if !rvs[1].IsNil() {
				err = rvs[1].Interface().(error)
			}
			return rvs[0], err
		}
	}

	return nil
}

func parseAny(ctx context.Context, v any, bStream bool) string {
	if v == nil {
		return ""
	}

	switch t := v.(type) {
	case []*schema.Message:
		return toJson(t, bStream)

	case *schema.Message:
		return toJson(t, bStream)

	case string:
		if bStream {
			return toJson(t, bStream)
		}
		return t

	case json.Marshaler:
		return toJson(v, bStream)

	case map[string]any:
		return toJson(t, bStream)

	case []callbacks.CallbackInput:
		return parseAny(ctx, toAnySlice(t), bStream)

	case []callbacks.CallbackOutput:
		return parseAny(ctx, toAnySlice(t), bStream)

	case []any:
		if len(t) > 0 {
			if _, ok := t[0].(*schema.Message); ok {
				msgs := make([]*schema.Message, 0, len(t))
				for i := range t {
					msg, ok := t[i].(*schema.Message)
					if ok {
						msgs = append(msgs, msg)
					}
				}

				return parseAny(ctx, msgs, bStream)
			}
		}

		return toJson(t, bStream)

	default:
		return toJson(v, bStream)
	}
}

func toAnySlice[T any](src []T) []any {
	resp := make([]any, len(src))
	for i := range src {
		resp[i] = src[i]
	}

	return resp
}

// parseSpanTypeFromComponent 转换 component 到 fornax 可以识别的 span_type
// span_type 会影响到 fornax 界面的展示
// TODO:
//   - 当前框架相比于之前缺失的后续需要补齐, 当前按照`原来的字符串`处理
//   - compose 相关概念的 component 概念(Chain/Graph/...), 当前也先按照`原来的字符串`处理
func parseSpanTypeFromComponent(c components.Component) string {
	switch c {
	case components.ComponentOfPrompt:
		return "prompt"

	case components.ComponentOfChatModel:
		return "model"

	case components.ComponentOfEmbedding:
		return "embedding"

	case components.ComponentOfIndexer:
		return "store"

	case components.ComponentOfRetriever:
		return "retriever"

	case components.ComponentOfLoader:
		return "loader"

	case components.ComponentOfTool:
		return "tool"

	default:
		return string(c)
	}
}
