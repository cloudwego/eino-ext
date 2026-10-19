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

package agenticopenai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3/responses"
)

func TestInputFileID(t *testing.T) {
	t.Run("image", func(t *testing.T) {
		input := &schema.UserInputImage{FileID: "file-123"}
		value, err := userInputImageToInputItem(responses.EasyInputMessageRoleUser, input)
		if err != nil {
			t.Fatal(err)
		}
		assertFileIDJSON(t, value)
		input.URL = "https://example.com/media"
		if _, err := userInputImageToInputItem(responses.EasyInputMessageRoleUser, input); err == nil {
			t.Fatal("expected conflicting sources to fail")
		}
		input.URL = ""
		input.Base64Data = "YWJj"
		if _, err := userInputImageToInputItem(responses.EasyInputMessageRoleUser, input); err == nil {
			t.Fatal("expected conflicting sources to fail")
		}
	})
	t.Run("file", func(t *testing.T) {
		input := &schema.UserInputFile{FileID: "file-123"}
		value, err := userInputFileToInputItem(responses.EasyInputMessageRoleUser, input)
		if err != nil {
			t.Fatal(err)
		}
		assertFileIDJSON(t, value)
		input.URL = "https://example.com/media"
		if _, err := userInputFileToInputItem(responses.EasyInputMessageRoleUser, input); err == nil {
			t.Fatal("expected conflicting sources to fail")
		}
		input.URL = ""
		input.Base64Data = "YWJj"
		if _, err := userInputFileToInputItem(responses.EasyInputMessageRoleUser, input); err == nil {
			t.Fatal("expected conflicting sources to fail")
		}
	})
}

func assertFileIDJSON(t *testing.T, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"file_id":"file-123"`) {
		t.Fatalf("missing file reference in %s", data)
	}
	for _, field := range []string{"image_url", "file_url", "video_url", "audio_url", "file_data"} {
		if strings.Contains(string(data), `"`+field+`"`) {
			t.Fatalf("unexpected %s alongside FileID in %s", field, data)
		}
	}
}

func TestToolResultFileID(t *testing.T) {
	value, err := functionToolResultContentToOutput([]*schema.FunctionToolResultContentBlock{
		{Type: schema.FunctionToolResultContentBlockTypeImage, Image: &schema.UserInputImage{FileID: "file-123"}},
		{Type: schema.FunctionToolResultContentBlockTypeFile, File: &schema.UserInputFile{FileID: "file-123"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertFileIDJSON(t, value)
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), `"file_id":"file-123"`) != 2 {
		t.Fatalf("both tool media references must survive: %s", data)
	}
}
