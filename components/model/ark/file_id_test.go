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

package ark

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestInputFileID(t *testing.T) {
	t.Run("image", func(t *testing.T) {
		fileID := "file-123"
		input := &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{FileID: &fileID}}
		value, err := convMsgInputImageToResponseContentItem(input)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"file_id":"file-123"`) {
			t.Fatalf("missing reference: %s", data)
		}
		for _, field := range []string{"image_url", "audio_url", "video_url", "file_url", "file_data"} {
			if strings.Contains(string(data), `"`+field+`"`) {
				t.Fatalf("unexpected source field: %s", data)
			}
		}
		url := "https://example.com/media"
		input.URL = &url
		if _, err := convMsgInputImageToResponseContentItem(input); err == nil {
			t.Fatal("expected source conflict")
		}
		input.URL = nil
		base64 := "YWJj"
		input.Base64Data = &base64
		if _, err := convMsgInputImageToResponseContentItem(input); err == nil {
			t.Fatal("expected source conflict")
		}
		input.Base64Data = nil
		fileID = ""
		if _, err := convMsgInputImageToResponseContentItem(input); err == nil {
			t.Fatal("expected empty FileID to fail")
		}
	})
	t.Run("audio", func(t *testing.T) {
		fileID := "file-123"
		input := &schema.MessageInputAudio{MessagePartCommon: schema.MessagePartCommon{FileID: &fileID}}
		value, err := convMsgInputAudioToResponseContentItem(input)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"file_id":"file-123"`) {
			t.Fatalf("missing reference: %s", data)
		}
		for _, field := range []string{"image_url", "audio_url", "video_url", "file_url", "file_data"} {
			if strings.Contains(string(data), `"`+field+`"`) {
				t.Fatalf("unexpected source field: %s", data)
			}
		}
		url := "https://example.com/media"
		input.URL = &url
		if _, err := convMsgInputAudioToResponseContentItem(input); err == nil {
			t.Fatal("expected source conflict")
		}
		input.URL = nil
		base64 := "YWJj"
		input.Base64Data = &base64
		if _, err := convMsgInputAudioToResponseContentItem(input); err == nil {
			t.Fatal("expected source conflict")
		}
		input.Base64Data = nil
		fileID = ""
		if _, err := convMsgInputAudioToResponseContentItem(input); err == nil {
			t.Fatal("expected empty FileID to fail")
		}
	})
	t.Run("video", func(t *testing.T) {
		fileID := "file-123"
		input := &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{FileID: &fileID}}
		value, err := convMsgInputVideoToResponseContentItem(input)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"file_id":"file-123"`) {
			t.Fatalf("missing reference: %s", data)
		}
		for _, field := range []string{"image_url", "audio_url", "video_url", "file_url", "file_data"} {
			if strings.Contains(string(data), `"`+field+`"`) {
				t.Fatalf("unexpected source field: %s", data)
			}
		}
		url := "https://example.com/media"
		input.URL = &url
		if _, err := convMsgInputVideoToResponseContentItem(input); err == nil {
			t.Fatal("expected source conflict")
		}
		input.URL = nil
		base64 := "YWJj"
		input.Base64Data = &base64
		if _, err := convMsgInputVideoToResponseContentItem(input); err == nil {
			t.Fatal("expected source conflict")
		}
		input.Base64Data = nil
		fileID = ""
		if _, err := convMsgInputVideoToResponseContentItem(input); err == nil {
			t.Fatal("expected empty FileID to fail")
		}
	})
	t.Run("file", func(t *testing.T) {
		fileID := "file-123"
		input := &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{FileID: &fileID}}
		value, err := convMsgInputFileToResponseContentItem(input)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"file_id":"file-123"`) {
			t.Fatalf("missing reference: %s", data)
		}
		for _, field := range []string{"image_url", "audio_url", "video_url", "file_url", "file_data"} {
			if strings.Contains(string(data), `"`+field+`"`) {
				t.Fatalf("unexpected source field: %s", data)
			}
		}
		url := "https://example.com/media"
		input.URL = &url
		if _, err := convMsgInputFileToResponseContentItem(input); err == nil {
			t.Fatal("expected source conflict")
		}
		input.URL = nil
		base64 := "YWJj"
		input.Base64Data = &base64
		if _, err := convMsgInputFileToResponseContentItem(input); err == nil {
			t.Fatal("expected source conflict")
		}
		input.Base64Data = nil
		fileID = ""
		if _, err := convMsgInputFileToResponseContentItem(input); err == nil {
			t.Fatal("expected empty FileID to fail")
		}
	})
}

func TestChatFileIDUnsupported(t *testing.T) {
	id := "file-123"
	if _, _, err := validateMessagePartCommon(schema.MessagePartCommon{FileID: &id}, "image"); err == nil {
		t.Fatal("the pinned Chat SDK cannot represent FileID")
	}
}
