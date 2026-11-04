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

package agenticark

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

func TestInputFileID(t *testing.T) {
	t.Run("image", func(t *testing.T) {
		input := &schema.UserInputImage{FileID: "file-123"}
		value, err := userInputImageToInputItem(responses.MessageRole_user, input)
		if err != nil {
			t.Fatal(err)
		}
		assertFileIDJSON(t, value)
	})
	t.Run("audio", func(t *testing.T) {
		input := &schema.UserInputAudio{FileID: "file-123"}
		value, err := userInputAudioToInputItem(responses.MessageRole_user, input)
		if err != nil {
			t.Fatal(err)
		}
		assertFileIDJSON(t, value)
	})
	t.Run("video", func(t *testing.T) {
		input := &schema.UserInputVideo{FileID: "file-123"}
		value, err := userInputVideoToInputItem(responses.MessageRole_user, input)
		if err != nil {
			t.Fatal(err)
		}
		assertFileIDJSON(t, value)
	})
	t.Run("file", func(t *testing.T) {
		input := &schema.UserInputFile{FileID: "file-123"}
		value, err := userInputFileToInputItem(responses.MessageRole_user, input)
		if err != nil {
			t.Fatal(err)
		}
		assertFileIDJSON(t, value)
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

func TestInputFileIDPrecedence(t *testing.T) {
	for _, media := range []struct {
		name    string
		convert func(url, base64Data, mimeType, fileID string) (any, error)
	}{
		{name: "image", convert: func(url, base64Data, mimeType, fileID string) (any, error) {
			return userInputImageToInputItem(responses.MessageRole_user, &schema.UserInputImage{URL: url, Base64Data: base64Data, MIMEType: mimeType, FileID: fileID})
		}},
		{name: "audio", convert: func(url, base64Data, mimeType, fileID string) (any, error) {
			return userInputAudioToInputItem(responses.MessageRole_user, &schema.UserInputAudio{URL: url, Base64Data: base64Data, MIMEType: mimeType, FileID: fileID})
		}},
		{name: "video", convert: func(url, base64Data, mimeType, fileID string) (any, error) {
			return userInputVideoToInputItem(responses.MessageRole_user, &schema.UserInputVideo{URL: url, Base64Data: base64Data, MIMEType: mimeType, FileID: fileID})
		}},
		{name: "file", convert: func(url, base64Data, mimeType, fileID string) (any, error) {
			return userInputFileToInputItem(responses.MessageRole_user, &schema.UserInputFile{URL: url, Base64Data: base64Data, MIMEType: mimeType, FileID: fileID})
		}},
	} {
		t.Run(media.name, func(t *testing.T) {
			checkFileIDPrecedence(t, media.convert)
		})
	}
}

func checkFileIDPrecedence(t *testing.T, convert func(url, base64Data, mimeType, fileID string) (any, error)) {
	t.Helper()
	for _, tc := range []struct {
		name, url, base64Data, mimeType string
	}{
		{name: "URL", url: "https://example.com/media"},
		{name: "Base64Data", base64Data: "YWJj", mimeType: "image/png"},
		{name: "URL_and_Base64Data", url: "https://example.com/media", base64Data: "YWJj", mimeType: "image/png"},
		{name: "invalid_Base64Data", base64Data: "data:image/png;base64,YWJj", mimeType: "image/png"},
		{name: "missing_MIMEType", base64Data: "YWJj"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want, wantErr := convert(tc.url, tc.base64Data, tc.mimeType, "")
			got, err := convert(tc.url, tc.base64Data, tc.mimeType, "file-123")
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
