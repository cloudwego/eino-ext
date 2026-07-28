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
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	sem_ai "github.com/cloudwego/eino-ext/callbacks/tls/semconv"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// CallbackDataParser tag parser for trace
// Implement CallbackDataParser and replace defaultDataParser by WithCallbackDataParser if needed
type CallbackDataParser interface {
	ParseInput(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) (map[string]any, error)
	ParseOutput(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) (map[string]any, error)
	ParseStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) (map[string]any, error)
	ParseStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) (map[string]any, error)
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

// setSpanAttributesFromTags 将解析后的标签设置到span
func setSpanAttributesFromTags(span trace.Span, tags map[string]any) {
	if tags == nil {
		return
	}

	for key, value := range tags {
		switch v := value.(type) {
		case string:
			span.SetAttributes(attribute.String(key, v))
		case int:
			span.SetAttributes(attribute.Int(key, v))
		case int64:
			span.SetAttributes(attribute.Int64(key, v))
		case float32:
			span.SetAttributes(attribute.Float64(key, float64(v)))
		case float64:
			span.SetAttributes(attribute.Float64(key, v))
		case bool:
			span.SetAttributes(attribute.Bool(key, v))
		case []string:
			span.SetAttributes(attribute.StringSlice(key, v))
		case []any:
			// 尝试转换为字符串切片
			strSlice := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					strSlice = append(strSlice, str)
				}
			}
			if len(strSlice) > 0 {
				span.SetAttributes(attribute.StringSlice(key, strSlice))
			} else {
				// 否则序列化为JSON字符串
				if jsonStr, err := json.Marshal(v); err == nil {
					span.SetAttributes(attribute.String(key, string(jsonStr)))
				}
			}
		default:
			// 其他类型尝试序列化为JSON
			if jsonStr, err := json.Marshal(v); err == nil {
				span.SetAttributes(attribute.String(key, string(jsonStr)))
			}
		}
	}
}

func (d defaultDataParser) ParseInput(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) (map[string]any, error) {
	if info == nil {
		return nil, nil
	}

	tags := make(spanTags)
	switch info.Component {
	case components.ComponentOfChatModel:
		cbInput := model.ConvCallbackInput(input)
		if cbInput != nil {
			// 设置请求参数
			if cbInput.Config != nil {
				tags.set(sem_ai.GEN_AI_MODEL_NAME, cbInput.Config.Model)
				tags.set(sem_ai.GEN_AI_REQUEST_MODEL, cbInput.Config.Model)
				tags.set(sem_ai.GEN_AI_REQUEST_TEMPERATURE, cbInput.Config.Temperature)
				tags.set(sem_ai.GEN_AI_REQUEST_MAX_TOKENS, cbInput.Config.MaxTokens)
				tags.set(sem_ai.GEN_AI_REQUEST_TOP_P, cbInput.Config.TopP)
				if len(cbInput.Config.Stop) > 0 {
					tags.set(sem_ai.GEN_AI_REQUEST_STOP, cbInput.Config.Stop)
					tags.set(sem_ai.GEN_AI_REQUEST_STOP_SEQUENCES, cbInput.Config.Stop)
				}
			}

			// 设置消息内容
			if cbInput.Messages != nil {
				for i, msg := range cbInput.Messages {
					if msg == nil {
						continue
					}
					if content := messageDisplayText(msg); content != "" {
						tags.set(fmt.Sprintf("%s.%d.role", sem_ai.GEN_AI_PROMPT, i), string(msg.Role))
						tags.set(fmt.Sprintf("%s.%d.content", sem_ai.GEN_AI_PROMPT, i), content)
					}
					if multiContentJSON := modelMessagePartsJSON(msg); len(multiContentJSON) > 0 {
						for j, part := range multiContentJSON {
							if contentJSON, err := json.Marshal(part); err == nil {
								tags.set(fmt.Sprintf("%s.%d.content_part.%d", sem_ai.GEN_AI_PROMPT, i, j), string(contentJSON))
							}
						}
					}
				}
			}
			tags.set(sem_ai.GEN_AI_INPUT, convertModelInput(cbInput))
			tags.set(sem_ai.GEN_AI_INPUT_MESSAGES, iterSlice(cbInput.Messages, convertModelMessage))

			// 设置工具调用信息
			if len(cbInput.Tools) > 0 {
				toolDefinitions := iterSlice(cbInput.Tools, convertTool)
				if jsonBytes, err := json.Marshal(toolDefinitions); err == nil {
					tags.set(sem_ai.GEN_AI_REQUEST_TOOL_DEFINITIONS, string(jsonBytes))
				}
			}
		}
	case components.ComponentOfPrompt:
		cbInput := prompt.ConvCallbackInput(input)
		if cbInput != nil {
			// 应用提示模板属性
			if kvs := _promptTemplateInput(cbInput); kvs != nil {
				for _, kv := range kvs {
					tags.set(string(kv.Key), kv.Value.AsInterface())
				}
			}
		}

	case components.ComponentOfEmbedding:
		cbInput := embedding.ConvCallbackInput(input)
		if cbInput != nil {
			tags.set(sem_ai.GEN_AI_INPUT, cbInput.Texts)
			tags.set(sem_ai.EMBEDDING_TEXT, cbInput.Texts)

			if cbInput.Config != nil {
				tags.set(sem_ai.GEN_AI_MODEL_NAME, cbInput.Config.Model)
			}
		}

	case components.ComponentOfRetriever:
		cbInput := retriever.ConvCallbackInput(input)
		if cbInput != nil {
			tags.set(sem_ai.GEN_AI_INPUT, parseAny(ctx, cbInput.Query, false))
			tags.set(sem_ai.GEN_AI_REQUEST_PARAMETERS, convertRetrieverCallOption(cbInput))
			tags.set(sem_ai.RERANKER_QUERY, parseAny(ctx, cbInput.Query, false))

			// 处理检索器特定参数
			if cbInput.Extra != nil {
				if topK, ok := cbInput.Extra["top_k"]; ok {
					tags.set(sem_ai.RERANKER_TOP_K, topK)
				}
				if modelName, ok := cbInput.Extra["model_name"]; ok {
					tags.set(sem_ai.RERANKER_MODEL_NAME, modelName)
				}
			}
		}

	case components.ComponentOfIndexer:
		cbInput := indexer.ConvCallbackInput(input)
		if cbInput != nil {
			tags.set(sem_ai.GEN_AI_INPUT, parseAny(ctx, cbInput.Docs, false))
		}

	case components.ComponentOfTool:
		cbInput := tool.ConvCallbackInput(input)
		if cbInput != nil {
			tags.set(sem_ai.GEN_AI_TOOL_NAME, getName(info))
			tags.set(sem_ai.GEN_AI_TOOL_TYPE, info.Type)
			toolInput := cbInput.ArgumentsInJSON
			tags.set(sem_ai.GEN_AI_INPUT, toolInput)
			tags.set(sem_ai.GEN_AI_TOOL_CALL_ARGUMENTS, toolInput)
		}

	case compose.ComponentOfLambda:
		tags.set(sem_ai.GEN_AI_INPUT, parseAny(ctx, input, false))

	default:
		// 默认设置为任务类型
		tags.set(sem_ai.GEN_AI_INPUT, parseAny(ctx, input, false))
	}

	return tags, nil
}

// 设置模型配置和token使用情况的公共函数
func setModelConfigAndTokenUsage(tags spanTags, cbOutput *model.CallbackOutput, setModelName bool) {
	// 设置响应模型
	if cbOutput.Config != nil {
		tags.set(sem_ai.GEN_AI_RESPONSE_MODEL, cbOutput.Config.Model)
		// 可选设置模型名称
		if setModelName {
			tags.set(sem_ai.GEN_AI_MODEL_NAME, cbOutput.Config.Model)
		}
	}

	// 设置token使用情况
	if cbOutput.TokenUsage != nil {
		tags.set(sem_ai.GEN_AI_USAGE_TOTAL_TOKENS, cbOutput.TokenUsage.TotalTokens).
			set(sem_ai.GEN_AI_USAGE_PROMPT_TOKENS, cbOutput.TokenUsage.PromptTokens).
			set(sem_ai.GEN_AI_USAGE_COMPLETION_TOKENS, cbOutput.TokenUsage.CompletionTokens).
			set(sem_ai.GEN_AI_USAGE_INPUT_TOKENS, cbOutput.TokenUsage.PromptTokens).
			set(sem_ai.GEN_AI_USAGE_OUTPUT_TOKENS, cbOutput.TokenUsage.CompletionTokens)

		// Keep all cost dimensions present (including zero) so dashboard SUM
		// queries return 0 instead of an empty result when a provider does not
		// report cache or reasoning usage.
		cachedTokens := cbOutput.TokenUsage.PromptTokenDetails.CachedTokens
		tags.set(sem_ai.GEN_AI_USAGE_CACHE_READ_INPUT_TOKENS, cachedTokens).
			set(sem_ai.GEN_AI_USAGE_CACHE_READ_INPUT_TOKENS_V2, cachedTokens).
			set(sem_ai.GEN_AI_USAGE_CACHED_TOKENS, cachedTokens).
			set(sem_ai.GEN_AI_USAGE_CACHE_CREATE_INPUT_TOKENS, 0).
			set(sem_ai.GEN_AI_USAGE_CACHE_CREATION_INPUT_TOKENS, 0).
			set(sem_ai.GEN_AI_USAGE_REASONING_OUTPUT_TOKENS, cbOutput.TokenUsage.CompletionTokensDetails.ReasoningTokens)
	}
}

// 设置完成原因和完成内容的公共函数
func setMessageCompletionDetails(tags spanTags, msg *schema.Message) {
	// 设置完成原因
	if msg != nil && msg.ResponseMeta != nil && len(msg.ResponseMeta.FinishReason) > 0 {
		tags.set(sem_ai.GEN_AI_RESPONSE_FINISH_REASON, msg.ResponseMeta.FinishReason)
	}
	// 设置推理内容
	if msg != nil && len(msg.ReasoningContent) > 0 {
		tags.set(sem_ai.GEN_AI_REASONING_CONTENT, msg.ReasoningContent)
	}
	// 设置完成内容
	if msg != nil && messageDisplayText(msg) != "" {
		tags.set(fmt.Sprintf("%s.%d.role", sem_ai.GEN_AI_COMPLETION, 0), string(msg.Role))
		tags.set(fmt.Sprintf("%s.%d.content", sem_ai.GEN_AI_COMPLETION, 0), messageDisplayText(msg))
	}
	if msg != nil {
		tags.set(sem_ai.GEN_AI_OUTPUT_MESSAGES, []*sem_ai.ModelMessage{convertModelMessage(msg)})
	}
}

func (d defaultDataParser) ParseOutput(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) (map[string]any, error) {
	if info == nil {
		return nil, nil
	}

	tags := make(spanTags)
	switch info.Component {
	case components.ComponentOfChatModel:
		cbOutput := model.ConvCallbackOutput(output)
		if cbOutput != nil {
			// 设置输出内容
			finalOutput := convertModelOutput(cbOutput)
			tags.set(sem_ai.GEN_AI_OUTPUT, finalOutput)

			// 使用公共函数设置模型配置和token使用情况
			setModelConfigAndTokenUsage(tags, cbOutput, false)

			// 使用公共函数设置完成原因和完成内容
			setMessageCompletionDetails(tags, cbOutput.Message)
		}

	case components.ComponentOfPrompt:
		cbOutput := prompt.ConvCallbackOutput(output)
		if cbOutput != nil {
			// 可以添加提示模板输出相关字段
		}

	case components.ComponentOfEmbedding:
		cbOutput := embedding.ConvCallbackOutput(output)
		if cbOutput != nil {
			// 设置嵌入向量
			tags.set(sem_ai.GEN_AI_OUTPUT, parseAny(ctx, cbOutput.Embeddings, false))
			tags.set(sem_ai.EMBEDDING_EMBEDDINGS, parseAny(ctx, cbOutput.Embeddings, false))

			// 设置向量大小
			if len(cbOutput.Embeddings) > 0 {
				vec := cbOutput.Embeddings[0]
				if len(vec) > 0 {
					tags.set(sem_ai.GEN_AI_EMBEDDINGS_DIMENSION_COUNT, len(vec))
				}
			}
			// 设置token使用情况
			if cbOutput.TokenUsage != nil {
				tags.set(sem_ai.GEN_AI_USAGE_TOTAL_TOKENS, cbOutput.TokenUsage.TotalTokens).
					set(sem_ai.GEN_AI_USAGE_INPUT_TOKENS, cbOutput.TokenUsage.PromptTokens).
					set(sem_ai.GEN_AI_USAGE_COMPLETION_TOKENS, cbOutput.TokenUsage.CompletionTokens)
			}

			// 设置模型信息
			if cbOutput.Config != nil {
				tags.set(sem_ai.GEN_AI_REQUEST_MODEL, cbOutput.Config.Model)
				tags.set(sem_ai.GEN_AI_RESPONSE_MODEL, cbOutput.Config.Model)
			}
		}

	case components.ComponentOfIndexer:
		cbOutput := indexer.ConvCallbackOutput(output)
		if cbOutput != nil {
			// 设置索引文档ID
			tags.set(sem_ai.GEN_AI_OUTPUT, parseAny(ctx, cbOutput.IDs, false))
			for i, id := range cbOutput.IDs {
				tags.set(fmt.Sprintf("%s.%d", sem_ai.DOCUMENT_ID, i), id)
			}
		}

	case components.ComponentOfRetriever:
		cbOutput := retriever.ConvCallbackOutput(output)
		if cbOutput != nil {
			// 设置检索结果
			result := convertRetrieverOutput(cbOutput)
			tags.set(sem_ai.GEN_AI_OUTPUT, result)

			// 处理检索文档详情
			if cbOutput.Docs != nil {
				// 按要求格式解析并配置 GEN_AI_RETRIEVAL_DOCUMENTS
				retrievalDocs := make([]map[string]interface{}, 0, len(cbOutput.Docs))
				for _, doc := range cbOutput.Docs {
					docMap := map[string]interface{}{
						"document": map[string]interface{}{
							"content":  doc.Content,
							"metadata": doc.MetaData,
							"score":    doc.MetaData["_score"], // 假设分数存储在 _score 元数据中
							"id":       doc.ID,
						},
					}
					retrievalDocs = append(retrievalDocs, docMap)
				}
				tags.set(sem_ai.GEN_AI_RETRIEVAL_DOCUMENTS, retrievalDocs)
			}
		}

	case components.ComponentOfTool:
		cbOutput := tool.ConvCallbackOutput(output)
		if cbOutput != nil {
			toolOutput := cbOutput.Response
			if toolOutput == "" && cbOutput.ToolOutput != nil {
				toolOutput = parseAny(ctx, cbOutput.ToolOutput, false)
			}
			tags.set(sem_ai.GEN_AI_OUTPUT, toolOutput)
			tags.set(sem_ai.GEN_AI_TOOL_CALL_RESULT, toolOutput)
		}

	case compose.ComponentOfLambda:
		tags.set(sem_ai.GEN_AI_OUTPUT, parseAny(ctx, output, false))

	case compose.ComponentOfToolsNode:
		messages, ok := output.([]*schema.Message)
		if ok {
			tags.set(sem_ai.GEN_AI_OUTPUT, parseAny(ctx, iterSliceWithCtx(ctx, iterSlice(messages, convertModelMessage), addToolName), false))
		} else {
			tags.set(sem_ai.GEN_AI_OUTPUT, parseAny(ctx, output, false))
		}
	default:
		// 默认设置为任务类型
		tags.set(sem_ai.GEN_AI_OUTPUT, parseAny(ctx, output, false))
	}

	return tags, nil
}

func (d defaultDataParser) ParseStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) (map[string]any, error) {
	defer input.Close()

	if info == nil {
		return nil, nil
	}

	tags := make(spanTags)
	// 设置流相关标志
	tags.set(sem_ai.GEN_AI_REQUEST_IS_STREAM, true)
	tags.set(sem_ai.GEN_AI_IS_STREAMING, true)

	switch info.Component {
	case components.ComponentOfChatModel:
		chunks, recvErr := d.ParseDefaultStreamInput(ctx, input)
		if recvErr != nil {
			return nil, recvErr
		}

		// try concat
		config, messages, _, concatErr := d.tryConcatInputChunks(convModelCallbackInput(chunks))
		if concatErr != nil {
			return nil, concatErr
		}

		tags.set(sem_ai.GEN_AI_INPUT, parseAny(ctx, messages, true))
		tags.set(sem_ai.GEN_AI_INPUT_MESSAGES, iterSlice(messages, convertModelMessage))
		// 设置聚合后的消息
		if len(messages) > 0 {
			for i, msg := range messages {
				if msg != nil && messageDisplayText(msg) != "" {
					tags.set(fmt.Sprintf("%s.%d.role", sem_ai.GEN_AI_PROMPT, i), string(msg.Role))
					tags.set(fmt.Sprintf("%s.%d.content", sem_ai.GEN_AI_PROMPT, i), messageDisplayText(msg))
				}
			}
		}
		if config != nil {
			tags.set(sem_ai.GEN_AI_REQUEST_MODEL, config.Model)
			tags.set(sem_ai.GEN_AI_REQUEST_MAX_TOKENS, config.MaxTokens)
			tags.set(sem_ai.GEN_AI_REQUEST_TEMPERATURE, config.Temperature)
			tags.set(sem_ai.GEN_AI_REQUEST_TOP_P, config.TopP)
		}
	case components.ComponentOfTool:
		chunks, recvErr := d.ParseDefaultStreamInput(ctx, input)
		if recvErr != nil {
			return nil, recvErr
		}
		arguments := make([]string, 0, len(chunks))
		for _, chunk := range chunks {
			if cbInput := tool.ConvCallbackInput(chunk); cbInput != nil {
				arguments = append(arguments, cbInput.ArgumentsInJSON)
			}
		}
		toolInput := strings.Join(arguments, "")
		tags.set(sem_ai.GEN_AI_TOOL_NAME, getName(info))
		tags.set(sem_ai.GEN_AI_TOOL_TYPE, info.Type)
		tags.set(sem_ai.GEN_AI_INPUT, toolInput)
		tags.set(sem_ai.GEN_AI_TOOL_CALL_ARGUMENTS, toolInput)
	default:
		chunks, recvErr := d.ParseDefaultStreamInput(ctx, input)
		if recvErr != nil {
			return nil, recvErr
		}

		i, concatErr := d.tryConcatChunks(toAnySlice(chunks))
		if concatErr != nil {
			return nil, concatErr
		}

		tags.set(sem_ai.GEN_AI_INPUT, parseAny(ctx, i, true))
	}

	return tags, nil
}

func (d defaultDataParser) ParseStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) (map[string]any, error) {
	defer output.Close()

	if info == nil {
		return nil, nil
	}

	tags := make(spanTags)
	var (
		hasFirstToken  bool
		firstTokenTime time.Time
		startTime      = time.Now()
	)
	// 设置流相关标志
	tags.set(sem_ai.GEN_AI_REQUEST_IS_STREAM, true)
	tags.set(sem_ai.GEN_AI_IS_STREAMING, true)
	switch info.Component {
	case components.ComponentOfChatModel:
		var chunks []callbacks.CallbackOutput
		for {
			item, recvErr := output.Recv()
			if recvErr != nil {
				if recvErr == io.EOF {
					break
				}
				return nil, recvErr
			}

			cbOutput := model.ConvCallbackOutput(item)
			// 记录首次token时间
			if !hasFirstToken && cbOutput.Message != nil && len(cbOutput.Message.Content) > 0 {
				hasFirstToken = true
				firstTokenTime = time.Now()
				timeToFirstToken := firstTokenTime.Sub(startTime).Milliseconds()
				tags.set(sem_ai.GEN_AI_RESPONSE_TIME_TO_FIRST_TOKEN, timeToFirstToken)
				tags.set(sem_ai.GEN_AI_USER_TIME_TO_FIRST_TOKEN, timeToFirstToken)
			}

			chunks = append(chunks, item)
		}

		// 转换为模型输出格式
		modelChunks := convModelCallbackOutput(chunks)

		// 合并输出
		usage, message, _, err := d.tryConcatOutputChunks(modelChunks)
		if err != nil {
			return nil, err
		}

		var mergedOutput *model.CallbackOutput
		for _, chunk := range modelChunks {
			if chunk == nil {
				continue
			}

			mergedOutput = &model.CallbackOutput{
				Config:     chunk.Config,
				TokenUsage: usage,
			}
			if chunk.Config != nil {
				break
			}
		}
		if mergedOutput != nil {
			setModelConfigAndTokenUsage(tags, mergedOutput, true)
		}
		if message != nil {
			setMessageCompletionDetails(tags, message)
			tags.set(sem_ai.GEN_AI_OUTPUT, convertModelOutput(&model.CallbackOutput{
				Message:    message,
				TokenUsage: usage,
			}))
		}
	case components.ComponentOfTool:
		chunks, recvErr := d.ParseDefaultStreamOutput(ctx, output)
		if recvErr != nil {
			return nil, recvErr
		}
		responses := make([]string, 0, len(chunks))
		for _, chunk := range chunks {
			if cbOutput := tool.ConvCallbackOutput(chunk); cbOutput != nil {
				if cbOutput.Response != "" {
					responses = append(responses, cbOutput.Response)
				} else if cbOutput.ToolOutput != nil {
					responses = append(responses, parseAny(ctx, cbOutput.ToolOutput, true))
				}
			}
		}
		toolOutput := strings.Join(responses, "")
		tags.set(sem_ai.GEN_AI_OUTPUT, toolOutput)
		tags.set(sem_ai.GEN_AI_TOOL_CALL_RESULT, toolOutput)
	default:
		chunks, recvErr := d.ParseDefaultStreamOutput(ctx, output)
		if recvErr != nil {
			return nil, recvErr
		}

		o, concatErr := d.tryConcatChunks(toAnySlice(chunks))
		if concatErr != nil {
			return nil, concatErr
		}

		tags.set(sem_ai.GEN_AI_OUTPUT, parseAny(ctx, o, true))
	}

	return tags, nil
}

func (d defaultDataParser) ParseDefaultStreamInput(ctx context.Context, input *schema.StreamReader[callbacks.CallbackInput]) (chunks []callbacks.CallbackInput, err error) {
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

func (d defaultDataParser) ParseDefaultStreamOutput(ctx context.Context, output *schema.StreamReader[callbacks.CallbackOutput]) (chunks []callbacks.CallbackOutput, err error) {
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

func (d defaultDataParser) tryConcatInputChunks(chunks []*model.CallbackInput) (config *model.Config, messages []*schema.Message, extra map[string]interface{}, err error) {
	var mas [][]*schema.Message
	for _, c := range chunks {
		if c == nil {
			continue
		}
		if len(c.Messages) > 0 {
			mas = append(mas, c.Messages)
		}
		if len(c.Extra) > 0 {
			extra = c.Extra
		}
		if c.Config != nil {
			config = c.Config
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

func (d defaultDataParser) tryConcatOutputChunks(chunks []*model.CallbackOutput) (usage *model.TokenUsage, message *schema.Message, extra map[string]interface{}, err error) {
	var mas []*schema.Message
	for _, c := range chunks {
		if c == nil {
			continue
		}
		if c.TokenUsage != nil {
			usage = c.TokenUsage
		}
		if c.Message != nil {
			mas = append(mas, c.Message)
		}
		if c.Extra != nil {
			extra = c.Extra
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

func (d defaultDataParser) tryConcatChunks(chunks []any) (any, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}

	var first any
	for _, chunk := range chunks {
		if chunk != nil {
			first = chunk
			break
		}
	}
	if first == nil {
		return chunks, nil
	}

	typ := reflect.TypeOf(first)
	if fn := d.getConcatFunc(typ); fn != nil {
		s := reflect.MakeSlice(reflect.SliceOf(typ), 0, len(chunks))
		for _, chunk := range chunks {
			if chunk == nil {
				continue
			}
			s = reflect.Append(s, reflect.ValueOf(chunk))
		}

		val, concatErr := fn(s)
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

// messageDisplayText supplies the compact input/output used by the session
// table.  Detailed multimodal structure stays in gen_ai.*.messages.
func messageDisplayText(message *schema.Message) string {
	if message == nil {
		return ""
	}
	lensMessages := toTLSLensMessages([]*sem_ai.ModelMessage{convertModelMessage(message)})
	if len(lensMessages) == 0 {
		return ""
	}
	return lensMessageSummary(lensMessages[0])
}

func modelMessagePartsJSON(message *schema.Message) []map[string]any {
	converted := convertModelMessage(message)
	if converted == nil || len(converted.Parts) == 0 {
		return nil
	}
	parts := make([]map[string]any, 0, len(converted.Parts))
	for _, part := range converted.Parts {
		if part == nil {
			continue
		}
		encoded, err := json.Marshal(part)
		if err != nil {
			continue
		}
		var value map[string]any
		if err := json.Unmarshal(encoded, &value); err == nil {
			parts = append(parts, value)
		}
	}
	return parts
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

// 辅助函数: 处理提示模板
func _promptTemplateInput(input *prompt.CallbackInput) []attribute.KeyValue {
	if len(input.Templates) == 0 {
		return nil
	}

	var kvs []attribute.KeyValue

	// 处理PromptTemplate内容
	// if templateStr, ok := input.Templates[0].Template.(string); ok && templateStr != "" {
	// 	kvs = append(kvs, attribute.String(sem_ai.GEN_AI_PROMPT_TEMPLATE, templateStr))
	// }

	// if inputVars, ok := input.Variables.(map[string]any); ok && len(inputVars) > 0 {
	// 	varsStr, _ := json.Marshal(inputVars)
	// 	kvs = append(kvs, attribute.String(sem_ai.GEN_AI_PROMPT_VARIABLES, string(varsStr)))
	// }

	return kvs
}

func _llmModelInput(input *model.CallbackInput) []attribute.KeyValue {
	if input == nil || len(input.Messages) == 0 {
		return nil
	}

	var kvs []attribute.KeyValue
	inputs := make([]map[string]interface{}, 0)
	for i, in := range input.Messages {
		if in != nil && len(in.Content) > 0 {
			kvs = append(kvs, attribute.String(fmt.Sprintf(sem_ai.GEN_AI_PROMPT+".%d.role", i), string(in.Role)))
			kvs = append(kvs, attribute.String(fmt.Sprintf(sem_ai.GEN_AI_PROMPT+".%d.content", i), in.Content))
		}
		if in != nil && len(in.MultiContent) > 0 {
			multiContentJSON, err := convertMultiContentToJSON(in)
			if err == nil && multiContentJSON != nil {
				inputs = append(inputs, multiContentJSON...)
			}
		}
	}

	if len(inputs) > 0 {
		inputmap := map[string]interface{}{
			"inputs": map[string]interface{}{
				"input": inputs,
			},
		}
		jsonStr, _ := json.Marshal(inputmap)
		kvs = append(kvs, attribute.String(fmt.Sprintf(sem_ai.GEN_AI_INPUT), string(jsonStr)))
	}

	for i, tool := range input.Tools {
		if tool != nil && len(tool.Name) > 0 {
			kvs = append(kvs, attribute.String(fmt.Sprintf(sem_ai.GEN_AI_COMPLETION+".%d"+sem_ai.TOOL_CALL_FUNCTION_NAME, i),
				tool.Name))
			kvs = append(kvs, attribute.String(fmt.Sprintf(sem_ai.GEN_AI_COMPLETION+".%d"+sem_ai.TOOL_CALL_FUNCTION_ARGUMENTS,
				i), tool.Desc))
			kvs = append(kvs, attribute.String(fmt.Sprintf(sem_ai.GEN_AI_COMPLETION+".%d"+sem_ai.TOOL_CALL_FUNCTION_DESCRIPTION, i),
				tool.Desc))
		}
	}

	if input.Config != nil {
		kvs = append(kvs, attribute.String(sem_ai.GEN_AI_MODEL_NAME, input.Config.Model))
		kvs = append(kvs, attribute.Float64(sem_ai.GEN_AI_REQUEST_TEMPERATURE, float64(input.Config.Temperature)))
		kvs = append(kvs, attribute.Float64(sem_ai.GEN_AI_REQUEST_TOP_P, float64(input.Config.TopP)))
	}

	return kvs
}

func _llmModelOutput(input *model.CallbackOutput) []attribute.KeyValue {
	if input == nil || input.TokenUsage == nil {
		return nil
	}

	var kvs []attribute.KeyValue

	if input.TokenUsage != nil {
		kvs = append(kvs, attribute.Int(sem_ai.GEN_AI_USAGE_TOTAL_TOKENS, input.TokenUsage.TotalTokens))
		kvs = append(kvs, attribute.Int(sem_ai.GEN_AI_USAGE_COMPLETION_TOKENS, input.TokenUsage.CompletionTokens))
		kvs = append(kvs, attribute.Int(sem_ai.GEN_AI_USAGE_PROMPT_TOKENS, input.TokenUsage.PromptTokens))
	}

	if input.Message != nil && len(input.Message.ReasoningContent) > 0 {
		kvs = append(kvs, attribute.String(sem_ai.GEN_AI_RESPONSE_FINISH_REASON, input.Message.ReasoningContent))
	}

	return kvs
}

func _toolsOutput(input *tool.CallbackOutput) []attribute.KeyValue {
	if input == nil || len(input.Response) == 0 {
		return nil
	}

	var kvs []attribute.KeyValue

	// 设置工具响应相关属性
	// kvs = append(kvs, attribute.String(sem_ai.AGENT_OBSERVATION, input.Response))

	return kvs
}

func convertMultiContentToJSON(in *schema.Message) ([]map[string]interface{}, error) {
	if in == nil || len(in.MultiContent) == 0 {
		return nil, fmt.Errorf("input message is nil or has no multi-content")
	}

	inputs := make([]map[string]interface{}, 0, len(in.MultiContent))

	for _, part := range in.MultiContent {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			inputs = append(inputs, map[string]interface{}{
				"type": "text",
				"text": part.Text,
			})
		case schema.ChatMessagePartTypeImageURL:
			if part.ImageURL != nil {
				inputs = append(inputs, map[string]interface{}{
					"type": "image",
					"image_url": map[string]interface{}{
						"url": part.ImageURL.URL,
					},
				})
			}
		case schema.ChatMessagePartTypeAudioURL:
			if part.AudioURL != nil {
				inputs = append(inputs, map[string]interface{}{
					"type": "audio",
					"audio_url": map[string]interface{}{
						"url": part.AudioURL.URL,
					},
				})
			}
		case schema.ChatMessagePartTypeVideoURL:
			if part.VideoURL != nil {
				inputs = append(inputs, map[string]interface{}{
					"type": "video",
					"video_url": map[string]interface{}{
						"url": part.VideoURL.URL,
					},
				})
			}
		case schema.ChatMessagePartTypeFileURL:
			if part.FileURL != nil {
				inputs = append(inputs, map[string]interface{}{
					"type": "file",
					"file_url": map[string]interface{}{
						"url": part.FileURL.URL,
					},
				})
			}
		}
	}

	return inputs, nil
}
