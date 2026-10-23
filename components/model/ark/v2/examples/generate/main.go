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
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cm, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{APIKey: os.Getenv("ARK_API_KEY"), Model: os.Getenv("ARK_MODEL")})
	if err != nil {
		log.Fatal(err)
	}
	messages := []*schema.Message{{Role: schema.User, Content: "Hello"}}
	// With ARK_FILE_ID, send a provider-hosted PDF instead. Files are uploaded
	// and made active separately through the provider's Files API.
	if fileID := os.Getenv("ARK_FILE_ID"); fileID != "" {
		messages = []*schema.Message{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "Summarize this document."},
			{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{FileID: &fileID}}},
		}}}
	}
	result, err := cm.Generate(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Content)
	reader, err := cm.Stream(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()
	for {
		chunk, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(chunk.Content)
	}
	fmt.Println()
}
