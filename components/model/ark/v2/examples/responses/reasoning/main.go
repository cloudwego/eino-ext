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
	"fmt"
	"io"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func main() {
	if err := exampleutil.RunResponses(run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ResponsesAPIConfig) error {

	config.Thinking = &responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeEnabled)}
	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	question := schema.UserMessage("What is 17 times 19? Answer briefly.")
	reader, err := cm.Stream(ctx, []*schema.Message{question})
	if err != nil {
		return err
	}
	defer reader.Close()
	var chunks []*schema.Message
	for {
		chunk, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		fmt.Print(chunk.ReasoningContent, chunk.Content)
		chunks = append(chunks, chunk)
	}
	assistant, err := schema.ConcatMessages(chunks)
	if err != nil {
		return err
	}
	if err := exampleutil.PrintResponsesConcat(chunks, assistant); err != nil {
		return err
	}
	fmt.Println("\nReasoning characters:", len([]rune(assistant.ReasoningContent)))
	// Reuse the assistant message to replay its complete native output, including reasoning.
	reply, err := cm.Generate(ctx, []*schema.Message{question, assistant, schema.UserMessage("Add one to that answer. Reply with only the number.")},
		ark.WithResponsesThinking(&responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeDisabled)}))
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(reply)
	return nil
}
