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

package agenticgemini

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestInputFileID(t *testing.T) {
	t.Run("image", func(t *testing.T) {
		const uri = "https://generativelanguage.googleapis.com/v1beta/files/example"
		input := &schema.UserInputImage{FileID: uri, MIMEType: "application/octet-stream"}
		part, err := convUserInputImage(input)
		if err != nil {
			t.Fatal(err)
		}
		if part.FileData == nil || part.FileData.FileURI != uri || part.FileData.MIMEType != input.MIMEType || part.InlineData != nil {
			t.Fatalf("wrong file reference: %+v", part)
		}
		input.URL = "https://example.com"
		if _, err := convUserInputImage(input); err == nil {
			t.Fatal("expected source conflict")
		}
		input.URL = ""
		input.Base64Data = "YWJj"
		if _, err := convUserInputImage(input); err == nil {
			t.Fatal("expected source conflict")
		}
	})
	t.Run("audio", func(t *testing.T) {
		const uri = "https://generativelanguage.googleapis.com/v1beta/files/example"
		input := &schema.UserInputAudio{FileID: uri, MIMEType: "application/octet-stream"}
		part, err := convUserInputAudio(input)
		if err != nil {
			t.Fatal(err)
		}
		if part.FileData == nil || part.FileData.FileURI != uri || part.FileData.MIMEType != input.MIMEType || part.InlineData != nil {
			t.Fatalf("wrong file reference: %+v", part)
		}
		input.URL = "https://example.com"
		if _, err := convUserInputAudio(input); err == nil {
			t.Fatal("expected source conflict")
		}
		input.URL = ""
		input.Base64Data = "YWJj"
		if _, err := convUserInputAudio(input); err == nil {
			t.Fatal("expected source conflict")
		}
	})
	t.Run("video", func(t *testing.T) {
		const uri = "https://generativelanguage.googleapis.com/v1beta/files/example"
		input := &schema.UserInputVideo{FileID: uri, MIMEType: "application/octet-stream"}
		part, err := convUserInputVideo(input)
		if err != nil {
			t.Fatal(err)
		}
		if part.FileData == nil || part.FileData.FileURI != uri || part.FileData.MIMEType != input.MIMEType || part.InlineData != nil {
			t.Fatalf("wrong file reference: %+v", part)
		}
		input.URL = "https://example.com"
		if _, err := convUserInputVideo(input); err == nil {
			t.Fatal("expected source conflict")
		}
		input.URL = ""
		input.Base64Data = "YWJj"
		if _, err := convUserInputVideo(input); err == nil {
			t.Fatal("expected source conflict")
		}
	})
	t.Run("file", func(t *testing.T) {
		const uri = "https://generativelanguage.googleapis.com/v1beta/files/example"
		input := &schema.UserInputFile{FileID: uri, MIMEType: "application/octet-stream"}
		part, err := convUserInputFile(input)
		if err != nil {
			t.Fatal(err)
		}
		if part.FileData == nil || part.FileData.FileURI != uri || part.FileData.MIMEType != input.MIMEType || part.InlineData != nil {
			t.Fatalf("wrong file reference: %+v", part)
		}
		input.URL = "https://example.com"
		if _, err := convUserInputFile(input); err == nil {
			t.Fatal("expected source conflict")
		}
		input.URL = ""
		input.Base64Data = "YWJj"
		if _, err := convUserInputFile(input); err == nil {
			t.Fatal("expected source conflict")
		}
	})
}
