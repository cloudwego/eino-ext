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

package ark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/callbacks"
	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

func ptr[T any](v T) *T { return &v }
func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func equal(t *testing.T, want, got any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("want %#v, got %#v", want, got)
	}
}
func decodeRequest(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	check(t, json.NewDecoder(r.Body).Decode(&body))
	return body
}

const responseJSON = `{"id":"req-1","object":"chat.completion","created":1,"model":"actual-model","choices":[{"index":0,"message":{"role":"assistant","content":"done","reasoning_content":"think","encrypted_content":"cipher","tool_calls":[{"id":"call-1","type":"function","function":{"name":"inspect","arguments":"{}"}}]},"finish_reason":"tool_calls","logprobs":{"content":[{"token":"done","logprob":-0.2,"bytes":[100],"top_logprobs":[{"token":"ok","logprob":-0.4,"bytes":[111]}]}]}}],"usage":{"prompt_tokens":8,"completion_tokens":3,"total_tokens":11,"prompt_tokens_details":{"cached_tokens":2,"audio_tokens":1},"completion_tokens_details":{"reasoning_tokens":1}},"service_tier":"default","service_status":{"model_fallback":{"fallback_triggered":true,"original_model":"requested-model"}}}`

func testModel(t *testing.T, handler http.HandlerFunc, config *ChatModelConfig) *ChatModel {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	if config == nil {
		config = &ChatModelConfig{}
	}
	config.APIKey, config.BaseURL, config.RetryTimes = "test-key", server.URL+"/api/v3", ptr(0)
	cm, err := NewChatModel(context.Background(), config)
	check(t, err)
	return cm
}
func writeResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, responseJSON)
}
func toolInfo(name string) *schema.ToolInfo {
	return &schema.ToolInfo{Name: name, Desc: "inspect a file", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{"path": {Type: schema.String, Required: true}})}
}

func TestMediaWire(t *testing.T) {
	for _, role := range []schema.RoleType{schema.User, schema.Tool} {
		for _, kind := range []string{"image", "audio", "video", "file"} {
			for _, source := range []string{"url", "base64", "file_id", "all", "base64_and_id"} {
				t.Run(string(role)+"/"+kind+"/"+source, func(t *testing.T) {
					common := schema.MessagePartCommon{MIMEType: map[string]string{"image": "image/png", "audio": "audio/mpeg; codecs=mp3", "video": "video/mp4", "file": "application/pdf"}[kind]}
					switch source {
					case "url":
						common.URL = ptr("https://example.com/media")
					case "base64":
						common.Base64Data = ptr("aGVsbG8=")
					case "file_id":
						common.FileID = ptr("file-1")
					case "all":
						common.URL, common.Base64Data, common.FileID = ptr("https://example.com/media"), ptr("aGVsbG8="), ptr("file-1")
					case "base64_and_id":
						common.Base64Data, common.FileID = ptr("aGVsbG8="), ptr("file-1")
					}
					part := schema.MessageInputPart{}
					field, sourceField := "", "url"
					switch kind {
					case "image":
						part.Type, part.Image = schema.ChatMessagePartTypeImageURL, &schema.MessageInputImage{MessagePartCommon: common, Detail: schema.ImageURLDetailHigh}
						field = "image_url"
					case "audio":
						part.Type, part.Audio = schema.ChatMessagePartTypeAudioURL, &schema.MessageInputAudio{MessagePartCommon: common}
						field = "input_audio"
					case "video":
						part.Type, part.Video = schema.ChatMessagePartTypeVideoURL, &schema.MessageInputVideo{MessagePartCommon: common}
						SetInputVideoFPS(part.Video, 0.5)
						field = "video_url"
					case "file":
						part.Type, part.File = schema.ChatMessagePartTypeFileURL, &schema.MessageInputFile{MessagePartCommon: common, Name: "report.pdf"}
						field, sourceField = "file", "file_url"
					}
					cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
						equal(t, "/api/v3/chat/completions", r.URL.Path)
						message := decodeRequest(t, r)["messages"].([]any)[0].(map[string]any)
						equal(t, string(role), message["role"])
						if role == schema.Tool {
							equal(t, "call-1", message["tool_call_id"])
						}
						parts := message["content"].([]any)
						equal(t, 2, len(parts))
						media := parts[1].(map[string]any)[field].(map[string]any)
						want := "https://example.com/media"
						switch source {
						case "file_id":
							sourceField, want = "file_id", "file-1"
						case "base64", "base64_and_id":
							want = "aGVsbG8="
							if kind == "audio" {
								sourceField = "data"
							} else if kind == "file" {
								sourceField = "file_data"
								want = "data:" + common.MIMEType + ";base64," + want
							} else {
								want = "data:" + common.MIMEType + ";base64," + want
							}
						}
						equal(t, want, media[sourceField])
						for _, key := range []string{"url", "file_url", "data", "file_data", "file_id"} {
							if key != sourceField {
								if _, ok := media[key]; ok {
									t.Errorf("unexpected source %s: %#v", key, media)
								}
							}
						}
						if kind == "image" {
							equal(t, "high", media["detail"])
						}
						if kind == "audio" {
							equal(t, "mp3", media["format"])
						}
						if kind == "video" {
							equal(t, 0.5, media["fps"])
						}
						if kind == "file" {
							equal(t, "report.pdf", media["filename"])
						}
						writeResponse(w)
					}, &ChatModelConfig{Model: "test"})
					_, err := cm.Generate(context.Background(), []*schema.Message{{Role: role, ToolCallID: "call-1", UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "inspect"}, part}}})
					check(t, err)
				})
			}
		}
	}
}

func TestGenerateOptionsAndCallbacks(t *testing.T) {
	var start *fmodel.CallbackInput
	var output *fmodel.CallbackOutput
	handler := callbacks.NewHandlerBuilder().OnStartFn(func(ctx context.Context, _ *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		start = input.(*fmodel.CallbackInput)
		return ctx
	}).OnEndFn(func(ctx context.Context, _ *callbacks.RunInfo, result callbacks.CallbackOutput) context.Context {
		output = result.(*fmodel.CallbackOutput)
		return ctx
	}).Build()
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		equal(t, "override", r.Header.Get("X-Custom"))
		equal(t, "", r.Header.Get("X-Default"))
		body := decodeRequest(t, r)
		for key, want := range map[string]any{"model": "override-model", "temperature": float64(0), "top_p": float64(0), "max_tokens": float64(0), "max_completion_tokens": float64(16), "parallel_tool_calls": false, "user": "user-1", "repetition_penalty": float64(1), "logprobs": true, "top_logprobs": float64(3), "thinking": map[string]any{"type": "disabled"}, "reasoning_effort": "high", "response_format": map[string]any{"type": "json_object"}, "stream": false} {
			equal(t, want, body[key])
		}
		equal(t, []any{"stop"}, body["stop"])
		equal(t, map[string]any{"7": float64(-10)}, body["logit_bias"])
		tool := body["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)
		equal(t, "inspect", tool["name"])
		equal(t, true, tool["strict"])
		equal(t, map[string]any{"type": "function", "function": map[string]any{"name": "inspect"}}, body["tool_choice"])
		messages := body["messages"].([]any)
		assistant := messages[1].(map[string]any)
		equal(t, "prior thinking", assistant["reasoning_content"])
		equal(t, "prior cipher", assistant["encrypted_content"])
		equal(t, "developer", messages[0].(map[string]any)["role"])
		writeResponse(w)
	}, &ChatModelConfig{Model: "base", CustomHeader: map[string]string{"X-Default": "default"}, ParallelToolCalls: ptr(true), ToolStrict: ptr(true),
		LogProbs: true, TopLogProbs: 3, RepetitionPenalty: ptr(float32(1)), LogitBias: map[string]int{"7": -10}, User: ptr("user-1"),
		ResponseFormat: &chat.ChatCompletionResponseFormat{Type: chat.NewOptResponseFormatType(chat.ResponseFormatTypeJSONObject)},
	})
	bound, err := cm.WithTools([]*schema.ToolInfo{toolInfo("inspect")})
	check(t, err)
	equal(t, 0, len(cm.tools))
	ctx := callbacks.InitCallbacks(context.Background(), &callbacks.RunInfo{}, handler)
	msg, err := bound.Generate(ctx, []*schema.Message{
		{Role: schema.RoleType("developer"), Content: "instructions"},
		{Role: schema.Assistant, Content: "prior", ReasoningContent: "prior thinking", Extra: map[string]any{keyEncryptedContent: "prior cipher"}},
	}, fmodel.WithModel("override-model"), fmodel.WithTemperature(0), fmodel.WithTopP(0), fmodel.WithMaxTokens(0), fmodel.WithStop([]string{"stop"}),
		fmodel.WithToolChoice(schema.ToolChoiceForced), WithMaxCompletionTokens(16), WithParallelToolCalls(false),
		WithThinking(&chat.Thinking{Type: chat.NewOptThinkingMode(chat.ThinkingModeDisabled)}), WithReasoningEffort(chat.ReasoningEffortHigh), WithCustomHeader(map[string]string{"X-Custom": "override"}))
	check(t, err)
	equal(t, "done", msg.Content)
	equal(t, "think", msg.ReasoningContent)
	equal(t, "req-1", GetArkRequestID(msg))
	name, ok := GetModelName(msg)
	equal(t, true, ok)
	equal(t, "actual-model", name)
	tier, ok := GetServiceTier(msg)
	equal(t, true, ok)
	equal(t, "default", tier)
	cipher, ok := GetEncryptedContent(msg)
	equal(t, true, ok)
	equal(t, "cipher", cipher)
	status, ok := GetServiceStatus(msg)
	equal(t, true, ok)
	equal(t, true, status.ModelFallback.Value.FallbackTriggered)
	usage, ok := GetTokenUsageDetails(msg)
	equal(t, true, ok)
	equal(t, int32(1), usage.PromptTokensDetails.AudioTokens.Value)
	equal(t, 11, msg.ResponseMeta.Usage.TotalTokens)
	equal(t, "call-1", msg.ToolCalls[0].ID)
	equal(t, -0.2, msg.ResponseMeta.LogProbs.Content[0].LogProb)
	equal(t, []int64{100}, msg.ResponseMeta.LogProbs.Content[0].Bytes)
	if start == nil || output == nil {
		t.Fatal("callbacks missing")
	}
	equal(t, "override-model", start.Config.Model)
	equal(t, 11, output.TokenUsage.TotalTokens)
}

func TestSamplingPrecision(t *testing.T) {
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequest(t, r)
		equal(t, 0.7, body["temperature"])
		equal(t, 0.9, body["top_p"])
		equal(t, 0.2, body["presence_penalty"])
		writeResponse(w)
	}, &ChatModelConfig{Model: "test", Temperature: ptr(float32(0.7)), TopP: ptr(float32(0.9)), PresencePenalty: ptr(float32(0.2))})
	_, err := cm.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "hello"}})
	check(t, err)
}

func sseChunk(w io.Writer, content string) { _, _ = fmt.Fprintf(w, "data: %s\n\n", content) }
func TestStream(t *testing.T) {
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequest(t, r)
		equal(t, true, body["stream"])
		equal(t, map[string]any{"include_usage": true}, body["stream_options"])
		message := body["messages"].([]any)[0].(map[string]any)
		equal(t, "file-1", message["content"].([]any)[0].(map[string]any)["image_url"].(map[string]any)["file_id"])
		w.Header().Set("Content-Type", "text/event-stream")
		sseChunk(w, `{"id":"req-stream","object":"chat.completion.chunk","created":1,"model":"actual-model","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"think","encrypted_content":"enc","tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"inspect","arguments":"{\"path\":"}}]},"finish_reason":null}],"usage":null}`)
		sseChunk(w, `{"id":"req-stream","object":"chat.completion.chunk","created":1,"model":"actual-model","choices":[{"index":0,"delta":{"content":"done","encrypted_content":"rypted","tool_calls":[{"index":0,"function":{"arguments":"\"report.pdf\"}"}}]},"finish_reason":"tool_calls","moderation_hit_type":"text"}],"usage":null}`)
		sseChunk(w, `{"id":"req-stream","object":"chat.completion.chunk","created":1,"model":"actual-model","choices":[],"usage":{"prompt_tokens":8,"completion_tokens":3,"total_tokens":11,"prompt_tokens_details":{"cached_tokens":2},"completion_tokens_details":{"reasoning_tokens":1}},"service_tier":"default"}`)
		sseChunk(w, `[DONE]`)
	}, &ChatModelConfig{Model: "test"})
	sr, err := cm.Stream(context.Background(), []*schema.Message{{Role: schema.Tool, ToolCallID: "previous-call", UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{FileID: ptr("file-1")}}}}}})
	check(t, err)
	defer sr.Close()
	var chunks []*schema.Message
	for {
		chunk, e := sr.Recv()
		if errors.Is(e, io.EOF) {
			break
		}
		check(t, e)
		chunks = append(chunks, chunk)
	}
	equal(t, 3, len(chunks))
	final, err := schema.ConcatMessages(chunks)
	check(t, err)
	equal(t, "done", final.Content)
	equal(t, "think", final.ReasoningContent)
	equal(t, 11, final.ResponseMeta.Usage.TotalTokens)
	equal(t, 2, final.ResponseMeta.Usage.PromptTokenDetails.CachedTokens)
	equal(t, "req-stream", GetArkRequestID(final))
	cipher, ok := GetEncryptedContent(final)
	equal(t, true, ok)
	equal(t, "encrypted", cipher)
	equal(t, 1, len(final.ToolCalls))
	equal(t, `{"path":"report.pdf"}`, final.ToolCalls[0].Function.Arguments)
	equal(t, 0, *final.ToolCalls[0].Index)
}

func TestStreamRequiresDoneBeforeFinalModifier(t *testing.T) {
	for _, done := range []bool{false, true} {
		t.Run(map[bool]string{false: "truncated", true: "complete"}[done], func(t *testing.T) {
			cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				sseChunk(w, modifierChunk)
				if done {
					sseChunk(w, "[DONE]")
				}
			}, nil)
			finalized := make(chan struct{}, 1)
			reader, err := cm.Stream(context.Background(), nil, WithResponseChunkMessageModifier(
				func(_ context.Context, msg *schema.Message, _ []byte, end bool) (*schema.Message, error) {
					if end {
						finalized <- struct{}{}
					}
					return msg, nil
				}))
			check(t, err)
			defer reader.Close()
			msg, err := reader.Recv()
			check(t, err)
			equal(t, "done", msg.Content)
			_, err = reader.Recv()
			if done {
				equal(t, io.EOF, err)
				equal(t, 1, len(finalized))
			} else {
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("truncated stream reported success: %v", err)
				}
				equal(t, 0, len(finalized))
			}
		})
	}
}

func TestBatchAndErrors(t *testing.T) {
	t.Run("batch", func(t *testing.T) {
		cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
			equal(t, "/api/v3/batch/chat/completions", r.URL.Path)
			equal(t, false, decodeRequest(t, r)["stream"])
			writeResponse(w)
		}, &ChatModelConfig{Model: "test", BatchChat: &BatchChatConfig{EnableBatchChat: true, BatchChatAsyncRetryTimeout: time.Second, BatchMaxParallel: ptr(2)}})
		_, err := cm.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "hello"}})
		check(t, err)
		_, err = cm.Stream(context.Background(), nil)
		if err == nil {
			t.Fatal("expected streaming batch error")
		}
	})
	t.Run("api error", func(t *testing.T) {
		cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(400)
			_, _ = io.WriteString(w, `{"error":{"code":"InvalidParameter","message":"bad request","type":"invalid_request_error"}}`)
		}, nil)
		var reported error
		h := callbacks.NewHandlerBuilder().OnErrorFn(func(ctx context.Context, _ *callbacks.RunInfo, err error) context.Context { reported = err; return ctx }).Build()
		ctx := callbacks.InitCallbacks(context.Background(), &callbacks.RunInfo{}, h)
		_, err := cm.Generate(ctx, []*schema.Message{{Role: schema.User, Content: "hello"}})
		if err == nil || !strings.Contains(err.Error(), "bad request") || reported == nil {
			t.Fatalf("error=%v callback=%v", err, reported)
		}
	})
	t.Run("empty response", func(t *testing.T) {
		cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"id":"x","object":"chat.completion","created":1,"model":"m","choices":[]}`)
		}, nil)
		_, err := cm.Generate(context.Background(), nil)
		if !errors.Is(err, ErrEmptyResponse) {
			t.Fatalf("unexpected error %v", err)
		}
	})
	t.Run("stream error", func(t *testing.T) {
		cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			sseChunk(w, `{"error":{"message":"stream failed","type":"server_error","code":"InternalError"}}`)
		}, nil)
		sr, err := cm.Stream(context.Background(), nil)
		check(t, err)
		defer sr.Close()
		_, err = sr.Recv()
		if err == nil || !strings.Contains(err.Error(), "stream failed") {
			t.Fatalf("unexpected error %v", err)
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		stopped := make(chan struct{})
		cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(stopped)
		}, nil)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sr, err := cm.Stream(ctx, nil)
		check(t, err)
		defer sr.Close()
		cancel()
		_, err = sr.Recv()
		if err == nil {
			t.Fatal("expected cancellation")
		}
		select {
		case <-stopped:
		case <-time.After(2 * time.Second):
			t.Fatal("HTTP request not cancelled")
		}
	})
}
