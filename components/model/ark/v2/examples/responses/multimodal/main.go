/*
 * Copyright 2026 CloudWeGo Authors
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

package main

import (
	"context"
	"log"

	"encoding/base64"
	"mime"
	"os"
	"path/filepath"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
)

func main() {
	if err := exampleutil.RunResponses(run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ResponsesAPIConfig) error {

	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	prompt := os.Getenv("ARK_PROMPT")
	if prompt == "" {
		prompt = "Describe the supplied media briefly."
	}
	parts := []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: prompt}}
	if url := os.Getenv("ARK_IMAGE_URL"); url != "" {
		parts = append(parts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &url}}})
	}
	if url := os.Getenv("ARK_AUDIO_URL"); url != "" {
		parts = append(parts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: schema.MessagePartCommon{URL: &url}}})
	}
	if url := os.Getenv("ARK_VIDEO_URL"); url != "" {
		parts = append(parts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{URL: &url}}})
	}
	if len(parts) == 1 {
		// With no remote media configured, demonstrate a local base64 image.
		path := os.Getenv("ARK_IMAGE_FILE")
		if path == "" {
			path = "../examples/generate_with_image/eino.png"
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		parts = append(parts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{
			Base64Data: &encoded, MIMEType: mime.TypeByExtension(filepath.Ext(path)),
		}}})
	}
	reply, err := cm.Generate(ctx, []*schema.Message{{Role: schema.User, UserInputMultiContent: parts}})
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(reply)
	return nil
}
