/*
 * Copyright 2025 CloudWeGo Authors
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
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/litellm"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	chatModel, err := litellm.NewChatModel(ctx, &litellm.Config{
		BaseURL: os.Getenv("LITELLM_BASE_URL"),
		APIKey:  os.Getenv("LITELLM_API_KEY"),
		Model:   "openai/gpt-4o-mini",
	})
	if err != nil {
		log.Fatal(err)
	}

	stream, err := chatModel.Stream(ctx, []*schema.Message{
		{Role: schema.System, Content: "You are a helpful assistant."},
		{Role: schema.User, Content: "Tell me a short joke."},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	for {
		msg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		fmt.Print(msg.Content)
	}
	fmt.Println()
}
