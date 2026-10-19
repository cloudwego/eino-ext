/*
 * Copyright 2024 CloudWeGo Authors
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

package claude

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestInputFileID(t *testing.T) {
	id := "file-123"
	empty := ""
	for _, part := range []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{FileID: &id}}},
		{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{FileID: &id}}},
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &empty, Base64Data: &empty, FileID: &id}}},
		{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{URL: &empty, Base64Data: &empty, FileID: &id}}},
	} {
		for _, role := range []schema.RoleType{schema.User, schema.Tool} {
			msg, err := convSchemaMessage(&schema.Message{Role: role, ToolCallID: "call-1", UserInputMultiContent: []schema.MessageInputPart{part}})
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(msg)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			block := decoded["content"].([]any)[0].(map[string]any)
			if role == schema.Tool {
				block = block["content"].([]any)[0].(map[string]any)
			}
			source := block["source"].(map[string]any)
			if source["type"] != "file" || source["file_id"] != id || len(source) != 2 {
				t.Fatalf("wrong source: %s", data)
			}
		}
	}
	for _, common := range []schema.MessagePartCommon{
		{FileID: new(string)},
	} {
		_, _, err := fileIDContentBlock(schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: common}})
		if err == nil {
			t.Fatal("expected invalid source to fail")
		}
	}
}

func TestInputFileIDPrecedence(t *testing.T) {
	for _, media := range []struct {
		name    string
		convert func(schema.MessagePartCommon) (any, error)
	}{
		{name: "user_image", convert: func(common schema.MessagePartCommon) (any, error) {
			return convSchemaMessage(&schema.Message{Role: schema.User, ToolCallID: "call-1", UserInputMultiContent: []schema.MessageInputPart{schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: common}}}})
		}},
		{name: "tool_image", convert: func(common schema.MessagePartCommon) (any, error) {
			return convSchemaMessage(&schema.Message{Role: schema.Tool, ToolCallID: "call-1", UserInputMultiContent: []schema.MessageInputPart{schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: common}}}})
		}},
		{name: "user_file", convert: func(common schema.MessagePartCommon) (any, error) {
			return convSchemaMessage(&schema.Message{Role: schema.User, ToolCallID: "call-1", UserInputMultiContent: []schema.MessageInputPart{schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: common}}}})
		}},
		{name: "tool_file", convert: func(common schema.MessagePartCommon) (any, error) {
			return convSchemaMessage(&schema.Message{Role: schema.Tool, ToolCallID: "call-1", UserInputMultiContent: []schema.MessageInputPart{schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: common}}}})
		}},
	} {
		t.Run(media.name, func(t *testing.T) {
			checkFileIDPrecedence(t, media.convert)
		})
	}
}

func checkFileIDPrecedence(t *testing.T, convert func(schema.MessagePartCommon) (any, error)) {
	t.Helper()
	id := "file-123"
	url := "https://example.com/media"
	base64Data := "YWJj"
	invalidBase64 := "data:image/png;base64,YWJj"
	for _, tc := range []struct {
		name   string
		common schema.MessagePartCommon
	}{
		{name: "URL", common: schema.MessagePartCommon{URL: &url}},
		{name: "Base64Data", common: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "image/png"}},
		{name: "URL_and_Base64Data", common: schema.MessagePartCommon{URL: &url, Base64Data: &base64Data, MIMEType: "image/png"}},
		{name: "invalid_Base64Data", common: schema.MessagePartCommon{Base64Data: &invalidBase64, MIMEType: "image/png"}},
		{name: "missing_MIMEType", common: schema.MessagePartCommon{Base64Data: &base64Data}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want, wantErr := convert(tc.common)
			tc.common.FileID = &id
			got, err := convert(tc.common)
			if wantErr != nil {
				if err == nil || err.Error() != wantErr.Error() {
					t.Fatalf("FileID changed the selected source error: got %v, want %v", err, wantErr)
				}
				return
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
				t.Fatalf("FileID changed the selected source: got %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}
