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
	"fmt"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"

	"github.com/cloudwego/eino-ext/components/agentic/gemini"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("GEMINI_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL")

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("genai.NewClient failed: %v", err)
	}

	cm, err := agenticgemini.NewAgenticModel(ctx, &agenticgemini.Config{
		Model:  modelName,
		Client: client,
	})
	if err != nil {
		log.Fatalf("gemini.NewAgenticModel failed: %v", err)
	}

	type toolCallInput struct {
		Answer string `json:"answer" jsonschema_description:"the answer of the question"`
	}
	answerTool, err := utils.InferTool("answer_to_user",
		"answer to user",
		func(ctx context.Context, in *toolCallInput) (string, error) {
			return fmt.Sprintf("answer: %v", in.Answer), nil
		})
	if err != nil {
		log.Fatalf("utils.InferTool failed: %v", err)
	}

	info, err := answerTool.Info(ctx)
	if err != nil {
		log.Fatalf("get tool info failed: %v", err)
	}

	// this file is from gemini cache usage example
	fileData, err := os.ReadFile("./examples/generate_with_prefix_cache/a11.test.txt")
	if err != nil {
		log.Fatalf("os.ReadFile failed: %v", err)
	}

	txtFileBase64 := base64.StdEncoding.EncodeToString(fileData)
	cacheInfo, err := cm.CreatePrefixCache(ctx, []*schema.AgenticMessage{
		schema.SystemAgenticMessage(`You are an expert at analyzing transcripts.
answer the question with the tool "answer_to_user"
always include the start_time and end_time of the transcript in the output`),
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.UserInputFile{
				Base64Data: txtFileBase64,
				MIMEType:   "text/plain",
			})},
		}}, model.WithTools([]*schema.ToolInfo{info}))
	if err != nil {
		log.Fatalf("CreatePrefixCache failed: %v", err)
	}

	data, _ := sonic.MarshalIndent(cacheInfo, "", "  ")
	fmt.Printf("prefix cache info:\n%v\n", string(data))

	msg, err := cm.Generate(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("give a very short summary about this transcript"),
	}, agenticgemini.WithCachedContentName(cacheInfo.Name),
		model.WithTools([]*schema.ToolInfo{info}),
		model.WithAgenticToolChoice(&schema.AgenticToolChoice{
			Type: schema.ToolChoiceForced,
		}))
	if err != nil {
		log.Fatalf("Generate failed: %v", err)
	}
	fmt.Printf("model output:\n%s\n", msg.String())
}
