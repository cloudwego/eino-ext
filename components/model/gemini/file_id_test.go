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

package gemini

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestInputFileID(t *testing.T) {
	uri := "https://generativelanguage.googleapis.com/v1beta/files/example"
	common := schema.MessagePartCommon{FileID: &uri, MIMEType: "application/octet-stream"}
	inputs := []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: common}},
		{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: common}},
		{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: common}},
		{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: common}},
	}
	parts, err := convInputMedia(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 4 {
		t.Fatalf("wrong part count: %d", len(parts))
	}
	for _, part := range parts {
		if part.FileData == nil || part.FileData.FileURI != uri || part.FileData.MIMEType != common.MIMEType || part.InlineData != nil {
			t.Fatalf("wrong reference: %+v", part)
		}
	}
	tool, err := convMultiModalToolMessageToPart("tool", inputs)
	if err != nil {
		t.Fatal(err)
	}
	if len(tool.FunctionResponse.Parts) != 4 {
		t.Fatal("missing tool media")
	}
	for _, part := range tool.FunctionResponse.Parts {
		if part.FileData == nil || part.FileData.FileURI != uri || part.InlineData != nil {
			t.Fatalf("wrong tool reference: %+v", part)
		}
	}
	for _, invalid := range []schema.MessagePartCommon{
		{FileID: new(string)},
	} {
		if err := validateMediaFileID(invalid); err == nil {
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
			return convInputMedia([]schema.MessageInputPart{schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: common}}})
		}},
		{name: "tool_image", convert: func(common schema.MessagePartCommon) (any, error) {
			return convMultiModalToolMessageToPart("tool", []schema.MessageInputPart{schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: common}}})
		}},
		{name: "user_audio", convert: func(common schema.MessagePartCommon) (any, error) {
			return convInputMedia([]schema.MessageInputPart{schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: common}}})
		}},
		{name: "tool_audio", convert: func(common schema.MessagePartCommon) (any, error) {
			return convMultiModalToolMessageToPart("tool", []schema.MessageInputPart{schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: common}}})
		}},
		{name: "user_video", convert: func(common schema.MessagePartCommon) (any, error) {
			return convInputMedia([]schema.MessageInputPart{schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: common}}})
		}},
		{name: "tool_video", convert: func(common schema.MessagePartCommon) (any, error) {
			return convMultiModalToolMessageToPart("tool", []schema.MessageInputPart{schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: common}}})
		}},
		{name: "user_file", convert: func(common schema.MessagePartCommon) (any, error) {
			return convInputMedia([]schema.MessageInputPart{schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: common}}})
		}},
		{name: "tool_file", convert: func(common schema.MessagePartCommon) (any, error) {
			return convMultiModalToolMessageToPart("tool", []schema.MessageInputPart{schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: common}}})
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
	empty := ""
	for _, tc := range []struct {
		name   string
		common schema.MessagePartCommon
	}{
		{name: "URL", common: schema.MessagePartCommon{URL: &url}},
		{name: "Base64Data", common: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "image/png"}},
		{name: "URL_and_Base64Data", common: schema.MessagePartCommon{URL: &url, Base64Data: &base64Data, MIMEType: "image/png"}},
		{name: "invalid_Base64Data", common: schema.MessagePartCommon{Base64Data: &invalidBase64, MIMEType: "image/png"}},
		{name: "missing_MIMEType", common: schema.MessagePartCommon{Base64Data: &base64Data}},
		{name: "empty_URL", common: schema.MessagePartCommon{URL: &empty}},
		{name: "empty_Base64Data", common: schema.MessagePartCommon{Base64Data: &empty, MIMEType: "image/png"}},
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
