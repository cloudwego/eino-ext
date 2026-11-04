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
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

func callChatModel(t *testing.T, ctx context.Context, cm model.ToolCallingChatModel, mode string, options ...model.Option) *schema.Message {
	t.Helper()
	input := []*schema.Message{schema.UserMessage("hello")}
	if mode == "stream" {
		reader, err := cm.Stream(ctx, input, options...)
		check(t, err)
		message, err := schema.ConcatMessageStream(reader)
		check(t, err)
		return message
	}
	message, err := cm.Generate(ctx, input, options...)
	check(t, err)
	return message
}

func writeChatModelResponse(w http.ResponseWriter, r *http.Request) {
	body := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body["stream"] == true {
		w.Header().Set("Content-Type", "text/event-stream")
		sseChunk(w, modifierChunk)
		sseChunk(w, "[DONE]")
	} else {
		writeResponse(w)
	}
}

func TestChatToolSelectionAndCallbacks(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, source := range []string{"bound", "option"} {
			for _, tc := range []struct {
				name     string
				choice   schema.ToolChoice
				names    []string
				want     any
				selected []string
			}{
				{"allowed subset", schema.ToolChoiceAllowed, []string{"other"}, "auto", []string{"other"}},
				{"forced subset", schema.ToolChoiceForced, []string{"inspect", "other"}, "required", []string{"inspect", "other"}},
				{"named", schema.ToolChoiceForced, []string{"inspect"}, map[string]any{"type": "function", "function": map[string]any{"name": "inspect"}}, []string{"inspect"}},
				{"unnamed single", schema.ToolChoiceForced, nil, "required", []string{"inspect"}},
			} {
				t.Run(mode+"/"+source+"/"+tc.name, func(t *testing.T) {
					tools := []*schema.ToolInfo{toolInfo("inspect"), toolInfo("other"), toolInfo("excluded")}
					if tc.name == "unnamed single" {
						tools = tools[:1]
					}
					var started []string
					handler := callbacks.NewHandlerBuilder().OnStartFn(func(ctx context.Context, _ *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
						for _, tool := range input.(*model.CallbackInput).Tools {
							started = append(started, tool.Name)
						}
						return ctx
					}).Build()
					cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
						raw, err := io.ReadAll(r.Body)
						if err != nil {
							t.Error(err)
							return
						}
						var body map[string]any
						check(t, json.Unmarshal(raw, &body))
						equal(t, tc.want, body["tool_choice"])
						var actual []string
						for _, value := range body["tools"].([]any) {
							actual = append(actual, value.(map[string]any)["function"].(map[string]any)["name"].(string))
						}
						equal(t, tc.selected, actual)
						if body["stream"] == true {
							w.Header().Set("Content-Type", "text/event-stream")
							sseChunk(w, modifierChunk)
							sseChunk(w, "[DONE]")
						} else {
							writeResponse(w)
						}
					}, nil)
					options := []model.Option{model.WithToolChoice(tc.choice, tc.names...)}
					if source == "bound" {
						check(t, cm.BindTools(tools))
					} else {
						options = append(options, model.WithTools(tools))
					}
					callChatModel(t, callbacks.InitCallbacks(context.Background(), nil, handler), cm, mode, options...)
					equal(t, tc.selected, started)
				})
			}
		}
	}
}

func TestChatBindingOwnsToolSlice(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		t.Run(mode, func(t *testing.T) {
			tools := []*schema.ToolInfo{toolInfo("inspect"), toolInfo("other")}
			cm := testModel(t, writeChatModelResponse, nil)
			bound, err := cm.WithTools(tools)
			check(t, err)
			tools[0] = toolInfo("replacement")
			tools[1] = nil
			var names []string
			handler := callbacks.NewHandlerBuilder().OnStartFn(func(ctx context.Context, _ *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
				for _, tool := range input.(*model.CallbackInput).Tools {
					if tool == nil {
						t.Error("binding retained caller's slice")
						continue
					}
					names = append(names, tool.Name)
				}
				return ctx
			}).Build()
			callChatModel(t, callbacks.InitCallbacks(context.Background(), nil, handler), bound, mode)
			equal(t, []string{"inspect", "other"}, names)
		})
	}
}

func TestChatSelectsToolsBeforeEncoding(t *testing.T) {
	cm := testModel(t, writeChatModelResponse, nil)
	tools := []*schema.ToolInfo{toolInfo("inspect"), {Name: "unused", Extra: map[string]any{toolStrictKey: "unused"}}}
	callChatModel(t, context.Background(), cm, "generate", model.WithTools(tools), model.WithToolChoice(schema.ToolChoiceAllowed, "inspect"))
}

func TestChatModifierCallbackConfig(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, remove := range []bool{false, true} {
			name := mode + "/edit"
			if remove {
				name = mode + "/delete"
			}
			t.Run(name, func(t *testing.T) {
				var observed []*model.CallbackOutput
				handler := callbacks.NewHandlerBuilder().OnEndFn(func(ctx context.Context, _ *callbacks.RunInfo, out callbacks.CallbackOutput) context.Context {
					observed = append(observed, out.(*model.CallbackOutput))
					return ctx
				}).OnEndWithStreamOutputFn(func(ctx context.Context, _ *callbacks.RunInfo, reader *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
					defer reader.Close()
					for {
						out, err := reader.Recv()
						if err != nil {
							return ctx
						}
						observed = append(observed, out.(*model.CallbackOutput))
					}
				}).Build()
				config := &ChatModelConfig{Model: "original", MaxTokens: ptr(7), Temperature: ptr(float32(0.5)), TopP: ptr(float32(0.9)), Stop: []string{"old"}, Thinking: &chat.Thinking{Type: chat.NewOptThinkingMode(chat.ThinkingModeDisabled)}}
				config.RequestPayloadModifier = func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
					return modifyJSON(raw, func(fields map[string]json.RawMessage) {
						fields["model"] = json.RawMessage(`"modified"`)
						if remove {
							for _, key := range []string{"max_tokens", "temperature", "top_p", "stop", "thinking"} {
								delete(fields, key)
							}
						} else {
							fields["max_tokens"] = json.RawMessage(`123`)
							fields["temperature"] = json.RawMessage(`0.25`)
							fields["top_p"] = json.RawMessage(`0.75`)
							fields["stop"] = json.RawMessage(`"END"`)
							fields["thinking"] = json.RawMessage(`{"type":"enabled"}`)
						}
					})
				}
				cm := testModel(t, writeChatModelResponse, config)
				callChatModel(t, callbacks.InitCallbacks(context.Background(), nil, handler), cm, mode)
				if len(observed) == 0 {
					t.Fatal("missing output callback")
				}
				for _, out := range observed {
					equal(t, "modified", out.Config.Model)
					if remove {
						equal(t, 0, out.Config.MaxTokens)
						equal(t, float32(0), out.Config.Temperature)
						equal(t, float32(0), out.Config.TopP)
						equal(t, []string(nil), out.Config.Stop)
						equal(t, (*chat.Thinking)(nil), out.Extra["thinking"])
					} else {
						equal(t, 123, out.Config.MaxTokens)
						equal(t, float32(0.25), out.Config.Temperature)
						equal(t, float32(0.75), out.Config.TopP)
						equal(t, []string{"END"}, out.Config.Stop)
						equal(t, chat.ThinkingModeEnabled, out.Extra["thinking"].(*chat.Thinking).Type.Or(""))
					}
				}
			})
		}
	}
}

func TestChatEmptyChoiceStream(t *testing.T) {
	for _, tc := range []struct{ name, payload string }{
		{"done only", ""},
		{"usage only", `{"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":0,"total_tokens":1,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`},
		{"other choice", `{"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":1,"delta":{"content":"ignored"},"finish_reason":null}],"usage":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			finalized := false
			cm := testModel(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				if tc.payload != "" {
					sseChunk(w, tc.payload)
				}
				sseChunk(w, "[DONE]")
			}, nil)
			reader, err := cm.Stream(context.Background(), nil, WithResponseChunkMessageModifier(func(_ context.Context, message *schema.Message, _ []byte, end bool) (*schema.Message, error) {
				if end {
					finalized = true
				}
				return message, nil
			}))
			check(t, err)
			_, err = schema.ConcatMessageStream(reader)
			if !errors.Is(err, ErrEmptyResponse) {
				t.Fatalf("want ErrEmptyResponse, got %v", err)
			}
			if finalized {
				t.Error("empty choice stream finalized successfully")
			}
		})
	}
}

func TestChatRoleOnlyStream(t *testing.T) {
	cm := testModel(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseChunk(w, `{"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":"stop"}],"usage":null}`)
		sseChunk(w, "[DONE]")
	}, nil)
	finalized := false
	message := callChatModel(t, context.Background(), cm, "stream", WithResponseChunkMessageModifier(func(_ context.Context, message *schema.Message, _ []byte, end bool) (*schema.Message, error) {
		finalized = finalized || end
		return message, nil
	}))
	equal(t, schema.Assistant, message.Role)
	equal(t, "", message.Content)
	equal(t, true, finalized)
}

func TestChatSharedRequestInputs(t *testing.T) {
	cm := testModel(t, writeChatModelResponse, &ChatModelConfig{
		Model: "original", ToolStrict: ptr(true), Stop: []string{"instance"},
		CustomHeader: map[string]string{"X-Instance": "original"},
		Thinking:     &chat.Thinking{Type: chat.NewOptThinkingMode(chat.ThinkingModeDisabled)},
		ResponseFormat: &chat.ChatCompletionResponseFormat{
			Type: chat.NewOptResponseFormatType(chat.ResponseFormatTypeJSONSchema),
			JSONSchema: chat.NewOptChatCompletionResponseFormatJsonSchema(chat.ChatCompletionResponseFormatJsonSchema{
				Name: "answer", Schema: []byte(`{"type":"object"}`),
			}),
		},
	})
	check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("bound")}))
	// Sentinels reveal writes through the capacity of a caller-owned slice.
	tools := []*schema.ToolInfo{toolInfo("inspect"), toolInfo("other"), toolInfo("sentinel")}
	stops := []string{"call", "sentinel"}
	names := []string{"inspect", "sentinel"}
	options := []model.Option{
		model.WithTools(tools[:2]), model.WithToolChoice(schema.ToolChoiceForced, names[:1]...), model.WithStop(stops[:1]),
		WithMaxCompletionTokens(16), WithParallelToolCalls(false), WithCustomHeader(map[string]string{"X-Call": "original"}),
	}
	common, specified := cm.getOptions(options...)
	snapshot := func() string {
		current, ark := cm.getOptions(options...)
		data, err := json.Marshal([]any{cm.config, cm.rawTools, common, current, tools, stops, names,
			specified.customHeaders, specified.thinking, specified.maxCompletionTokens, specified.parallelToolCalls,
			ark.customHeaders, ark.thinking, ark.maxCompletionTokens, ark.parallelToolCalls})
		check(t, err)
		return string(data)
	}
	before := snapshot()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(stream bool) {
			defer wg.Done()
			if stream {
				reader, err := cm.Stream(context.Background(), nil, options...)
				if err == nil {
					_, err = schema.ConcatMessageStream(reader)
				}
				if err != nil {
					t.Error(err)
				}
			} else if _, err := cm.Generate(context.Background(), nil, options...); err != nil {
				t.Error(err)
			}
		}(i%2 == 0)
	}
	wg.Wait()
	equal(t, before, snapshot())

	request, err := cm.genRequest(nil, common, specified)
	check(t, err)
	request.payload.Tools[0].Function.Name = "changed"
	request.payload.ResponseFormat.Value.JSONSchema.Value.Schema[0] = '['
	stop, _ := request.payload.Stop.Value.GetStringArray()
	stop[0] = "changed"
	equal(t, before, snapshot())
}
