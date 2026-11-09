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
	"fmt"
	"log"

	"os"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func main() {
	if err := exampleutil.RunResponses(run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ResponsesAPIConfig) error {

	// Doubao Search Custom is activated separately from the legacy web plugin.
	config.EnableToolWebSearch = &responses.WebSearchTool{Sources: []responses.WebSearchSource{"doubao"}}
	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	prompt := os.Getenv("ARK_PROMPT")
	if prompt == "" {
		prompt = "Search the web for the official CloudWeGo Eino GitHub repository and return its URL."
	}
	// Require the configured search tool instead of allowing an answer from
	// model knowledge alone. Bound the response to one tool round.
	reply, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)},
		model.WithToolChoice(schema.ToolChoiceForced), ark.WithMaxToolCalls(1))
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(reply)
	for _, part := range reply.AssistantGenMultiContent {
		if part.Type == ark.ResponsesOutputItem && fmt.Sprint(part.Extra["ark-responses-item-type"]) == "web_search_call" {
			fmt.Printf("Search item: %v status=%v\n", part.Extra["ark-responses-item-id"], part.Extra["ark-responses-item-status"])
		}
	}
	if usage, ok := ark.GetResponsesTokenUsageDetails(reply); ok {
		fmt.Println("Web search calls:", usage.ToolUsage.Or(responses.ToolUsage{}).WebSearch.Or(0))
	}
	return nil
}
