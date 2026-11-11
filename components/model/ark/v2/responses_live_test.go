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
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/volcengine/ark-runtime-go/arkruntime"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/file"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

// Opt in explicitly: ARK_RESPONSES_LIVE=1 ARK_API_KEY=... ARK_MODEL=... go test -run TestResponsesLive -v.
func TestResponsesLive(t *testing.T) {
	if os.Getenv("ARK_RESPONSES_LIVE") != "1" {
		t.Skip("set ARK_RESPONSES_LIVE=1 to call the live Ark API")
	}
	if os.Getenv("ARK_API_KEY") == "" || os.Getenv("ARK_MODEL") == "" {
		t.Fatal("set ARK_API_KEY and ARK_MODEL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	config := &ResponsesAPIConfig{APIKey: os.Getenv("ARK_API_KEY"), Model: os.Getenv("ARK_MODEL"), BaseURL: os.Getenv("ARK_BASE_URL"),
		Thinking: &responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeDisabled)}}
	cm, err := NewResponsesAPIChatModel(ctx, config)
	check(t, err)
	t.Run("generate", func(t *testing.T) {
		msg, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("Reply with exactly ARK_OK.")})
		check(t, err)
		if !strings.Contains(msg.Content, "ARK_OK") || msg.ResponseMeta.Usage == nil {
			t.Fatalf("unexpected content or missing usage: %q", msg.Content)
		}
		id, _ := GetResponseID(msg)
		if id == "" {
			t.Fatal("missing response ID")
		}
	})
	t.Run("stream", func(t *testing.T) {
		sr, err := cm.Stream(ctx, []*schema.Message{schema.UserMessage("Reply with exactly STREAM_OK.")})
		check(t, err)
		msg, err := readResponsesStream(t, sr)
		check(t, err)
		if !strings.Contains(msg.Content, "STREAM_OK") || msg.ResponseMeta.Usage == nil {
			t.Fatalf("unexpected stream content or missing usage: %q", msg.Content)
		}
	})
	t.Run("tools", func(t *testing.T) {
		bound, err := cm.WithTools([]*schema.ToolInfo{toolInfo("inspect")})
		check(t, err)
		history := []*schema.Message{schema.UserMessage("Call inspect with path /tmp/example, then tell me its result.")}
		sr, err := bound.Stream(ctx, history, model.WithToolChoice(schema.ToolChoiceForced, "inspect"))
		check(t, err)
		assistant, err := readResponsesStream(t, sr)
		check(t, err)
		if len(assistant.ToolCalls) != 1 || assistant.ToolCalls[0].Function.Name != "inspect" {
			t.Fatalf("unexpected calls: %+v", assistant.ToolCalls)
		}
		call := assistant.ToolCalls[0]
		if call.ID == "" || !strings.Contains(call.Function.Arguments, "/tmp/example") {
			t.Fatalf("invalid tool call: %+v", call)
		}
		history = append(history, assistant, &schema.Message{Role: schema.Tool, ToolCallID: call.ID, Content: "The file contains TOOL_OK."})
		final, err := bound.Generate(ctx, history, model.WithToolChoice(schema.ToolChoiceForbidden))
		check(t, err)
		if !strings.Contains(final.Content, "TOOL_OK") {
			t.Fatalf("tool result missing: %q", final.Content)
		}
	})
	t.Run("session", func(t *testing.T) {
		if os.Getenv("ARK_RESPONSES_LIVE_CACHE") != "1" {
			t.Skip("set ARK_RESPONSES_LIVE_CACHE=1 for accounts with the cache service enabled")
		}
		next := *config
		next.SessionCache = &SessionCacheConfig{EnableCache: true, TTL: 3600}
		next.ToolStrict = ptr(true)
		cached, err := NewResponsesAPIChatModel(ctx, &next)
		check(t, err)
		check(t, cached.BindTools([]*schema.ToolInfo{toolInfo("inspect")}))
		first, err := cached.Generate(ctx, []*schema.Message{schema.UserMessage("Remember the code CACHE_731. Reply OK.")}, model.WithToolChoice(schema.ToolChoiceForbidden))
		check(t, err)
		expires, ok := GetCacheExpiration(first)
		if !ok || expires <= time.Now().Unix() {
			t.Fatal("missing valid cache expiration")
		}
		msg, err := cached.Generate(ctx, []*schema.Message{first, schema.UserMessage("What was the code? Reply with the code only.")}, model.WithToolChoice(schema.ToolChoiceForbidden))
		check(t, err)
		if !strings.Contains(msg.Content, "CACHE_731") {
			t.Fatalf("context missing: %q", msg.Content)
		}
	})
	t.Run("structured", func(t *testing.T) {
		next := *config
		next.ResponseFormat = &responses.TextFormat{Type: responses.TextFormatTypeJSONObject}
		formatted, err := NewResponsesAPIChatModel(ctx, &next)
		check(t, err)
		msg, err := formatted.Generate(ctx, []*schema.Message{schema.UserMessage(`Return a JSON object with field status equal to OK.`)})
		check(t, err)
		if !strings.Contains(msg.Content, `"status"`) {
			t.Fatalf("not JSON: %q", msg.Content)
		}
	})
	t.Run("strict_schema", func(t *testing.T) {
		next := *config
		next.ResponseFormat = &responses.TextFormat{}
		check(t, sonic.UnmarshalString(`{"type":"json_schema","name":"answer","strict":true,"schema":{"type":"object","properties":{"status":{"type":"string","enum":["OK"]}},"required":["status"],"additionalProperties":false}}`, next.ResponseFormat))
		formatted, err := NewResponsesAPIChatModel(ctx, &next)
		check(t, err)
		msg, err := formatted.Generate(ctx, []*schema.Message{schema.UserMessage("Return status OK.")})
		check(t, err)
		var answer struct {
			Status string `json:"status"`
		}
		check(t, sonic.UnmarshalString(msg.Content, &answer))
		equal(t, "OK", answer.Status)
	})
	t.Run("modifiers", func(t *testing.T) {
		request := WithRequestPayloadModifier(func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
			var fields map[string]interface{}
			if err := sonic.Unmarshal(raw, &fields); err != nil {
				return nil, err
			}
			fields["max_output_tokens"] = 128
			return sonic.Marshal(fields)
		})
		msg, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("Reply OK.")}, request, WithResponseMessageModifier(func(_ context.Context, msg *schema.Message, raw []byte) (*schema.Message, error) {
			if len(raw) == 0 {
				t.Error("missing raw response")
			}
			msg.Content += "_MODIFIED"
			return msg, nil
		}))
		check(t, err)
		if !strings.HasSuffix(msg.Content, "_MODIFIED") {
			t.Fatal("Generate modifier missing")
		}
		finalized := 0
		sr, err := cm.Stream(ctx, []*schema.Message{schema.UserMessage("Reply OK.")}, request, WithResponseChunkMessageModifier(func(_ context.Context, msg *schema.Message, raw []byte, end bool) (*schema.Message, error) {
			if end {
				finalized++
				return &schema.Message{Content: "_FINAL"}, nil
			}
			if len(raw) == 0 {
				t.Error("missing raw event")
			}
			return msg, nil
		}))
		check(t, err)
		msg, err = readResponsesStream(t, sr)
		check(t, err)
		equal(t, 1, finalized)
		if !strings.HasSuffix(msg.Content, "_FINAL") {
			t.Fatal("stream finalization missing")
		}
	})
	t.Run("reasoning_passback", func(t *testing.T) {
		next := *config
		next.Thinking = &responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeEnabled)}
		reasoning, err := NewResponsesAPIChatModel(ctx, &next)
		check(t, err)
		sr, err := reasoning.Stream(ctx, []*schema.Message{schema.UserMessage("What is 17 times 19? Answer briefly.")})
		check(t, err)
		assistant, err := readResponsesStream(t, sr)
		check(t, err)
		hasReasoning := false
		for _, part := range assistant.AssistantGenMultiContent {
			if part.Reasoning != nil && (part.Reasoning.Text != "" || part.Reasoning.Signature != "") {
				hasReasoning = true
			}
		}
		if !hasReasoning {
			t.Fatal("missing reasoning parts")
		}
		history := []*schema.Message{schema.UserMessage("What is 17 times 19? Answer briefly."), assistant, schema.UserMessage("Add one to that answer. Reply with only the number.")}
		answer, err := reasoning.Generate(ctx, history, WithResponsesThinking(config.Thinking))
		check(t, err)
		if !strings.Contains(answer.Content, "324") {
			t.Fatalf("reasoning followup failed: %q", answer.Content)
		}
	})
	t.Run("image_base64", func(t *testing.T) {
		data, err := os.ReadFile("../examples/generate_with_image/eino.png")
		check(t, err)
		encoded := base64.StdEncoding.EncodeToString(data)
		msg, err := cm.Generate(ctx, []*schema.Message{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "Describe this image briefly."},
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &encoded, MIMEType: "image/png"}}},
		}}})
		check(t, err)
		if msg.Content == "" {
			t.Fatal("empty image description")
		}
	})
	t.Run("pdf", func(t *testing.T) {
		data := responsesTestPDF()
		encoded := base64.StdEncoding.EncodeToString(data)
		ask := func(common schema.MessagePartCommon) {
			t.Helper()
			msg, err := cm.Generate(ctx, []*schema.Message{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "What is the code in this document? Reply with the code only."},
				{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: common, Name: "fixture.pdf"}},
			}}})
			check(t, err)
			if !strings.Contains(msg.Content, "PDF_OK_731") {
				t.Fatalf("PDF code missing: %q", msg.Content)
			}
		}
		ask(schema.MessagePartCommon{Base64Data: &encoded, MIMEType: "application/pdf"})
		uploaded, err := cm.client.UploadFile(ctx, &file.FileCreateRequest{Purpose: file.PurposeUserData}, bytes.NewReader(data))
		check(t, err)
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, err := cm.client.DeleteFile(cleanup, uploaded.ID)
			if err != nil {
				t.Errorf("delete uploaded fixture: %v", err)
			}
		}()
		_, err = cm.client.WaitForFileProcessing(ctx, uploaded.ID, arkruntime.WaitForFileProcessingOptions{})
		check(t, err)
		ask(schema.MessagePartCommon{FileID: &uploaded.ID})
		t.Run("tool_result", func(t *testing.T) {
			if os.Getenv("ARK_RESPONSES_LIVE_MULTIMODAL_TOOLS") != "1" {
				t.Skip("set ARK_RESPONSES_LIVE_MULTIMODAL_TOOLS=1 for models that read multimodal tool results")
			}
			bound, err := cm.WithTools([]*schema.ToolInfo{toolInfo("inspect")})
			check(t, err)
			history := []*schema.Message{schema.UserMessage("Use inspect to read /tmp/fixture.pdf. The attached document contains one verification marker. Extract that exact marker from the actual document. Do not invent source code or any document content.")}
			call, err := bound.Generate(ctx, history, model.WithToolChoice(schema.ToolChoiceForced, "inspect"))
			check(t, err)
			if len(call.ToolCalls) != 1 {
				t.Fatalf("expected one tool call, got %d", len(call.ToolCalls))
			}
			history = append(history, call, &schema.Message{Role: schema.Tool, ToolCallID: call.ToolCalls[0].ID, UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "Read this attached PDF and report the verification marker printed on its page."},
				{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{FileID: &uploaded.ID}, Name: "fixture.pdf"}},
			}})
			answer, err := bound.Generate(ctx, history, model.WithToolChoice(schema.ToolChoiceForbidden))
			check(t, err)
			if !strings.Contains(answer.Content, "PDF_OK_731") {
				// Reproduce directly with SDK extra-body JSON, independent of the
				// adapter's message conversion, before attributing this to the adapter.
				native, nativeErr := cm.client.CreateResponses(ctx, &responses.ResponsesRequest{Model: config.Model, Input: responses.NewStringResponsesInput(""), Thinking: responses.NewOptThinking(responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeDisabled)})}, arkruntime.WithExtraBody(map[string]interface{}{
					"input": []interface{}{
						map[string]interface{}{"role": "user", "content": "Read the attached PDF from inspect and return its exact verification marker. Do not invent any content."},
						map[string]interface{}{"type": "function_call", "call_id": call.ToolCalls[0].ID, "name": "inspect", "arguments": call.ToolCalls[0].Function.Arguments},
						map[string]interface{}{"type": "function_call_output", "call_id": call.ToolCalls[0].ID, "output": []interface{}{
							map[string]interface{}{"type": "input_text", "text": "Read the attached PDF and return its exact verification marker."},
							map[string]interface{}{"type": "input_file", "file_id": uploaded.ID},
						}},
					},
				}))
				if nativeErr != nil {
					t.Fatalf("adapter failed to read tool PDF; native SDK also errored: %v", nativeErr)
				}
				var nativeText string
				for _, item := range native.Output {
					if message, ok := item.OneOf.GetItemOutputMessage(); ok {
						for _, part := range message.Content {
							if text, ok := part.OneOf.GetOutputContentItemText(); ok {
								nativeText += text.Text.Or("")
							}
						}
					}
				}
				t.Fatalf("tool PDF marker missing; adapter usage=%+v content=%q; native SDK usage=%+v content=%q", answer.ResponseMeta.Usage, answer.Content, native.Usage, nativeText)
			}
		})
	})
	t.Run("web_search", func(t *testing.T) {
		if os.Getenv("ARK_RESPONSES_LIVE_WEBSEARCH") != "1" {
			t.Skip("set ARK_RESPONSES_LIVE_WEBSEARCH=1 for accounts with web search enabled")
		}
		query := []*schema.Message{schema.UserMessage("Search the web for the CloudWeGo Eino official GitHub repository and reply with its URL.")}
		msg, err := cm.Generate(ctx, query, WithEnableToolWebSearch(&responses.WebSearchTool{Limit: responses.NewOptInt64(2)}), WithMaxToolCalls(1), WithRequestPayloadModifier(func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
			var fields map[string]interface{}
			if err := sonic.Unmarshal(raw, &fields); err != nil {
				return nil, err
			}
			fields["tool_choice"] = map[string]string{"type": "web_search"}
			return sonic.Marshal(fields)
		}))
		check(t, err)
		if !strings.Contains(strings.ToLower(msg.Content), "github.com/cloudwego/eino") {
			t.Fatalf("search result missing: %q", msg.Content)
		}
		found := false
		for _, part := range msg.AssistantGenMultiContent {
			if kind, _ := extraString(part.Extra, keyResponsesItemType); part.Type == ResponsesOutputItem && kind == "web_search_call" {
				found = true
			}
		}
		if !found {
			t.Fatal("missing built-in web search output")
		}
	})

}

func responsesTestPDF() []byte {
	stream := "BT /F1 24 Tf 40 100 Td (PDF_OK_731) Tj ET"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 400 200] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}
