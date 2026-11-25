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
	"encoding/json"
	"fmt"
	"log"
	"strings"

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
	config.Thinking = &responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeDisabled)}
	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	user := schema.UserMessage("Reply with exactly OLD_LABEL_731.")
	assistant, err := cm.Generate(ctx, []*schema.Message{user})
	if err != nil {
		return err
	}
	// Parts are the editable content. Preserve their item metadata while editing.
	var edited strings.Builder
	for i := range assistant.AssistantGenMultiContent {
		part := &assistant.AssistantGenMultiContent[i]
		if part.Type == schema.ChatMessagePartTypeText {
			part.Text = strings.ReplaceAll(part.Text, "OLD_LABEL_731", "NEW_LABEL_731")
			edited.WriteString(part.Text)
		}
	}
	fmt.Println("Edited assistant text:", edited.String())
	// Aggregate fields are compatibility views and do not override existing parts.
	assistant.Content = "IGNORED_LABEL"
	assistant.ReasoningContent = "ignored compatibility reasoning"
	// Native item positions and metadata survive ordinary JSON persistence.
	raw, err := json.Marshal(assistant)
	if err != nil {
		return err
	}
	var restored schema.Message
	if err := json.Unmarshal(raw, &restored); err != nil {
		return err
	}
	reader, err := cm.Stream(ctx, []*schema.Message{user, &restored, schema.UserMessage("Which label appeared in your previous assistant message? Reply with that label only.")})
	if err != nil {
		return err
	}
	answer, err := schema.ConcatMessageStream(reader)
	if err != nil {
		return err
	}
	fmt.Println("Answer:", answer.Content)
	fmt.Printf("Output parts: %d\n", len(answer.AssistantGenMultiContent))
	return nil
}
