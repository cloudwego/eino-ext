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
	for _, part := range []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{FileID: &id}}},
		{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{FileID: &id}}},
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
	url := "https://example.com"
	for _, common := range []schema.MessagePartCommon{
		{FileID: &id, URL: &url},
		{FileID: &id, Base64Data: &url},
		{FileID: new(string)},
	} {
		_, _, err := fileIDContentBlock(schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: common}})
		if err == nil {
			t.Fatal("expected invalid source to fail")
		}
	}
}
