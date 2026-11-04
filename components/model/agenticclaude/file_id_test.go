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

package agenticclaude

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestInputFileID(t *testing.T) {
	for _, media := range []string{"image", "file"} {
		t.Run(media, func(t *testing.T) {
			var value any
			var err error
			if media == "image" {
				value, err = imageToBlockParam(&schema.UserInputImage{FileID: "file-123"})
			} else {
				value, err = documentToBlockParam(&schema.UserInputFile{FileID: "file-123"})
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var block map[string]any
			if err := json.Unmarshal(data, &block); err != nil {
				t.Fatal(err)
			}
			source := block["source"].(map[string]any)
			if source["type"] != "file" || source["file_id"] != "file-123" || len(source) != 2 {
				t.Fatalf("wrong source: %s", data)
			}
			if media == "image" && block["type"] != "image" {
				t.Fatalf("wrong block: %s", data)
			}
			if media == "file" && block["type"] != "document" {
				t.Fatalf("wrong block: %s", data)
			}
		})
	}
}

func TestInputFileIDPrecedence(t *testing.T) {
	for _, media := range []struct {
		name    string
		convert func(url, base64Data, mimeType, fileID string) (any, error)
	}{
		{name: "image", convert: func(url, base64Data, mimeType, fileID string) (any, error) {
			return imageToBlockParam(&schema.UserInputImage{URL: url, Base64Data: base64Data, MIMEType: mimeType, FileID: fileID})
		}},
		{name: "file", convert: func(url, base64Data, mimeType, fileID string) (any, error) {
			return documentToBlockParam(&schema.UserInputFile{URL: url, Base64Data: base64Data, MIMEType: mimeType, FileID: fileID})
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
