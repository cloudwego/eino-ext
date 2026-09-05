/*
 * Copyright 2026 CloudWeGo Authors
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
	"reflect"
	"strings"
	"testing"

	sem_ai "github.com/cloudwego/eino-ext/callbacks/tls/semconv"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func TestDefaultDataParserParseInputRetrieverSetsRequestParameters(t *testing.T) {
	parser := defaultDataParser{}

	tags, err := parser.ParseInput(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfRetriever}, &retriever.CallbackInput{
		Query:  "what is tls callback",
		TopK:   3,
		Filter: "lang=zh",
	})
	if err != nil {
		t.Fatalf("ParseInput returned error: %v", err)
	}

	params, ok := tags[sem_ai.GEN_AI_REQUEST_PARAMETERS].(string)
	if !ok {
		t.Fatalf("expected %q to be stored as JSON string, got %#v", sem_ai.GEN_AI_REQUEST_PARAMETERS, tags[sem_ai.GEN_AI_REQUEST_PARAMETERS])
	}
	if !strings.Contains(params, `"top_k":3`) {
		t.Fatalf("expected retriever top_k in request parameters, got %s", params)
	}
	if !strings.Contains(params, `"filter":"lang=zh"`) {
		t.Fatalf("expected retriever filter in request parameters, got %s", params)
	}
	if _, exists := tags[sem_ai.GEN_AI_SPAN_KIND_RETRIEVER]; exists {
		t.Fatalf("unexpected retriever span kind value used as attribute key: %q", sem_ai.GEN_AI_SPAN_KIND_RETRIEVER)
	}
}

func TestDefaultDataParserParseOutputEmbeddingUsesDimensionCount(t *testing.T) {
	parser := defaultDataParser{}

	tags, err := parser.ParseOutput(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfEmbedding}, &embedding.CallbackOutput{
		Embeddings: [][]float64{{1, 2, 3}},
	})
	if err != nil {
		t.Fatalf("ParseOutput returned error: %v", err)
	}

	got, ok := tags[sem_ai.GEN_AI_EMBEDDINGS_DIMENSION_COUNT].(int)
	if !ok {
		t.Fatalf("expected dimension count to be int, got %#v", tags[sem_ai.GEN_AI_EMBEDDINGS_DIMENSION_COUNT])
	}
	if got != 3 {
		t.Fatalf("expected dimension count 3, got %d", got)
	}
}

func TestDefaultDataParserParseStreamOutputIncludesCompletionDetails(t *testing.T) {
	parser := defaultDataParser{}
	reader, writer := schema.Pipe[callbacks.CallbackOutput](2)
	writer.Send(&model.CallbackOutput{
		Message: &schema.Message{
			Role:    schema.Assistant,
			Content: "partial",
		},
	}, nil)
	writer.Send(&model.CallbackOutput{
		Message: &schema.Message{
			Role:    schema.Assistant,
			Content: " response",
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: "stop",
			},
		},
		TokenUsage: &model.TokenUsage{
			PromptTokens:     5,
			CompletionTokens: 2,
			TotalTokens:      7,
		},
	}, nil)
	writer.Close()

	tags, err := parser.ParseStreamOutput(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfChatModel}, reader)
	if err != nil {
		t.Fatalf("ParseStreamOutput returned error: %v", err)
	}

	if got := tags[sem_ai.GEN_AI_RESPONSE_FINISH_REASON]; got != "stop" {
		t.Fatalf("expected finish reason stop, got %#v", got)
	}
	if got := tags[sem_ai.GEN_AI_COMPLETION+".0.content"]; got != "partial response" {
		t.Fatalf("expected aggregated completion content partial response, got %#v", got)
	}

	output, ok := tags[sem_ai.GEN_AI_OUTPUT].(string)
	if !ok {
		t.Fatalf("expected stream output to be serialized JSON, got %#v", tags[sem_ai.GEN_AI_OUTPUT])
	}
	if !strings.Contains(output, `"finish_reason":"stop"`) {
		t.Fatalf("expected serialized output to include finish_reason, got %s", output)
	}
	if !strings.Contains(output, `"content":"partial response"`) {
		t.Fatalf("expected serialized output to include aggregated content, got %s", output)
	}
}

func TestConvertModelMessagePreservesReasoningAndFilePart(t *testing.T) {
	got := convertModelMessage(&schema.Message{
		Role:             schema.Assistant,
		ReasoningContent: "need to inspect the document first",
		MultiContent: []schema.ChatMessagePart{
			{
				Type: schema.ChatMessagePartTypeFileURL,
				FileURL: &schema.ChatMessageFileURL{
					Name: "spec.pdf",
					URL:  "https://example.com/spec.pdf",
				},
			},
		},
	})

	if got == nil {
		t.Fatal("expected converted message")
	}
	if got.ReasoningContent != "need to inspect the document first" {
		t.Fatalf("expected reasoning content to be preserved, got %q", got.ReasoningContent)
	}
	if len(got.Parts) != 1 || got.Parts[0] == nil || got.Parts[0].FileURL == nil {
		t.Fatalf("expected file part to be preserved, got %#v", got.Parts)
	}
	if got.Parts[0].FileURL.Name != "spec.pdf" {
		t.Fatalf("expected file name spec.pdf, got %q", got.Parts[0].FileURL.Name)
	}
	if got.Parts[0].FileURL.URL != "https://example.com/spec.pdf" {
		t.Fatalf("expected file url to be preserved, got %q", got.Parts[0].FileURL.URL)
	}
}

func TestDefaultDataParserParseOutputToolsNodeAddsToolName(t *testing.T) {
	parser := defaultDataParser{}
	ctx := context.WithValue(context.Background(), tlsToolIDNameMapKey{}, map[string]string{
		"call-1": "search",
	})

	tags, err := parser.ParseOutput(ctx, &callbacks.RunInfo{Component: compose.ComponentOfToolsNode}, []*schema.Message{
		{
			Role:       schema.Tool,
			ToolCallID: "call-1",
			Content:    "result",
		},
	})
	if err != nil {
		t.Fatalf("ParseOutput returned error: %v", err)
	}

	output, ok := tags[sem_ai.GEN_AI_OUTPUT].(string)
	if !ok {
		t.Fatalf("expected tools node output to be serialized JSON, got %#v", tags[sem_ai.GEN_AI_OUTPUT])
	}
	if !strings.Contains(output, `"name":"search"`) {
		t.Fatalf("expected serialized output to include tool name, got %s", output)
	}
}

func TestDefaultDataParserParseStreamInputUsesCustomConcatFunction(t *testing.T) {
	parser := defaultDataParser{
		concatFuncs: map[reflect.Type]any{
			reflect.TypeOf(""): func(chunks []string) (string, error) {
				return strings.Join(chunks, ""), nil
			},
		},
	}

	reader, writer := schema.Pipe[callbacks.CallbackInput](2)
	writer.Send("foo", nil)
	writer.Send("bar", nil)
	writer.Close()

	tags, err := parser.ParseStreamInput(context.Background(), &callbacks.RunInfo{Component: compose.ComponentOfLambda}, reader)
	if err != nil {
		t.Fatalf("ParseStreamInput returned error: %v", err)
	}

	input, ok := tags[sem_ai.GEN_AI_INPUT].(string)
	if !ok {
		t.Fatalf("expected stream input to be serialized JSON, got %#v", tags[sem_ai.GEN_AI_INPUT])
	}
	if !strings.Contains(input, `"foobar"`) {
		t.Fatalf("expected custom concat result in input, got %s", input)
	}
	if strings.Contains(input, `"foo","bar"`) {
		t.Fatalf("expected chunks to be concatenated before serialization, got %s", input)
	}
}

func TestDefaultDataParserParseStreamOutputUsesCustomConcatFunction(t *testing.T) {
	parser := defaultDataParser{
		concatFuncs: map[reflect.Type]any{
			reflect.TypeOf(""): func(chunks []string) (string, error) {
				return strings.Join(chunks, ""), nil
			},
		},
	}

	reader, writer := schema.Pipe[callbacks.CallbackOutput](2)
	writer.Send("foo", nil)
	writer.Send("bar", nil)
	writer.Close()

	tags, err := parser.ParseStreamOutput(context.Background(), &callbacks.RunInfo{Component: compose.ComponentOfLambda}, reader)
	if err != nil {
		t.Fatalf("ParseStreamOutput returned error: %v", err)
	}

	output, ok := tags[sem_ai.GEN_AI_OUTPUT].(string)
	if !ok {
		t.Fatalf("expected stream output to be serialized JSON, got %#v", tags[sem_ai.GEN_AI_OUTPUT])
	}
	if !strings.Contains(output, `"foobar"`) {
		t.Fatalf("expected custom concat result in output, got %s", output)
	}
	if strings.Contains(output, `"foo","bar"`) {
		t.Fatalf("expected chunks to be concatenated before serialization, got %s", output)
	}
}
