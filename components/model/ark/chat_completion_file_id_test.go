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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func chatFileIDTestPart(kind string, common schema.MessagePartCommon) schema.MessageInputPart {
	switch kind {
	case "image":
		return schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: common, Detail: schema.ImageURLDetailHigh}}
	case "video":
		video := &schema.MessageInputVideo{MessagePartCommon: common}
		SetInputVideoFPS(video, 2)
		return schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: video}
	case "audio":
		return schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: common}}
	default:
		return schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: common, Name: "report.pdf"}}
	}
}

func TestChatFileIDRequestWire(t *testing.T) {
	for _, mode := range []string{"generate", "stream", "batch"} {
		t.Run(mode, func(t *testing.T) {
			type capturedRequest struct {
				body   []byte
				header http.Header
				path   string
				err    error
			}
			requests := make(chan capturedRequest, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				requests <- capturedRequest{body: body, header: r.Header.Clone(), path: r.URL.Path, err: err}
				if mode == "stream" {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "%s\n\n", `data: {"id":"test","model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"},"finish_reason":null}]}`)
					fmt.Fprintf(w, "%s\n\ndata: [DONE]\n\n", `data: {"id":"test","model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"test","model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
			}))
			defer server.Close()

			config := &ChatModelConfig{
				APIKey: "test-key", Model: "test-model", BaseURL: server.URL, HTTPClient: server.Client(),
				Temperature: ptrOf(float32(0)), MaxTokens: ptrOf(0),
				CustomHeader: map[string]string{"X-Test-Header": "kept"},
			}
			if mode == "batch" {
				config.BatchChat = &BatchChatConfig{EnableBatchChat: true, BatchChatAsyncRetryTimeout: time.Second}
			}
			cm, err := NewChatModel(context.Background(), config)
			if err != nil {
				t.Fatal(err)
			}
			parts := []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "inspect"}}
			for _, kind := range []string{"image", "video", "audio", "file"} {
				id := "file-" + kind
				// Empty legacy sources do not select a source in the Chat adapter.
				parts = append(parts, chatFileIDTestPart(kind, schema.MessagePartCommon{FileID: &id, URL: new(string), Base64Data: new(string)}))
			}
			input := []*schema.Message{
				{Role: schema.System, Content: "system text"},
				{Role: schema.User, Name: "reader", UserInputMultiContent: parts},
				{Role: schema.Tool, ToolCallID: "call-1", UserInputMultiContent: []schema.MessageInputPart{parts[1]}},
			}
			if mode == "stream" {
				stream, err := cm.Stream(context.Background(), input)
				if err != nil {
					t.Fatal(err)
				}
				defer stream.Close()
				var output strings.Builder
				for {
					chunk, err := stream.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					output.WriteString(chunk.Content)
				}
				if output.String() != "ok" {
					t.Fatalf("wrong stream output: %q", output.String())
				}
			} else {
				output, err := cm.Generate(context.Background(), input)
				if err != nil {
					t.Fatal(err)
				}
				if output.Content != "ok" {
					t.Fatalf("wrong output: %+v", output)
				}
			}
			request := <-requests
			if request.err != nil {
				t.Fatal(request.err)
			}
			expectedPath := "/chat/completions"
			if mode == "batch" {
				expectedPath = "/batch/chat/completions"
			}
			if request.path != expectedPath || request.header.Get("X-Test-Header") != "kept" || request.header.Get("Authorization") != "Bearer test-key" {
				t.Fatalf("wrong transport metadata: %+v", request)
			}
			var payload map[string]any
			if err := json.Unmarshal(request.body, &payload); err != nil {
				t.Fatal(err)
			}
			if payload["model"] != "test-model" || payload["temperature"] != float64(0) || payload["max_tokens"] != float64(0) {
				t.Fatalf("request options changed: %s", request.body)
			}
			if mode == "stream" && payload["stream"] != true {
				t.Fatalf("missing stream flag: %s", request.body)
			}
			messages := payload["messages"].([]any)
			first := messages[0].(map[string]any)
			last := messages[2].(map[string]any)
			if first["content"] != "system text" || first["role"] != "system" || last["tool_call_id"] != "call-1" {
				t.Fatalf("message metadata changed: %s", request.body)
			}
			user := messages[1].(map[string]any)
			wantContent := []any{
				map[string]any{"type": "text", "text": "inspect"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"file_id": "file-image", "detail": "high"}},
				map[string]any{"type": "video_url", "video_url": map[string]any{"file_id": "file-video", "fps": float64(2)}},
				map[string]any{"type": "input_audio", "input_audio": map[string]any{"file_id": "file-audio"}},
				map[string]any{"type": "file", "file": map[string]any{"file_id": "file-file", "filename": "report.pdf"}},
			}
			if user["name"] != "reader" || !reflect.DeepEqual(user["content"], wantContent) {
				t.Fatalf("wrong wire content: %s", request.body)
			}
			if !reflect.DeepEqual(last["content"], []any{wantContent[1]}) {
				t.Fatalf("tool reference was lost: %s", request.body)
			}
		})
	}
}

func TestChatFileIDSourcePrecedence(t *testing.T) {
	cm := &completionAPIChatModel{}
	id := "file-123"
	url := "https://example.com/media"
	data := "YWJj"
	invalidData := "data:image/png;base64,YWJj"
	for _, kind := range []string{"image", "video", "audio", "file"} {
		t.Run(kind, func(t *testing.T) {
			for _, common := range []schema.MessagePartCommon{
				{URL: &url},
				{Base64Data: &data, MIMEType: "image/png"},
				{URL: &url, Base64Data: &invalidData},
				{Base64Data: &data},
				{Base64Data: &invalidData, MIMEType: "image/png"},
			} {
				convert := func(common schema.MessagePartCommon) (*chatCompletionMessageContent, error) {
					return cm.toArkContent(&schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{chatFileIDTestPart(kind, common)}})
				}
				want, wantErr := convert(common)
				common.FileID = &id
				got, err := convert(common)
				if wantErr != nil {
					if err == nil || err.Error() != wantErr.Error() {
						t.Fatalf("selected source error changed: got %v, want %v", err, wantErr)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				wantJSON, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				gotJSON, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				if string(gotJSON) != string(wantJSON) {
					t.Fatalf("source precedence changed: got %s, want %s", gotJSON, wantJSON)
				}
			}
			common := schema.MessagePartCommon{FileID: new(string)}
			if _, err := cm.toArkContent(&schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{chatFileIDTestPart(kind, common)}}); err == nil {
				t.Fatal("expected an empty FileID to fail")
			}
		})
	}
}

func TestChatFileIDContextUnsupported(t *testing.T) {
	id := "file-123"
	input := []*schema.Message{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		chatFileIDTestPart("image", schema.MessagePartCommon{FileID: &id}),
	}}}
	cm := &completionAPIChatModel{}
	_, err := cm.genRequest(input, &fmodel.Options{}, &arkOptions{cache: &CacheOption{ContextID: ptrOf("context-1")}})
	if err == nil || !strings.Contains(err.Error(), "Context API caching") {
		t.Fatalf("context chat must not silently drop FileID: %v", err)
	}
	chatModel := &ChatModel{chatModel: cm}
	_, err = chatModel.createContextByContextAPI(context.Background(), input, 0, model.ContextModeSession, nil)
	if err == nil || !strings.Contains(err.Error(), "Context API caching") {
		t.Fatalf("context creation must not silently drop FileID: %v", err)
	}
}
