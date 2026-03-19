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
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino-ext/components/model/claude"
)

// This example demonstrates client-executed (custom) tool search with Claude.
// Instead of relying on the server's built-in BM25/Regex search, you implement
// your own tool search logic (e.g., using embeddings, a vector database, or custom ranking).
//
// The flow is:
// 1. First turn: the model calls a custom "search_tools" function you define.
// 2. Your code resolves the search by looking up matching tools.
// 3. You return the results as a ToolSearchResult (containing tool_reference blocks).
// 4. Second turn: the model calls the discovered tools.
func main() {
	ctx := context.Background()

	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")
	if apiKey == "" {
		log.Fatal("CLAUDE_API_KEY environment variable is not set")
	}

	var baseURLPtr *string
	if len(baseURL) > 0 {
		baseURLPtr = &baseURL
	}

	cm, err := claude.NewChatModel(ctx, &claude.Config{
		APIKey:    apiKey,
		BaseURL:   baseURLPtr,
		Model:     modelName,
		MaxTokens: 4096,
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	m, err := toolsearch.New(ctx, &toolsearch.Config{
		DynamicTools: []tool.BaseTool{
			&toolGetShippingEta{},
		},
		UseModelToolSearch: true,
	})

	a, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "ShippingAgent",
		Description: "a Shipping Agent",
		Instruction: "You are a helpful shipping assistant.",
		Model:       cm,
		Handlers:    []adk.ChatModelAgentMiddleware{m},
	})
	if err != nil {
		log.Fatalf("NewChatModelAgent failed, err=%v", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           a,
		EnableStreaming: false,
	})

	iter := runner.Query(ctx, "I want to check the shipping ETA for order_42")
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}

		printEvent(ev)
	}
}

type toolGetShippingEta struct{}

func (t *toolGetShippingEta) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_shipping_eta",
		Desc: "Get the estimated shipping arrival time for an order",
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
			Type: "object",
			Properties: orderedmap.New[string, *jsonschema.Schema](
				orderedmap.WithInitialData(
					orderedmap.Pair[string, *jsonschema.Schema]{
						Key: "order_id",
						Value: &jsonschema.Schema{
							Type:        "string",
							Description: "the order id",
						},
					},
				),
			),
			Required: []string{"order_id"},
		}),
	}, nil
}

func (t *toolGetShippingEta) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	return "{\"time\":\"10:00\"}", nil
}

func printEvent(event *adk.AgentEvent) {
	fmt.Printf("name: %s\npath: %s", event.AgentName, event.RunPath)
	if event.Output != nil && event.Output.MessageOutput != nil {
		if m := event.Output.MessageOutput.Message; m != nil {
			if len(m.Content) > 0 {
				if m.Role == schema.Tool {
					if len(m.Content) > 0 {
						fmt.Printf("\ntool response: %s", m.Content)
					}
				} else {
					fmt.Printf("\nanswer: %s", m.Content)
				}
			} else if len(m.UserInputMultiContent) > 0 && m.Role == schema.Tool {
				for _, part := range m.UserInputMultiContent {
					switch part.Type {
					case schema.ChatMessagePartTypeText:
						fmt.Printf("\ntool response: %s", part.Text)
					case schema.ChatMessagePartTypeToolSearchResult:
						fmt.Printf("\ntool response: %s", part.ToolSearchResult.String())
					}
				}
			}
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					fmt.Printf("\ntool name: %s", tc.Function.Name)
					fmt.Printf("\narguments: %s", tc.Function.Arguments)
				}
			}
		} else if s := event.Output.MessageOutput.MessageStream; s != nil {
			toolMap := map[int][]*schema.Message{}
			var contentStart bool
			charNumOfOneRow := 0
			maxCharNumOfOneRow := 120
			for {
				chunk, err := s.Recv()
				if err != nil {
					if err == io.EOF {
						break
					}
					fmt.Printf("error: %v", err)
					return
				}
				if chunk.Content != "" {
					if !contentStart {
						contentStart = true
						if chunk.Role == schema.Tool {
							fmt.Printf("\ntool response: ")
						} else {
							fmt.Printf("\nanswer: ")
						}
					}

					charNumOfOneRow += len(chunk.Content)
					if strings.Contains(chunk.Content, "\n") {
						charNumOfOneRow = 0
					} else if charNumOfOneRow >= maxCharNumOfOneRow {
						fmt.Printf("\n")
						charNumOfOneRow = 0
					}
					fmt.Printf("%v", chunk.Content)
				}

				if len(chunk.ToolCalls) > 0 {
					for _, tc := range chunk.ToolCalls {
						index := tc.Index
						if index == nil {
							log.Fatalf("index is nil")
						}
						toolMap[*index] = append(toolMap[*index], &schema.Message{
							Role: chunk.Role,
							ToolCalls: []schema.ToolCall{
								{
									ID:    tc.ID,
									Type:  tc.Type,
									Index: tc.Index,
									Function: schema.FunctionCall{
										Name:      tc.Function.Name,
										Arguments: tc.Function.Arguments,
									},
								},
							},
						})
					}
				}
			}

			for _, msgs := range toolMap {
				m, err := schema.ConcatMessages(msgs)
				if err != nil {
					log.Fatalf("ConcatMessage failed: %v", err)
					return
				}
				fmt.Printf("\ntool name: %s", m.ToolCalls[0].Function.Name)
				fmt.Printf("\narguments: %s", m.ToolCalls[0].Function.Arguments)
			}
		}
	}
	if event.Action != nil {
		if event.Action.TransferToAgent != nil {
			fmt.Printf("\naction: transfer to %v", event.Action.TransferToAgent.DestAgentName)
		}
		if event.Action.Interrupted != nil {
			for _, ic := range event.Action.Interrupted.InterruptContexts {
				str, ok := ic.Info.(fmt.Stringer)
				if ok {
					fmt.Printf("\n%s", str.String())
				} else {
					fmt.Printf("\n%v", ic.Info)
				}
			}
		}
		if event.Action.Exit {
			fmt.Printf("\naction: exit")
		}
	}
	if event.Err != nil {
		fmt.Printf("\nerror: %v", event.Err)
	}
	fmt.Println()
	fmt.Println()
}
