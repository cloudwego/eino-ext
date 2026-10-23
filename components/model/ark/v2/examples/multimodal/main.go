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
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"time"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

var (
	kind     = flag.String("kind", "image", "image, audio, video, or file")
	source   = flag.String("source", "url", "url, base64 (read a local file), or file_id")
	value    = flag.String("value", "", "URL, local file path, or provider FileID")
	mimeType = flag.String("mime", "", "MIME type; inferred from the local file extension when possible")
	prompt   = flag.String("prompt", "Describe the content of this media.", "Question about the media")
	thinking = flag.String("thinking", "", "enabled or disabled; empty uses the model default")
)

func main() {
	flag.Parse()
	if err := exampleutil.Run(2*time.Minute, run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ChatModelConfig) error {
	if *value == "" {
		return fmt.Errorf("-value is required; see -help")
	}
	switch *thinking {
	case "":
	case "enabled", "disabled":
		config.Thinking = &chat.Thinking{Type: chat.NewOptThinkingMode(chat.ThinkingMode(*thinking))}
	default:
		return fmt.Errorf("unsupported -thinking %q", *thinking)
	}
	common := schema.MessagePartCommon{MIMEType: *mimeType}
	var filename string
	switch *source {
	case "url":
		common.URL = value
	case "file_id":
		common.FileID = value
	case "base64":
		data, err := os.ReadFile(*value)
		if err != nil {
			return err
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		common.Base64Data = &encoded
		filename = filepath.Base(*value)
		if common.MIMEType == "" {
			common.MIMEType = mime.TypeByExtension(filepath.Ext(*value))
		}
		if *kind != "file" && common.MIMEType == "" {
			return fmt.Errorf("set -mime for this local media file")
		}
	default:
		return fmt.Errorf("unsupported -source %q", *source)
	}
	var part schema.MessageInputPart
	switch *kind {
	case "image":
		part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{MessagePartCommon: common, Detail: schema.ImageURLDetailAuto}}
	case "audio":
		part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL,
			Audio: &schema.MessageInputAudio{MessagePartCommon: common}}
	case "video":
		video := &schema.MessageInputVideo{MessagePartCommon: common}
		ark.SetInputVideoFPS(video, 1)
		part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: video}
	case "file":
		part = schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL,
			File: &schema.MessageInputFile{MessagePartCommon: common, Name: filename}}
	default:
		return fmt.Errorf("unsupported -kind %q", *kind)
	}
	cm, err := ark.NewChatModel(ctx, config)
	if err != nil {
		return err
	}
	reply, err := cm.Generate(ctx, []*schema.Message{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: *prompt}, part,
	}}})
	if err != nil {
		return err
	}
	fmt.Println(reply.Content)
	return nil
}
