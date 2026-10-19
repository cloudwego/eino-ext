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
		{FileID: &uri, URL: &uri},
		{FileID: &uri, Base64Data: &uri},
		{FileID: new(string)},
	} {
		if err := validateMediaFileID(invalid); err == nil {
			t.Fatal("expected invalid source to fail")
		}
	}
}
