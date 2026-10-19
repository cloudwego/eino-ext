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
		fileID = ""
		if _, err := convMsgInputFileToResponseContentItem(input); err == nil {
			t.Fatal("expected empty FileID to fail")
		}
	})
}

func TestInputFileIDPrecedence(t *testing.T) {
	for _, media := range []struct {
		name    string
		convert func(schema.MessagePartCommon) (any, error)
	}{
		{name: "image", convert: func(common schema.MessagePartCommon) (any, error) {
			return convMsgInputImageToResponseContentItem(&schema.MessageInputImage{MessagePartCommon: common})
		}},
		{name: "audio", convert: func(common schema.MessagePartCommon) (any, error) {
			return convMsgInputAudioToResponseContentItem(&schema.MessageInputAudio{MessagePartCommon: common})
		}},
		{name: "video", convert: func(common schema.MessagePartCommon) (any, error) {
			return convMsgInputVideoToResponseContentItem(&schema.MessageInputVideo{MessagePartCommon: common})
		}},
		{name: "file", convert: func(common schema.MessagePartCommon) (any, error) {
			return convMsgInputFileToResponseContentItem(&schema.MessageInputFile{MessagePartCommon: common})
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

func TestChatFileIDPrecedence(t *testing.T) {
	id := "file-123"
	url := "https://example.com/media"
	base64Data := "YWJj"
	invalidBase64 := "data:image/png;base64,YWJj"
	for _, common := range []schema.MessagePartCommon{
		{URL: &url},
		{Base64Data: &base64Data, MIMEType: "image/png"},
		{URL: &url, Base64Data: &base64Data},
		{Base64Data: &base64Data},
		{Base64Data: &invalidBase64, MIMEType: "image/png"},
	} {
		wantRaw, wantBase64, wantErr := validateMessagePartCommon(common, "image")
		common.FileID = &id
		gotRaw, gotBase64, err := validateMessagePartCommon(common, "image")
		if wantErr != nil {
			if err == nil || err.Error() != wantErr.Error() {
				t.Fatalf("FileID changed the selected source error: got %v, want %v", err, wantErr)
			}
			continue
		}
		if err != nil || gotRaw != wantRaw || gotBase64 != wantBase64 {
			t.Fatalf("FileID changed the selected source: got (%q, %v, %v), want (%q, %v, nil)", gotRaw, gotBase64, err, wantRaw, wantBase64)
		}
	}
}
