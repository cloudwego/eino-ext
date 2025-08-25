package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/a2a/extension/eino"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc"
)

func main() {
	ctx := context.Background()

	fmt.Println(">>>>>>>>>>>>>>>>>>>non-stream chat<<<<<<<<<<<<<<<<<<<<<")
	nonStreamChat(ctx)
	fmt.Println(">>>>>>>>>>>>>>>>>>>>>stream chat<<<<<<<<<<<<<<<<<<<<<<<")
	streamChat(ctx)
	fmt.Println(">>>>>>>>>>>>>>>>>>human-in-the-loop<<<<<<<<<<<<<<<<<<<<")
	humanInTheLoop(ctx)
	fmt.Println(">>>>>>>>>>>>>>>stream-human-in-the-loop<<<<<<<<<<<<<<<<")
	streamHumanInTheLoop(ctx)
}

func nonStreamChat(ctx context.Context) {
	streaming := false
	t, err := jsonrpc.NewTransport(ctx, &jsonrpc.ClientConfig{
		BaseURL:     "http://127.0.0.1:8888",
		HandlerPath: "/test",
	})
	a, err := eino.NewAgent(ctx, eino.AgentConfig{
		Transport:   t,
		Name:        nil,
		Description: nil,
		Streaming:   &streaming,
	})
	if err != nil {
		log.Fatal(err)
	}

	r := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: a,
	})
	iter := r.Run(ctx, []adk.Message{schema.UserMessage("recommend a fiction book to me")})
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		printEvent(event)
	}
}

func streamChat(ctx context.Context) {
	streaming := true
	t, err := jsonrpc.NewTransport(ctx, &jsonrpc.ClientConfig{
		BaseURL:     "http://127.0.0.1:8888",
		HandlerPath: "/test",
	})
	a, err := eino.NewAgent(ctx, eino.AgentConfig{
		Transport: t,
		Streaming: &streaming,
	})
	if err != nil {
		log.Fatal(err)
	}

	r := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           a,
		EnableStreaming: true,
	})
	iter := r.Run(ctx, []adk.Message{schema.UserMessage("recommend a fiction book to me")})
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		printEvent(event)
	}
}

func humanInTheLoop(ctx context.Context) {
	t, err := jsonrpc.NewTransport(ctx, &jsonrpc.ClientConfig{
		BaseURL:     "http://127.0.0.1:8888",
		HandlerPath: "/test",
	})
	a, err := eino.NewAgent(ctx, eino.AgentConfig{
		Transport: t,
	})
	if err != nil {
		log.Fatal(err)
	}

	runHumanInTheLoop(ctx, a)
}

func streamHumanInTheLoop(ctx context.Context) {
	streaming := true
	t, err := jsonrpc.NewTransport(ctx, &jsonrpc.ClientConfig{
		BaseURL:     "http://127.0.0.1:8888",
		HandlerPath: "/test",
	})
	a, err := eino.NewAgent(ctx, eino.AgentConfig{
		Transport: t,
		Streaming: &streaming,
	})
	if err != nil {
		log.Fatal(err)
	}

	runHumanInTheLoop(ctx, a)
}

func runHumanInTheLoop(ctx context.Context, a adk.Agent) {
	var err error
	r := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           a,
		CheckPointStore: &inMemoryStore{},
	})
	checkPointID := "1"
	iter := r.Run(ctx, []adk.Message{schema.UserMessage("recommend a book to me")}, adk.WithCheckPointID(checkPointID))
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		printEvent(event)
	}

	newInput := "recommend a fiction book to me"
	fmt.Printf("provide new input: %s\n\n", newInput)

	iter, err = r.Resume(ctx, checkPointID, eino.WithResumeMessages([]adk.Message{schema.UserMessage(newInput)}))
	if err != nil {
		log.Fatal(err)
	}
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		printEvent(event)
	}
}

type inMemoryStore struct {
	m sync.Map
}

func (i *inMemoryStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	v, ok := i.m.Load(checkPointID)
	if !ok {
		return nil, false, nil
	}
	return v.([]byte), true, nil
}

func (i *inMemoryStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	i.m.Store(checkPointID, checkPoint)
	return nil
}

func printEvent(event *adk.AgentEvent) {
	fmt.Printf("name: %s\npath: %s", event.AgentName, event.RunPath)
	if event.Output != nil && event.Output.MessageOutput != nil {
		if m := event.Output.MessageOutput.Message; m != nil {
			if len(m.Content) > 0 {
				if m.Role == schema.Tool {
					fmt.Printf("\ntool response: %s", m.Content)
				} else {
					fmt.Printf("\nanswer: %s", m.Content)
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
					fmt.Printf(chunk.Content)
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
			ii, _ := json.MarshalIndent(event.Action.Interrupted.Data, "  ", "  ")
			fmt.Printf("\naction: interrupted")
			fmt.Printf("\ninterrupt snapshot: %v", string(ii))
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
