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

	"errors"
	"os"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func main() {
	if err := exampleutil.RunResponses(run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ResponsesAPIConfig) error {

	id := os.Getenv("ARK_FILE_ID")
	if id == "" {
		return errors.New("set ARK_FILE_ID to a processed document file")
	}
	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	bound, err := cm.WithTools([]*schema.ToolInfo{{Name: "read_document", Desc: "Return the document requested by the user", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})}})
	if err != nil {
		return err
	}
	prompt := os.Getenv("ARK_PROMPT")
	if prompt == "" {
		prompt = "Read the exact verification marker printed on the document page."
	}
	messages := []*schema.Message{schema.UserMessage("Call read_document, then answer using the actual document: " + prompt)}
	assistant, err := bound.Generate(ctx, messages, model.WithToolChoice(schema.ToolChoiceForced, "read_document"))
	if err != nil {
		return err
	}
	if len(assistant.ToolCalls) != 1 {
		return errors.New("expected one document tool call")
	}
	messages = append(messages, assistant, &schema.Message{Role: schema.Tool, ToolCallID: assistant.ToolCalls[0].ID, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "Read this document to answer the user's question; do not invent content."},
		{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{FileID: &id}}},
	}})
	reply, err := bound.Generate(ctx, messages, model.WithToolChoice(schema.ToolChoiceForbidden))
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(reply)
	return nil
}
