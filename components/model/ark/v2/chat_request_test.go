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
	"bytes"
	"context"
	"encoding/gob"
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
		sseChunk(w, chatTextChunk)
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
							sseChunk(w, chatTextChunk)
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

func TestChatEmptyChoiceStream(t *testing.T) {
	for _, tc := range []struct{ name, payload string }{
		{"done only", ""},
		{"usage only", `{"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":0,"total_tokens":1,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`},
		{"other choice", `{"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":1,"delta":{"content":"ignored"}}],"usage":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cm := testModel(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				if tc.payload != "" {
					sseChunk(w, tc.payload)
				}
				sseChunk(w, "[DONE]")
			}, nil)
			reader, err := cm.Stream(context.Background(), nil)
			check(t, err)
			_, err = schema.ConcatMessageStream(reader)
			if !errors.Is(err, ErrEmptyResponse) {
				t.Fatalf("want ErrEmptyResponse, got %v", err)
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
	message := callChatModel(t, context.Background(), cm, "stream")
	equal(t, schema.Assistant, message.Role)
	equal(t, "", message.Content)
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
		config := &cm.config
		data, err := json.Marshal([]any{config.Model, config.ToolStrict, config.Stop, config.CustomHeader,
			config.Thinking, config.ResponseFormat, config.MaxCompletionTokens, config.ParallelToolCalls,
			cm.rawTools, common, current, tools, stops, names,
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

func TestInvalidInputsDoNotSendRequests(t *testing.T) {
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid input reached HTTP transport")
		writeResponse(w)
	}, nil)
	cases := map[string][]*schema.Message{
		"nil":                      {nil},
		"unknown role":             {{Role: "unknown", Content: "text"}},
		"mixed":                    {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText}}, AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText}}}},
		"wrong input role":         {{Role: schema.Assistant, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText}}}},
		"nil image":                {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeImageURL}}}},
		"nil audio":                {{Role: schema.Tool, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeAudioURL}}}},
		"nil video":                {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeVideoURL}}}},
		"nil file":                 {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeFileURL}}}},
		"no source":                {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{}}}}},
		"data URL in base64":       {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: ptr("data:image/png;base64,eA=="), MIMEType: "image/png", FileID: ptr("fallback")}}}}}},
		"base64 without MIME":      {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: ptr("eA=="), FileID: ptr("fallback")}}}}}},
		"file without name":        {{Role: schema.Tool, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{Base64Data: ptr("eA=="), MIMEType: "application/pdf"}}}}}},
		"file base64 without MIME": {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{Name: "fixture.pdf", MessagePartCommon: schema.MessagePartCommon{Base64Data: ptr("eA=="), FileID: ptr("fallback")}}}}}},
	}
	for name, messages := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := cm.Generate(context.Background(), messages)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	_, err := cm.Generate(context.Background(), nil, model.WithMaxTokens(1<<32))
	if err == nil {
		t.Fatal("expected integer overflow error")
	}
	_, err = cm.Generate(context.Background(), nil, WithMaxCompletionTokens(1<<32))
	if err == nil {
		t.Fatal("expected integer overflow error")
	}
}

func TestLegacyContentAndAssistantHistory(t *testing.T) {
	video := schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{URL: ptr("https://example.com/video")}}}
	SetInputVideoFPS(&video, 0.25)
	output := schema.MessageOutputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{FileID: ptr("video-file")}}, Extra: map[string]any{keyVideoFPS: 0.5}}
	cases := []struct {
		msg        *schema.Message
		field, key string
		want       any
	}{
		{&schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{video}}, "video_url", "fps", 0.25},
		{&schema.Message{Role: schema.User, MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: "https://example.com/image", Detail: schema.ImageURLDetailLow}}}}, "image_url", "detail", "low"},
		{&schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{output}}, "video_url", "file_id", "video-file"},
		{&schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{FileID: ptr("image-file")}}}}}, "image_url", "file_id", "image-file"},
	}
	for _, tc := range cases {
		msg, err := toArkMessage(tc.msg)
		check(t, err)
		data, err := json.Marshal(&msg)
		check(t, err)
		var body map[string]any
		check(t, json.Unmarshal(data, &body))
		equal(t, tc.want, body["content"].([]any)[0].(map[string]any)[tc.field].(map[string]any)[tc.key])
	}
	equal(t, 0.5, *GetOutputVideoFPS(&output))
	equal(t, 0.25, *GetInputVideoFPS(&video))
}

func TestToolChoiceAndBinding(t *testing.T) {
	cm, err := NewChatModel(context.Background(), &ChatModelConfig{APIKey: "test"})
	check(t, err)
	if _, err = cm.WithTools(nil); err == nil {
		t.Fatal("expected empty tools error")
	}
	if err = cm.BindTools([]*schema.ToolInfo{nil}); err == nil {
		t.Fatal("expected nil tool error")
	}
	check(t, cm.BindForcedTools([]*schema.ToolInfo{toolInfo("a"), toolInfo("b")}))
	for _, tc := range []struct {
		choice schema.ToolChoice
		names  []string
		want   string
		fail   bool
	}{
		{schema.ToolChoiceAllowed, nil, `"auto"`, false},
		{schema.ToolChoiceForbidden, nil, `"none"`, false},
		{schema.ToolChoiceForced, nil, `"required"`, false},
		{schema.ToolChoiceForced, []string{"b"}, `{"type":"function","function":{"name":"b"}}`, false},
		{schema.ToolChoiceForced, []string{"a", "b"}, `"required"`, false},
		{schema.ToolChoiceForced, []string{"absent"}, "", true},
	} {
		tools, err := cm.buildChatTools(&model.Options{ToolChoice: &tc.choice, AllowedToolNames: tc.names})
		if tc.fail {
			if err == nil {
				t.Fatal("expected tool choice error")
			}
			continue
		}
		check(t, err)
		data, err := json.Marshal(&tools.choice)
		check(t, err)
		equal(t, tc.want, string(data))
	}
	_, err = (&ChatModel{}).buildChatTools(&model.Options{ToolChoice: ptr(schema.ToolChoiceForced)})
	if err == nil {
		t.Fatal("expected no tools error")
	}
}

func TestMetadataSurvivesCheckpoint(t *testing.T) {
	var resp chat.ChatCompletionResponse
	check(t, json.Unmarshal([]byte(responseJSON), &resp))
	msg, err := resolveChatResponse(&resp)
	check(t, err)
	var buffer bytes.Buffer
	check(t, gob.NewEncoder(&buffer).Encode(msg))
	var restored schema.Message
	check(t, gob.NewDecoder(&buffer).Decode(&restored))
	equal(t, "req-1", GetArkRequestID(&restored))
	status, ok := GetServiceStatus(&restored)
	equal(t, true, ok)
	equal(t, true, status.ModelFallback.Value.FallbackTriggered)
	usage, ok := GetTokenUsageDetails(&restored)
	equal(t, true, ok)
	equal(t, int32(11), usage.TotalTokens)
}

func TestFileBase64UsesDataURL(t *testing.T) {
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequest(t, r)
		file := body["messages"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)["file"].(map[string]any)
		// The real Chat API rejects bare base64 with InvalidParameter(file_data).
		equal(t, "data:application/pdf;base64,eA==", file["file_data"])
		equal(t, "fixture.pdf", file["filename"])
		writeResponse(w)
	}, &ChatModelConfig{Model: "model"})
	_, err := cm.Generate(context.Background(), []*schema.Message{{Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeFileURL,
			File: &schema.MessageInputFile{Name: "fixture.pdf", MessagePartCommon: schema.MessagePartCommon{
				Base64Data: ptr("eA=="), MIMEType: "application/pdf",
			}},
		}},
	}})
	check(t, err)
}

func TestStructuredOutputSerialization(t *testing.T) {
	schemaJSON := []byte(`{"type":"object","properties":{"count":{"type":"integer","minimum":9007199254740993}},"required":["count"],"additionalProperties":false}`)
	format := &chat.ChatCompletionResponseFormat{
		Type: chat.NewOptResponseFormatType(chat.ResponseFormatTypeJSONSchema),
		JSONSchema: chat.NewOptChatCompletionResponseFormatJsonSchema(chat.ChatCompletionResponseFormatJsonSchema{
			Name: "result", Strict: chat.NewOptBool(true), Schema: schemaJSON,
		}),
	}
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		// Use the standard decoder as an independent check of Sonic and SDK output.
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		var body map[string]any
		check(t, decoder.Decode(&body))
		actual := body["response_format"].(map[string]any)
		equal(t, "json_schema", actual["type"])
		configuration := actual["json_schema"].(map[string]any)
		equal(t, true, configuration["strict"])
		equal(t, "result", configuration["name"])
		schema := configuration["schema"].(map[string]any)
		equal(t, false, schema["additionalProperties"])
		count := schema["properties"].(map[string]any)["count"].(map[string]any)
		equal(t, json.Number("9007199254740993"), count["minimum"])
		writeResponse(w)
	}, &ChatModelConfig{Model: "model", ResponseFormat: format})
	_, err := cm.Generate(context.Background(), nil)
	check(t, err)
	equal(t, string(schemaJSON), string(format.JSONSchema.Value.Schema))
}
