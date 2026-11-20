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
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

var fileID = flag.String("file-id", os.Getenv("ARK_FILE_ID"), "An active document FileID from the Ark Files API")

func main() {
	flag.Parse()
	if err := exampleutil.Run(2*time.Minute, run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ChatModelConfig) error {
	if *fileID == "" {
		return errors.New("set ARK_FILE_ID or -file-id to an active document FileID")
	}
	config.Thinking = &chat.Thinking{Type: chat.NewOptThinkingMode(chat.ThinkingModeDisabled)}
	cm, err := ark.NewChatModel(ctx, config)
	if err != nil {
		return err
	}
	bound, err := cm.WithTools([]*schema.ToolInfo{{
		Name: "get_document", Desc: "Return the document to summarize",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}})
	if err != nil {
		return err
	}
	messages := []*schema.Message{{Role: schema.User, Content: "Use get_document and summarize the returned document."}}
	assistant, err := bound.Generate(ctx, messages, model.WithToolChoice(schema.ToolChoiceForced, "get_document"))
	if err != nil {
		return err
	}
	if len(assistant.ToolCalls) == 0 {
		return errors.New("model returned no tool calls")
	}
	messages = append(messages, assistant)
	for _, call := range assistant.ToolCalls {
		if call.Function.Name != "get_document" {
			return fmt.Errorf("unexpected tool %q", call.Function.Name)
		}
		// This tool returns a previously uploaded document. Upload and wait for
		// processing through the Files API before running the example.
		messages = append(messages, &schema.Message{Role: schema.Tool, ToolCallID: call.ID,
			UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "Here is the requested document."},
				{Type: schema.ChatMessagePartTypeFileURL,
					File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{FileID: fileID}}},
			},
		})
	}
	answer, err := bound.Generate(ctx, messages, model.WithToolChoice(schema.ToolChoiceForbidden))
	if err != nil {
		return err
	}
	fmt.Println(answer.Content)
	return nil
}
