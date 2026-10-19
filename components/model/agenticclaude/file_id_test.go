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
				if _, conflict := imageToBlockParam(&schema.UserInputImage{FileID: "file-123", URL: "https://example.com"}); conflict == nil {
					t.Fatal("expected source conflict")
				}
				if _, conflict := imageToBlockParam(&schema.UserInputImage{FileID: "file-123", Base64Data: "YWJj"}); conflict == nil {
					t.Fatal("expected source conflict")
				}
			} else {
				value, err = documentToBlockParam(&schema.UserInputFile{FileID: "file-123"})
				if _, conflict := documentToBlockParam(&schema.UserInputFile{FileID: "file-123", URL: "https://example.com"}); conflict == nil {
					t.Fatal("expected source conflict")
				}
				if _, conflict := documentToBlockParam(&schema.UserInputFile{FileID: "file-123", Base64Data: "YWJj"}); conflict == nil {
					t.Fatal("expected source conflict")
				}
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
