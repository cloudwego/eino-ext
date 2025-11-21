package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"

	"github.com/cloudwego/eino-ext/components/model/gemini"
)

func main() {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		log.Fatalf("genai.NewClient failed, err=%v", err)
	}

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Model:  os.Getenv("GEMINI_MODEL"),
		Client: client,
	})
	if err != nil {
		log.Fatalf("gemini.NewChatModel failed, err=%v", err)
	}

	type toolCallInput struct {
		LastCount int `json:"last_count" jsonschema_description:"the last count"`
	}
	countsTool, err := utils.InferTool("count_tool_call",
		"count the number of tool calls",
		func(ctx context.Context, in *toolCallInput) (string, error) {
			counts := in.LastCount + 1
			return fmt.Sprintf("tool call counts: %v", counts), nil
		})
	if err != nil {
		log.Fatalf("utils.InferTool failed, err=%v", err)
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "react_agent",
		Description: "react_agent",
		Instruction: `call count_tool_call 2 times, then say 'done'`,
		Model:       cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{
					countsTool,
				},
			},
		},
	})
	if err != nil {
		log.Fatalf("adk.NewChatModelAgent failed, err=%v", err)
	}

	iter := agent.Run(ctx, &adk.AgentInput{
		Messages: []adk.Message{
			{
				Role:    schema.User,
				Content: "start to count",
			},
		},
	})
	idx := 0
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			log.Fatalf("agent.Run failed, err=%v", event.Err)
		}

		msg, err_ := event.Output.MessageOutput.GetMessage()
		if err_ != nil {
			log.Fatalf("GetMessage failed, err=%v", err_)
		}

		idx++
		msgData, _ := sonic.MarshalIndent(msg, "", "  ")
		fmt.Printf("\nmessage %v:\n", idx)
		fmt.Printf("%s\n", string(msgData))
	}
}
